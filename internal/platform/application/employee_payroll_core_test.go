package application

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestEmployeePayrollNormalizePayloadComputesCrossDomainTotals(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	service := NewEmployeePayrollCoreService(docs, models, nil, nil)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id": "party_emp",
		"status":   "active",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	if _, err := models.Create("employee_assignment", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"organization_id":      "org_default",
		"location_id":          "loc_hq",
		"organization_unit_id": "ou_payroll",
		"department_id":        "dept_payroll",
		"cost_center_id":       "cc_payroll",
		"effective_from":       "2099-10-01",
		"status":               "active",
	}); err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	if _, err := models.Create("employee_compensation_profile", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"currency_code":        "IDR",
		"standard_hourly_rate": 10.0,
		"overtime_hourly_rate": 15.0,
		"status":               "active",
	}); err != nil {
		t.Fatalf("create compensation profile: %v", err)
	}
	if _, err := models.Create("employee_payroll_profile", "user_admin", map[string]any{
		"employee_id":                employee.ID,
		"salary_structure_id":        "struct_1",
		"currency_code":              "IDR",
		"payment_method_code":        "BANK",
		"treasury_account_id":        "treasury_main",
		"reimbursement_in_payroll":   true,
		"leave_deduction_daily_rate": 20.0,
		"status":                     "active",
	}); err != nil {
		t.Fatalf("create payroll profile: %v", err)
	}
	for _, component := range []map[string]any{
		{"code": "BASIC", "name": "Basic", "component_class": "earning", "status": "active"},
		{"code": "OT", "name": "Overtime", "component_class": "earning", "status": "active"},
		{"code": "LEAVE", "name": "Leave Deduction", "component_class": "deduction", "status": "active"},
		{"code": "REIM", "name": "Reimbursement", "component_class": "reimbursement", "status": "active"},
	} {
		if _, err := models.Create("pay_component", "user_admin", component); err != nil {
			t.Fatalf("create pay component %s: %v", component["code"], err)
		}
	}
	for _, line := range []map[string]any{
		{"salary_structure_id": "struct_1", "component_code": "BASIC", "sequence": 1, "formula_key": "fixed_amount", "fixed_amount": 1000.0, "status": "active"},
		{"salary_structure_id": "struct_1", "component_code": "OT", "sequence": 2, "formula_key": "overtime_hours", "status": "active"},
		{"salary_structure_id": "struct_1", "component_code": "LEAVE", "sequence": 3, "formula_key": "leave_deduction", "status": "active"},
		{"salary_structure_id": "struct_1", "component_code": "REIM", "sequence": 4, "formula_key": "reimbursement", "status": "active"},
	} {
		if _, err := models.Create("salary_structure_line", "user_admin", line); err != nil {
			t.Fatalf("create salary structure line %s: %v", line["component_code"], err)
		}
	}
	absenceCode, err := models.Create("absence_code", "user_admin", map[string]any{
		"code":                "LV",
		"name":                "Leave",
		"deduct_from_payroll": true,
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create absence code: %v", err)
	}
	leave, err := models.Create("leave_request", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"absence_code_id": absenceCode.ID,
		"start_date":      "2099-10-06",
		"end_date":        "2099-10-06",
		"approval_status": "approved",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	if _, err := models.Create("attendance_day", "user_admin", map[string]any{
		"employee_id":     employee.ID,
		"attendance_date": "2099-10-05",
		"worked_hours":    160.0,
		"overtime_hours":  10.0,
		"status":          "active",
	}); err != nil {
		t.Fatalf("create attendance day: %v", err)
	}
	if _, err := models.Create("attendance_day", "user_admin", map[string]any{
		"employee_id":      employee.ID,
		"attendance_date":  "2099-10-06",
		"leave_request_id": leave.ID,
		"status":           "active",
	}); err != nil {
		t.Fatalf("create leave attendance day: %v", err)
	}
	reimbursement, err := docs.Create("reimbursement_payment", "org_default", "loc_hq", "user_admin", map[string]any{
		"employee_id":        employee.ID,
		"payment_date":       "2099-10-28",
		"amount_paid":        50.0,
		"include_in_payroll": true,
	})
	if err != nil {
		t.Fatalf("create reimbursement payment: %v", err)
	}
	reimbursement.Header.Status = "paid"
	if err := docs.Save(reimbursement); err != nil {
		t.Fatalf("save reimbursement payment: %v", err)
	}
	period, err := models.Create("payroll_period", "user_admin", map[string]any{
		"code":            "PR-2099-10",
		"name":            "October 2099",
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"start_date":      "2099-10-01",
		"end_date":        "2099-10-31",
		"pay_date":        "2099-10-31",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create payroll period: %v", err)
	}

	payload := service.NormalizePayload("payroll_run", map[string]any{
		"payroll_period_id": period.ID,
		"employee_ids":      []string{employee.ID},
	})

	if got := numberValue(payload["employee_count"]); got != 1 {
		t.Fatalf("expected employee_count 1, got %v", got)
	}
	if got := numberValue(payload["gross_pay_total"]); got != 1150.0 {
		t.Fatalf("expected gross_pay_total 1150, got %v", got)
	}
	if got := numberValue(payload["employee_deductions_total"]); got != 20.0 {
		t.Fatalf("expected employee_deductions_total 20, got %v", got)
	}
	if got := numberValue(payload["reimbursement_total"]); got != 50.0 {
		t.Fatalf("expected reimbursement_total 50, got %v", got)
	}
	if got := numberValue(payload["net_pay_total"]); got != 1180.0 {
		t.Fatalf("expected net_pay_total 1180, got %v", got)
	}
	if got := numberValue(payload["employer_cost_total"]); got != 1200.0 {
		t.Fatalf("expected employer_cost_total 1200, got %v", got)
	}
	lines := recordList(payload["payroll_lines"])
	if len(lines) != 1 {
		t.Fatalf("expected 1 payroll line, got %d", len(lines))
	}
	if got := numberValue(lines[0]["worked_hours"]); got != 160.0 {
		t.Fatalf("expected worked_hours 160, got %v", got)
	}
	if got := numberValue(lines[0]["overtime_hours"]); got != 10.0 {
		t.Fatalf("expected overtime_hours 10, got %v", got)
	}
	if got := numberValue(lines[0]["deductible_leave_days"]); got != 1.0 {
		t.Fatalf("expected deductible_leave_days 1, got %v", got)
	}
}

func TestEmployeePayrollCreatePaymentBatchFromProcessedRun(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	service := NewEmployeePayrollCoreService(docs, models, nil, nil)

	run, err := docs.Create("payroll_run", "org_default", "loc_hq", "user_admin", map[string]any{
		"payroll_period_id":   "period_1",
		"pay_date":            "2099-10-31",
		"currency_code":       "IDR",
		"treasury_account_id": "treasury_main",
		"payment_method_code": "BANK",
		"net_pay_total":       1180.0,
		"employer_cost_total": 1200.0,
		"payroll_lines": []map[string]any{{
			"employee_id":          "emp_1",
			"party_id":             "party_emp",
			"organization_unit_id": "ou_payroll",
			"department_id":        "dept_payroll",
			"cost_center_id":       "cc_payroll",
			"payment_method_code":  "BANK",
			"treasury_account_id":  "treasury_main",
			"net_pay":              1180.0,
		}},
	})
	if err != nil {
		t.Fatalf("create payroll run: %v", err)
	}
	run.Header.Status = "processed"
	if err := docs.Save(run); err != nil {
		t.Fatalf("save payroll run: %v", err)
	}

	batch, payments, posting, err := service.CreatePaymentBatchFromRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create payment batch from run: %v", err)
	}
	if batch.Header.Type != "payroll_payment_batch" {
		t.Fatalf("expected payroll_payment_batch, got %s", batch.Header.Type)
	}
	if len(payments) != 1 {
		t.Fatalf("expected 1 payroll payment, got %d", len(payments))
	}
	if payments[0].Header.Type != "payroll_payment" {
		t.Fatalf("expected payroll_payment, got %s", payments[0].Header.Type)
	}
	if posting.Header.Type != "ledger_posting" {
		t.Fatalf("expected ledger_posting, got %s", posting.Header.Type)
	}
	if posting.Header.Status != "posted" {
		t.Fatalf("expected ledger_posting status posted, got %s", posting.Header.Status)
	}
	if got := len(payrollStringList(batch.Body.Payload["payroll_payment_ids"])); got != 1 {
		t.Fatalf("expected 1 payroll payment id on batch, got %d", got)
	}
	if got := textValue(batch.Body.Payload["ledger_posting_id"]); got != posting.Header.ID {
		t.Fatalf("expected ledger_posting_id %s, got %q", posting.Header.ID, got)
	}
}

func TestEmployeePayrollCreatePaymentBatchFromProcessedRunRejectsDuplicate(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	service := NewEmployeePayrollCoreService(docs, models, nil, nil)

	run, err := docs.Create("payroll_run", "org_default", "loc_hq", "user_admin", map[string]any{
		"payroll_period_id":   "period_1",
		"pay_date":            "2099-10-31",
		"currency_code":       "IDR",
		"treasury_account_id": "treasury_main",
		"payment_method_code": "BANK",
		"net_pay_total":       1180.0,
		"employer_cost_total": 1200.0,
		"payroll_lines": []map[string]any{{
			"employee_id": "emp_1",
			"party_id":    "party_emp",
			"net_pay":     1180.0,
		}},
	})
	if err != nil {
		t.Fatalf("create payroll run: %v", err)
	}
	run.Header.Status = "processed"
	if err := docs.Save(run); err != nil {
		t.Fatalf("save payroll run: %v", err)
	}

	if _, _, _, err := service.CreatePaymentBatchFromRun(run.Header.ID, "user_admin"); err != nil {
		t.Fatalf("first batch generation: %v", err)
	}
	if _, _, _, err := service.CreatePaymentBatchFromRun(run.Header.ID, "user_admin"); err == nil {
		t.Fatal("expected duplicate batch generation to be rejected")
	}
}

func registerEmployeePayrollTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{Key: "employee_profile", DisplayName: "Employee Profile", DefaultSort: "party_id", Fields: []model.FieldDefinition{{Key: "party_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "organization_unit_id", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "cost_center_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "employee_assignment", DisplayName: "Employee Assignment", DefaultSort: "effective_from", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "organization_unit_id", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "cost_center_id", Type: "string"}, {Key: "effective_from", Type: "string"}, {Key: "effective_to", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "employee_compensation_profile", DisplayName: "Employee Compensation Profile", DefaultSort: "employee_id", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "currency_code", Type: "string"}, {Key: "standard_hourly_rate", Type: "number"}, {Key: "overtime_hourly_rate", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "attendance_day", DisplayName: "Attendance Day", DefaultSort: "attendance_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "attendance_date", Type: "string"}, {Key: "worked_hours", Type: "number"}, {Key: "overtime_hours", Type: "number"}, {Key: "leave_request_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "absence_code", DisplayName: "Absence Code", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "deduct_from_payroll", Type: "bool"}, {Key: "status", Type: "string"}}},
		{Key: "leave_request", DisplayName: "Leave Request", DefaultSort: "start_date", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "absence_code_id", Type: "string"}, {Key: "start_date", Type: "string"}, {Key: "end_date", Type: "string"}, {Key: "approval_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "pay_component", DisplayName: "Pay Component", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "component_class", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "salary_structure_line", DisplayName: "Salary Structure Line", DefaultSort: "sequence", Fields: []model.FieldDefinition{{Key: "salary_structure_id", Type: "string"}, {Key: "component_code", Type: "string"}, {Key: "sequence", Type: "number"}, {Key: "formula_key", Type: "string"}, {Key: "fixed_amount", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "employee_payroll_profile", DisplayName: "Employee Payroll Profile", DefaultSort: "employee_id", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "salary_structure_id", Type: "string"}, {Key: "currency_code", Type: "string"}, {Key: "payment_method_code", Type: "string"}, {Key: "treasury_account_id", Type: "string"}, {Key: "payroll_party_id", Type: "string"}, {Key: "tax_rule_id", Type: "string"}, {Key: "contribution_rule_id", Type: "string"}, {Key: "leave_deduction_daily_rate", Type: "number"}, {Key: "reimbursement_in_payroll", Type: "bool"}, {Key: "status", Type: "string"}}},
		{Key: "payroll_period", DisplayName: "Payroll Period", DefaultSort: "start_date", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "start_date", Type: "string"}, {Key: "end_date", Type: "string"}, {Key: "pay_date", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "payroll_tax_rule", DisplayName: "Payroll Tax Rule", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "employee_rate_percent", Type: "number"}, {Key: "employer_rate_percent", Type: "number"}, {Key: "fixed_amount", Type: "number"}, {Key: "threshold_amount", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "payroll_contribution_rule", DisplayName: "Payroll Contribution Rule", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "employee_rate_percent", Type: "number"}, {Key: "employee_fixed_amount", Type: "number"}, {Key: "employer_rate_percent", Type: "number"}, {Key: "employer_fixed_amount", Type: "number"}, {Key: "threshold_amount", Type: "number"}, {Key: "status", Type: "string"}}},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}

func registerEmployeePayrollTestDocuments(t *testing.T, docs *document.Service) {
	t.Helper()
	defs := []document.Definition{
		{Type: "payroll_run", DisplayName: "Payroll Run", SchemaVersion: "v1", WorkflowKey: "payroll_run_flow", NumberingKey: "payroll_run_number"},
		{Type: "payroll_payment_batch", DisplayName: "Payroll Payment Batch", SchemaVersion: "v1", WorkflowKey: "payroll_payment_batch_flow", NumberingKey: "payroll_payment_batch_number"},
		{Type: "payroll_payment", DisplayName: "Payroll Payment", SchemaVersion: "v1", WorkflowKey: "payroll_payment_flow", NumberingKey: "payroll_payment_number"},
		{Type: "reimbursement_payment", DisplayName: "Reimbursement Payment", SchemaVersion: "v1", WorkflowKey: "reimbursement_payment_flow", NumberingKey: "reimbursement_payment_number"},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", WorkflowKey: "ledger_posting_flow", NumberingKey: "ledger_posting_number"},
	}
	for _, def := range defs {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document %s: %v", def.Type, err)
		}
	}
}
