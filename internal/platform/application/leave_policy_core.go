package application

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
)

type LeavePolicyCoreService struct {
	models     *model.Service
	workforce  *EmployeeWorkforceCoreService
	attendance *WorkforceAttendanceCoreService
}

func NewLeavePolicyCoreService(models *model.Service, workforce *EmployeeWorkforceCoreService, attendance *WorkforceAttendanceCoreService) *LeavePolicyCoreService {
	return &LeavePolicyCoreService{models: models, workforce: workforce, attendance: attendance}
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
	if strings.EqualFold(textValue(run.Values["run_status"]), "processed") {
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
		days := 0.0
		switch strings.ToLower(runMode) {
		case "monthly_accrual":
			if !strings.EqualFold(textValue(rule.Values["grant_mode"]), "monthly_accrual") {
				continue
			}
			days = numberValue(rule.Values["monthly_accrual_days"])
		default:
			if !strings.EqualFold(textValue(rule.Values["grant_mode"]), "annual_grant") {
				continue
			}
			days = numberValue(rule.Values["annual_entitlement_days"])
		}
		if days <= 0 {
			continue
		}
		account, err := s.ensureBalanceAccount(profile, policy, actorID)
		if err != nil {
			return model.Record{}, err
		}
		if _, err := s.createBalanceEntry(account.ID, profile, policy, actorID, map[string]any{
			"entry_type":      runMode,
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

	values := cloneMap(run.Values)
	values["run_status"] = "processed"
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
	updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	s.syncLeaveRange(updated)
	return updated, nil
}

func (s *LeavePolicyCoreService) ApproveLeaveRequest(requestID, actorID string) (model.Record, error) {
	record, err := s.models.Get("leave_request", strings.TrimSpace(requestID))
	if err != nil {
		return model.Record{}, err
	}
	if !strings.EqualFold(textValue(record.Values["approval_status"]), "submitted") {
		return model.Record{}, shared.Validation("leave request is not submitted")
	}
	consumptionIDs, err := s.consumeReservedBalance(record, actorID)
	if err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	values["approval_status"] = "approved"
	values["approved_days"] = numberValue(record.Values["requested_days"])
	values["consumption_entry_ids_json"] = marshalStringList(consumptionIDs)
	values["reservation_entry_ids_json"] = "[]"
	updated, err := s.models.Update("leave_request", record.ID, actorID, values, record.Version)
	if err != nil {
		return model.Record{}, err
	}
	s.syncLeaveRange(updated)
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
	if err := s.releaseReservedBalance(record, actorID); err != nil {
		return model.Record{}, err
	}
	values := cloneMap(record.Values)
	values["approval_status"] = "rejected"
	values["reservation_entry_ids_json"] = "[]"
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
		requestedDays := numberValue(record.Values["requested_days"])
		if requestedDays <= 0 {
			return model.Record{}, shared.Validation("requested_days must be greater than zero")
		}
		if numberValue(account.Values["available_days"]) < requestedDays {
			return model.Record{}, shared.Validation("leave balance is insufficient")
		}
		entry, err := s.createBalanceEntry(account.ID, model.Record{}, policy, actorID, map[string]any{
			"entry_type":        "reservation",
			"days":              requestedDays,
			"effective_date":    textValue(record.Values["start_date"]),
			"leave_request_id":  record.ID,
			"employee_id":       textValue(record.Values["employee_id"]),
			"leave_policy_id":   policy.ID,
			"status":            "active",
		})
		if err != nil {
			return model.Record{}, err
		}
		reservationIDs = append(reservationIDs, entry.ID)
	}
	values := cloneMap(record.Values)
	values["approval_status"] = "submitted"
	values["reservation_entry_ids_json"] = marshalStringList(reservationIDs)
	return s.models.Update("leave_request", record.ID, actorID, values, record.Version)
}

func (s *LeavePolicyCoreService) prepareLeaveRequestValues(employee, assignment, current model.Record, payload map[string]any, actorID string) (map[string]any, model.Record, model.Record, model.Record, model.Record, error) {
	values := cloneMap(current.Values)
	if values == nil {
		values = map[string]any{}
	}
	for key, value := range cloneMap(payload) {
		switch key {
		case "employee_id", "organization_id", "location_id", "request_source", "self_service_actor_user_id", "approval_status", "reservation_entry_ids_json", "consumption_entry_ids_json":
			continue
		default:
			values[key] = value
		}
	}
	policy, profile, absenceCode, account, err := s.resolveApplicableLeaveContext(employee, assignment, values, actorID)
	if err != nil {
		return nil, model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
	}
	startDate := strings.TrimSpace(textValue(values["start_date"]))
	endDate := strings.TrimSpace(textValue(values["end_date"]))
	if startDate == "" || endDate == "" {
		return nil, model.Record{}, model.Record{}, model.Record{}, model.Record{}, shared.Validation("start_date and end_date are required")
	}
	requestUnit, requestedDays, halfDaySession, err := deriveRequestedDays(policy, values)
	if err != nil {
		return nil, model.Record{}, model.Record{}, model.Record{}, model.Record{}, err
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
	values["absence_code_id"] = absenceCode.ID
	values["leave_policy_id"] = policy.ID
	values["employee_leave_profile_id"] = profile.ID
	values["request_unit"] = requestUnit
	values["requested_days"] = requestedDays
	values["half_day_session"] = halfDaySession
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
	if values["status"] == nil || textValue(values["status"]) == "" {
		values["status"] = "active"
	}
	return values, policy, profile, absenceCode, account, nil
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
		"carry_forward_expiry_date":  textValue(profile.Values["carry_forward_expiry_date"]),
		"status":                     "active",
	})
	if err != nil {
		return model.Record{}, err
	}
	opening := numberValue(profile.Values["opening_balance_days"])
	if opening > 0 {
		if _, err := s.createBalanceEntry(account.ID, profile, policy, actorID, map[string]any{
			"entry_type":       "opening",
			"days":             opening,
			"effective_date":   time.Now().UTC().Format("2006-01-02"),
			"employee_id":      textValue(profile.Values["employee_id"]),
			"leave_policy_id":  policy.ID,
			"status":           "active",
		}); err != nil {
			return model.Record{}, err
		}
		return s.models.Get("leave_balance_account", account.ID)
	}
	return account, nil
}

func (s *LeavePolicyCoreService) createBalanceEntry(accountID string, profile, policy model.Record, actorID string, values map[string]any) (model.Record, error) {
	entry, err := s.models.Create("leave_balance_entry", actorID, mergeMaps(map[string]any{
		"balance_account_id": accountID,
		"employee_leave_profile_id": profile.ID,
		"status": "active",
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
	var lastAccrual string
	for _, entry := range entries {
		if !strings.EqualFold(textValue(entry.Values["status"]), "active") {
			continue
		}
		days := numberValue(entry.Values["days"])
		switch strings.ToLower(textValue(entry.Values["entry_type"])) {
		case "opening", "annual_grant", "monthly_accrual", "carry_forward", "adjustment":
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
	available := current - reserved
	values := cloneMap(account.Values)
	values["current_balance_days"] = roundAttendanceHours(current)
	values["reserved_days"] = roundAttendanceHours(reserved)
	values["available_days"] = roundAttendanceHours(available)
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
	entry, err := s.createBalanceEntry(account.ID, model.Record{}, policy, actorID, map[string]any{
		"entry_type":       "consumption",
		"days":             numberValue(record.Values["requested_days"]),
		"effective_date":   textValue(record.Values["start_date"]),
		"leave_request_id": record.ID,
		"employee_id":      textValue(record.Values["employee_id"]),
		"leave_policy_id":  policy.ID,
		"status":           "active",
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
		values := cloneMap(entry.Values)
		values["status"] = "inactive"
		if _, err := s.models.Update("leave_balance_entry", entry.ID, actorID, values, entry.Version); err != nil {
			return err
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

func (s *LeavePolicyCoreService) syncLeaveRange(record model.Record) {
	if s == nil || s.attendance == nil {
		return
	}
	start := parseDateOnly(record.Values["start_date"])
	end := parseDateOnly(record.Values["end_date"])
	if start.IsZero() {
		return
	}
	if end.IsZero() || end.Before(start) {
		end = start
	}
	for day := start; !day.After(end); day = day.Add(24 * time.Hour) {
		_, _ = s.attendance.SyncAttendanceDay(textValue(record.Values["employee_id"]), day)
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
		earliest := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, noticeDays)
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

func deriveRequestedDays(policy model.Record, values map[string]any) (string, float64, string, error) {
	start := parseDateOnly(values["start_date"])
	end := parseDateOnly(values["end_date"])
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return "", 0, "", shared.Validation("leave request date range is invalid")
	}
	requestUnit := strings.ToLower(strings.TrimSpace(textValue(values["request_unit"])))
	if requestUnit == "" {
		requestUnit = "day"
	}
	switch requestUnit {
	case "half_day":
		if !boolValue(policy.Values["allows_half_day"]) {
			return "", 0, "", shared.Validation("leave policy does not allow half day requests")
		}
		if start.Format("2006-01-02") != end.Format("2006-01-02") {
			return "", 0, "", shared.Validation("half day leave must be requested for a single day")
		}
		session := strings.ToLower(strings.TrimSpace(textValue(values["half_day_session"])))
		if session != "morning" && session != "afternoon" {
			return "", 0, "", shared.Validation("half_day_session must be morning or afternoon")
		}
		return requestUnit, 0.5, session, nil
	default:
		days := 1.0 + end.Sub(start).Hours()/24
		days = roundAttendanceHours(days)
		if days <= 0 {
			return "", 0, "", shared.Validation("requested_days must be greater than zero")
		}
		return "day", days, "", nil
	}
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

func marshalStringList(items []string) string {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			filtered = append(filtered, strings.TrimSpace(item))
		}
	}
	sort.Strings(filtered)
	data, _ := json.Marshal(filtered)
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
			"id":                   item.ID,
			"employee_id":          textValue(item.Values["employee_id"]),
			"leave_policy_id":      textValue(item.Values["leave_policy_id"]),
			"current_balance_days": numberValue(item.Values["current_balance_days"]),
			"reserved_days":        numberValue(item.Values["reserved_days"]),
			"available_days":       numberValue(item.Values["available_days"]),
			"status":               textValue(item.Values["status"]),
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

func (s *LeavePolicyCoreService) leaveRequestSummary(record model.Record) map[string]any {
	return map[string]any{
		"id":                     record.ID,
		"employee_id":            textValue(record.Values["employee_id"]),
		"leave_policy_id":        textValue(record.Values["leave_policy_id"]),
		"employee_leave_profile_id": textValue(record.Values["employee_leave_profile_id"]),
		"absence_code_id":        textValue(record.Values["absence_code_id"]),
		"start_date":             textValue(record.Values["start_date"]),
		"end_date":               textValue(record.Values["end_date"]),
		"request_unit":           textValue(record.Values["request_unit"]),
		"requested_days":         numberValue(record.Values["requested_days"]),
		"half_day_session":       textValue(record.Values["half_day_session"]),
		"approval_status":        textValue(record.Values["approval_status"]),
		"approved_days":          numberValue(record.Values["approved_days"]),
		"balance_account_id":     textValue(record.Values["balance_account_id"]),
		"request_source":         textValue(record.Values["request_source"]),
		"self_service_actor_user_id": textValue(record.Values["self_service_actor_user_id"]),
		"notes":                  textValue(record.Values["notes"]),
		"status":                 textValue(record.Values["status"]),
	}
}

func (s *LeavePolicyCoreService) DebugDescribeRequest(requestID string) (string, error) {
	record, err := s.models.Get("leave_request", requestID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s:%s", record.ID, textValue(record.Values["approval_status"]), textValue(record.Values["leave_policy_id"])), nil
}
