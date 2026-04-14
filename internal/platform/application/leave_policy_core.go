package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type LeavePolicyCoreService struct {
	documents  *document.Service
	models     *model.Service
	workforce  *EmployeeWorkforceCoreService
	attendance *WorkforceAttendanceCoreService
	approvals  *ApprovalPolicyService
}

func NewLeavePolicyCoreService(models *model.Service, workforce *EmployeeWorkforceCoreService, attendance *WorkforceAttendanceCoreService, approvals *ApprovalPolicyService) *LeavePolicyCoreService {
	return &LeavePolicyCoreService{models: models, workforce: workforce, attendance: attendance, approvals: approvals}
}

func (s *LeavePolicyCoreService) SetDocuments(documents *document.Service) {
	if s == nil {
		return
	}
	s.documents = documents
}

const leaveRequestWorkflowKey = "leave_request_flow"

type leaveApprovalState struct {
	PolicyID               string
	StageKey               string
	StageSequence          int
	StageTotal             int
	RoutingMode            string
	RequiredApproverCount  int
	AssigneeUserID         string
	CandidateUserIDs       []string
	RecordedUserIDs        []string
	RequiresDifferentActor bool
}

type leaveCountResolution struct {
	RequestUnit      string
	RequestedDays    float64
	RequestedHours   float64
	HalfDaySession   string
	Basis            string
	CountedDates     []string
	WorkCalendarID   string
	RosterSlotIDs    []string
	CountExplanation string
}

func (s *LeavePolicyCoreService) ExecuteAccrualRun(runID, actorID string) (model.Record, error) {
	if s == nil || s.models == nil || strings.TrimSpace(runID) == "" {
		return model.Record{}, nil
	}
	run, err := s.models.Get("leave_accrual_run", strings.TrimSpace(runID))
	if err != nil {
		return model.Record{}, err
	}
	if !strings.EqualFold(textValue(run.Values["status"]), "active") {
		return model.Record{}, shared.Validation("leave accrual run is not active")
	}
	if strings.EqualFold(textValue(run.Values["run_status"]), "completed") {
		return run, nil
	}
	runMode := strings.TrimSpace(textValue(run.Values["run_mode"]))
	if runMode == "" {
		runMode = "annual_grant"
	}
	effectiveDate := parseDateOnly(run.Values["effective_date"])
	if effectiveDate.IsZero() {
		return model.Record{}, shared.Validation("leave accrual run effective_date is required")
	}
	policyID := strings.TrimSpace(textValue(run.Values["leave_policy_id"]))

	profiles, _, err := s.models.List("employee_leave_profile", model.Query{
		Filters:  map[string]string{"status": "active"},
		SortKey:  "employee_id",
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, err
	}
	for _, profile := range profiles {
		if !leaveProfileEffectiveAt(profile, effectiveDate) {
			continue
		}
		if policyID != "" && strings.TrimSpace(textValue(profile.Values["leave_policy_id"])) != policyID {
			continue
		}
		policy, rule, ok := s.resolveLeavePolicyForProfile(profile)
		if !ok {
			continue
		}
		account, err := s.ensureBalanceAccount(profile, policy, actorID)
		if err != nil {
			return model.Record{}, err
		}
		account, err = s.applyPendingCarryForwardExpiry(account, profile, policy, actorID, effectiveDate)
		if err != nil {
			return model.Record{}, err
		}
		switch strings.ToLower(runMode) {
		case "monthly_accrual":
			if !strings.EqualFold(textValue(rule.Values["grant_mode"]), "monthly_accrual") {
				continue
			}
			days := leaveAccrualGrantDays(rule, profile, effectiveDate)
			if days <= 0 {
				continue
			}
			if _, err := s.createBalanceEntry(account.ID, profile, policy, actorID, map[string]any{
				"entry_type":      "monthly_accrual",
				"days":            days,
				"effective_date":  effectiveDate.Format("2006-01-02"),
				"accrual_run_id":  run.ID,
				"employee_id":     textValue(profile.Values["employee_id"]),
				"leave_policy_id": policy.ID,
				"status":          "active",
			}); err != nil {
				return model.Record{}, err
			}
		default:
			if !strings.EqualFold(textValue(rule.Values["grant_mode"]), "annual_grant") {
				continue
			}
			account, err = s.applyAnnualCarryForward(account, profile, policy, rule, actorID, effectiveDate, run.ID)
			if err != nil {
				return model.Record{}, err
			}
			days := numberValue(rule.Values["annual_entitlement_days"])
			if days <= 0 {
				continue
			}
			if _, err := s.createBalanceEntry(account.ID, profile, policy, actorID, map[string]any{
				"entry_type":      "annual_grant",
				"days":            days,
				"effective_date":  effectiveDate.Format("2006-01-02"),
				"accrual_run_id":  run.ID,
				"employee_id":     textValue(profile.Values["employee_id"]),
				"leave_policy_id": policy.ID,
				"status":          "active",
			}); err != nil {
				return model.Record{}, err
			}
		}
	}

	values := cloneMap(run.Values)
	values["run_status"] = "completed"
	values["processed_at"] = time.Now().UTC().Format(time.RFC3339)
	values["processed_by"] = strings.TrimSpace(actorID)
	return s.models.Update("leave_accrual_run", run.ID, actorID, values, run.Version)
}

func (s *LeavePolicyCoreService) ListBalancesForUser(userID string) ([]model.Record, error) {
	employee, _, err := s.resolveEmployeeForUser(userID)
	if err != nil {
		return nil, err
	}
	items, _, err := s.models.List("leave_balance_account", model.Query{
		Filters:  map[string]string{"employee_id": employee.ID, "status": "active"},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *LeavePolicyCoreService) BalanceEntriesForUser(userID, accountID string) ([]map[string]any, error) {
	employee, _, err := s.resolveEmployeeForUser(userID)
	if err != nil {
		return nil, err
	}
	account, err := s.models.Get("leave_balance_account", strings.TrimSpace(accountID))
	if err != nil {
		return nil, err
	}
	if textValue(account.Values["employee_id"]) != employee.ID {
		return nil, shared.Forbidden("leave balance account is not allowed")
	}
	return s.balanceEntrySummaries(account.ID)
}

func (s *LeavePolicyCoreService) ListRequestsForUser(userID string, filters map[string]string) ([]model.Record, error) {
	employee, _, err := s.resolveEmployeeForUser(userID)
	if err != nil {
		return nil, err
	}
	queryFilters := map[string]string{"employee_id": employee.ID}
	for key, value := range filters {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		queryFilters[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	items, _, err := s.models.List("leave_request", model.Query{
		Filters:  queryFilters,
		SortKey:  "start_date",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *LeavePolicyCoreService) GetRequestForUser(userID, requestID string) (model.Record, error) {
	employee, _, err := s.resolveEmployeeForUser(userID)
	if err != nil {
		return model.Record{}, err
	}
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return model.Record{}, err
	}
	if textValue(record.Values["employee_id"]) != employee.ID {
		return model.Record{}, shared.Forbidden("leave request is not allowed")
	}
	return record, nil
}

func (s *LeavePolicyCoreService) CreateSelfServiceLeaveRequest(userID string, payload map[string]any, actorID string) (model.Record, error) {
	employee, assignment, err := s.resolveEmployeeForUser(userID)
	if err != nil {
		return model.Record{}, err
	}
	values, _, _, _, _, err := s.prepareLeaveRequestValues(employee, assignment, model.Record{}, payload, actorID)
	if err != nil {
		return model.Record{}, err
	}
	values["approval_status"] = "draft"
	values["request_source"] = "self_service"
	values["self_service_actor_user_id"] = strings.TrimSpace(userID)
	values["status"] = "active"
	return s.models.Create("leave_request", actorID, values)
}

func (s *LeavePolicyCoreService) UpdateSelfServiceLeaveRequest(userID, requestID string, payload map[string]any, actorID string) (model.Record, error) {
	employee, assignment, err := s.resolveEmployeeForUser(userID)
	if err != nil {
		return model.Record{}, err
	}
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return model.Record{}, err
	}
	if textValue(record.Values["employee_id"]) != employee.ID {
		return model.Record{}, shared.Forbidden("leave request is not allowed")
	}
	if !strings.EqualFold(textValue(record.Values["approval_status"]), "draft") {
		return model.Record{}, shared.Validation("only draft leave requests can be updated")
	}
	values, _, _, _, _, err := s.prepareLeaveRequestValues(employee, assignment, record, payload, actorID)
	if err != nil {
		return model.Record{}, err
	}
	values["approval_status"] = "draft"
	values["request_source"] = "self_service"
	values["self_service_actor_user_id"] = strings.TrimSpace(userID)
	values["status"] = textValue(record.Values["status"])
	if values["status"] == "" {
		values["status"] = "active"
	}
	return s.models.Update("leave_request", record.ID, actorID, values, record.Version)
}

func (s *LeavePolicyCoreService) SubmitSelfServiceLeaveRequest(userID, requestID, actorID string) (model.Record, error) {
	record, err := s.GetRequestForUser(userID, requestID)
	if err != nil {
		return model.Record{}, err
	}
	if !strings.EqualFold(textValue(record.Values["approval_status"]), "draft") {
		return model.Record{}, shared.Validation("leave request is not in draft state")
	}
	return s.submitLeaveRequest(record, actorID)
}

func (s *LeavePolicyCoreService) AmendSelfServiceLeaveRequest(userID, requestID string, payload map[string]any, actorID string) (model.Record, error) {
	record, err := s.GetRequestForUser(userID, requestID)
	if err != nil {
		return model.Record{}, err
	}
	switch strings.ToLower(textValue(record.Values["approval_status"])) {
	case "draft", "submitted":
	default:
		return model.Record{}, shared.Validation("leave request cannot be amended")
	}
	return s.amendLeaveRequest(record, payload, actorID)
}

func (s *LeavePolicyCoreService) CancelSelfServiceLeaveRequest(userID, requestID, actorID string) (model.Record, error) {
	record, err := s.GetRequestForUser(userID, requestID)
	if err != nil {
		return model.Record{}, err
	}
	state := strings.ToLower(textValue(record.Values["approval_status"]))
	if state != "draft" && state != "submitted" {
		return model.Record{}, shared.Validation("leave request cannot be cancelled")
	}
	if err := s.releaseReservedBalance(record, actorID); err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	values["approval_status"] = "cancelled"
	values["reservation_entry_ids_json"] = "[]"
	clearLeaveApprovalState(values)
	updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	s.syncLeaveRange(updated)
	return updated, nil
}

func (s *LeavePolicyCoreService) CancelApprovedLeaveRequest(requestID, actorID, note string) (model.Record, error) {
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return model.Record{}, err
	}
	if !strings.EqualFold(textValue(record.Values["status"]), "active") {
		return model.Record{}, shared.NotFound("leave request was not found")
	}
	if !strings.EqualFold(textValue(record.Values["approval_status"]), "approved") {
		return model.Record{}, shared.Validation("only approved leave requests can be cancelled")
	}
	if blocked, reason, err := s.payrollCutoffForRequest(record, nil); err != nil {
		return model.Record{}, err
	} else if blocked {
		return model.Record{}, shared.Conflict(reason)
	}
	if err := s.reverseConsumedBalance(record, actorID); err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	values["approval_status"] = "cancelled"
	values["approved_days"] = 0.0
	values["consumption_entry_ids_json"] = "[]"
	clearLeaveApprovalState(values)
	if strings.TrimSpace(note) != "" {
		values["notes"] = strings.TrimSpace(strings.TrimSpace(textValue(values["notes"])) + "\n" + strings.TrimSpace(note))
	}
	updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	s.syncLeaveRange(updated)
	return updated, nil
}

func (s *LeavePolicyCoreService) AmendManagedLeaveRequest(requestID string, payload map[string]any, actorID string) (model.Record, error) {
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return model.Record{}, err
	}
	if !strings.EqualFold(textValue(record.Values["status"]), "active") {
		return model.Record{}, shared.NotFound("leave request was not found")
	}
	switch strings.ToLower(textValue(record.Values["approval_status"])) {
	case "submitted", "approved":
	default:
		return model.Record{}, shared.Validation("leave request cannot be amended")
	}
	return s.amendLeaveRequest(record, payload, actorID)
}

func (s *LeavePolicyCoreService) ApproveLeaveRequest(requestID, actorID string) (model.Record, error) {
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return model.Record{}, err
	}
	if !strings.EqualFold(textValue(record.Values["approval_status"]), "submitted") {
		return model.Record{}, shared.Validation("leave request is not submitted")
	}
	state := parseLeaveApprovalState(record.Values)
	if err := s.authorizeLeaveApprovalAction(record, state, actorID); err != nil {
		return model.Record{}, err
	}
	if state.StageKey == "" || state.RequiredApproverCount <= 1 && state.StageTotal <= 1 {
		return s.finalizeApprovedLeave(record, actorID)
	}
	recorded := append(state.RecordedUserIDs, actorID)
	state.RecordedUserIDs = uniqueStrings(recorded)
	if len(state.RecordedUserIDs) < state.RequiredApproverCount {
		values := cloneMap(record.Values)
		values["approval_recorded_user_ids_json"] = marshalStringList(state.RecordedUserIDs)
		updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
		if err != nil {
			return model.Record{}, err
		}
		return updated, nil
	}
	nextState, hasNext, err := s.nextLeaveApprovalState(record, state)
	if err != nil {
		return model.Record{}, err
	}
	if !hasNext {
		return s.finalizeApprovedLeave(record, actorID)
	}
	values := cloneMap(record.Values)
	applyLeaveApprovalState(values, nextState)
	updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	return updated, nil
}

func (s *LeavePolicyCoreService) RejectLeaveRequest(requestID, actorID, note string) (model.Record, error) {
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return model.Record{}, err
	}
	if !strings.EqualFold(textValue(record.Values["approval_status"]), "submitted") {
		return model.Record{}, shared.Validation("leave request is not submitted")
	}
	state := parseLeaveApprovalState(record.Values)
	if err := s.authorizeLeaveApprovalAction(record, state, actorID); err != nil {
		return model.Record{}, err
	}
	if err := s.releaseReservedBalance(record, actorID); err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	values["approval_status"] = "rejected"
	values["reservation_entry_ids_json"] = "[]"
	clearLeaveApprovalState(values)
	if strings.TrimSpace(note) != "" {
		values["notes"] = strings.TrimSpace(strings.TrimSpace(textValue(values["notes"])) + "\n" + strings.TrimSpace(note))
	}
	updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	s.syncLeaveRange(updated)
	return updated, nil
}

func (s *LeavePolicyCoreService) LeaveDeductsFromPayroll(leaveID string) bool {
	if s == nil || s.models == nil || strings.TrimSpace(leaveID) == "" {
		return false
	}
	leave, err := s.models.Get("leave_request", strings.TrimSpace(leaveID))
	if err != nil || !strings.EqualFold(textValue(leave.Values["status"]), "active") || !strings.EqualFold(textValue(leave.Values["approval_status"]), "approved") {
		return false
	}
	if policyID := strings.TrimSpace(textValue(leave.Values["leave_policy_id"])); policyID != "" {
		policy, err := s.models.Get("leave_policy", policyID)
		if err == nil && strings.EqualFold(textValue(policy.Values["status"]), "active") {
			return !boolValue(policy.Values["paid_leave"])
		}
	}
	absenceCodeID := strings.TrimSpace(textValue(leave.Values["absence_code_id"]))
	if absenceCodeID == "" {
		return false
	}
	absenceCode, err := s.models.Get("absence_code", absenceCodeID)
	if err != nil {
		return false
	}
	return boolValue(absenceCode.Values["deduct_from_payroll"])
}

func (s *LeavePolicyCoreService) submitLeaveRequest(record model.Record, actorID string) (model.Record, error) {
	policyID := strings.TrimSpace(textValue(record.Values["leave_policy_id"]))
	if policyID == "" {
		return model.Record{}, shared.Validation("leave policy is required")
	}
	policy, err := s.models.Get("leave_policy", policyID)
	if err != nil {
		return model.Record{}, err
	}
	if err := validateLeaveRequestPolicy(policy, record); err != nil {
		return model.Record{}, err
	}
	reservationIDs := []string{}
	if boolValue(policy.Values["requires_balance"]) {
		account, err := s.accountForRequest(record, actorID)
		if err != nil {
			return model.Record{}, err
		}
		profile, err := s.models.Get("employee_leave_profile", textValue(record.Values["employee_leave_profile_id"]))
		if err != nil {
			return model.Record{}, err
		}
		account, err = s.applyPendingCarryForwardExpiry(account, profile, policy, actorID, parseDateOnly(record.Values["start_date"]))
		if err != nil {
			return model.Record{}, err
		}
		requestedDays := numberValue(record.Values["requested_days"])
		if requestedDays <= 0 {
			return model.Record{}, shared.Validation("requested_days must be greater than zero")
		}
		if numberValue(account.Values["available_days"]) < requestedDays {
			return model.Record{}, shared.Validation("leave balance is insufficient")
		}
		entry, err := s.createBalanceEntry(account.ID, model.Record{}, policy, actorID, map[string]any{
			"entry_type":       "reservation",
			"days":             requestedDays,
			"effective_date":   textValue(record.Values["start_date"]),
			"leave_request_id": record.ID,
			"employee_id":      textValue(record.Values["employee_id"]),
			"leave_policy_id":  policy.ID,
			"status":           "active",
		})
		if err != nil {
			return model.Record{}, err
		}
		reservationIDs = append(reservationIDs, entry.ID)
	}
	values := cloneMap(record.Values)
	values["approval_status"] = "submitted"
	values["reservation_entry_ids_json"] = marshalStringList(reservationIDs)
	resolution, ok, err := s.resolveLeaveApprovalPolicy(record)
	if err != nil {
		return model.Record{}, err
	}
	if ok && len(resolution.Stages) > 0 {
		stage := resolution.Stages[0]
		applyLeaveApprovalState(values, leaveApprovalState{
			PolicyID:               resolution.Policy.ID,
			StageKey:               stage.StageKey,
			StageSequence:          stage.Sequence,
			StageTotal:             len(resolution.Stages),
			RoutingMode:            firstNonEmpty(stage.RoutingMode, stage.AssignmentStrategy),
			RequiredApproverCount:  max(1, stage.RequiredApproverCount),
			AssigneeUserID:         firstNonEmpty(stage.AssigneeUserID, stage.ExplicitUserID),
			CandidateUserIDs:       append([]string(nil), stage.CandidateUserIDs...),
			RecordedUserIDs:        nil,
			RequiresDifferentActor: stage.RequiresDifferentActor,
		})
	} else {
		clearLeaveApprovalState(values)
	}
	return s.models.Update("leave_request", record.ID, actorID, values, record.Version)
}

func (s *LeavePolicyCoreService) prepareLeaveRequestValues(employee, assignment, current model.Record, payload map[string]any, actorID string) (map[string]any, model.Record, model.Record, model.Record, model.Record, error) {
	values := cloneMap(current.Values)
	if values == nil {
		values = map[string]any{}
	}
	for key, value := range cloneMap(payload) {
		switch key {
		case "employee_id", "organization_id", "location_id", "organization_unit_id", "department_id", "cost_center_id", "request_source", "self_service_actor_user_id", "approval_status", "reservation_entry_ids_json", "consumption_entry_ids_json", "approved_days", "amendment_count", "last_amended_at", "last_amended_by", "last_amendment_reason", "reason":
			continue
		default:
			values[key] = value
		}
	}
	policy, profile, absenceCode, account, err := s.resolveApplicableLeaveContext(employee, assignment, values, actorID)
	if err != nil {
		var platformErr shared.Error
		if !errors.As(err, &platformErr) || platformErr.Kind != shared.KindValidation {
			return nil, model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
		}
		policy, profile, absenceCode, account, err = s.resolveExistingLeaveContext(current, assignment, values)
		if err != nil {
			return nil, model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
		}
	}
	startDate := strings.TrimSpace(textValue(values["start_date"]))
	endDate := strings.TrimSpace(textValue(values["end_date"]))
	if startDate == "" || endDate == "" {
		return nil, model.Record{}, model.Record{}, model.Record{}, model.Record{}, shared.Validation("start_date and end_date are required")
	}
	values["employee_id"] = employee.ID
	values["organization_id"] = leaveFirstNonEmpty(
		textValue(values["organization_id"]),
		textValue(profile.Values["organization_id"]),
		textValue(assignment.Values["organization_id"]),
	)
	values["location_id"] = leaveFirstNonEmpty(
		textValue(values["location_id"]),
		textValue(profile.Values["location_id"]),
		textValue(assignment.Values["location_id"]),
	)
	values["organization_unit_id"] = leaveFirstNonEmpty(textValue(values["organization_unit_id"]), textValue(assignment.Values["organization_unit_id"]))
	values["department_id"] = leaveFirstNonEmpty(textValue(values["department_id"]), textValue(assignment.Values["department_id"]))
	values["cost_center_id"] = leaveFirstNonEmpty(textValue(values["cost_center_id"]), textValue(assignment.Values["cost_center_id"]))
	count, err := s.resolveRequestedLeaveCount(policy, values)
	if err != nil {
		return nil, model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
	}
	values["absence_code_id"] = absenceCode.ID
	values["leave_policy_id"] = policy.ID
	values["employee_leave_profile_id"] = profile.ID
	values["request_unit"] = count.RequestUnit
	values["requested_days"] = count.RequestedDays
	values["requested_hours"] = count.RequestedHours
	values["half_day_session"] = count.HalfDaySession
	values["count_basis"] = count.Basis
	values["counted_dates_json"] = marshalStringList(count.CountedDates)
	values["counted_work_calendar_id"] = count.WorkCalendarID
	values["counted_roster_slot_ids_json"] = marshalStringList(count.RosterSlotIDs)
	values["balance_account_id"] = account.ID
	values["request_source"] = "self_service"
	values["self_service_actor_user_id"] = textValue(values["self_service_actor_user_id"])
	values["approved_days"] = 0.0
	if values["reservation_entry_ids_json"] == nil {
		values["reservation_entry_ids_json"] = "[]"
	}
	if values["consumption_entry_ids_json"] == nil {
		values["consumption_entry_ids_json"] = "[]"
	}
	clearLeaveApprovalState(values)
	if values["status"] == nil || textValue(values["status"]) == "" {
		values["status"] = "active"
	}
	return values, policy, profile, absenceCode, account, nil
}

func (s *LeavePolicyCoreService) resolveExistingLeaveContext(current, assignment model.Record, values map[string]any) (model.Record, model.Record, model.Record, model.Record, error) {
	if current.ID == "" {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, shared.Validation("no active leave profile matches this request")
	}
	policyID := strings.TrimSpace(textValue(values["leave_policy_id"]))
	if policyID == "" {
		policyID = strings.TrimSpace(textValue(current.Values["leave_policy_id"]))
	}
	if policyID == "" {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, shared.Validation("no active leave profile matches this request")
	}
	policy, err := s.models.Get("leave_policy", policyID)
	if err != nil || !strings.EqualFold(textValue(policy.Values["status"]), "active") {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, shared.Validation("no active leave profile matches this request")
	}
	if boolValue(policy.Values["requires_balance"]) {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, shared.Validation("no active leave profile matches this request")
	}
	absenceCodeID := strings.TrimSpace(textValue(values["absence_code_id"]))
	if absenceCodeID == "" {
		absenceCodeID = strings.TrimSpace(textValue(current.Values["absence_code_id"]))
	}
	if absenceCodeID != "" && absenceCodeID != strings.TrimSpace(textValue(policy.Values["absence_code_id"])) {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, shared.Validation("no active leave profile matches this request")
	}
	absenceCode, err := s.models.Get("absence_code", textValue(policy.Values["absence_code_id"]))
	if err != nil {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
	}
	profile := model.Record{
		ID: strings.TrimSpace(textValue(current.Values["employee_leave_profile_id"])),
		Values: map[string]any{
			"employee_id":     textValue(current.Values["employee_id"]),
			"leave_policy_id": policy.ID,
			"organization_id": leaveFirstNonEmpty(textValue(current.Values["organization_id"]), textValue(assignment.Values["organization_id"])),
			"location_id":     leaveFirstNonEmpty(textValue(current.Values["location_id"]), textValue(assignment.Values["location_id"])),
		},
	}
	return policy, profile, absenceCode, model.Record{}, nil
}

func (s *LeavePolicyCoreService) amendLeaveRequest(record model.Record, payload map[string]any, actorID string) (model.Record, error) {
	status := strings.ToLower(textValue(record.Values["approval_status"]))
	employee, assignment, err := s.resolveEmployeeContextForRecord(record)
	if err != nil {
		return model.Record{}, err
	}
	values, policy, profile, _, _, err := s.prepareLeaveRequestValues(employee, assignment, record, payload, actorID)
	if err != nil {
		return model.Record{}, err
	}
	if err := validateLeaveRequestPolicy(policy, model.Record{ID: record.ID, Values: values}); err != nil {
		return model.Record{}, err
	}
	amendReason := strings.TrimSpace(textValue(payload["reason"]))
	switch status {
	case "draft":
		values["approval_status"] = "draft"
		values["request_source"] = textValue(record.Values["request_source"])
		values["self_service_actor_user_id"] = textValue(record.Values["self_service_actor_user_id"])
		values["reservation_entry_ids_json"] = textValue(record.Values["reservation_entry_ids_json"])
		values["consumption_entry_ids_json"] = textValue(record.Values["consumption_entry_ids_json"])
		applyLeaveAmendmentMetadata(values, record, actorID, amendReason)
		return s.models.Update("leave_request", record.ID, actorID, values, record.Version)
	case "submitted":
		if err := s.preflightAmendmentBalance(record, values, policy, profile, actorID); err != nil {
			return model.Record{}, err
		}
		if err := s.releaseReservedBalance(record, actorID); err != nil {
			return model.Record{}, err
		}
		values["approval_status"] = "submitted"
		values["approved_days"] = 0.0
		values["request_source"] = textValue(record.Values["request_source"])
		values["self_service_actor_user_id"] = textValue(record.Values["self_service_actor_user_id"])
		values["reservation_entry_ids_json"] = "[]"
		values["consumption_entry_ids_json"] = textValue(record.Values["consumption_entry_ids_json"])
		applyLeaveAmendmentMetadata(values, record, actorID, amendReason)
		updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
		if err != nil {
			return model.Record{}, err
		}
		return s.submitLeaveRequest(updated, actorID)
	case "approved":
		oldStartDate := textValue(record.Values["start_date"])
		oldEndDate := textValue(record.Values["end_date"])
		if blocked, reason, err := s.payrollCutoffForRequest(record, values); err != nil {
			return model.Record{}, err
		} else if blocked {
			return model.Record{}, shared.Conflict(reason)
		}
		if err := s.preflightAmendmentBalance(record, values, policy, profile, actorID); err != nil {
			return model.Record{}, err
		}
		if err := s.reverseConsumedBalance(record, actorID); err != nil {
			return model.Record{}, err
		}
		values["approval_status"] = "submitted"
		values["approved_days"] = 0.0
		values["request_source"] = textValue(record.Values["request_source"])
		values["self_service_actor_user_id"] = textValue(record.Values["self_service_actor_user_id"])
		values["reservation_entry_ids_json"] = "[]"
		values["consumption_entry_ids_json"] = "[]"
		applyLeaveAmendmentMetadata(values, record, actorID, amendReason)
		updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
		if err != nil {
			return model.Record{}, err
		}
		s.syncLeaveRangeForDates(textValue(updated.Values["employee_id"]), oldStartDate, oldEndDate)
		return s.submitLeaveRequest(updated, actorID)
	default:
		return model.Record{}, shared.Validation("leave request cannot be amended")
	}
}

func (s *LeavePolicyCoreService) resolveApplicableLeaveContext(employee, assignment model.Record, values map[string]any, actorID string) (model.Record, model.Record, model.Record, model.Record, error) {
	policyID := strings.TrimSpace(textValue(values["leave_policy_id"]))
	absenceCodeID := strings.TrimSpace(textValue(values["absence_code_id"]))
	effectiveAt := parseDateOnly(values["start_date"])
	if effectiveAt.IsZero() {
		effectiveAt = time.Now().UTC()
	}
	profiles, _, err := s.models.List("employee_leave_profile", model.Query{
		Filters:  map[string]string{"employee_id": employee.ID, "status": "active"},
		SortKey:  "effective_from",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
	}
	var selectedProfile model.Record
	var selectedPolicy model.Record
	for _, profile := range profiles {
		if !leaveProfileEffectiveAt(profile, effectiveAt) {
			continue
		}
		policy, err := s.models.Get("leave_policy", textValue(profile.Values["leave_policy_id"]))
		if err != nil || !strings.EqualFold(textValue(policy.Values["status"]), "active") {
			continue
		}
		if policyID != "" && policy.ID != policyID {
			continue
		}
		if absenceCodeID != "" && textValue(policy.Values["absence_code_id"]) != absenceCodeID {
			continue
		}
		selectedProfile = profile
		selectedPolicy = policy
		break
	}
	if selectedProfile.ID == "" {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, shared.Validation("no active leave profile matches this request")
	}
	absenceCode, err := s.models.Get("absence_code", textValue(selectedPolicy.Values["absence_code_id"]))
	if err != nil {
		return model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
	}
	var account model.Record
	if boolValue(selectedPolicy.Values["requires_balance"]) {
		account, err = s.ensureBalanceAccount(selectedProfile, selectedPolicy, actorID)
		if err != nil {
			return model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
		}
	}
	return selectedPolicy, selectedProfile, absenceCode, account, nil
}

func (s *LeavePolicyCoreService) accountForRequest(record model.Record, actorID string) (model.Record, error) {
	if accountID := strings.TrimSpace(textValue(record.Values["balance_account_id"])); accountID != "" {
		return s.models.Get("leave_balance_account", accountID)
	}
	profileID := strings.TrimSpace(textValue(record.Values["employee_leave_profile_id"]))
	if profileID == "" {
		return model.Record{}, shared.Validation("leave balance account is required")
	}
	profile, err := s.models.Get("employee_leave_profile", profileID)
	if err != nil {
		return model.Record{}, err
	}
	policy, err := s.models.Get("leave_policy", textValue(record.Values["leave_policy_id"]))
	if err != nil {
		return model.Record{}, err
	}
	if !boolValue(policy.Values["requires_balance"]) {
		return model.Record{}, shared.Validation("leave balance account is not applicable for this leave policy")
	}
	return s.ensureBalanceAccount(profile, policy, actorID)
}

func (s *LeavePolicyCoreService) resolveEmployeeContextForRecord(record model.Record) (model.Record, model.Record, error) {
	employee, err := s.models.Get("employee_profile", textValue(record.Values["employee_id"]))
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	if s == nil || s.workforce == nil {
		return employee, model.Record{}, nil
	}
	assignment, ok, err := s.workforce.ResolveCurrentAssignment(employee.ID, time.Now().UTC())
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	if !ok {
		return employee, model.Record{}, nil
	}
	return employee, assignment, nil
}

func (s *LeavePolicyCoreService) resolveLeavePolicyForProfile(profile model.Record) (model.Record, model.Record, bool) {
	policyID := strings.TrimSpace(textValue(profile.Values["leave_policy_id"]))
	if policyID == "" {
		return model.Record{}, model.Record{}, false
	}
	policy, err := s.models.Get("leave_policy", policyID)
	if err != nil || !strings.EqualFold(textValue(policy.Values["status"]), "active") {
		return model.Record{}, model.Record{}, false
	}
	rules, _, err := s.models.List("leave_entitlement_rule", model.Query{
		Filters:  map[string]string{"leave_policy_id": policy.ID, "status": "active"},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(rules) == 0 {
		return model.Record{}, model.Record{}, false
	}
	return policy, rules[0], true
}

func (s *LeavePolicyCoreService) ensureBalanceAccount(profile, policy model.Record, actorID string) (model.Record, error) {
	items, _, err := s.models.List("leave_balance_account", model.Query{
		Filters:  map[string]string{"employee_id": textValue(profile.Values["employee_id"]), "leave_policy_id": policy.ID, "status": "active"},
		SortKey:  "updated_at",
		Desc:     true,
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		return model.Record{}, err
	}
	if len(items) > 0 {
		return items[0], nil
	}
	account, err := s.models.Create("leave_balance_account", actorID, map[string]any{
		"employee_id":                textValue(profile.Values["employee_id"]),
		"leave_policy_id":            policy.ID,
		"employee_leave_profile_id":  profile.ID,
		"organization_id":            leaveFirstNonEmpty(textValue(profile.Values["organization_id"]), textValue(policy.Values["organization_id"])),
		"location_id":                leaveFirstNonEmpty(textValue(profile.Values["location_id"]), textValue(policy.Values["location_id"])),
		"current_balance_days":       0.0,
		"reserved_days":              0.0,
		"available_days":             0.0,
		"carry_forward_balance_days": 0.0,
		"carry_forward_expiry_date":  "",
		"status":                     "active",
	})
	if err != nil {
		return model.Record{}, err
	}
	opening := numberValue(profile.Values["opening_balance_days"])
	if opening > 0 {
		if _, err := s.createBalanceEntry(account.ID, profile, policy, actorID, map[string]any{
			"entry_type":      "opening",
			"days":            opening,
			"effective_date":  time.Now().UTC().Format("2006-01-02"),
			"employee_id":     textValue(profile.Values["employee_id"]),
			"leave_policy_id": policy.ID,
			"status":          "active",
		}); err != nil {
			return model.Record{}, err
		}
		return s.models.Get("leave_balance_account", account.ID)
	}
	return account, nil
}

func (s *LeavePolicyCoreService) createBalanceEntry(accountID string, profile, policy model.Record, actorID string, values map[string]any) (model.Record, error) {
	entry, err := s.models.Create("leave_balance_entry", actorID, mergeMaps(map[string]any{
		"balance_account_id":        accountID,
		"employee_leave_profile_id": profile.ID,
		"status":                    "active",
	}, values))
	if err != nil {
		return model.Record{}, err
	}
	if _, err := s.recomputeBalanceAccount(accountID, actorID); err != nil {
		return model.Record{}, err
	}
	return entry, nil
}

func (s *LeavePolicyCoreService) recomputeBalanceAccount(accountID, actorID string) (model.Record, error) {
	account, err := s.models.Get("leave_balance_account", strings.TrimSpace(accountID))
	if err != nil {
		return model.Record{}, err
	}
	entries, _, err := s.models.List("leave_balance_entry", model.Query{
		Filters:  map[string]string{"balance_account_id": account.ID},
		SortKey:  "effective_date",
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, err
	}
	current := 0.0
	reserved := 0.0
	carryForward := 0.0
	var lastAccrual string
	for _, entry := range entries {
		if !strings.EqualFold(textValue(entry.Values["status"]), "active") {
			continue
		}
		days := numberValue(entry.Values["days"])
		carryForward += numberValue(entry.Values["carry_forward_days_delta"])
		switch strings.ToLower(textValue(entry.Values["entry_type"])) {
		case "opening", "annual_grant", "monthly_accrual", "carry_forward", "adjustment", "reversal":
			current += days
			if entryType := strings.ToLower(textValue(entry.Values["entry_type"])); entryType == "annual_grant" || entryType == "monthly_accrual" {
				lastAccrual = textValue(entry.Values["effective_date"])
			}
		case "consumption", "expiry":
			current -= days
		case "reservation":
			reserved += days
		case "release":
			reserved -= days
		}
	}
	if reserved < 0 {
		reserved = 0
	}
	if carryForward < 0 {
		carryForward = 0
	}
	available := current - reserved
	values := cloneMap(account.Values)
	values["current_balance_days"] = roundAttendanceHours(current)
	values["reserved_days"] = roundAttendanceHours(reserved)
	values["available_days"] = roundAttendanceHours(available)
	values["carry_forward_balance_days"] = roundAttendanceHours(carryForward)
	if carryForward <= 0 {
		values["carry_forward_expiry_date"] = ""
	}
	if lastAccrual != "" {
		values["last_accrual_date"] = lastAccrual
	}
	return s.models.Update("leave_balance_account", account.ID, actorID, values, account.Version)
}

func (s *LeavePolicyCoreService) consumeReservedBalance(record model.Record, actorID string) ([]string, error) {
	if err := s.releaseReservedBalance(record, actorID); err != nil {
		return nil, err
	}
	policyID := strings.TrimSpace(textValue(record.Values["leave_policy_id"]))
	policy, err := s.models.Get("leave_policy", policyID)
	if err != nil {
		return nil, err
	}
	if !boolValue(policy.Values["requires_balance"]) {
		return nil, nil
	}
	account, err := s.accountForRequest(record, actorID)
	if err != nil {
		return nil, err
	}
	profile, err := s.models.Get("employee_leave_profile", textValue(record.Values["employee_leave_profile_id"]))
	if err != nil {
		return nil, err
	}
	startDate := parseDateOnly(record.Values["start_date"])
	account, err = s.applyPendingCarryForwardExpiry(account, profile, policy, actorID, startDate)
	if err != nil {
		return nil, err
	}
	requestedDays := numberValue(record.Values["requested_days"])
	carryForwardConsumed, err := s.carryForwardBalanceAt(account.ID, startDate)
	if err != nil {
		return nil, err
	}
	carryForwardConsumed = leaveMinFloat(carryForwardConsumed, requestedDays)
	entry, err := s.createBalanceEntry(account.ID, model.Record{}, policy, actorID, map[string]any{
		"entry_type":               "consumption",
		"days":                     requestedDays,
		"carry_forward_days_delta": -carryForwardConsumed,
		"effective_date":           textValue(record.Values["start_date"]),
		"leave_request_id":         record.ID,
		"employee_id":              textValue(record.Values["employee_id"]),
		"leave_policy_id":          policy.ID,
		"status":                   "active",
	})
	if err != nil {
		return nil, err
	}
	return []string{entry.ID}, nil
}

func (s *LeavePolicyCoreService) releaseReservedBalance(record model.Record, actorID string) error {
	entryIDs := parseStringList(record.Values["reservation_entry_ids_json"])
	if len(entryIDs) == 0 {
		return nil
	}
	policyID := strings.TrimSpace(textValue(record.Values["leave_policy_id"]))
	policy, err := s.models.Get("leave_policy", policyID)
	if err != nil {
		return err
	}
	account, err := s.accountForRequest(record, actorID)
	if err != nil {
		return err
	}
	for _, entryID := range entryIDs {
		entry, err := s.models.Get("leave_balance_entry", entryID)
		if err != nil {
			continue
		}
		if !strings.EqualFold(textValue(entry.Values["status"]), "active") {
			continue
		}
		if _, err := s.createBalanceEntry(account.ID, model.Record{}, policy, actorID, map[string]any{
			"entry_type":       "release",
			"days":             numberValue(entry.Values["days"]),
			"effective_date":   textValue(record.Values["start_date"]),
			"leave_request_id": record.ID,
			"employee_id":      textValue(record.Values["employee_id"]),
			"leave_policy_id":  policy.ID,
			"status":           "active",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *LeavePolicyCoreService) reverseConsumedBalance(record model.Record, actorID string) error {
	entryIDs := parseStringList(record.Values["consumption_entry_ids_json"])
	if len(entryIDs) == 0 {
		return nil
	}
	policyID := strings.TrimSpace(textValue(record.Values["leave_policy_id"]))
	policy, err := s.models.Get("leave_policy", policyID)
	if err != nil {
		return err
	}
	account, err := s.accountForRequest(record, actorID)
	if err != nil {
		return err
	}
	for _, entryID := range entryIDs {
		entry, err := s.models.Get("leave_balance_entry", entryID)
		if err != nil || !strings.EqualFold(textValue(entry.Values["status"]), "active") {
			continue
		}
		if _, err := s.createBalanceEntry(account.ID, model.Record{}, policy, actorID, map[string]any{
			"entry_type":               "reversal",
			"days":                     numberValue(entry.Values["days"]),
			"carry_forward_days_delta": -numberValue(entry.Values["carry_forward_days_delta"]),
			"reversal_of_entry_id":     entry.ID,
			"effective_date":           time.Now().UTC().Format("2006-01-02"),
			"leave_request_id":         record.ID,
			"employee_id":              textValue(record.Values["employee_id"]),
			"leave_policy_id":          policy.ID,
			"status":                   "active",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *LeavePolicyCoreService) applyPendingCarryForwardExpiry(account, profile, policy model.Record, actorID string, effectiveDate time.Time) (model.Record, error) {
	expiryDate := parseDateOnly(account.Values["carry_forward_expiry_date"])
	if expiryDate.IsZero() || !effectiveDate.After(expiryDate) {
		return account, nil
	}
	carryForwardDays := numberValue(account.Values["carry_forward_balance_days"])
	if carryForwardDays <= 0 {
		values := cloneMap(account.Values)
		values["carry_forward_expiry_date"] = ""
		return s.models.Update("leave_balance_account", account.ID, actorID, values, account.Version)
	}
	entry, err := s.createBalanceEntry(account.ID, profile, policy, actorID, map[string]any{
		"entry_type":               "expiry",
		"days":                     carryForwardDays,
		"carry_forward_days_delta": -carryForwardDays,
		"effective_date":           effectiveDate.Format("2006-01-02"),
		"employee_id":              textValue(profile.Values["employee_id"]),
		"leave_policy_id":          policy.ID,
		"status":                   "active",
	})
	if err != nil {
		return model.Record{}, err
	}
	_ = entry
	account, err = s.models.Get("leave_balance_account", account.ID)
	if err != nil {
		return model.Record{}, err
	}
	values := cloneMap(account.Values)
	values["carry_forward_expiry_date"] = ""
	return s.models.Update("leave_balance_account", account.ID, actorID, values, account.Version)
}

func (s *LeavePolicyCoreService) carryForwardBalanceAt(accountID string, at time.Time) (float64, error) {
	if at.IsZero() {
		return 0, nil
	}
	entries, _, err := s.models.List("leave_balance_entry", model.Query{
		Filters:  map[string]string{"balance_account_id": strings.TrimSpace(accountID)},
		SortKey:  "effective_date",
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return 0, err
	}
	carryForward := 0.0
	for _, entry := range entries {
		if !strings.EqualFold(textValue(entry.Values["status"]), "active") {
			continue
		}
		entryDate := parseDateOnly(entry.Values["effective_date"])
		if !entryDate.IsZero() && entryDate.After(at) {
			continue
		}
		carryForward += numberValue(entry.Values["carry_forward_days_delta"])
	}
	if carryForward < 0 {
		carryForward = 0
	}
	return roundAttendanceHours(carryForward), nil
}

func (s *LeavePolicyCoreService) preflightAmendmentBalance(record model.Record, proposedValues map[string]any, policy, profile model.Record, actorID string) error {
	if !boolValue(policy.Values["requires_balance"]) {
		return nil
	}
	account, err := s.ensureBalanceAccount(profile, policy, actorID)
	if err != nil {
		return err
	}
	startDate := parseDateOnly(proposedValues["start_date"])
	if startDate.IsZero() {
		startDate = parseDateOnly(record.Values["start_date"])
	}
	projected, err := s.projectAvailableDays(record, account, startDate)
	if err != nil {
		return err
	}
	requestedDays := numberValue(proposedValues["requested_days"])
	if requestedDays <= 0 {
		return shared.Validation("requested_days must be greater than zero")
	}
	if projected < requestedDays {
		return shared.Validation("leave balance is insufficient")
	}
	_ = profile
	return nil
}

func (s *LeavePolicyCoreService) projectAvailableDays(record, account model.Record, startDate time.Time) (float64, error) {
	current := numberValue(account.Values["current_balance_days"])
	reserved := numberValue(account.Values["reserved_days"])
	carryForward := numberValue(account.Values["carry_forward_balance_days"])
	if textValue(record.Values["balance_account_id"]) == account.ID {
		for _, entryID := range parseStringList(record.Values["reservation_entry_ids_json"]) {
			entry, err := s.models.Get("leave_balance_entry", entryID)
			if err != nil || !strings.EqualFold(textValue(entry.Values["status"]), "active") {
				continue
			}
			reserved -= numberValue(entry.Values["days"])
		}
		for _, entryID := range parseStringList(record.Values["consumption_entry_ids_json"]) {
			entry, err := s.models.Get("leave_balance_entry", entryID)
			if err != nil || !strings.EqualFold(textValue(entry.Values["status"]), "active") {
				continue
			}
			current += numberValue(entry.Values["days"])
			carryForward -= numberValue(entry.Values["carry_forward_days_delta"])
		}
	}
	if reserved < 0 {
		reserved = 0
	}
	if carryForward < 0 {
		carryForward = 0
	}
	expiryDate := parseDateOnly(account.Values["carry_forward_expiry_date"])
	if !expiryDate.IsZero() && startDate.After(expiryDate) {
		current -= carryForward
		carryForward = 0
	}
	available := roundAttendanceHours(current - reserved)
	if available < 0 {
		return 0, nil
	}
	return available, nil
}

func (s *LeavePolicyCoreService) applyAnnualCarryForward(account, profile, policy, rule model.Record, actorID string, effectiveDate time.Time, runID string) (model.Record, error) {
	leftover := numberValue(account.Values["current_balance_days"])
	if leftover <= 0 {
		values := cloneMap(account.Values)
		values["carry_forward_expiry_date"] = ""
		return s.models.Update("leave_balance_account", account.ID, actorID, values, account.Version)
	}
	existingCarry := leaveMinFloat(numberValue(account.Values["carry_forward_balance_days"]), leftover)
	capDays := numberValue(rule.Values["carry_forward_cap_days"])
	expiryRule := strings.ToLower(strings.TrimSpace(textValue(rule.Values["carry_forward_expiry_rule"])))
	carryEnabled := capDays > 0 || expiryRule != ""
	carryAmount := 0.0
	if carryEnabled {
		if capDays > 0 {
			carryAmount = leaveMinFloat(leftover, capDays)
		} else {
			carryAmount = leftover
		}
	}
	if _, err := s.createBalanceEntry(account.ID, profile, policy, actorID, map[string]any{
		"entry_type":               "expiry",
		"days":                     leftover,
		"carry_forward_days_delta": -existingCarry,
		"effective_date":           effectiveDate.Format("2006-01-02"),
		"accrual_run_id":           runID,
		"employee_id":              textValue(profile.Values["employee_id"]),
		"leave_policy_id":          policy.ID,
		"status":                   "active",
	}); err != nil {
		return model.Record{}, err
	}
	if carryAmount > 0 {
		if _, err := s.createBalanceEntry(account.ID, profile, policy, actorID, map[string]any{
			"entry_type":               "carry_forward",
			"days":                     carryAmount,
			"carry_forward_days_delta": carryAmount,
			"effective_date":           effectiveDate.Format("2006-01-02"),
			"accrual_run_id":           runID,
			"employee_id":              textValue(profile.Values["employee_id"]),
			"leave_policy_id":          policy.ID,
			"status":                   "active",
		}); err != nil {
			return model.Record{}, err
		}
	}
	account, err := s.models.Get("leave_balance_account", account.ID)
	if err != nil {
		return model.Record{}, err
	}
	values := cloneMap(account.Values)
	values["carry_forward_expiry_date"] = ""
	if carryAmount > 0 && expiryRule != "" {
		values["carry_forward_expiry_date"] = leaveCarryForwardExpiryDate(expiryRule, effectiveDate).Format("2006-01-02")
	}
	return s.models.Update("leave_balance_account", account.ID, actorID, values, account.Version)
}

func leaveAccrualGrantDays(rule, profile model.Record, effectiveDate time.Time) float64 {
	days := numberValue(rule.Values["monthly_accrual_days"])
	if days <= 0 {
		return 0
	}
	if !boolValue(rule.Values["prorate_on_join"]) {
		return roundAttendanceHours(days)
	}
	profileStart := parseDateOnly(profile.Values["effective_from"])
	if profileStart.IsZero() {
		return roundAttendanceHours(days)
	}
	monthStart := time.Date(effectiveDate.Year(), effectiveDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, -1)
	if profileStart.Before(monthStart) || profileStart.After(monthEnd) {
		return roundAttendanceHours(days)
	}
	activeDays := float64(monthEnd.Day() - profileStart.Day() + 1)
	totalDays := float64(monthEnd.Day())
	if totalDays <= 0 || activeDays <= 0 {
		return 0
	}
	return roundAttendanceHours(days * activeDays / totalDays)
}

func leaveCarryForwardExpiryDate(rule string, effectiveDate time.Time) time.Time {
	switch strings.ToLower(strings.TrimSpace(rule)) {
	case "year_end":
		return time.Date(effectiveDate.Year(), time.December, 31, 0, 0, 0, 0, time.UTC)
	case "q1_end":
		return time.Date(effectiveDate.Year(), time.March, 31, 0, 0, 0, 0, time.UTC)
	case "month_end":
		return time.Date(effectiveDate.Year(), effectiveDate.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}
	}
}

func leaveMinFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (s *LeavePolicyCoreService) syncLeaveRange(record model.Record) {
	if s == nil || s.attendance == nil {
		return
	}
	s.syncLeaveRangeForDates(textValue(record.Values["employee_id"]), textValue(record.Values["start_date"]), textValue(record.Values["end_date"]))
}

func (s *LeavePolicyCoreService) syncLeaveRangeForDates(employeeID, startDate, endDate string) {
	if s == nil || s.attendance == nil {
		return
	}
	start := parseDateOnly(startDate)
	end := parseDateOnly(endDate)
	if start.IsZero() {
		return
	}
	if end.IsZero() || end.Before(start) {
		end = start
	}
	for day := start; !day.After(end); day = day.Add(24 * time.Hour) {
		_, _ = s.attendance.SyncAttendanceDay(strings.TrimSpace(employeeID), day)
	}
}

func (s *LeavePolicyCoreService) resolveEmployeeForUser(userID string) (model.Record, model.Record, error) {
	if s == nil || s.workforce == nil {
		return model.Record{}, model.Record{}, shared.Validation("leave policy service is not available")
	}
	employee, ok, err := s.workforce.ResolveEmployeeByUser(strings.TrimSpace(userID))
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	if !ok {
		return model.Record{}, model.Record{}, shared.Forbidden("employee self-service is not available")
	}
	assignment, ok, err := s.workforce.ResolveCurrentAssignment(employee.ID, time.Now().UTC())
	if err != nil {
		return model.Record{}, model.Record{}, err
	}
	if !ok {
		return model.Record{}, model.Record{}, shared.Forbidden("employee self-service requires a current assignment")
	}
	return employee, assignment, nil
}

func validateLeaveRequestPolicy(policy model.Record, record model.Record) error {
	start := parseDateOnly(record.Values["start_date"])
	if start.IsZero() {
		return shared.Validation("start_date is required")
	}
	noticeDays := int(numberValue(policy.Values["notice_days"]))
	if noticeDays > 0 {
		earliest := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, noticeDays)
		if start.Before(earliest) {
			return shared.Validation("leave request violates policy notice period")
		}
	}
	return nil
}

func leaveProfileEffectiveAt(profile model.Record, at time.Time) bool {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	from := parseDateOnly(profile.Values["effective_from"])
	if !from.IsZero() && at.Before(from) {
		return false
	}
	to := parseDateOnly(profile.Values["effective_to"])
	if !to.IsZero() && at.After(to.Add(23*time.Hour+59*time.Minute+59*time.Second)) {
		return false
	}
	return strings.EqualFold(textValue(profile.Values["status"]), "active")
}

func (s *LeavePolicyCoreService) resolveRequestedLeaveCount(policy model.Record, values map[string]any) (leaveCountResolution, error) {
	start := parseDateOnly(values["start_date"])
	end := parseDateOnly(values["end_date"])
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return leaveCountResolution{}, shared.Validation("leave request date range is invalid")
	}
	requestUnit := strings.ToLower(strings.TrimSpace(textValue(values["request_unit"])))
	if requestUnit == "" {
		requestUnit = "day"
	}
	if requestUnit == "half_day" {
		if !boolValue(policy.Values["allows_half_day"]) {
			return leaveCountResolution{}, shared.Validation("leave policy does not allow half day requests")
		}
		if start.Format("2006-01-02") != end.Format("2006-01-02") {
			return leaveCountResolution{}, shared.Validation("half day leave must be requested for a single day")
		}
	}
	resolution, ok, err := s.resolveRosterLeaveCount(values, start, end, requestUnit)
	if err != nil {
		return leaveCountResolution{}, err
	}
	if !ok {
		resolution, ok, err = s.resolveCalendarLeaveCount(values, start, end, requestUnit)
		if err != nil {
			return leaveCountResolution{}, err
		}
	}
	if !ok {
		resolution, err = resolveLegacyLeaveCount(policy, values, start, end, requestUnit)
		if err != nil {
			return leaveCountResolution{}, err
		}
	}
	if resolution.RequestedDays <= 0 {
		return leaveCountResolution{}, shared.Validation("requested_days must be greater than zero")
	}
	return resolution, nil
}

func resolveLegacyLeaveCount(policy model.Record, values map[string]any, start, end time.Time, requestUnit string) (leaveCountResolution, error) {
	switch requestUnit {
	case "half_day":
		session := strings.ToLower(strings.TrimSpace(textValue(values["half_day_session"])))
		if session != "morning" && session != "afternoon" {
			return leaveCountResolution{}, shared.Validation("half_day_session must be morning or afternoon")
		}
		date := start.Format("2006-01-02")
		return leaveCountResolution{
			RequestUnit:      requestUnit,
			RequestedDays:    0.5,
			RequestedHours:   0,
			HalfDaySession:   session,
			Basis:            "legacy",
			CountedDates:     []string{date},
			CountExplanation: "0.5 day from date range fallback",
		}, nil
	default:
		days := 1.0 + end.Sub(start).Hours()/24
		days = roundAttendanceHours(days)
		countedDates := make([]string, 0)
		for day := start; !day.After(end); day = day.Add(24 * time.Hour) {
			countedDates = append(countedDates, day.Format("2006-01-02"))
		}
		return leaveCountResolution{
			RequestUnit:      "day",
			RequestedDays:    days,
			RequestedHours:   0,
			HalfDaySession:   "",
			Basis:            "legacy",
			CountedDates:     countedDates,
			CountExplanation: fmt.Sprintf("%.1f days from date range fallback", days),
		}, nil
	}
}

func (s *LeavePolicyCoreService) resolveRosterLeaveCount(values map[string]any, start, end time.Time, requestUnit string) (leaveCountResolution, bool, error) {
	if s == nil || s.models == nil {
		return leaveCountResolution{}, false, nil
	}
	employeeID := strings.TrimSpace(textValue(values["employee_id"]))
	if employeeID == "" {
		return leaveCountResolution{}, false, nil
	}
	slots, err := s.listAllRecords("workforce_roster_slot", model.Query{
		Filters:  map[string]string{"employee_id": employeeID},
		SortKey:  "shift_date",
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return leaveCountResolution{}, false, err
	}
	type rosterDay struct {
		date  string
		hours float64
		slots []string
	}
	perDate := map[string]rosterDay{}
	locationID := strings.TrimSpace(textValue(values["location_id"]))
	organizationID := strings.TrimSpace(textValue(values["organization_id"]))
	for _, slot := range slots {
		if !strings.EqualFold(textValue(slot.Values["status"]), "active") {
			continue
		}
		shiftDate := parseDateOnly(slot.Values["shift_date"])
		if shiftDate.IsZero() || shiftDate.Before(start) || shiftDate.After(end) {
			continue
		}
		if organizationID != "" && textValue(slot.Values["organization_id"]) != "" && textValue(slot.Values["organization_id"]) != organizationID {
			continue
		}
		if locationID != "" && textValue(slot.Values["location_id"]) != "" && textValue(slot.Values["location_id"]) != locationID {
			continue
		}
		if s.attendance != nil && !s.attendance.rosterPublished(slot) {
			continue
		}
		dateKey := shiftDate.Format("2006-01-02")
		item := perDate[dateKey]
		item.date = dateKey
		item.hours += leaveRosterSlotHours(slot)
		item.slots = append(item.slots, slot.ID)
		perDate[dateKey] = item
	}
	if len(perDate) == 0 {
		return leaveCountResolution{}, false, nil
	}
	countedDates := make([]string, 0, len(perDate))
	rosterSlotIDs := make([]string, 0)
	requestedHours := 0.0
	for dateKey, item := range perDate {
		countedDates = append(countedDates, dateKey)
		requestedHours += item.hours
		rosterSlotIDs = append(rosterSlotIDs, item.slots...)
	}
	sort.Strings(countedDates)
	sort.Strings(rosterSlotIDs)
	if requestUnit == "half_day" {
		if len(countedDates) != 1 {
			return leaveCountResolution{}, false, shared.Validation("half day leave must fall on exactly one working date")
		}
		session := strings.ToLower(strings.TrimSpace(textValue(values["half_day_session"])))
		if session != "morning" && session != "afternoon" {
			return leaveCountResolution{}, false, shared.Validation("half_day_session must be morning or afternoon")
		}
		requestedHours = roundAttendanceHours(requestedHours / 2)
		return leaveCountResolution{
			RequestUnit:      "half_day",
			RequestedDays:    0.5,
			RequestedHours:   requestedHours,
			HalfDaySession:   session,
			Basis:            "roster",
			CountedDates:     countedDates,
			RosterSlotIDs:    rosterSlotIDs,
			CountExplanation: "0.5 day from published roster",
		}, true, nil
	}
	requestedDays := roundAttendanceHours(float64(len(countedDates)))
	return leaveCountResolution{
		RequestUnit:      "day",
		RequestedDays:    requestedDays,
		RequestedHours:   roundAttendanceHours(requestedHours),
		HalfDaySession:   "",
		Basis:            "roster",
		CountedDates:     countedDates,
		RosterSlotIDs:    rosterSlotIDs,
		CountExplanation: fmt.Sprintf("%.1f working days from published roster", requestedDays),
	}, true, nil
}

func (s *LeavePolicyCoreService) resolveCalendarLeaveCount(values map[string]any, start, end time.Time, requestUnit string) (leaveCountResolution, bool, error) {
	calendar, ok, err := s.resolveWorkCalendar(values)
	if err != nil || !ok {
		return leaveCountResolution{}, false, err
	}
	workingDays := parseWorkCalendarWeekdays(calendar.Values["working_days_json"])
	holidays := parseWorkCalendarDates(calendar.Values["holiday_dates_json"])
	countedDates := make([]string, 0)
	for day := start; !day.After(end); day = day.Add(24 * time.Hour) {
		dateKey := day.Format("2006-01-02")
		if holidays[dateKey] {
			continue
		}
		if len(workingDays) > 0 && !workingDays[day.Weekday()] {
			continue
		}
		countedDates = append(countedDates, dateKey)
	}
	if len(countedDates) == 0 {
		return leaveCountResolution{}, true, shared.Validation("leave request does not include any working dates")
	}
	if requestUnit == "half_day" {
		if len(countedDates) != 1 {
			return leaveCountResolution{}, true, shared.Validation("half day leave must fall on exactly one working date")
		}
		session := strings.ToLower(strings.TrimSpace(textValue(values["half_day_session"])))
		if session != "morning" && session != "afternoon" {
			return leaveCountResolution{}, true, shared.Validation("half_day_session must be morning or afternoon")
		}
		return leaveCountResolution{
			RequestUnit:      "half_day",
			RequestedDays:    0.5,
			RequestedHours:   0,
			HalfDaySession:   session,
			Basis:            "calendar",
			CountedDates:     countedDates,
			WorkCalendarID:   calendar.ID,
			CountExplanation: "0.5 day from work calendar",
		}, true, nil
	}
	requestedDays := roundAttendanceHours(float64(len(countedDates)))
	return leaveCountResolution{
		RequestUnit:      "day",
		RequestedDays:    requestedDays,
		RequestedHours:   0,
		HalfDaySession:   "",
		Basis:            "calendar",
		CountedDates:     countedDates,
		WorkCalendarID:   calendar.ID,
		CountExplanation: fmt.Sprintf("%.1f working days from work calendar", requestedDays),
	}, true, nil
}

func (s *LeavePolicyCoreService) resolveWorkCalendar(values map[string]any) (model.Record, bool, error) {
	if s == nil || s.models == nil {
		return model.Record{}, false, nil
	}
	items, err := s.listAllRecords("work_calendar", model.Query{
		SortKey:  "updated_at",
		Desc:     true,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return model.Record{}, false, err
	}
	locationID := strings.TrimSpace(textValue(values["location_id"]))
	organizationID := strings.TrimSpace(textValue(values["organization_id"]))
	var orgOnly model.Record
	for _, item := range items {
		if !strings.EqualFold(textValue(item.Values["status"]), "active") {
			continue
		}
		if organizationID != "" && textValue(item.Values["organization_id"]) != "" && textValue(item.Values["organization_id"]) != organizationID {
			continue
		}
		if locationID != "" && textValue(item.Values["location_id"]) == locationID {
			return item, true, nil
		}
		if orgOnly.ID == "" && textValue(item.Values["location_id"]) == "" {
			orgOnly = item
		}
	}
	if orgOnly.ID != "" {
		return orgOnly, true, nil
	}
	return model.Record{}, false, nil
}

func (s *LeavePolicyCoreService) listAllRecords(modelKey string, query model.Query) ([]model.Record, error) {
	if s == nil || s.models == nil {
		return nil, nil
	}
	if query.PageSize <= 0 {
		query.PageSize = model.MaxPageSize
	}
	page := 1
	out := make([]model.Record, 0)
	for {
		query.Page = page
		items, total, err := s.models.List(modelKey, query)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(items) == 0 || len(out) >= total || len(items) < query.PageSize {
			break
		}
		page++
	}
	return out, nil
}

func leaveRosterSlotHours(slot model.Record) float64 {
	start, end := rosterSlotWindow(slot)
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	hours := end.Sub(start).Hours() - float64(numberValue(slot.Values["break_minutes"]))/60
	if hours < 0 {
		return 0
	}
	return roundAttendanceHours(hours)
}

func parseWorkCalendarWeekdays(value any) map[time.Weekday]bool {
	var raw []any
	if err := json.Unmarshal([]byte(strings.TrimSpace(textValue(value))), &raw); err != nil {
		return nil
	}
	result := map[time.Weekday]bool{}
	for _, item := range raw {
		switch typed := item.(type) {
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "sun", "sunday":
				result[time.Sunday] = true
			case "mon", "monday":
				result[time.Monday] = true
			case "tue", "tues", "tuesday":
				result[time.Tuesday] = true
			case "wed", "wednesday":
				result[time.Wednesday] = true
			case "thu", "thur", "thurs", "thursday":
				result[time.Thursday] = true
			case "fri", "friday":
				result[time.Friday] = true
			case "sat", "saturday":
				result[time.Saturday] = true
			}
		case float64:
			weekday := int(typed)
			if weekday == 7 {
				weekday = 0
			}
			if weekday >= 0 && weekday <= 6 {
				result[time.Weekday(weekday)] = true
			}
		}
	}
	return result
}

func parseWorkCalendarDates(value any) map[string]bool {
	var raw []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(textValue(value))), &raw); err != nil {
		return map[string]bool{}
	}
	result := map[string]bool{}
	for _, item := range raw {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result[trimmed] = true
		}
	}
	return result
}

func (s *LeavePolicyCoreService) resolveLeaveApprovalPolicy(record model.Record) (ApprovalPolicyResolution, bool, error) {
	if s == nil || s.approvals == nil {
		return ApprovalPolicyResolution{}, false, nil
	}
	policyID := strings.TrimSpace(textValue(record.Values["leave_policy_id"]))
	if policyID == "" {
		return ApprovalPolicyResolution{}, false, nil
	}
	leavePolicy, err := s.models.Get("leave_policy", policyID)
	if err != nil {
		return ApprovalPolicyResolution{}, false, err
	}
	explicitPolicyID := strings.TrimSpace(textValue(leavePolicy.Values["approval_policy_id"]))
	if explicitPolicyID != "" {
		policyRecord, err := s.models.Get("approval_policy", explicitPolicyID)
		if err != nil {
			return ApprovalPolicyResolution{}, false, err
		}
		if !strings.EqualFold(textValue(policyRecord.Values["status"]), "active") {
			return ApprovalPolicyResolution{}, false, nil
		}
		stages, err := s.approvals.StagesForPolicy(policyRecord.ID)
		if err != nil {
			return ApprovalPolicyResolution{}, false, err
		}
		if len(stages) == 0 {
			stage, err := s.leaveApprovalFallbackStage(policyRecord)
			if err != nil {
				return ApprovalPolicyResolution{}, false, err
			}
			stages = []ApprovalPolicyStageResolution{stage}
		}
		for i := range stages {
			stages[i].PolicyID = policyRecord.ID
			stages[i].TotalStages = len(stages)
		}
		return ApprovalPolicyResolution{Policy: policyRecord, Stages: stages}, true, nil
	}
	return s.approvals.ResolveDocumentPolicy(s.syntheticLeaveDocument(record), "submit", leaveRequestWorkflowKey)
}

func (s *LeavePolicyCoreService) syntheticLeaveDocument(record model.Record) document.Record {
	payload := map[string]any{
		"operating_unit_id": textValue(record.Values["organization_unit_id"]),
		"department_id":     textValue(record.Values["department_id"]),
		"cost_center_id":    textValue(record.Values["cost_center_id"]),
	}
	return document.Record{
		Header: document.Header{
			ID:             record.ID,
			Type:           "leave_request",
			Status:         textValue(record.Values["approval_status"]),
			OrganizationID: textValue(record.Values["organization_id"]),
			LocationID:     textValue(record.Values["location_id"]),
			Metadata: map[string]any{
				"workflow_key":      leaveRequestWorkflowKey,
				"operating_unit_id": payload["operating_unit_id"],
				"department_id":     payload["department_id"],
				"cost_center_id":    payload["cost_center_id"],
			},
		},
		Body: document.Body{Payload: payload},
	}
}

func (s *LeavePolicyCoreService) leaveApprovalFallbackStage(policy model.Record) (ApprovalPolicyStageResolution, error) {
	stage := ApprovalPolicyStageResolution{
		PolicyID:               policy.ID,
		StageKey:               leaveFirstNonEmpty(strings.TrimSpace(textValue(policy.Values["default_stage_key"])), "approval"),
		Sequence:               1,
		TotalStages:            1,
		RequiredApproverCount:  1,
		RoutingMode:            strings.TrimSpace(textValue(policy.Values["routing_mode"])),
		AssignmentStrategy:     strings.TrimSpace(textValue(policy.Values["assignment_strategy"])),
		AssignmentMode:         strings.TrimSpace(textValue(policy.Values["assignment_mode"])),
		AssigneeRoleKey:        strings.TrimSpace(textValue(policy.Values["assignee_role_key"])),
		FallbackRoleKey:        strings.TrimSpace(textValue(policy.Values["fallback_role_key"])),
		ApproverGroupID:        strings.TrimSpace(textValue(policy.Values["approver_group_id"])),
		ExplicitUserID:         strings.TrimSpace(textValue(policy.Values["explicit_user_id"])),
		CandidateRoleKeys:      leaveSplitCSV(textValue(policy.Values["candidate_role_keys"])),
		DueAfterSeconds:        int(numberValue(policy.Values["due_after_seconds"])),
		EscalateAfterSeconds:   int(numberValue(policy.Values["escalate_after_seconds"])),
		RequiresDifferentActor: boolValue(policy.Values["requires_different_actor"]),
	}
	stage.AssigneeUserID, stage.CandidateUserIDs, _ = s.approvals.resolveStageUsers(stage)
	return stage, nil
}

func parseLeaveApprovalState(values map[string]any) leaveApprovalState {
	return leaveApprovalState{
		PolicyID:               textValue(values["approval_policy_id"]),
		StageKey:               textValue(values["approval_stage_key"]),
		StageSequence:          int(numberValue(values["approval_stage_sequence"])),
		StageTotal:             int(numberValue(values["approval_stage_total"])),
		RoutingMode:            textValue(values["approval_routing_mode"]),
		RequiredApproverCount:  max(1, int(numberValue(values["required_approver_count"]))),
		AssigneeUserID:         textValue(values["approver_user_id"]),
		CandidateUserIDs:       parseStringList(values["approval_candidate_user_ids_json"]),
		RecordedUserIDs:        parseStringList(values["approval_recorded_user_ids_json"]),
		RequiresDifferentActor: boolValue(values["approval_requires_different_actor"]),
	}
}

func applyLeaveApprovalState(values map[string]any, state leaveApprovalState) {
	values["approval_policy_id"] = state.PolicyID
	values["approval_stage_key"] = state.StageKey
	values["approval_stage_sequence"] = state.StageSequence
	values["approval_stage_total"] = state.StageTotal
	values["approval_routing_mode"] = state.RoutingMode
	values["required_approver_count"] = max(1, state.RequiredApproverCount)
	values["approver_user_id"] = state.AssigneeUserID
	values["approval_candidate_user_ids_json"] = marshalStringList(state.CandidateUserIDs)
	values["approval_recorded_user_ids_json"] = marshalStringList(state.RecordedUserIDs)
	values["approval_requires_different_actor"] = state.RequiresDifferentActor
}

func clearLeaveApprovalState(values map[string]any) {
	values["approval_policy_id"] = ""
	values["approval_stage_key"] = ""
	values["approval_stage_sequence"] = 0
	values["approval_stage_total"] = 0
	values["approval_routing_mode"] = ""
	values["required_approver_count"] = 0
	values["approval_candidate_user_ids_json"] = "[]"
	values["approval_requires_different_actor"] = false
}

func (s *LeavePolicyCoreService) authorizeLeaveApprovalAction(record model.Record, state leaveApprovalState, actorID string) error {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return shared.Forbidden("leave approval actor is required")
	}
	if state.RequiresDifferentActor && actorID == strings.TrimSpace(textValue(record.Values["self_service_actor_user_id"])) {
		return shared.Forbidden("leave request requires a different approver")
	}
	if state.StageKey == "" {
		return nil
	}
	if leaveContainsString(state.RecordedUserIDs, actorID) {
		return shared.Forbidden("leave request approval was already recorded for this actor")
	}
	if state.AssigneeUserID == "" && len(state.CandidateUserIDs) == 0 {
		return shared.Forbidden("leave request approval routing is unresolved")
	}
	if state.AssigneeUserID != "" && state.AssigneeUserID != actorID {
		if !leaveContainsString(state.CandidateUserIDs, actorID) {
			return shared.Forbidden("leave request approval is not assigned to this actor")
		}
	}
	if state.AssigneeUserID == "" && len(state.CandidateUserIDs) > 0 && !leaveContainsString(state.CandidateUserIDs, actorID) {
		return shared.Forbidden("leave request approval is not assigned to this actor")
	}
	return nil
}

func (s *LeavePolicyCoreService) nextLeaveApprovalState(record model.Record, current leaveApprovalState) (leaveApprovalState, bool, error) {
	resolution, ok, err := s.resolveLeaveApprovalPolicy(record)
	if err != nil || !ok {
		return leaveApprovalState{}, false, err
	}
	for _, stage := range resolution.Stages {
		if stage.Sequence != current.StageSequence+1 {
			continue
		}
		return leaveApprovalState{
			PolicyID:               resolution.Policy.ID,
			StageKey:               stage.StageKey,
			StageSequence:          stage.Sequence,
			StageTotal:             len(resolution.Stages),
			RoutingMode:            firstNonEmpty(stage.RoutingMode, stage.AssignmentStrategy),
			RequiredApproverCount:  max(1, stage.RequiredApproverCount),
			AssigneeUserID:         firstNonEmpty(stage.AssigneeUserID, stage.ExplicitUserID),
			CandidateUserIDs:       append([]string(nil), stage.CandidateUserIDs...),
			RecordedUserIDs:        nil,
			RequiresDifferentActor: stage.RequiresDifferentActor,
		}, true, nil
	}
	return leaveApprovalState{}, false, nil
}

func (s *LeavePolicyCoreService) finalizeApprovedLeave(record model.Record, actorID string) (model.Record, error) {
	consumptionIDs, err := s.consumeReservedBalance(record, actorID)
	if err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	values["approval_status"] = "approved"
	values["approved_days"] = numberValue(record.Values["requested_days"])
	values["consumption_entry_ids_json"] = marshalStringList(consumptionIDs)
	values["reservation_entry_ids_json"] = "[]"
	clearLeaveApprovalState(values)
	updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	s.syncLeaveRange(updated)
	return updated, nil
}

func applyLeaveAmendmentMetadata(values map[string]any, record model.Record, actorID, reason string) {
	values["amendment_count"] = int(numberValue(record.Values["amendment_count"])) + 1
	values["last_amended_at"] = time.Now().UTC().Format(time.RFC3339)
	values["last_amended_by"] = strings.TrimSpace(actorID)
	values["last_amendment_reason"] = strings.TrimSpace(reason)
}

func leaveContainsString(items []string, expected string) bool {
	expected = strings.TrimSpace(expected)
	for _, item := range items {
		if strings.TrimSpace(item) == expected {
			return true
		}
	}
	return false
}

func parseStringList(value any) []string {
	raw := strings.TrimSpace(textValue(value))
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		out := make([]string, 0, len(items))
		for _, item := range items {
			if strings.TrimSpace(item) != "" {
				out = append(out, strings.TrimSpace(item))
			}
		}
		return out
	}
	return nil
}

func leaveSplitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return uniqueStrings(strings.Split(value, ","))
}

func marshalStringList(items []string) string {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			filtered = append(filtered, strings.TrimSpace(item))
		}
	}
	sort.Strings(filtered)
	data, _ := json.Marshal(uniqueStrings(filtered))
	return string(data)
}

func mergeMaps(base, overlay map[string]any) map[string]any {
	out := cloneMap(base)
	for key, value := range overlay {
		out[key] = value
	}
	return out
}

func leaveFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *LeavePolicyCoreService) BalanceSummaryForUser(userID string) ([]map[string]any, error) {
	items, err := s.ListBalancesForUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		summary := map[string]any{
			"id":                         item.ID,
			"employee_id":                textValue(item.Values["employee_id"]),
			"leave_policy_id":            textValue(item.Values["leave_policy_id"]),
			"current_balance_days":       numberValue(item.Values["current_balance_days"]),
			"reserved_days":              numberValue(item.Values["reserved_days"]),
			"available_days":             numberValue(item.Values["available_days"]),
			"carry_forward_balance_days": numberValue(item.Values["carry_forward_balance_days"]),
			"carry_forward_expiry_date":  textValue(item.Values["carry_forward_expiry_date"]),
			"status":                     textValue(item.Values["status"]),
		}
		if policyID := textValue(item.Values["leave_policy_id"]); policyID != "" {
			if policy, err := s.models.Get("leave_policy", policyID); err == nil {
				summary["leave_policy_code"] = textValue(policy.Values["code"])
				summary["leave_policy_name"] = textValue(policy.Values["name"])
				summary["paid_leave"] = boolValue(policy.Values["paid_leave"])
			}
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *LeavePolicyCoreService) RequestSummaryForUser(userID string, requestID string) (map[string]any, error) {
	record, err := s.GetRequestForUser(userID, requestID)
	if err != nil {
		return nil, err
	}
	return s.leaveRequestSummary(record), nil
}

func (s *LeavePolicyCoreService) RequestSummariesForUser(userID string, filters map[string]string) ([]map[string]any, error) {
	items, err := s.ListRequestsForUser(userID, filters)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, s.leaveRequestSummary(item))
	}
	return out, nil
}

func (s *LeavePolicyCoreService) PendingRequestSummariesForApprover(actorID string) ([]map[string]any, error) {
	if s == nil || s.models == nil {
		return nil, shared.Validation("leave policy service is not available")
	}
	items, _, err := s.models.List("leave_request", model.Query{
		Filters:  map[string]string{"approval_status": "submitted", "status": "active"},
		SortKey:  "start_date",
		Desc:     false,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		state := parseLeaveApprovalState(item.Values)
		if state.StageKey != "" && s.authorizeLeaveApprovalAction(item, state, actorID) != nil {
			continue
		}
		out = append(out, s.leaveRequestSummary(item))
	}
	return out, nil
}

func (s *LeavePolicyCoreService) InboxRequestSummariesForActor(actorID string, filters map[string]string) ([]map[string]any, error) {
	if s == nil || s.models == nil {
		return nil, shared.Validation("leave policy service is not available")
	}
	bucket := strings.ToLower(strings.TrimSpace(filters["bucket"]))
	if bucket == "" {
		bucket = "actionable"
	}
	statusFilter := strings.ToLower(strings.TrimSpace(filters["status"]))
	employeeFilter := strings.TrimSpace(filters["employee_id"])
	items, _, err := s.models.List("leave_request", model.Query{
		Filters:  map[string]string{"status": "active"},
		SortKey:  "start_date",
		Desc:     false,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if employeeFilter != "" && textValue(item.Values["employee_id"]) != employeeFilter {
			continue
		}
		if statusFilter != "" && strings.ToLower(textValue(item.Values["approval_status"])) != statusFilter {
			continue
		}
		if !leaveRequestVisibleToActor(item, actorID) {
			continue
		}
		summary := s.leaveRequestSummaryForActor(item, actorID)
		switch bucket {
		case "approved":
			if strings.ToLower(textValue(item.Values["approval_status"])) != "approved" {
				continue
			}
		case "all":
		default:
			if bucket == "actionable" && !boolValue(summary["is_actionable"]) {
				continue
			}
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *LeavePolicyCoreService) RequestSummaryForApprover(requestID, actorID string) (map[string]any, error) {
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(textValue(record.Values["status"]), "active") {
		return nil, shared.NotFound("leave request was not found")
	}
	if err := s.authorizeLeaveApprovalAction(record, parseLeaveApprovalState(record.Values), actorID); err != nil {
		return nil, err
	}
	return s.leaveRequestSummary(record), nil
}

func (s *LeavePolicyCoreService) RequestSummaryForInboxActor(requestID, actorID string) (map[string]any, error) {
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(textValue(record.Values["status"]), "active") {
		return nil, shared.NotFound("leave request was not found")
	}
	if !leaveRequestVisibleToActor(record, actorID) {
		return nil, shared.Forbidden("leave request is not allowed")
	}
	return s.leaveRequestSummaryForActor(record, actorID), nil
}

func (s *LeavePolicyCoreService) RequestSummary(requestID string) (map[string]any, error) {
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(textValue(record.Values["status"]), "active") {
		return nil, shared.NotFound("leave request was not found")
	}
	return s.leaveRequestSummary(record), nil
}

func (s *LeavePolicyCoreService) leaveRequestSummary(record model.Record) map[string]any {
	return s.leaveRequestSummaryForActor(record, "")
}

func (s *LeavePolicyCoreService) leaveRequestSummaryForActor(record model.Record, actorID string) map[string]any {
	state := parseLeaveApprovalState(record.Values)
	cutoffBlocked, cutoffReason, _ := s.payrollCutoffForRequest(record, nil)
	allowedActions := s.leaveAllowedActionsForActor(record, state, actorID, cutoffBlocked)
	summary := map[string]any{
		"id":                          record.ID,
		"employee_id":                 textValue(record.Values["employee_id"]),
		"organization_id":             textValue(record.Values["organization_id"]),
		"location_id":                 textValue(record.Values["location_id"]),
		"organization_unit_id":        textValue(record.Values["organization_unit_id"]),
		"department_id":               textValue(record.Values["department_id"]),
		"cost_center_id":              textValue(record.Values["cost_center_id"]),
		"leave_policy_id":             textValue(record.Values["leave_policy_id"]),
		"employee_leave_profile_id":   textValue(record.Values["employee_leave_profile_id"]),
		"absence_code_id":             textValue(record.Values["absence_code_id"]),
		"start_date":                  textValue(record.Values["start_date"]),
		"end_date":                    textValue(record.Values["end_date"]),
		"request_unit":                textValue(record.Values["request_unit"]),
		"requested_days":              numberValue(record.Values["requested_days"]),
		"requested_hours":             numberValue(record.Values["requested_hours"]),
		"half_day_session":            textValue(record.Values["half_day_session"]),
		"count_basis":                 textValue(record.Values["count_basis"]),
		"counted_dates":               parseStringList(record.Values["counted_dates_json"]),
		"counted_work_calendar_id":    textValue(record.Values["counted_work_calendar_id"]),
		"counted_roster_slot_ids":     parseStringList(record.Values["counted_roster_slot_ids_json"]),
		"approval_status":             textValue(record.Values["approval_status"]),
		"approved_days":               numberValue(record.Values["approved_days"]),
		"approval_stage_key":          textValue(record.Values["approval_stage_key"]),
		"approval_stage_sequence":     int(numberValue(record.Values["approval_stage_sequence"])),
		"approval_stage_total":        int(numberValue(record.Values["approval_stage_total"])),
		"approval_routing_mode":       textValue(record.Values["approval_routing_mode"]),
		"required_approver_count":     int(numberValue(record.Values["required_approver_count"])),
		"approval_policy_id":          textValue(record.Values["approval_policy_id"]),
		"approver_user_id":            textValue(record.Values["approver_user_id"]),
		"approval_candidate_user_ids": parseStringList(record.Values["approval_candidate_user_ids_json"]),
		"approval_recorded_user_ids":  parseStringList(record.Values["approval_recorded_user_ids_json"]),
		"balance_account_id":          textValue(record.Values["balance_account_id"]),
		"request_source":              textValue(record.Values["request_source"]),
		"self_service_actor_user_id":  textValue(record.Values["self_service_actor_user_id"]),
		"notes":                       textValue(record.Values["notes"]),
		"status":                      textValue(record.Values["status"]),
		"amendment_count":             int(numberValue(record.Values["amendment_count"])),
		"last_amended_at":             textValue(record.Values["last_amended_at"]),
		"last_amended_by":             textValue(record.Values["last_amended_by"]),
		"last_amendment_reason":       textValue(record.Values["last_amendment_reason"]),
		"status_label":                leaveApprovalStatusLabel(textValue(record.Values["approval_status"])),
		"recorded_approver_count":     len(state.RecordedUserIDs),
		"stage_progress_label":        leaveStageProgressLabel(state),
		"allowed_actions":             allowedActions,
		"is_actionable":               leaveContainsString(allowedActions, "approve") || leaveContainsString(allowedActions, "reject"),
		"can_cancel":                  leaveContainsString(allowedActions, "cancel"),
		"can_amend":                   leaveContainsString(allowedActions, "amend"),
		"payroll_cutoff_blocked":      cutoffBlocked,
		"payroll_cutoff_reason":       cutoffReason,
	}
	summary["count_explanation"] = leaveCountExplanation(record)
	if employee, err := s.models.Get("employee_profile", textValue(record.Values["employee_id"])); err == nil {
		summary["employee_code"] = textValue(employee.Values["employee_code"])
	}
	if policyID := textValue(record.Values["leave_policy_id"]); policyID != "" {
		if policy, err := s.models.Get("leave_policy", policyID); err == nil {
			summary["leave_policy_code"] = textValue(policy.Values["code"])
			summary["leave_policy_name"] = textValue(policy.Values["name"])
		}
	}
	if absenceCodeID := textValue(record.Values["absence_code_id"]); absenceCodeID != "" {
		if absenceCode, err := s.models.Get("absence_code", absenceCodeID); err == nil {
			summary["absence_code_code"] = textValue(absenceCode.Values["code"])
			summary["absence_code_name"] = textValue(absenceCode.Values["name"])
		}
	}
	if accountID := textValue(record.Values["balance_account_id"]); accountID != "" {
		if account, err := s.models.Get("leave_balance_account", accountID); err == nil {
			summary["balance_snapshot"] = map[string]any{
				"current_balance_days":       numberValue(account.Values["current_balance_days"]),
				"reserved_days":              numberValue(account.Values["reserved_days"]),
				"available_days":             numberValue(account.Values["available_days"]),
				"carry_forward_balance_days": numberValue(account.Values["carry_forward_balance_days"]),
				"carry_forward_expiry_date":  textValue(account.Values["carry_forward_expiry_date"]),
			}
		}
	}
	return summary
}

func (s *LeavePolicyCoreService) balanceEntrySummaries(accountID string) ([]map[string]any, error) {
	entries, _, err := s.models.List("leave_balance_entry", model.Query{
		Filters:  map[string]string{"balance_account_id": strings.TrimSpace(accountID)},
		SortKey:  "effective_date",
		Desc:     true,
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if !strings.EqualFold(textValue(entry.Values["status"]), "active") {
			continue
		}
		out = append(out, map[string]any{
			"id":                       entry.ID,
			"entry_type":               textValue(entry.Values["entry_type"]),
			"days":                     numberValue(entry.Values["days"]),
			"carry_forward_days_delta": numberValue(entry.Values["carry_forward_days_delta"]),
			"reversal_of_entry_id":     textValue(entry.Values["reversal_of_entry_id"]),
			"effective_date":           textValue(entry.Values["effective_date"]),
			"leave_request_id":         textValue(entry.Values["leave_request_id"]),
			"status":                   textValue(entry.Values["status"]),
		})
	}
	return out, nil
}

func leaveCountExplanation(record model.Record) string {
	basis := strings.ToLower(textValue(record.Values["count_basis"]))
	days := numberValue(record.Values["requested_days"])
	switch basis {
	case "roster":
		return fmt.Sprintf("%.1f working days from published roster", days)
	case "calendar":
		return fmt.Sprintf("%.1f working days from work calendar", days)
	case "legacy":
		return fmt.Sprintf("%.1f days from date range fallback", days)
	default:
		return fmt.Sprintf("%.1f requested days", days)
	}
}

func (s *LeavePolicyCoreService) leaveAllowedActionsForActor(record model.Record, state leaveApprovalState, actorID string, cutoffBlocked bool) []string {
	approvalStatus := strings.ToLower(textValue(record.Values["approval_status"]))
	actorID = strings.TrimSpace(actorID)
	actions := make([]string, 0, 3)
	if actorID != "" && strings.TrimSpace(textValue(record.Values["self_service_actor_user_id"])) == actorID {
		switch approvalStatus {
		case "draft":
			actions = append(actions, "edit", "submit", "cancel")
		case "submitted":
			actions = append(actions, "cancel", "amend")
		}
	}
	if actorID != "" {
		if approvalStatus == "submitted" {
			allowed := false
			if state.StageKey == "" {
				allowed = true
			} else if state.RequiresDifferentActor && actorID == strings.TrimSpace(textValue(record.Values["self_service_actor_user_id"])) {
				allowed = false
			} else if leaveContainsString(state.RecordedUserIDs, actorID) {
				allowed = false
			} else if state.AssigneeUserID != "" && state.AssigneeUserID == actorID {
				allowed = true
			} else if leaveContainsString(state.CandidateUserIDs, actorID) {
				allowed = true
			}
			if allowed {
				actions = append(actions, "approve", "reject", "amend")
			}
		}
		if approvalStatus == "approved" && !cutoffBlocked && leaveRequestVisibleToActor(record, actorID) {
			actions = append(actions, "cancel", "amend")
		}
	}
	return uniqueStrings(actions)
}

func leaveRequestVisibleToActor(record model.Record, actorID string) bool {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return false
	}
	if actorID == strings.TrimSpace(textValue(record.Values["approver_user_id"])) {
		return true
	}
	if leaveContainsString(parseStringList(record.Values["approval_candidate_user_ids_json"]), actorID) {
		return true
	}
	if leaveContainsString(parseStringList(record.Values["approval_recorded_user_ids_json"]), actorID) {
		return true
	}
	return false
}

func (s *LeavePolicyCoreService) payrollCutoffForRequest(record model.Record, proposedValues map[string]any) (bool, string, error) {
	if s == nil || s.models == nil || s.documents == nil {
		return false, "", nil
	}
	ranges := leaveRequestDateRanges(record, proposedValues)
	if len(ranges) == 0 {
		return false, "", nil
	}
	organizationID := leaveFirstNonEmpty(textValue(record.Values["organization_id"]), textValue(proposedValues["organization_id"]))
	locationID := leaveFirstNonEmpty(textValue(record.Values["location_id"]), textValue(proposedValues["location_id"]))
	periods, _, err := s.models.List("payroll_period", model.Query{
		SortKey:  "start_date",
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		return false, "", err
	}
	for _, period := range periods {
		if !strings.EqualFold(textValue(period.Values["status"]), "active") && textValue(period.Values["status"]) != "" {
			normalized := strings.ToLower(textValue(period.Values["status"]))
			if normalized != "open" && normalized != "locked" && normalized != "processed" {
				continue
			}
		}
		if organizationID != "" && textValue(period.Values["organization_id"]) != organizationID {
			continue
		}
		if locationID != "" && textValue(period.Values["location_id"]) != "" && locationID != "" && textValue(period.Values["location_id"]) != locationID {
			continue
		}
		if !leavePeriodOverlapsRanges(period, ranges) {
			continue
		}
		for _, doc := range s.documents.List() {
			if doc.Header.Type != "payroll_run" || !strings.EqualFold(doc.Header.Status, "processed") {
				continue
			}
			if textValue(doc.Body.Payload["payroll_period_id"]) != period.ID {
				continue
			}
			if organizationID != "" && doc.Header.OrganizationID != organizationID {
				continue
			}
			if locationID != "" && doc.Header.LocationID != "" && doc.Header.LocationID != locationID {
				continue
			}
			return true, fmt.Sprintf("leave request is locked because payroll run %s is already processed for overlapping period %s", leaveFirstNonEmpty(doc.Header.Number, doc.Header.ID), leaveFirstNonEmpty(textValue(period.Values["code"]), period.ID)), nil
		}
	}
	return false, "", nil
}

func leaveRequestDateRanges(record model.Record, proposedValues map[string]any) [][2]string {
	ranges := make([][2]string, 0, 2)
	appendRange := func(startDate, endDate string) {
		startDate = strings.TrimSpace(startDate)
		endDate = strings.TrimSpace(endDate)
		if startDate == "" {
			return
		}
		if endDate == "" {
			endDate = startDate
		}
		ranges = append(ranges, [2]string{startDate, endDate})
	}
	appendRange(textValue(record.Values["start_date"]), textValue(record.Values["end_date"]))
	if proposedValues != nil {
		startDate := strings.TrimSpace(textValue(proposedValues["start_date"]))
		endDate := strings.TrimSpace(textValue(proposedValues["end_date"]))
		if startDate != "" {
			candidate := [2]string{startDate, leaveFirstNonEmpty(endDate, startDate)}
			duplicate := false
			for _, existing := range ranges {
				if existing == candidate {
					duplicate = true
					break
				}
			}
			if !duplicate {
				ranges = append(ranges, candidate)
			}
		}
	}
	return ranges
}

func leavePeriodOverlapsRanges(period model.Record, ranges [][2]string) bool {
	periodStart := strings.TrimSpace(textValue(period.Values["start_date"]))
	periodEnd := strings.TrimSpace(textValue(period.Values["end_date"]))
	if periodStart == "" {
		return false
	}
	if periodEnd == "" {
		periodEnd = periodStart
	}
	for _, item := range ranges {
		if item[0] == "" {
			continue
		}
		if item[1] == "" {
			item[1] = item[0]
		}
		if periodEnd < item[0] || item[1] < periodStart {
			continue
		}
		return true
	}
	return false
}

func leaveApprovalStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "draft":
		return "Draft"
	case "submitted":
		return "Pending approval"
	case "approved":
		return "Approved"
	case "rejected":
		return "Rejected"
	case "cancelled":
		return "Cancelled"
	default:
		return strings.TrimSpace(status)
	}
}

func leaveStageProgressLabel(state leaveApprovalState) string {
	if state.StageKey == "" || state.StageTotal <= 0 {
		return ""
	}
	return fmt.Sprintf("Stage %d of %d", state.StageSequence, state.StageTotal)
}

func (s *LeavePolicyCoreService) DebugDescribeRequest(requestID string) (string, error) {
	record, err := s.models.Get("leave_request", requestID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s:%s", record.ID, textValue(record.Values["approval_status"]), textValue(record.Values["leave_policy_id"])), nil
}
