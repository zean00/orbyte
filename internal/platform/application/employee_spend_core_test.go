package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestEmployeeSpendNormalizePayloadAppliesAssignmentAndTotals(t *testing.T) {
	models := model.NewService()
	registerEmployeeSpendTestModels(t, models)
	docs := document.NewService()
	service := NewEmployeeSpendCoreService(docs, models)

	employee, err := models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          "party_emp",
		"employee_code":     "EMP-100",
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
		"organization_unit_id": "ou_ops",
		"department_id":        "dept_ops",
		"cost_center_id":       "cc_ops",
		"effective_from":       time.Now().UTC().Format("2006-01-02"),
		"status":               "active",
	}); err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	if _, err := models.Create("employee_spend_profile", "user_admin", map[string]any{
		"employee_id":                 employee.ID,
		"default_currency_code":       "USD",
		"default_payment_method_code": "BANK",
		"payable_account_code":        "2100-EMP",
		"expense_account_code":        "6100-TRAVEL",
		"treasury_account_id":         "treasury_main",
		"status":                      "active",
	}); err != nil {
		t.Fatalf("create spend profile: %v", err)
	}

	payload := service.NormalizePayload("expense_claim", map[string]any{
		"employee_id": employee.ID,
		"claim_lines": []map[string]any{
			{"description": "Taxi", "amount": 15.0},
			{"description": "Meals", "quantity": 2.0, "unit_price": 12.5},
		},
	})

	if got := textValue(payload["party_id"]); got != "party_emp" {
		t.Fatalf("expected party_id party_emp, got %q", got)
	}
	if got := textValue(payload["department_id"]); got != "dept_ops" {
		t.Fatalf("expected department_id dept_ops, got %q", got)
	}
	if got := textValue(payload["currency_code"]); got != "USD" {
		t.Fatalf("expected currency USD, got %q", got)
	}
	if got := numberValue(payload["total_amount"]); got != 40.0 {
		t.Fatalf("expected total_amount 40, got %v", got)
	}
	if got := numberValue(payload["reimbursable_amount"]); got != 40.0 {
		t.Fatalf("expected reimbursable_amount 40, got %v", got)
	}
}

func TestEmployeeSpendCreateReimbursementPaymentFromApprovedLiquidation(t *testing.T) {
	models := model.NewService()
	registerEmployeeSpendTestModels(t, models)
	docs := document.NewService()
	registerEmployeeSpendTestDocuments(t, docs)
	service := NewEmployeeSpendCoreService(docs, models)

	liquidation, err := docs.Create("advance_liquidation", "org_default", "loc_hq", "user_admin", map[string]any{
		"employee_id":           "emp_1",
		"party_id":              "party_emp",
		"currency_code":         "IDR",
		"net_settlement_amount": 75.0,
		"cash_advance_id":       "adv_1",
		"travel_request_id":     "trip_1",
	})
	if err != nil {
		t.Fatalf("create liquidation: %v", err)
	}
	liquidation.Header.Status = "approved"
	if err := docs.Save(liquidation); err != nil {
		t.Fatalf("save liquidation: %v", err)
	}

	payment, err := service.CreateReimbursementPaymentFromLiquidation(liquidation.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create reimbursement payment: %v", err)
	}
	if payment.Header.Type != "reimbursement_payment" {
		t.Fatalf("expected reimbursement_payment, got %s", payment.Header.Type)
	}
	if got := numberValue(payment.Body.Payload["amount_paid"]); got != 75.0 {
		t.Fatalf("expected amount_paid 75, got %v", got)
	}
	if got := textValue(payment.Body.Payload["source_liquidation_id"]); got != liquidation.Header.ID {
		t.Fatalf("expected source_liquidation_id %s, got %q", liquidation.Header.ID, got)
	}
}

func TestEmployeeSpendNormalizeLiquidationUsesRemainingAdvanceBalance(t *testing.T) {
	models := model.NewService()
	registerEmployeeSpendTestModels(t, models)
	docs := document.NewService()
	registerEmployeeSpendTestDocuments(t, docs)
	service := NewEmployeeSpendCoreService(docs, models)

	advance, err := docs.Create("cash_advance", "org_default", "loc_hq", "user_admin", map[string]any{
		"employee_id":        "emp_1",
		"party_id":           "party_emp",
		"currency_code":      "IDR",
		"approved_amount":    100.0,
		"outstanding_amount": 100.0,
		"total_amount":       100.0,
	})
	if err != nil {
		t.Fatalf("create cash advance: %v", err)
	}
	liquidation, err := docs.Create("advance_liquidation", "org_default", "loc_hq", "user_admin", map[string]any{
		"cash_advance_id":       advance.Header.ID,
		"advance_applied_amount": 70.0,
	})
	if err != nil {
		t.Fatalf("create liquidation: %v", err)
	}
	liquidation.Header.Status = "approved"
	if err := docs.Save(liquidation); err != nil {
		t.Fatalf("save liquidation: %v", err)
	}

	payload := service.NormalizePayload("advance_liquidation", map[string]any{
		"cash_advance_id":   advance.Header.ID,
		"expense_claim_id":  "claim_2",
		"claim_total_amount": 170.0,
	})

	if got := numberValue(payload["advance_amount"]); got != 30.0 {
		t.Fatalf("expected remaining advance_amount 30, got %v", got)
	}
	if got := numberValue(payload["advance_applied_amount"]); got != 30.0 {
		t.Fatalf("expected remaining advance_applied_amount 30, got %v", got)
	}
	if got := numberValue(payload["net_settlement_amount"]); got != 140.0 {
		t.Fatalf("expected net_settlement_amount 140, got %v", got)
	}
}

func registerEmployeeSpendTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{Key: "employee_profile", DisplayName: "Employee Profile", DefaultSort: "employee_code", Fields: []model.FieldDefinition{{Key: "party_id", Type: "string"}, {Key: "employee_code", Type: "string"}, {Key: "employment_status", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "employee_assignment", DisplayName: "Employee Assignment", DefaultSort: "effective_from", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "organization_unit_id", Type: "string"}, {Key: "department_id", Type: "string"}, {Key: "cost_center_id", Type: "string"}, {Key: "effective_from", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "employee_spend_profile", DisplayName: "Employee Spend Profile", DefaultSort: "employee_id", Fields: []model.FieldDefinition{{Key: "employee_id", Type: "string"}, {Key: "default_currency_code", Type: "string"}, {Key: "default_payment_method_code", Type: "string"}, {Key: "payable_account_code", Type: "string"}, {Key: "expense_account_code", Type: "string"}, {Key: "treasury_account_id", Type: "string"}, {Key: "status", Type: "string"}}},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}

func registerEmployeeSpendTestDocuments(t *testing.T, docs *document.Service) {
	t.Helper()
	defs := []document.Definition{
		{Type: "cash_advance", DisplayName: "Cash Advance", SchemaVersion: "v1", WorkflowKey: "cash_advance_flow", NumberingKey: "cash_advance_number"},
		{Type: "advance_liquidation", DisplayName: "Advance Liquidation", SchemaVersion: "v1", WorkflowKey: "advance_liquidation_flow", NumberingKey: "advance_liquidation_number"},
		{Type: "reimbursement_payment", DisplayName: "Reimbursement Payment", SchemaVersion: "v1", WorkflowKey: "reimbursement_payment_flow", NumberingKey: "reimbursement_payment_number"},
	}
	for _, def := range defs {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document %s: %v", def.Type, err)
		}
	}
}
