package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"orbyte/internal/platform/identity"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
)

func TestUILeaveSelfServiceRoutesAreEmployeeScoped(t *testing.T) {
	h := newTestHarness(t)
	for _, permissionKey := range []string{"leave.self_service.read", "leave.self_service.write", "attendance.read", "attendance.approve", "attendance.reject"} {
		if err := h.ident.UpsertPermission(identity.Permission{Key: permissionKey, Module: "leave_policy_core", Action: "use", Resource: "leave_self_service"}); err != nil {
			t.Fatalf("upsert permission %s: %v", permissionKey, err)
		}
		if err := h.ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permissionKey}); err != nil {
			t.Fatalf("grant permission %s: %v", permissionKey, err)
		}
	}
	if err := h.modules.Register(module.Manifest{
		Key:          "leave_policy_core",
		Name:         "Leave Policy Core",
		Version:      "1.0.0",
		DomainFamily: "business",
		SelfService: module.SelfServiceDefinition{
			APIs: []module.SelfServiceAPIDefinition{
				{Key: "leave.self_service.balances.list", Title: "List Leave Balances", Method: "GET", RoutePath: "/ui/self-service/leave/balances", HandlerKind: "custom", RequiredPermissions: []string{"leave.self_service.read"}, AudienceKinds: []string{"employee"}},
				{Key: "leave.self_service.requests.list", Title: "List Leave Requests", Method: "GET", RoutePath: "/ui/self-service/leave/requests", HandlerKind: "custom", RequiredPermissions: []string{"leave.self_service.read"}, AudienceKinds: []string{"employee"}},
				{Key: "leave.self_service.requests.create", Title: "Create Leave Request", Method: "POST", RoutePath: "/ui/self-service/leave/requests", HandlerKind: "custom", RequiredPermissions: []string{"leave.self_service.write"}, AudienceKinds: []string{"employee"}},
			},
		},
	}, "system"); err != nil {
		t.Fatalf("register leave self-service manifest: %v", err)
	}
	registerLeaveSelfServiceTestModels(t, h.models)

	employee, err := h.models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_leave",
		"user_id":           "user_admin",
		"employee_code":     "EMP-SELF-1",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if _, err := h.models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":    employee.ID,
		"organization_id": "org_default",
		"location_id":    "loc_hq",
		"effective_from": "2000-01-01",
		"status":         "active",
	}); err != nil {
		t.Fatalf("create employee assignment: %v", err)
	}
	absenceCode, err := h.models.Create("absence_code", "user_admin", map[string]any{
		"code":       "ANNUAL",
		"name":       "Annual Leave",
		"category":   "leave",
		"status":     "active",
	})
	if err != nil {
		t.Fatalf("create absence code: %v", err)
	}
	policyRecord, err := h.models.Create("leave_policy", "user_admin", map[string]any{
		"code":             "LP-SELF",
		"name":             "Self Service Policy",
		"absence_code_id":  absenceCode.ID,
		"paid_leave":       false,
		"requires_balance": true,
		"allows_half_day":  true,
		"status":           "active",
	})
	if err != nil {
		t.Fatalf("create leave policy: %v", err)
	}
	if _, err := h.models.Create("leave_entitlement_rule", "user_admin", map[string]any{
		"leave_policy_id":         policyRecord.ID,
		"grant_mode":              "annual_grant",
		"annual_entitlement_days": 8.0,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create entitlement rule: %v", err)
	}
	if _, err := h.models.Create("employee_leave_profile", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"leave_policy_id": policyRecord.ID,
		"effective_from":  "2000-01-01",
		"status":          "active",
	}); err != nil {
		t.Fatalf("create leave profile: %v", err)
	}
	if _, err := h.models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":           "ACC-SELF",
		"name":           "Self Grant",
		"leave_policy_id": policyRecord.ID,
		"run_mode":       "annual_grant",
		"effective_date": "2099-01-01",
		"status":         "active",
		"run_status":     "draft",
	}); err != nil {
		t.Fatalf("create accrual run: %v", err)
	}

	createBody, _ := json.Marshal(map[string]any{
		"leave_policy_id": policyRecord.ID,
		"start_date":      "2099-02-01",
		"end_date":        "2099-02-01",
		"half_day_session": "morning",
		"request_unit":    "half_day",
	})
	create := h.request(http.MethodPost, "/ui/self-service/leave/requests", createBody, true)
	if create.Code != http.StatusOK {
		t.Fatalf("expected create leave request to succeed, got %d body=%s", create.Code, create.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(create.Body.Bytes(), &created)
	record := created["record"].(map[string]any)
	if record["employee_id"] != employee.ID {
		t.Fatalf("expected self-service leave request to bind employee %s, got %+v", employee.ID, record)
	}

	list := h.request(http.MethodGet, "/ui/self-service/leave/requests", nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("expected list leave requests to succeed, got %d body=%s", list.Code, list.Body.String())
	}
	balances := h.request(http.MethodGet, "/ui/self-service/leave/balances", nil, true)
	if balances.Code != http.StatusOK {
		t.Fatalf("expected list leave balances to succeed, got %d body=%s", balances.Code, balances.Body.String())
	}
}

func TestUIAttendanceLeaveApprovalRoutesRequireAssignment(t *testing.T) {
	h := newTestHarness(t)
	for _, permissionKey := range []string{"leave.self_service.read", "leave.self_service.write", "attendance.read", "attendance.approve", "attendance.reject"} {
		if err := h.ident.UpsertPermission(identity.Permission{Key: permissionKey, Module: "leave_policy_core", Action: "use", Resource: "leave_self_service"}); err != nil {
			t.Fatalf("upsert permission %s: %v", permissionKey, err)
		}
		if err := h.ident.GrantRolePermission(identity.RolePermission{RoleID: "role_admin", PermissionKey: permissionKey}); err != nil {
			t.Fatalf("grant permission %s: %v", permissionKey, err)
		}
	}
	registerLeaveSelfServiceTestModels(t, h.models)
	employee, err := h.models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_leave",
		"user_id":           "user_admin",
		"employee_code":     "EMP-SELF-1",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if _, err := h.models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_hq",
		"department_id":        "dept_leave",
		"effective_from":       "2000-01-01",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create employee assignment: %v", err)
	}
	absenceCode, _ := h.models.Create("absence_code", "user_admin", map[string]any{"code": "ANNUAL", "name": "Annual Leave", "category": "leave", "status": "active"})
	policyRecord, _ := h.models.Create("leave_policy", "user_admin", map[string]any{"code": "LP-SELF", "name": "Self Service Policy", "absence_code_id": absenceCode.ID, "paid_leave": false, "requires_balance": false, "allows_half_day": true, "status": "active"})
	approvalPolicy, _ := h.models.Create("approval_policy", "user_admin", map[string]any{"code": "LEAVE-DEPT", "name": "Leave Department Policy", "document_type": "leave_request", "workflow_key": "leave_request_flow", "department_id": "dept_leave", "action": "submit", "status": "active"})
	if _, err := h.models.Create("approval_policy_stage", "user_admin", map[string]any{"policy_id": approvalPolicy.ID, "stage_key": "dept", "sequence": 1, "assignment_strategy": "explicit_user", "explicit_user_id": "user_admin", "status": "active"}); err != nil {
		t.Fatalf("create approval stage: %v", err)
	}
	if _, err := h.models.Create("leave_entitlement_rule", "user_admin", map[string]any{"leave_policy_id": policyRecord.ID, "grant_mode": "annual_grant", "annual_entitlement_days": 8.0, "status": "active"}); err != nil {
		t.Fatalf("create entitlement rule: %v", err)
	}
	if _, err := h.models.Create("employee_leave_profile", "user_admin", map[string]any{"employee_id": employee.ID, "leave_policy_id": policyRecord.ID, "effective_from": "2000-01-01", "status": "active"}); err != nil {
		t.Fatalf("create leave profile: %v", err)
	}
	if _, err := h.models.Create("leave_accrual_run", "user_admin", map[string]any{"code": "ACC-SELF", "name": "Self Grant", "leave_policy_id": policyRecord.ID, "run_mode": "annual_grant", "effective_date": "2099-01-01", "status": "active", "run_status": "draft"}); err != nil {
		t.Fatalf("create accrual run: %v", err)
	}
	createBody, _ := json.Marshal(map[string]any{"leave_policy_id": policyRecord.ID, "start_date": "2099-02-01", "end_date": "2099-02-01"})
	create := h.request(http.MethodPost, "/ui/self-service/leave/requests", createBody, true)
	if create.Code != http.StatusOK {
		t.Fatalf("create leave request failed: %d body=%s", create.Code, create.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(create.Body.Bytes(), &created)
	requestID := created["record"].(map[string]any)["id"].(string)
	submit := h.request(http.MethodPost, "/ui/self-service/leave/requests/"+requestID+"/submit", nil, true)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit leave request failed: %d body=%s", submit.Code, submit.Body.String())
	}
	pending := h.request(http.MethodGet, "/ui/attendance/leave-requests/pending", nil, true)
	if pending.Code != http.StatusOK {
		t.Fatalf("pending leave approvals failed: %d body=%s", pending.Code, pending.Body.String())
	}
	approve := h.request(http.MethodPost, "/ui/attendance/leave-requests/"+requestID+"/approve", nil, true)
	if approve.Code != http.StatusOK {
		t.Fatalf("approve leave request failed: %d body=%s", approve.Code, approve.Body.String())
	}
}

func registerLeaveSelfServiceTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{Key: "employee_profile", DisplayName: "Employee Profile", DefaultSort: "employee_code", Fields: []model.FieldDefinition{{Key: "party_id", Type: "string"}, {Key: "user_id", Type: "string"}, {Key: "employee_code", Type: "string"}, {Key: "employment_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "employee_assignment", DisplayName: "Employee Assignment", DefaultSort: "effective_from", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "organization_unit_id", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "cost_center_id", Type: "string"}, {Key: "effective_from", Type: "string"}, {Key: "effective_to", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "absence_code", DisplayName: "Absence Code", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "category", Type: "string"}, {Key: "deduct_from_payroll", Type: "bool"}, {Key: "status", Type: "string"}}},
		{Key: "leave_request", DisplayName: "Leave Request", DefaultSort: "start_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "organization_unit_id", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "cost_center_id", Type: "string"}, {Key: "absence_code_id", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "employee_leave_profile_id", Type: "string"}, {Key: "balance_account_id", Type: "string"}, {Key: "start_date", Type: "string"}, {Key: "end_date", Type: "string"}, {Key: "request_unit", Type: "string"}, {Key: "requested_days", Type: "number"}, {Key: "half_day_session", Type: "string"}, {Key: "request_source", Type: "string"}, {Key: "self_service_actor_user_id", Type: "string"}, {Key: "approval_status", Type: "string"}, {Key: "approved_days", Type: "number"}, {Key: "approval_policy_id", Type: "string"}, {Key: "approval_stage_key", Type: "string"}, {Key: "approval_stage_sequence", Type: "number"}, {Key: "approval_stage_total", Type: "number"}, {Key: "approval_routing_mode", Type: "string"}, {Key: "required_approver_count", Type: "number"}, {Key: "approver_user_id", Type: "string"}, {Key: "approval_candidate_user_ids_json", Type: "string"}, {Key: "approval_recorded_user_ids_json", Type: "string"}, {Key: "approval_requires_different_actor", Type: "bool"}, {Key: "reservation_entry_ids_json", Type: "string"}, {Key: "consumption_entry_ids_json", Type: "string"}, {Key: "notes", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "attendance_day", DisplayName: "Attendance Day", DefaultSort: "attendance_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "attendance_status", Type: "string"}, {Key: "absence_code_id", Type: "string"}, {Key: "leave_request_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "attendance_adjustment", DisplayName: "Attendance Adjustment", DefaultSort: "attendance_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "approval_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "overtime_request", DisplayName: "Overtime Request", DefaultSort: "attendance_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "approval_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "workforce_roster_slot", DisplayName: "Roster Slot", DefaultSort: "shift_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "shift_date", Type: "string"}, {Key: "planned_start_time", Type: "string"}, {Key: "planned_end_time", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "workforce_roster", DisplayName: "Roster", DefaultSort: "start_date", Fields: []model.FieldDefinition{{Key: "start_date", Type: "string"}, {Key: "end_date", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "attendance_event", DisplayName: "Attendance Event", DefaultSort: "occurred_at", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "event_type", Type: "string"}, {Key: "occurred_at", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_policy", DisplayName: "Leave Policy", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "absence_code_id", Type: "string"}, {Key: "paid_leave", Type: "bool"}, {Key: "requires_balance", Type: "bool"}, {Key: "allows_half_day", Type: "bool"}, {Key: "notice_days", Type: "number"}, {Key: "approval_policy_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_entitlement_rule", DisplayName: "Leave Entitlement Rule", DefaultSort: "leave_policy_id", Fields: []model.FieldDefinition{{Key: "leave_policy_id", Type: "string"}, {Key: "grant_mode", Type: "string"}, {Key: "annual_entitlement_days", Type: "number"}, {Key: "monthly_accrual_days", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "employee_leave_profile", DisplayName: "Employee Leave Profile", DefaultSort: "employee_id", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "effective_from", Type: "string"}, {Key: "effective_to", Type: "string"}, {Key: "opening_balance_days", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "leave_balance_account", DisplayName: "Leave Balance Account", DefaultSort: "employee_id", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "employee_leave_profile_id", Type: "string"}, {Key: "current_balance_days", Type: "number"}, {Key: "reserved_days", Type: "number"}, {Key: "available_days", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "leave_balance_entry", DisplayName: "Leave Balance Entry", DefaultSort: "effective_date", Fields: []model.FieldDefinition{{Key: "balance_account_id", Type: "string"}, {Key: "employee_id", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "employee_leave_profile_id", Type: "string"}, {Key: "leave_request_id", Type: "string"}, {Key: "accrual_run_id", Type: "string"}, {Key: "entry_type", Type: "string"}, {Key: "days", Type: "number"}, {Key: "effective_date", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_accrual_run", DisplayName: "Leave Accrual Run", DefaultSort: "effective_date", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "run_mode", Type: "string"}, {Key: "effective_date", Type: "string"}, {Key: "run_status", Type: "string"}, {Key: "processed_at", Type: "string"}, {Key: "processed_by", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_balance_adjustment", DisplayName: "Leave Balance Adjustment", DefaultSort: "updated_at", Fields: []model.FieldDefinition{{Key: "balance_account_id", Type: "string"}, {Key: "days", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "approval_policy", DisplayName: "Approval Policy", DefaultSort: "priority", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "document_type", Type: "string"}, {Key: "workflow_key", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "action", Type: "string"}, {Key: "assignment_strategy", Type: "string"}, {Key: "approver_group_id", Type: "string"}, {Key: "explicit_user_id", Type: "string"}, {Key: "priority", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "approval_policy_stage", DisplayName: "Approval Policy Stage", DefaultSort: "sequence", Fields: []model.FieldDefinition{{Key: "policy_id", Type: "string"}, {Key: "stage_key", Type: "string"}, {Key: "sequence", Type: "number"}, {Key: "required_approver_count", Type: "number"}, {Key: "assignment_strategy", Type: "string"}, {Key: "approver_group_id", Type: "string"}, {Key: "explicit_user_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "approver_group_member", DisplayName: "Approver Group Member", DefaultSort: "user_id", Fields: []model.FieldDefinition{{Key: "approver_group_id", Type: "string"}, {Key: "user_id", Type: "string"}, {Key: "status", Type: "string"}}},
	}
	for _, def := range defs {
		if _, ok := models.Definition(def.Key); ok {
			continue
		}
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}
