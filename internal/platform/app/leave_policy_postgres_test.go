package app

import (
	"encoding/json"
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
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, nil, testBootstrapAdminPassword); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	userID := "leave_pg_user_" + suffix
	baseYear := 2100 + time.Now().UTC().Nanosecond()%400
	grantDate := fmt.Sprintf("%04d-01-01", baseYear)
	requestDay := firstWeekdayOfMonth(baseYear, time.February)
	amendedDay := nextWeekday(requestDay.AddDate(0, 0, 1))
	cutoffDay := nextWeekday(amendedDay.AddDate(0, 0, 1))
	requestDate := requestDay.Format("2006-01-02")
	amendedDate := amendedDay.Format("2006-01-02")
	cutoffDate := cutoffDay.Format("2006-01-02")
	periodStart := time.Date(baseYear, time.February, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	periodEnd := time.Date(baseYear, time.February+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	q1ExpiryDate := fmt.Sprintf("%04d-03-31", baseYear)
	expiryTriggerDate := fmt.Sprintf("%04d-04-15", baseYear)
	monthlyEffectiveFrom := nextWeekday(time.Date(baseYear, time.February, 15, 0, 0, 0, 0, time.UTC)).Format("2006-01-02")
	monthlyRunDate := time.Date(baseYear, time.February+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	rosterStart := firstWeekdayOfMonth(baseYear, time.March)
	rosterEnd := nextWeekday(rosterStart.AddDate(0, 0, 2))
	rosterStartDate := rosterStart.Format("2006-01-02")
	rosterEndDate := rosterEnd.Format("2006-01-02")
	party := ensurePartyRecord(t, graph.models, "user_admin", "party_leave_pg_"+suffix, "Leave Party "+suffix)
	ensureLocationRecord(t, graph.models, "user_admin", "loc_hq")
	ensureOrganizationUnitRecord(t, graph.models, "user_admin", "ou_leave", "org_default", "loc_hq")
	ensureDepartmentRecord(t, graph.models, "user_admin", "dept_leave", "org_default", "loc_hq", "ou_leave")
	ensureCostCenterRecord(t, graph.models, "user_admin", "cc_leave", "org_default", "loc_hq", "ou_leave", "dept_leave")

	employee, err := graph.models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          party.ID,
		"user_id":           userID,
		"employee_code":     "EMP-LEAVE-PG-" + suffix,
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if _, err := graph.models.Create("employee_assignment", "user_admin", map[string]any{
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
	if _, err := graph.models.Create("work_calendar", "user_admin", map[string]any{
		"code":               "CAL-LEAVE-PG-" + suffix,
		"name":               "Leave Work Calendar",
		"organization_id":    "org_default",
		"location_id":        "loc_hq",
		"working_days_json":  `["monday","tuesday","wednesday","thursday","friday"]`,
		"holiday_dates_json": fmt.Sprintf(`["%s"]`, cutoffDate),
		"status":             "active",
	}); err != nil {
		t.Fatalf("create work calendar: %v", err)
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
		"effective_date":  grantDate,
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
		"leave_policy_id":  leavePolicy.ID,
		"start_date":       requestDate,
		"end_date":         requestDate,
		"request_unit":     "half_day",
		"half_day_session": "morning",
		"notes":            "Medical appointment",
	}, userID)
	if err != nil {
		t.Fatalf("create self-service leave request: %v", err)
	}
	if got := textValue(request.Values["count_basis"]); got != "calendar" {
		t.Fatalf("expected calendar count basis on initial request, got %s", got)
	}
	request, err = graph.leavePolicies.SubmitSelfServiceLeaveRequest(userID, request.ID, userID)
	if err != nil {
		t.Fatalf("submit self-service leave request: %v", err)
	}
	inboxItems, err := graph.leavePolicies.InboxRequestSummariesForActor("user_admin", map[string]string{"bucket": "actionable"})
	if err != nil {
		t.Fatalf("load approval inbox summaries: %v", err)
	}
	if len(inboxItems) == 0 {
		t.Fatalf("expected actionable leave approval inbox items after submit")
	}
	inboxDetail, err := graph.leavePolicies.RequestSummaryForInboxActor(request.ID, "user_admin")
	if err != nil {
		t.Fatalf("load approval inbox detail: %v", err)
	}
	if actionable, _ := inboxDetail["is_actionable"].(bool); !actionable {
		t.Fatalf("expected approval inbox detail to be actionable after submit, got %+v", inboxDetail)
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
	request, err = graph.leavePolicies.AmendManagedLeaveRequest(request.ID, map[string]any{
		"leave_policy_id":  leavePolicy.ID,
		"start_date":       amendedDate,
		"end_date":         amendedDate,
		"request_unit":     "day",
		"half_day_session": "",
		"notes":            "Rescheduled appointment",
		"reason":           "team coverage change",
	}, "user_admin")
	if err != nil {
		t.Fatalf("amend approved leave request before cutoff: %v", err)
	}
	if got := textValue(request.Values["approval_status"]); got != "submitted" {
		t.Fatalf("expected submitted after approved amendment, got %s", got)
	}
	days, _, err := graph.models.List("attendance_day", model.Query{
		Filters: map[string]string{
			"employee_id":     employee.ID,
			"attendance_date": requestDate,
		},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list attendance days after amendment: %v", err)
	}
	if len(days) > 0 && textValue(days[0].Values["attendance_status"]) == "on_leave" {
		t.Fatalf("expected old attendance leave to clear after amendment, got %+v", days)
	}
	request, err = graph.leavePolicies.ApproveLeaveRequest(request.ID, "user_admin")
	if err != nil {
		t.Fatalf("reapprove amended leave request: %v", err)
	}
	days, _, err = graph.models.List("attendance_day", model.Query{
		Filters: map[string]string{
			"employee_id":     employee.ID,
			"attendance_date": amendedDate,
		},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list attendance days after reapproval: %v", err)
	}
	if len(days) == 0 || textValue(days[0].Values["attendance_status"]) != "on_leave" {
		t.Fatalf("expected amended attendance day on_leave, got %+v", days)
	}

	account, err := graph.models.Get("leave_balance_account", textValue(request.Values["balance_account_id"]))
	if err != nil {
		t.Fatalf("get balance account: %v", err)
	}
	if got := numberValue(account.Values["available_days"]); got != 11 {
		t.Fatalf("expected available_days 11 after amended full-day approval with opening balance expired, got %v", got)
	}
	userBalances, err := graph.leavePolicies.BalanceSummaryForUser(userID)
	if err != nil {
		t.Fatalf("list employee leave balances: %v", err)
	}
	if len(userBalances) == 0 {
		t.Fatalf("expected employee leave balances after approval")
	}
	if got := numberValue(userBalances[0]["available_days"]); got != 11 {
		t.Fatalf("expected balance summary available_days 11 after approval, got %v", got)
	}
	roster, err := graph.models.Create("workforce_roster", "user_admin", map[string]any{
		"code":            "RST-LEAVE-PG-" + suffix,
		"name":            "Leave Roster",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"start_date":      rosterStartDate,
		"end_date":        rosterEndDate,
		"status":          "published",
	})
	if err != nil {
		t.Fatalf("create workforce roster: %v", err)
	}
	for _, shiftDate := range []string{rosterStartDate, rosterEndDate} {
		if _, err := graph.models.Create("workforce_roster_slot", "user_admin", map[string]any{
			"roster_id":          roster.ID,
			"employee_id":        employee.ID,
			"organization_id":    "org_default",
			"location_id":        "loc_hq",
			"shift_date":         shiftDate,
			"planned_start_time": "08:00",
			"planned_end_time":   "16:00",
			"break_minutes":      60.0,
			"status":             "active",
		}); err != nil {
			t.Fatalf("create roster slot: %v", err)
		}
	}
	rosterRequest, err := graph.leavePolicies.CreateSelfServiceLeaveRequest(userID, map[string]any{
		"leave_policy_id": leavePolicy.ID,
		"start_date":      rosterStartDate,
		"end_date":        rosterEndDate,
	}, userID)
	if err != nil {
		t.Fatalf("create roster-based leave request: %v", err)
	}
	if got := textValue(rosterRequest.Values["count_basis"]); got != "roster" {
		t.Fatalf("expected roster count basis, got %s", got)
	}
	if got := numberValue(rosterRequest.Values["requested_days"]); got != 2 {
		t.Fatalf("expected 2 roster-counted days, got %v", got)
	}
	if got := numberValue(rosterRequest.Values["requested_hours"]); got != 14 {
		t.Fatalf("expected 14 roster-counted hours, got %v", got)
	}
	var countedDates []string
	_ = json.Unmarshal([]byte(textValue(rosterRequest.Values["counted_dates_json"])), &countedDates)
	if len(countedDates) != 2 || countedDates[0] != rosterStartDate || countedDates[1] != rosterEndDate {
		t.Fatalf("expected counted roster dates [%s,%s], got %+v", rosterStartDate, rosterEndDate, countedDates)
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
		"start_date":      periodStart,
		"end_date":        periodEnd,
		"pay_date":        periodEnd,
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
	runPayload := graph.employeePayroll.NormalizePayload("payroll_run", map[string]any{
		"payroll_period_id": period.ID,
		"employee_ids":      []string{employee.ID},
	})
	runDoc, err := graph.documents.Create("payroll_run", "org_default", "loc_hq", "user_admin", runPayload)
	if err != nil {
		t.Fatalf("create payroll run document: %v", err)
	}
	runDoc.Header.Status = "processed"
	if err := graph.documents.Save(runDoc); err != nil {
		t.Fatalf("save processed payroll run document: %v", err)
	}
	if _, err := graph.leavePolicies.AmendManagedLeaveRequest(request.ID, map[string]any{
		"leave_policy_id":  leavePolicy.ID,
		"start_date":       cutoffDate,
		"end_date":         cutoffDate,
		"request_unit":     "day",
		"half_day_session": "",
		"reason":           "post payroll cutoff",
	}, "user_admin"); err == nil {
		t.Fatal("expected processed payroll run to block approved leave amendment")
	}
	if _, err := graph.leavePolicies.CancelApprovedLeaveRequest(request.ID, "user_admin", "validation reversal"); err == nil {
		t.Fatal("expected processed payroll run to block approved leave cancellation")
	}

	cfPolicy, err := graph.models.Create("leave_policy", "user_admin", map[string]any{
		"code":             "LP-CF-" + suffix,
		"name":             "Carry Forward Leave",
		"absence_code_id":  absenceCode.ID,
		"paid_leave":       false,
		"requires_balance": true,
		"organization_id":  "org_default",
		"location_id":      "loc_hq",
		"status":           "active",
	})
	if err != nil {
		t.Fatalf("create carry forward leave policy: %v", err)
	}
	if _, err := graph.models.Create("leave_entitlement_rule", "user_admin", map[string]any{
		"leave_policy_id":           cfPolicy.ID,
		"grant_mode":                "annual_grant",
		"annual_entitlement_days":   12.0,
		"carry_forward_cap_days":    5.0,
		"carry_forward_expiry_rule": "q1_end",
		"status":                    "active",
	}); err != nil {
		t.Fatalf("create carry forward entitlement rule: %v", err)
	}
	if _, err := graph.models.Create("employee_leave_profile", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"leave_policy_id":      cfPolicy.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_hq",
		"effective_from":       "2000-01-01",
		"opening_balance_days": 8.0,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create carry forward leave profile: %v", err)
	}
	cfRun, err := graph.models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":            "LAR-CF-" + suffix,
		"name":            "Carry Forward Grant",
		"leave_policy_id": cfPolicy.ID,
		"run_mode":        "annual_grant",
		"effective_date":  grantDate,
		"status":          "active",
		"run_status":      "draft",
	})
	if err != nil {
		t.Fatalf("create carry forward accrual run: %v", err)
	}
	if _, err := graph.leavePolicies.ExecuteAccrualRun(cfRun.ID, "user_admin"); err != nil {
		t.Fatalf("execute carry forward accrual run: %v", err)
	}
	cfAccounts, _, err := graph.models.List("leave_balance_account", model.Query{
		Filters:  map[string]string{"employee_id": employee.ID, "leave_policy_id": cfPolicy.ID},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(cfAccounts) == 0 {
		t.Fatalf("list carry forward leave balance accounts: %v", err)
	}
	if got := numberValue(cfAccounts[0].Values["carry_forward_balance_days"]); got != 5 {
		t.Fatalf("expected carry forward balance 5, got %v", got)
	}
	if got := textValue(cfAccounts[0].Values["carry_forward_expiry_date"]); got != q1ExpiryDate {
		t.Fatalf("expected carry forward expiry date %s, got %s", q1ExpiryDate, got)
	}
	expiryRun, err := graph.models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":            "LAR-CF-EXP-" + suffix,
		"name":            "Carry Forward Expiry Trigger",
		"leave_policy_id": cfPolicy.ID,
		"run_mode":        "monthly_accrual",
		"effective_date":  expiryTriggerDate,
		"status":          "active",
		"run_status":      "draft",
	})
	if err != nil {
		t.Fatalf("create carry forward expiry trigger run: %v", err)
	}
	if _, err := graph.leavePolicies.ExecuteAccrualRun(expiryRun.ID, "user_admin"); err != nil {
		t.Fatalf("execute carry forward expiry trigger run: %v", err)
	}
	cfAccount, err := graph.models.Get("leave_balance_account", cfAccounts[0].ID)
	if err != nil {
		t.Fatalf("reload carry forward account: %v", err)
	}
	if got := numberValue(cfAccount.Values["carry_forward_balance_days"]); got != 0 {
		t.Fatalf("expected carry forward balance to expire to 0, got %v", got)
	}

	monthlyPolicy, err := graph.models.Create("leave_policy", "user_admin", map[string]any{
		"code":             "LP-MONTH-" + suffix,
		"name":             "Monthly Accrual Leave",
		"absence_code_id":  absenceCode.ID,
		"paid_leave":       false,
		"requires_balance": true,
		"organization_id":  "org_default",
		"location_id":      "loc_hq",
		"status":           "active",
	})
	if err != nil {
		t.Fatalf("create monthly accrual leave policy: %v", err)
	}
	if _, err := graph.models.Create("leave_entitlement_rule", "user_admin", map[string]any{
		"leave_policy_id":      monthlyPolicy.ID,
		"grant_mode":           "monthly_accrual",
		"monthly_accrual_days": 2.0,
		"prorate_on_join":      true,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create monthly accrual entitlement rule: %v", err)
	}
	if _, err := graph.models.Create("employee_leave_profile", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"leave_policy_id":      monthlyPolicy.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_hq",
		"effective_from":       monthlyEffectiveFrom,
		"opening_balance_days": 0.0,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create monthly accrual leave profile: %v", err)
	}
	monthlyRun, err := graph.models.Create("leave_accrual_run", "user_admin", map[string]any{
		"code":            "LAR-MONTH-" + suffix,
		"name":            "Monthly Accrual Grant",
		"leave_policy_id": monthlyPolicy.ID,
		"run_mode":        "monthly_accrual",
		"effective_date":  monthlyRunDate,
		"status":          "active",
		"run_status":      "draft",
	})
	if err != nil {
		t.Fatalf("create monthly accrual run: %v", err)
	}
	if _, err := graph.leavePolicies.ExecuteAccrualRun(monthlyRun.ID, "user_admin"); err != nil {
		t.Fatalf("execute monthly accrual run: %v", err)
	}
	monthlyAccounts, _, err := graph.models.List("leave_balance_account", model.Query{
		Filters:  map[string]string{"employee_id": employee.ID, "leave_policy_id": monthlyPolicy.ID},
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(monthlyAccounts) == 0 {
		t.Fatalf("list monthly accrual accounts: %v", err)
	}
	if got := numberValue(monthlyAccounts[0].Values["available_days"]); got != 1 {
		t.Fatalf("expected prorated monthly accrual available_days 1, got %v", got)
	}

	reloaded := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(reloaded.config, reloaded.identity, reloaded.modules, reloaded.models, reloaded.reporting, reloaded.templates, reloaded.reference, reloaded.search, reloaded.documents, reloaded.workflows, reloaded.policy, nil, testBootstrapAdminPassword); err != nil {
		t.Fatalf("reseed platform kernel: %v", err)
	}
	reloadedRequest, err := reloaded.models.Get("leave_request", request.ID)
	if err != nil {
		t.Fatalf("reload leave request: %v", err)
	}
	if got := textValue(reloadedRequest.Values["approval_status"]); got != "approved" {
		t.Fatalf("expected reloaded leave request approved after cutoff block, got %s", got)
	}
	reloadedAccount, err := reloaded.models.Get("leave_balance_account", textValue(request.Values["balance_account_id"]))
	if err != nil {
		t.Fatalf("reload leave balance account: %v", err)
	}
	if got := numberValue(reloadedAccount.Values["available_days"]); got != 11 {
		t.Fatalf("expected reloaded available_days 11 after approved amendment, got %v", got)
	}
	days, _, err = reloaded.models.List("attendance_day", model.Query{
		Filters: map[string]string{
			"employee_id":     employee.ID,
			"attendance_date": amendedDate,
		},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("reload attendance days: %v", err)
	}
	if len(days) == 0 || textValue(days[0].Values["attendance_status"]) != "on_leave" {
		t.Fatalf("expected persisted attendance day on_leave after approved amendment, got %+v", days)
	}
}

func firstWeekdayOfMonth(year int, month time.Month) time.Time {
	day := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	return nextWeekday(day)
}

func nextWeekday(day time.Time) time.Time {
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	return day
}
