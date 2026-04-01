package application

import (
	"testing"

	"orbyte/internal/platform/model"
)

func TestLeavePolicyExecuteAccrualAndApproveLifecycle(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	service := NewLeavePolicyCoreService(models, workforce, attendance, nil)

	employee, policy, _ := seedLeavePolicyTestData(t, models, false)
	run, err := models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":           "LRUN-1",
		"name":           "Annual Grant",
		"leave_policy_id": policy.ID,
		"run_mode":       "annual_grant",
		"effective_date": "2099-01-01",
		"status":         "active",
	})
	if err != nil {
		t.Fatalf("create accrual run: %v", err)
	}
	if _, err := service.ExecuteAccrualRun(run.ID, "user_admin"); err != nil {
		t.Fatalf("execute accrual run: %v", err)
	}

	balances, err := service.ListBalancesForUser("leave_user")
	if err != nil {
		t.Fatalf("list balances: %v", err)
	}
	if len(balances) != 1 {
		t.Fatalf("expected 1 leave balance account, got %d", len(balances))
	}
	if got := numberValue(balances[0].Values["available_days"]); got != 12 {
		t.Fatalf("expected available balance 12, got %v", got)
	}

	record, err := service.CreateSelfServiceLeaveRequest("leave_user", map[string]any{
		"leave_policy_id": policy.ID,
		"start_date":      "2099-02-10",
		"end_date":        "2099-02-11",
		"notes":           "Family event",
	}, "leave_user")
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	record, err = service.SubmitSelfServiceLeaveRequest("leave_user", record.ID, "leave_user")
	if err != nil {
		t.Fatalf("submit leave request: %v", err)
	}
	account, err := models.Get("leave_balance_account", textValue(record.Values["balance_account_id"]))
	if err != nil {
		t.Fatalf("get balance account: %v", err)
	}
	if got := numberValue(account.Values["reserved_days"]); got != 2 {
		t.Fatalf("expected reserved_days 2 after submit, got %v", got)
	}
	if got := numberValue(account.Values["available_days"]); got != 10 {
		t.Fatalf("expected available_days 10 after submit, got %v", got)
	}

	record, err = service.ApproveLeaveRequest(record.ID, "manager_user")
	if err != nil {
		t.Fatalf("approve leave request: %v", err)
	}
	account, err = models.Get("leave_balance_account", textValue(record.Values["balance_account_id"]))
	if err != nil {
		t.Fatalf("reload balance account: %v", err)
	}
	if got := numberValue(account.Values["current_balance_days"]); got != 10 {
		t.Fatalf("expected current_balance_days 10 after approval, got %v", got)
	}
	if got := numberValue(account.Values["reserved_days"]); got != 0 {
		t.Fatalf("expected reserved_days 0 after approval, got %v", got)
	}
	days, _, err := models.List("attendance_day", model.Query{
		Filters:  map[string]string{"employee_id": employee.ID, "attendance_date": "2099-02-10"},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("list attendance days: %v", err)
	}
	if len(days) == 0 || textValue(days[0].Values["attendance_status"]) != "on_leave" {
		t.Fatalf("expected attendance day on_leave, got %+v", days)
	}
}

func TestLeavePolicySubmitUsesLinkedApprovalPolicyWithQuorum(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	approvals := NewApprovalPolicyService(models)
	service := NewLeavePolicyCoreService(models, workforce, attendance, approvals)

	_, policy, _ := seedLeavePolicyTestData(t, models, false)
	approvalPolicy, err := models.Create("approval_policy", "user_admin", map[string]any{
		"code":                "LEAVE-GROUP",
		"name":                "Leave Group Policy",
		"document_type":       "leave_request",
		"assignment_strategy": "approver_group",
		"approver_group_id":   "grp_leave",
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create approval policy: %v", err)
	}
	for _, userID := range []string{"approver_a", "approver_b"} {
		if _, err := models.Create("approver_group_member", "user_admin", map[string]any{
			"approver_group_id": "grp_leave",
			"user_id":           userID,
			"status":            "active",
		}); err != nil {
			t.Fatalf("create approver group member: %v", err)
		}
	}
	if _, err := models.Create("approval_policy_stage", "user_admin", map[string]any{
		"policy_id":                approvalPolicy.ID,
		"stage_key":                "manager",
		"sequence":                 1,
		"assignment_strategy":      "approver_group",
		"approver_group_id":        "grp_leave",
		"required_approver_count":  2,
		"status":                   "active",
	}); err != nil {
		t.Fatalf("create approval policy stage: %v", err)
	}
	policyValues := cloneMap(policy.Values)
	policyValues["approval_policy_id"] = approvalPolicy.ID
	policy, err = models.Update("leave_policy", policy.ID, "user_admin", policyValues, policy.Version)
	if err != nil {
		t.Fatalf("update leave policy: %v", err)
	}
	run, err := models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":            "LRUN-Q",
		"name":            "Annual Grant Q",
		"leave_policy_id": policy.ID,
		"run_mode":        "annual_grant",
		"effective_date":  "2099-01-01",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create accrual run: %v", err)
	}
	if _, err := service.ExecuteAccrualRun(run.ID, "user_admin"); err != nil {
		t.Fatalf("execute accrual run: %v", err)
	}
	record, err := service.CreateSelfServiceLeaveRequest("leave_user", map[string]any{
		"leave_policy_id": policy.ID,
		"start_date":      "2099-02-10",
		"end_date":        "2099-02-11",
	}, "leave_user")
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	record, err = service.SubmitSelfServiceLeaveRequest("leave_user", record.ID, "leave_user")
	if err != nil {
		t.Fatalf("submit leave request: %v", err)
	}
	if got := textValue(record.Values["approval_policy_id"]); got != approvalPolicy.ID {
		t.Fatalf("expected approval policy %s, got %s", approvalPolicy.ID, got)
	}
	if got := int(numberValue(record.Values["required_approver_count"])); got != 2 {
		t.Fatalf("expected quorum 2, got %d", got)
	}
	if got := parseStringList(record.Values["approval_candidate_user_ids_json"]); len(got) != 2 {
		t.Fatalf("expected 2 candidate approvers, got %+v", got)
	}
	record, err = service.ApproveLeaveRequest(record.ID, "approver_a")
	if err != nil {
		t.Fatalf("first approve: %v", err)
	}
	if got := textValue(record.Values["approval_status"]); got != "submitted" {
		t.Fatalf("expected still submitted after first approval, got %s", got)
	}
	if got := parseStringList(record.Values["approval_recorded_user_ids_json"]); len(got) != 1 || got[0] != "approver_a" {
		t.Fatalf("expected first approval recorded, got %+v", got)
	}
	record, err = service.ApproveLeaveRequest(record.ID, "approver_b")
	if err != nil {
		t.Fatalf("second approve: %v", err)
	}
	if got := textValue(record.Values["approval_status"]); got != "approved" {
		t.Fatalf("expected approved after quorum, got %s", got)
	}
}

func TestLeavePolicySubmitFallsBackToSharedApprovalPolicyMatcher(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	approvals := NewApprovalPolicyService(models)
	service := NewLeavePolicyCoreService(models, workforce, attendance, approvals)

	_, policy, _ := seedLeavePolicyTestData(t, models, false)
	approvalPolicy, err := models.Create("approval_policy", "user_admin", map[string]any{
		"code":          "LEAVE-DEPT",
		"name":          "Leave Department Policy",
		"document_type": "leave_request",
		"workflow_key":  leaveRequestWorkflowKey,
		"department_id": "dept_leave",
		"action":        "submit",
		"status":        "active",
	})
	if err != nil {
		t.Fatalf("create approval policy: %v", err)
	}
	if _, err := models.Create("approval_policy_stage", "user_admin", map[string]any{
		"policy_id":           approvalPolicy.ID,
		"stage_key":           "dept",
		"sequence":            1,
		"assignment_strategy": "explicit_user",
		"explicit_user_id":    "dept_manager",
		"status":              "active",
	}); err != nil {
		t.Fatalf("create policy stage: %v", err)
	}
	run, err := models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":            "LRUN-M",
		"name":            "Annual Grant M",
		"leave_policy_id": policy.ID,
		"run_mode":        "annual_grant",
		"effective_date":  "2099-01-01",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create accrual run: %v", err)
	}
	if _, err := service.ExecuteAccrualRun(run.ID, "user_admin"); err != nil {
		t.Fatalf("execute accrual run: %v", err)
	}
	record, err := service.CreateSelfServiceLeaveRequest("leave_user", map[string]any{
		"leave_policy_id": policy.ID,
		"start_date":      "2099-02-10",
		"end_date":        "2099-02-10",
	}, "leave_user")
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	record, err = service.SubmitSelfServiceLeaveRequest("leave_user", record.ID, "leave_user")
	if err != nil {
		t.Fatalf("submit leave request: %v", err)
	}
	if got := textValue(record.Values["approval_policy_id"]); got != approvalPolicy.ID {
		t.Fatalf("expected matched approval policy %s, got %s", approvalPolicy.ID, got)
	}
	if got := textValue(record.Values["approver_user_id"]); got != "dept_manager" {
		t.Fatalf("expected explicit approver dept_manager, got %s", got)
	}
}

func TestLeavePolicyApprovalRejectsUnresolvedStageRouting(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	approvals := NewApprovalPolicyService(models)
	service := NewLeavePolicyCoreService(models, workforce, attendance, approvals)

	_, policy, _ := seedLeavePolicyTestData(t, models, false)
	approvalPolicy, err := models.Create("approval_policy", "user_admin", map[string]any{
		"code":                "LEAVE-UNRESOLVED",
		"name":                "Leave Unresolved Policy",
		"assignment_strategy": "requester_manager",
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create approval policy: %v", err)
	}
	policyValues := cloneMap(policy.Values)
	policyValues["approval_policy_id"] = approvalPolicy.ID
	policy, err = models.Update("leave_policy", policy.ID, "user_admin", policyValues, policy.Version)
	if err != nil {
		t.Fatalf("update leave policy: %v", err)
	}
	run, err := models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":            "LRUN-U",
		"name":            "Annual Grant U",
		"leave_policy_id": policy.ID,
		"run_mode":        "annual_grant",
		"effective_date":  "2099-01-01",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create accrual run: %v", err)
	}
	if _, err := service.ExecuteAccrualRun(run.ID, "user_admin"); err != nil {
		t.Fatalf("execute accrual run: %v", err)
	}
	record, err := service.CreateSelfServiceLeaveRequest("leave_user", map[string]any{
		"leave_policy_id": policy.ID,
		"start_date":      "2099-02-10",
		"end_date":        "2099-02-10",
	}, "leave_user")
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	record, err = service.SubmitSelfServiceLeaveRequest("leave_user", record.ID, "leave_user")
	if err != nil {
		t.Fatalf("submit leave request: %v", err)
	}
	if _, err := service.ApproveLeaveRequest(record.ID, "random_approver"); err == nil {
		t.Fatal("expected unresolved stage approval to be rejected")
	}
}

func TestLeavePolicyApprovalRejectsDuplicateVotes(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	approvals := NewApprovalPolicyService(models)
	service := NewLeavePolicyCoreService(models, workforce, attendance, approvals)

	_, policy, _ := seedLeavePolicyTestData(t, models, false)
	approvalPolicy, err := models.Create("approval_policy", "user_admin", map[string]any{
		"code":                "LEAVE-DUP",
		"name":                "Leave Duplicate Vote Policy",
		"assignment_strategy": "approver_group",
		"approver_group_id":   "grp_dup",
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create approval policy: %v", err)
	}
	for _, userID := range []string{"approver_a", "approver_b"} {
		if _, err := models.Create("approver_group_member", "user_admin", map[string]any{
			"approver_group_id": "grp_dup",
			"user_id":           userID,
			"status":            "active",
		}); err != nil {
			t.Fatalf("create approver group member: %v", err)
		}
	}
	if _, err := models.Create("approval_policy_stage", "user_admin", map[string]any{
		"policy_id":               approvalPolicy.ID,
		"stage_key":               "manager",
		"sequence":                1,
		"assignment_strategy":     "approver_group",
		"approver_group_id":       "grp_dup",
		"required_approver_count": 2,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create approval policy stage: %v", err)
	}
	policyValues := cloneMap(policy.Values)
	policyValues["approval_policy_id"] = approvalPolicy.ID
	policy, err = models.Update("leave_policy", policy.ID, "user_admin", policyValues, policy.Version)
	if err != nil {
		t.Fatalf("update leave policy: %v", err)
	}
	run, err := models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":            "LRUN-D",
		"name":            "Annual Grant D",
		"leave_policy_id": policy.ID,
		"run_mode":        "annual_grant",
		"effective_date":  "2099-01-01",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create accrual run: %v", err)
	}
	if _, err := service.ExecuteAccrualRun(run.ID, "user_admin"); err != nil {
		t.Fatalf("execute accrual run: %v", err)
	}
	record, err := service.CreateSelfServiceLeaveRequest("leave_user", map[string]any{
		"leave_policy_id": policy.ID,
		"start_date":      "2099-02-10",
		"end_date":        "2099-02-11",
	}, "leave_user")
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	record, err = service.SubmitSelfServiceLeaveRequest("leave_user", record.ID, "leave_user")
	if err != nil {
		t.Fatalf("submit leave request: %v", err)
	}
	record, err = service.ApproveLeaveRequest(record.ID, "approver_a")
	if err != nil {
		t.Fatalf("first approve: %v", err)
	}
	if _, err := service.ApproveLeaveRequest(record.ID, "approver_a"); err == nil {
		t.Fatal("expected duplicate approve vote to be rejected")
	}
	if _, err := service.RejectLeaveRequest(record.ID, "approver_a", "second vote"); err == nil {
		t.Fatal("expected duplicate reject vote to be rejected")
	}
	pending, err := service.PendingRequestSummariesForApprover("approver_a")
	if err != nil {
		t.Fatalf("pending requests: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending requests for duplicate voter, got %+v", pending)
	}
}

func TestLeavePolicyCancelReleasesReservation(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	service := NewLeavePolicyCoreService(models, workforce, attendance, nil)

	_, policy, _ := seedLeavePolicyTestData(t, models, false)
	run, _ := models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":           "LRUN-2",
		"name":           "Annual Grant 2",
		"leave_policy_id": policy.ID,
		"run_mode":       "annual_grant",
		"effective_date": "2099-01-01",
		"status":         "active",
	})
	if _, err := service.ExecuteAccrualRun(run.ID, "user_admin"); err != nil {
		t.Fatalf("execute accrual run: %v", err)
	}
	record, err := service.CreateSelfServiceLeaveRequest("leave_user", map[string]any{
		"leave_policy_id": policy.ID,
		"start_date":      "2099-03-10",
		"end_date":        "2099-03-10",
	}, "leave_user")
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	record, err = service.SubmitSelfServiceLeaveRequest("leave_user", record.ID, "leave_user")
	if err != nil {
		t.Fatalf("submit request: %v", err)
	}
	record, err = service.CancelSelfServiceLeaveRequest("leave_user", record.ID, "leave_user")
	if err != nil {
		t.Fatalf("cancel request: %v", err)
	}
	account, err := models.Get("leave_balance_account", textValue(record.Values["balance_account_id"]))
	if err != nil {
		t.Fatalf("get balance account: %v", err)
	}
	if got := numberValue(account.Values["available_days"]); got != 12 {
		t.Fatalf("expected available_days restored to 12, got %v", got)
	}
	if got := textValue(record.Values["approval_status"]); got != "cancelled" {
		t.Fatalf("expected approval_status cancelled, got %s", got)
	}
}

func TestLeavePolicyPaidLeaveDoesNotDeductFromPayroll(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	service := NewLeavePolicyCoreService(models, workforce, attendance, nil)

	_, policy, absenceCode := seedLeavePolicyTestData(t, models, true)
	record, err := models.Create("leave_request", "user_admin", map[string]any{
		"employee_id":     "employee_profile-1",
		"absence_code_id": absenceCode.ID,
		"leave_policy_id": policy.ID,
		"start_date":      "2099-04-01",
		"end_date":        "2099-04-01",
		"approval_status": "approved",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	if service.LeaveDeductsFromPayroll(record.ID) {
		t.Fatal("expected paid leave policy not to deduct from payroll")
	}
}

func TestLeavePolicySelfServiceRequiresCurrentAssignment(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	service := NewLeavePolicyCoreService(models, workforce, attendance, nil)

	employee, policy, _ := seedLeavePolicyTestData(t, models, false)
	assignments, _, err := models.List("employee_assignment", model.Query{
		Filters:  map[string]string{"employee_id": employee.ID},
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil || len(assignments) == 0 {
		t.Fatalf("list employee assignments: %v", err)
	}
	values := cloneMap(assignments[0].Values)
	values["effective_from"] = "2099-12-01"
	if _, err := models.Update("employee_assignment", assignments[0].ID, "user_admin", values, assignments[0].Version); err != nil {
		t.Fatalf("update assignment: %v", err)
	}

	if _, err := service.CreateSelfServiceLeaveRequest("leave_user", map[string]any{
		"leave_policy_id": policy.ID,
		"start_date":      "2099-02-10",
		"end_date":        "2099-02-10",
	}, "leave_user"); err == nil {
		t.Fatal("expected self-service request creation to fail without a current assignment")
	}
}

func TestLeavePolicyNonBalancePolicyDoesNotCreateBalanceAccount(t *testing.T) {
	models := model.NewService()
	registerLeavePolicyTestModels(t, models)
	workforce := NewEmployeeWorkforceCoreService(models)
	attendance := NewWorkforceAttendanceCoreService(models, workforce)
	service := NewLeavePolicyCoreService(models, workforce, attendance, nil)

	employee, _, absenceCode := seedLeavePolicyTestData(t, models, false)
	policy, err := models.Create("leave_policy", "user_admin", map[string]any{
		"code":             "LP-SICK",
		"name":             "Sick Leave Policy",
		"absence_code_id":  absenceCode.ID,
		"paid_leave":       true,
		"requires_balance": false,
		"allows_half_day":  true,
		"organization_id":  "org_default",
		"location_id":      "loc_hq",
		"status":           "active",
	})
	if err != nil {
		t.Fatalf("create leave policy: %v", err)
	}
	if _, err := models.Create("employee_leave_profile", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"leave_policy_id":      policy.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_hq",
		"effective_from":       "2000-01-01",
		"opening_balance_days": 5.0,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create employee leave profile: %v", err)
	}

	record, err := service.CreateSelfServiceLeaveRequest("leave_user", map[string]any{
		"leave_policy_id": policy.ID,
		"start_date":      "2099-02-12",
		"end_date":        "2099-02-12",
	}, "leave_user")
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	if got := textValue(record.Values["balance_account_id"]); got != "" {
		t.Fatalf("expected no balance account for non-balance policy, got %s", got)
	}
	balances, _, err := models.List("leave_balance_account", model.Query{
		Filters:  map[string]string{"employee_id": employee.ID, "leave_policy_id": policy.ID},
		Page:     1,
		PageSize: model.MaxPageSize,
	})
	if err != nil {
		t.Fatalf("list leave balance accounts: %v", err)
	}
	if len(balances) != 0 {
		t.Fatalf("expected no leave balance accounts for non-balance policy, got %d", len(balances))
	}
}

func registerLeavePolicyTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	registerWorkforceAttendanceTestModels(t, models)
	registerApprovalPolicyTestModels(t, models)
	extras := []model.Definition{
		{Key: "employee_assignment", DisplayName: "Employee Assignment", DefaultSort: "effective_from", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "organization_unit_id", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "cost_center_id", Type: "string"}, {Key: "effective_from", Type: "string"}, {Key: "effective_to", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_policy", DisplayName: "Leave Policy", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "absence_code_id", Type: "string"}, {Key: "paid_leave", Type: "bool"}, {Key: "requires_balance", Type: "bool"}, {Key: "allows_half_day", Type: "bool"}, {Key: "notice_days", Type: "number"}, {Key: "approval_policy_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_entitlement_rule", DisplayName: "Leave Entitlement Rule", DefaultSort: "leave_policy_id", Fields: []model.FieldDefinition{{Key: "leave_policy_id", Type: "string"}, {Key: "grant_mode", Type: "string"}, {Key: "annual_entitlement_days", Type: "number"}, {Key: "monthly_accrual_days", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "employee_leave_profile", DisplayName: "Employee Leave Profile", DefaultSort: "employee_id", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "effective_from", Type: "string"}, {Key: "effective_to", Type: "string"}, {Key: "opening_balance_days", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "leave_balance_account", DisplayName: "Leave Balance Account", DefaultSort: "employee_id", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "employee_leave_profile_id", Type: "string"}, {Key: "current_balance_days", Type: "number"}, {Key: "reserved_days", Type: "number"}, {Key: "available_days", Type: "number"}, {Key: "last_accrual_date", Type: "string"}, {Key: "carry_forward_expiry_date", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_balance_entry", DisplayName: "Leave Balance Entry", DefaultSort: "effective_date", Fields: []model.FieldDefinition{{Key: "balance_account_id", Type: "string"}, {Key: "employee_id", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "employee_leave_profile_id", Type: "string"}, {Key: "leave_request_id", Type: "string"}, {Key: "accrual_run_id", Type: "string"}, {Key: "entry_type", Type: "string"}, {Key: "days", Type: "number"}, {Key: "effective_date", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_accrual_run", DisplayName: "Leave Accrual Run", DefaultSort: "effective_date", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "run_mode", Type: "string"}, {Key: "effective_date", Type: "string"}, {Key: "run_status", Type: "string"}, {Key: "processed_at", Type: "string"}, {Key: "processed_by", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "leave_balance_adjustment", DisplayName: "Leave Balance Adjustment", DefaultSort: "updated_at", Fields: []model.FieldDefinition{{Key: "balance_account_id", Type: "string"}, {Key: "employee_id", Type: "string"}, {Key: "leave_policy_id", Type: "string"}, {Key: "days", Type: "number"}, {Key: "reason_code", Type: "string"}, {Key: "notes", Type: "string"}, {Key: "status", Type: "string"}}},
	}
	for _, def := range extras {
		if _, ok := models.Definition(def.Key); ok {
			continue
		}
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s failed: %v", def.Key, err)
		}
	}
}

func seedLeavePolicyTestData(t *testing.T, models *model.Service, paidLeave bool) (model.Record, model.Record, model.Record) {
	t.Helper()
	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_emp",
		"user_id":           "leave_user",
		"employee_code":     "EMP-LEAVE-1",
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if _, err := models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_hq",
		"organization_unit_id": "ou_leave",
		"department_id":        "dept_leave",
		"cost_center_id":       "cc_leave",
		"effective_from":       "2000-01-01",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create employee assignment: %v", err)
	}
	absenceCode, err := models.Create("absence_code", "user_admin", map[string]any{
		"code":                "ANNUAL",
		"name":                "Annual Leave",
		"category":            "leave",
		"deduct_from_payroll": true,
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create absence code: %v", err)
	}
	policy, err := models.Create("leave_policy", "user_admin", map[string]any{
		"code":             "LP-1",
		"name":             "Annual Leave Policy",
		"absence_code_id":  absenceCode.ID,
		"paid_leave":       paidLeave,
		"requires_balance": true,
		"allows_half_day":  true,
		"organization_id":  "org_default",
		"location_id":      "loc_hq",
		"status":           "active",
	})
	if err != nil {
		t.Fatalf("create leave policy: %v", err)
	}
	if _, err := models.Create("leave_entitlement_rule", "user_admin", map[string]any{
		"leave_policy_id":         policy.ID,
		"grant_mode":              "annual_grant",
		"annual_entitlement_days": 12.0,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create entitlement rule: %v", err)
	}
	if _, err := models.Create("employee_leave_profile", "user_admin", map[string]any{
		"employee_id":      employee.ID,
		"leave_policy_id":  policy.ID,
		"organization_id":  "org_default",
		"location_id":      "loc_hq",
		"effective_from":   "2000-01-01",
		"opening_balance_days": 0.0,
		"status":           "active",
	}); err != nil {
		t.Fatalf("create employee leave profile: %v", err)
	}
	return employee, policy, absenceCode
}
