package app

import (
	"fmt"
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/store"
)

func TestLeavePolicyPostgresValidation(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed leave policy test")
	}
	postgres, err := store.OpenFromEnv()
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = postgres.Close() }()

	graph := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, nil, "bootstrap-123!"); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	userID := "leave_pg_user_" + suffix

	employee, err := graph.models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_leave_pg_" + suffix,
		"user_id":           userID,
		"employee_code":     "EMP-LEAVE-PG-" + suffix,
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if _, err := graph.models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":       employee.ID,
		"organization_id":   "org_default",
		"location_id":       "loc_hq",
		"organization_unit_id": "ou_leave",
		"department_id":     "dept_leave",
		"cost_center_id":    "cc_leave",
		"effective_from":    "2000-01-01",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create employee assignment: %v", err)
	}
	absenceCode, err := graph.models.Create("absence_code", "user_admin", map[string]any{
		"code":                "ANNUAL-PG-" + suffix,
		"name":                "Annual Leave",
		"category":            "leave",
		"deduct_from_payroll": true,
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create absence code: %v", err)
	}
	leavePolicy, err := graph.models.Create("leave_policy", "user_admin", map[string]any{
		"code":             "LP-PG-" + suffix,
		"name":             "Annual Unpaid Leave",
		"absence_code_id":  absenceCode.ID,
		"paid_leave":       false,
		"requires_balance": true,
		"allows_half_day":  true,
		"organization_id":  "org_default",
		"location_id":      "loc_hq",
		"status":           "active",
	})
	if err != nil {
		t.Fatalf("create leave policy: %v", err)
	}
	approvalPolicy, err := graph.models.Create("approval_policy", "user_admin", map[string]any{
		"code":          "LEAVE-PG-" + suffix,
		"name":          "Leave Approval Policy",
		"document_type": "leave_request",
		"workflow_key":  "leave_request_flow",
		"action":        "submit",
		"department_id": "dept_leave",
		"status":        "active",
	})
	if err != nil {
		t.Fatalf("create approval policy: %v", err)
	}
	if _, err := graph.models.Create("approval_policy_stage", "user_admin", map[string]any{
		"policy_id":           approvalPolicy.ID,
		"stage_key":           "manager",
		"sequence":            1,
		"assignment_strategy": "explicit_user",
		"explicit_user_id":    "user_admin",
		"status":              "active",
	}); err != nil {
		t.Fatalf("create approval policy stage: %v", err)
	}
	if _, err := graph.models.Create("leave_entitlement_rule", "user_admin", map[string]any{
		"leave_policy_id":         leavePolicy.ID,
		"grant_mode":              "annual_grant",
		"annual_entitlement_days": 12.0,
		"status":                  "active",
	}); err != nil {
		t.Fatalf("create entitlement rule: %v", err)
	}
	if _, err := graph.models.Create("employee_leave_profile", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"leave_policy_id":      leavePolicy.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_hq",
		"effective_from":       "2000-01-01",
		"opening_balance_days": 1.0,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create leave profile: %v", err)
	}
	run, err := graph.models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":            "LAR-PG-" + suffix,
		"name":            "Annual Leave Grant",
		"leave_policy_id": leavePolicy.ID,
		"run_mode":        "annual_grant",
		"effective_date":  "2099-01-01",
		"status":          "active",
		"run_status":      "draft",
	})
	if err != nil {
		t.Fatalf("create leave accrual run: %v", err)
	}
	if _, err := graph.leavePolicies.ExecuteAccrualRun(run.ID, "user_admin"); err != nil {
		t.Fatalf("execute accrual run: %v", err)
	}

	request, err := graph.leavePolicies.CreateSelfServiceLeaveRequest(userID, map[string]any{
		"leave_policy_id": leavePolicy.ID,
		"start_date":      "2099-02-10",
		"end_date":        "2099-02-10",
		"request_unit":    "half_day",
		"half_day_session": "morning",
		"notes":           "Medical appointment",
	}, userID)
	if err != nil {
		t.Fatalf("create self-service leave request: %v", err)
	}
	request, err = graph.leavePolicies.SubmitSelfServiceLeaveRequest(userID, request.ID, userID)
	if err != nil {
		t.Fatalf("submit self-service leave request: %v", err)
	}
	if got := textValue(request.Values["approval_policy_id"]); got != approvalPolicy.ID {
		t.Fatalf("expected approval policy %s after submit, got %s", approvalPolicy.ID, got)
	}
	if got := textValue(request.Values["approver_user_id"]); got != "user_admin" {
		t.Fatalf("expected approver user_admin after submit, got %s", got)
	}
	request, err = graph.leavePolicies.ApproveLeaveRequest(request.ID, "user_admin")
	if err != nil {
		t.Fatalf("approve leave request: %v", err)
	}

	account, err := graph.models.Get("leave_balance_account", textValue(request.Values["balance_account_id"]))
	if err != nil {
		t.Fatalf("get balance account: %v", err)
	}
	if got := numberValue(account.Values["available_days"]); got != 12.5 {
		t.Fatalf("expected available_days 12.5 after approval, got %v", got)
	}

	if _, err := graph.models.Create("employee_compensation_profile", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"currency_code":        "IDR",
		"standard_hourly_rate": 10.0,
		"overtime_hourly_rate": 15.0,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create employee compensation profile: %v", err)
	}
	if _, err := graph.models.Create("employee_payroll_profile", "user_admin", map[string]any{
		"employee_id":                employee.ID,
		"salary_structure_id":        "struct_leave_pg_" + suffix,
		"currency_code":              "IDR",
		"payment_method_code":        "BANK",
		"treasury_account_id":        "treasury_leave_pg_" + suffix,
		"leave_deduction_daily_rate": 20.0,
		"status":                     "active",
	}); err != nil {
		t.Fatalf("create employee payroll profile: %v", err)
	}
	for _, component := range []map[string]any{
		{"code": "BASIC-L-" + suffix, "name": "Basic", "component_class": "earning", "status": "active"},
		{"code": "LEAVE-L-" + suffix, "name": "Leave Deduction", "component_class": "deduction", "status": "active"},
	} {
		if _, err := graph.models.Create("pay_component", "user_admin", component); err != nil {
			t.Fatalf("create pay component %s: %v", component["code"], err)
		}
	}
	for _, line := range []map[string]any{
		{"salary_structure_id": "struct_leave_pg_" + suffix, "component_code": "BASIC-L-" + suffix, "sequence": 1, "formula_key": "fixed_amount", "fixed_amount": 1000.0, "status": "active"},
		{"salary_structure_id": "struct_leave_pg_" + suffix, "component_code": "LEAVE-L-" + suffix, "sequence": 2, "formula_key": "leave_deduction", "status": "active"},
	} {
		if _, err := graph.models.Create("salary_structure_line", "user_admin", line); err != nil {
			t.Fatalf("create salary structure line %s: %v", line["component_code"], err)
		}
	}
	period, err := graph.models.Create("payroll_period", "user_admin", map[string]any{
		"code":            "PR-LEAVE-PG-" + suffix,
		"name":            "Payroll Leave Period",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"start_date":      "2099-02-01",
		"end_date":        "2099-02-28",
		"pay_date":        "2099-02-28",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create payroll period: %v", err)
	}
	payrollPayload := graph.employeePayroll.NormalizePayload("payroll_run", map[string]any{
		"payroll_period_id": period.ID,
		"employee_ids":      []string{employee.ID},
	})
	lines, _ := payrollPayload["payroll_lines"].([]map[string]any)
	if len(lines) != 1 {
		t.Fatalf("expected payroll line for leave period, got %d", len(lines))
	}
	if got := numberValue(lines[0]["deductible_leave_days"]); got != 1 {
		t.Fatalf("expected deductible_leave_days 1 from approved unpaid leave, got %v", got)
	}

	reloaded := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(reloaded.config, reloaded.identity, reloaded.modules, reloaded.models, reloaded.reporting, reloaded.templates, reloaded.reference, reloaded.search, reloaded.documents, reloaded.workflows, reloaded.policy, nil, "bootstrap-123!"); err != nil {
		t.Fatalf("reseed platform kernel: %v", err)
	}
	reloadedRequest, err := reloaded.models.Get("leave_request", request.ID)
	if err != nil {
		t.Fatalf("reload leave request: %v", err)
	}
	if got := textValue(reloadedRequest.Values["approval_status"]); got != "approved" {
		t.Fatalf("expected reloaded leave request approved, got %s", got)
	}
	reloadedAccount, err := reloaded.models.Get("leave_balance_account", textValue(request.Values["balance_account_id"]))
	if err != nil {
		t.Fatalf("reload leave balance account: %v", err)
	}
	if got := numberValue(reloadedAccount.Values["available_days"]); got != 12.5 {
		t.Fatalf("expected reloaded available_days 12.5, got %v", got)
	}
	days, _, err := reloaded.models.List("attendance_day", model.Query{
		Filters: map[string]string{
		"employee_id":     employee.ID,
		"attendance_date": "2099-02-10",
		},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("reload attendance days: %v", err)
	}
	if len(days) == 0 || textValue(days[0].Values["attendance_status"]) != "on_leave" {
		t.Fatalf("expected persisted attendance day on_leave, got %+v", days)
	}
}
