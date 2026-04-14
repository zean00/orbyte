package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/store"
)

func TestEmployeePayrollPostgresValidation(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed payroll test")
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

	orgID := "org_default"
	locID := "loc_hq"
	suffix := time.Now().UTC().Format("20060102150405")
	party := ensurePartyRecord(t, graph.models, "user_admin", "party_payroll_"+suffix, "Payroll Party "+suffix)
	approverUser, err := graph.identity.CreateUser("payroll-approver-"+suffix, testBootstrapAdminPassword, locID, "role_admin", "location", locID)
	if err != nil {
		t.Fatalf("create approver user: %v", err)
	}
	employee, err := graph.models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          party.ID,
		"user_id":           "user_admin",
		"employee_code":     "EMP-PAY-" + suffix,
		"employment_status": "active",
		"status":            "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if _, err := graph.models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"organization_id":      orgID,
		"location_id":          locID,
		"organization_unit_id": "ou_payroll_" + suffix,
		"department_id":        "dept_payroll_" + suffix,
		"cost_center_id":       "cc_payroll_" + suffix,
		"effective_from":       "2099-10-01",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	if _, err := graph.models.Create("employee_compensation_profile", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"currency_code":        "IDR",
		"standard_hourly_rate": 10.0,
		"overtime_hourly_rate": 15.0,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create compensation profile: %v", err)
	}
	absenceCode, err := graph.models.Create("absence_code", "user_admin", map[string]any{
		"code":                "LV-" + suffix,
		"name":                "Payroll Leave",
		"category":            "leave",
		"deduct_from_payroll": true,
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create absence code: %v", err)
	}
	leave, err := graph.models.Create("leave_request", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"organization_id": orgID,
		"location_id":     locID,
		"absence_code_id": absenceCode.ID,
		"start_date":      "2099-10-06",
		"end_date":        "2099-10-06",
		"approval_status": "approved",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	if _, err := graph.models.Create("overtime_request", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"attendance_date": "2099-10-05",
		"organization_id": orgID,
		"location_id":     locID,
		"requested_hours": 5.0,
		"approved_hours":  5.0,
		"approval_status": "approved",
		"status":          "active",
	}); err != nil {
		t.Fatalf("create overtime request: %v", err)
	}
	if _, err := graph.models.Create("attendance_day", "user_admin", map[string]any{
		"employee_id":       employee.ID,
		"attendance_date":   "2099-10-05",
		"organization_id":   orgID,
		"location_id":       locID,
		"worked_hours":      160.0,
		"overtime_hours":    5.0,
		"attendance_status": "present",
		"status":            "active",
	}); err != nil {
		t.Fatalf("create attendance day: %v", err)
	}
	if _, err := graph.models.Create("attendance_day", "user_admin", map[string]any{
		"employee_id":       employee.ID,
		"attendance_date":   "2099-10-06",
		"organization_id":   orgID,
		"location_id":       locID,
		"attendance_status": "on_leave",
		"leave_request_id":  leave.ID,
		"status":            "active",
	}); err != nil {
		t.Fatalf("create leave attendance day: %v", err)
	}

	for _, component := range []map[string]any{
		{"code": "BASIC-" + suffix, "name": "Basic Salary", "component_class": "earning", "status": "active"},
		{"code": "OT-" + suffix, "name": "Overtime", "component_class": "earning", "status": "active"},
		{"code": "LEAVE-" + suffix, "name": "Leave Deduction", "component_class": "deduction", "status": "active"},
		{"code": "REIM-" + suffix, "name": "Payroll Reimbursement", "component_class": "reimbursement", "status": "active"},
	} {
		if _, err := graph.models.Create("pay_component", "user_admin", component); err != nil {
			t.Fatalf("create pay component %s: %v", component["code"], err)
		}
	}
	structure, err := graph.models.Create("salary_structure", "user_admin", map[string]any{
		"code":            "SAL-" + suffix,
		"name":            "Monthly Payroll",
		"organization_id": orgID,
		"location_id":     locID,
		"currency_code":   "IDR",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create salary structure: %v", err)
	}
	for _, line := range []map[string]any{
		{"salary_structure_id": structure.ID, "component_code": "BASIC-" + suffix, "sequence": 1, "formula_key": "fixed_amount", "fixed_amount": 1000.0, "status": "active"},
		{"salary_structure_id": structure.ID, "component_code": "OT-" + suffix, "sequence": 2, "formula_key": "overtime_hours", "status": "active"},
		{"salary_structure_id": structure.ID, "component_code": "LEAVE-" + suffix, "sequence": 3, "formula_key": "leave_deduction", "status": "active"},
		{"salary_structure_id": structure.ID, "component_code": "REIM-" + suffix, "sequence": 4, "formula_key": "reimbursement", "status": "active"},
	} {
		if _, err := graph.models.Create("salary_structure_line", "user_admin", line); err != nil {
			t.Fatalf("create salary structure line %s: %v", line["component_code"], err)
		}
	}
	if _, err := graph.models.Create("employee_payroll_profile", "user_admin", map[string]any{
		"employee_id":                employee.ID,
		"salary_structure_id":        structure.ID,
		"currency_code":              "IDR",
		"payment_method_code":        "BANK",
		"treasury_account_id":        "treasury_payroll_" + suffix,
		"reimbursement_in_payroll":   true,
		"leave_deduction_daily_rate": 20.0,
		"status":                     "active",
	}); err != nil {
		t.Fatalf("create payroll profile: %v", err)
	}
	period, err := graph.models.Create("payroll_period", "user_admin", map[string]any{
		"code":            "PR-" + suffix,
		"name":            "October 2099",
		"organization_id": orgID,
		"location_id":     locID,
		"start_date":      "2099-10-01",
		"end_date":        "2099-10-31",
		"pay_date":        "2099-10-31",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create payroll period: %v", err)
	}

	reimbursement, err := graph.documents.Create("reimbursement_payment", orgID, locID, "user_admin", map[string]any{
		"employee_id":        employee.ID,
		"party_id":           textValue(employee.Values["party_id"]),
		"payment_date":       "2099-10-28",
		"amount_paid":        75.0,
		"include_in_payroll": true,
		"currency_code":      "IDR",
		"total_amount":       75.0,
	})
	if err != nil {
		t.Fatalf("create reimbursement payment: %v", err)
	}
	reimbursement, err = graph.docActions.Submit(reimbursement.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit reimbursement payment: %v", err)
	}
	reimbursement, err = graph.docActions.Approve(reimbursement.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve reimbursement payment: %v", err)
	}
	if reimbursement.Header.Status != "paid" {
		t.Fatalf("expected reimbursement payment status paid, got %s", reimbursement.Header.Status)
	}

	runPayload := graph.employeePayroll.NormalizePayload("payroll_run", map[string]any{
		"payroll_period_id": period.ID,
		"employee_ids":      []string{employee.ID},
	})
	run, err := graph.documents.Create("payroll_run", orgID, locID, "user_admin", runPayload)
	if err != nil {
		t.Fatalf("create payroll run: %v", err)
	}
	run, err = graph.docActions.Submit(run.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit payroll run: %v", err)
	}
	run, err = graph.docActions.Approve(run.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve payroll run: %v", err)
	}
	if run.Header.Status != "processed" {
		t.Fatalf("expected payroll run status processed, got %s", run.Header.Status)
	}

	batch, payments, posting, err := graph.employeePayroll.CreatePaymentBatchFromRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create payment batch from payroll run: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("expected 1 payroll payment, got %d", len(payments))
	}
	batch, err = graph.docActions.Submit(batch.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit payroll payment batch: %v", err)
	}
	batch, err = graph.docActions.Approve(batch.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve payroll payment batch: %v", err)
	}
	payments[0], err = graph.docActions.Submit(payments[0].Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit payroll payment: %v", err)
	}
	payments[0], err = graph.docActions.Approve(payments[0].Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve payroll payment: %v", err)
	}

	reloaded := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(reloaded.config, reloaded.identity, reloaded.modules, reloaded.models, reloaded.reporting, reloaded.templates, reloaded.reference, reloaded.search, reloaded.documents, reloaded.workflows, reloaded.policy, nil, testBootstrapAdminPassword); err != nil {
		t.Fatalf("reseed platform kernel: %v", err)
	}
	persistedRun, err := reloaded.documents.Get(run.Header.ID)
	if err != nil {
		t.Fatalf("get persisted payroll run: %v", err)
	}
	if persistedRun.Header.Status != "processed" {
		t.Fatalf("expected persisted payroll run status processed, got %s", persistedRun.Header.Status)
	}
	if got := numberValue(persistedRun.Body.Payload["net_pay_total"]); got <= 0 {
		t.Fatalf("expected persisted payroll run net_pay_total > 0, got %v", got)
	}
	persistedBatch, err := reloaded.documents.Get(batch.Header.ID)
	if err != nil {
		t.Fatalf("get persisted payroll batch: %v", err)
	}
	if got := textValue(persistedBatch.Body.Payload["ledger_posting_id"]); got != posting.Header.ID {
		t.Fatalf("expected persisted ledger_posting_id %s, got %q", posting.Header.ID, got)
	}
	persistedPayment, err := reloaded.documents.Get(payments[0].Header.ID)
	if err != nil {
		t.Fatalf("get persisted payroll payment: %v", err)
	}
	if persistedPayment.Header.Status != "paid" {
		t.Fatalf("expected persisted payroll payment status paid, got %s", persistedPayment.Header.Status)
	}
}
