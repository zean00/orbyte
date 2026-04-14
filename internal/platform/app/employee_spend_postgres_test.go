package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/store"
)

func TestEmployeeSpendPostgresTravelAdvanceClaimLiquidationAndReimbursement(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed employee spend test")
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
	party := ensurePartyRecord(t, graph.models, "user_admin", "party_emp_"+suffix, "Employee Spend Party "+suffix)
	ensureOrganizationUnitRecord(t, graph.models, "user_admin", "ou_spend_"+suffix, orgID, locID)
	approverUser, err := graph.identity.CreateUser("employee-spend-approver-"+suffix, testBootstrapAdminPassword, locID, "role_admin", "location", locID)
	if err != nil {
		t.Fatalf("create approver user: %v", err)
	}
	employee, err := graph.models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          party.ID,
		"user_id":           "user_admin",
		"employee_code":     "EMP-SPEND-" + suffix,
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
		"organization_unit_id": "ou_spend_" + suffix,
		"department_id":        "dept_spend_" + suffix,
		"cost_center_id":       "cc_spend_" + suffix,
		"effective_from":       time.Now().UTC().Format("2006-01-02"),
		"status":               "active",
	}); err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	category, err := graph.models.Create("expense_category", "user_admin", map[string]any{
		"code":                 "TRAVEL-" + suffix,
		"name":                 "Travel Expense",
		"expense_account_code": "6100-TRAVEL",
		"payable_account_code": "2100-EMP",
		"status":               "active",
	})
	if err != nil {
		t.Fatalf("create expense category: %v", err)
	}
	policyRecord, err := graph.models.Create("expense_policy", "user_admin", map[string]any{
		"code":                         "POL-" + suffix,
		"name":                         "Travel Policy",
		"organization_id":              orgID,
		"location_id":                  locID,
		"default_currency_code":        "IDR",
		"default_payment_method_code":  "BANK",
		"default_payable_account_code": "2100-EMP",
		"default_expense_account_code": "6100-TRAVEL",
		"default_treasury_account_id":  "treasury_main_" + suffix,
		"status":                       "active",
	})
	if err != nil {
		t.Fatalf("create expense policy: %v", err)
	}
	if _, err := graph.models.Create("travel_policy", "user_admin", map[string]any{
		"code":                      "TRV-" + suffix,
		"name":                      "Domestic Travel",
		"organization_id":           orgID,
		"location_id":               locID,
		"default_expense_policy_id": policyRecord.ID,
		"status":                    "active",
	}); err != nil {
		t.Fatalf("create travel policy: %v", err)
	}
	if _, err := graph.models.Create("employee_spend_profile", "user_admin", map[string]any{
		"employee_id":                 employee.ID,
		"expense_policy_id":           policyRecord.ID,
		"default_currency_code":       "IDR",
		"default_payment_method_code": "BANK",
		"payable_account_code":        "2100-EMP",
		"expense_account_code":        "6100-TRAVEL",
		"treasury_account_id":         "treasury_main_" + suffix,
		"status":                      "active",
	}); err != nil {
		t.Fatalf("create employee spend profile: %v", err)
	}

	travelPayload := graph.employeeSpend.NormalizePayload("travel_request", map[string]any{
		"employee_id":         employee.ID,
		"travel_start_date":   "2099-10-10",
		"travel_end_date":     "2099-10-12",
		"destination":         "Jakarta",
		"purpose":             "Customer visit",
		"estimated_lines":     []map[string]any{{"description": "Hotel", "amount": 120.0}, {"description": "Meals", "amount": 30.0}},
		"expense_category_id": category.ID,
	})
	travelRequest, err := graph.documents.Create("travel_request", orgID, locID, "user_admin", travelPayload)
	if err != nil {
		t.Fatalf("create travel request: %v", err)
	}
	travelRequest, err = graph.docActions.Submit(travelRequest.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit travel request: %v", err)
	}
	travelRequest, err = graph.docActions.Approve(travelRequest.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve travel request: %v", err)
	}

	advancePayload := graph.employeeSpend.NormalizePayload("cash_advance", map[string]any{
		"employee_id":       employee.ID,
		"travel_request_id": travelRequest.Header.ID,
		"requested_amount":  100.0,
		"notes":             "Travel cash advance",
	})
	advance, err := graph.documents.Create("cash_advance", orgID, locID, "user_admin", advancePayload)
	if err != nil {
		t.Fatalf("create cash advance: %v", err)
	}
	advance, err = graph.docActions.Submit(advance.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit cash advance: %v", err)
	}
	advance, err = graph.docActions.Approve(advance.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve cash advance: %v", err)
	}

	claimPayload := graph.employeeSpend.NormalizePayload("expense_claim", map[string]any{
		"employee_id":       employee.ID,
		"travel_request_id": travelRequest.Header.ID,
		"claim_lines": []map[string]any{
			{"expense_category_code": textValue(category.Values["code"]), "description": "Taxi", "amount": 25.0},
			{"expense_category_code": textValue(category.Values["code"]), "description": "Meals", "amount": 45.0},
		},
	})
	claim, err := graph.documents.Create("expense_claim", orgID, locID, "user_admin", claimPayload)
	if err != nil {
		t.Fatalf("create expense claim: %v", err)
	}
	claim, err = graph.docActions.Submit(claim.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit expense claim: %v", err)
	}
	claim, err = graph.docActions.Approve(claim.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve expense claim: %v", err)
	}

	liquidationPayload := graph.employeeSpend.NormalizePayload("advance_liquidation", map[string]any{
		"employee_id":       employee.ID,
		"travel_request_id": travelRequest.Header.ID,
		"cash_advance_id":   advance.Header.ID,
		"expense_claim_id":  claim.Header.ID,
	})
	liquidation, err := graph.documents.Create("advance_liquidation", orgID, locID, "user_admin", liquidationPayload)
	if err != nil {
		t.Fatalf("create liquidation: %v", err)
	}
	liquidation, err = graph.docActions.Submit(liquidation.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit liquidation: %v", err)
	}
	liquidation, err = graph.docActions.Approve(liquidation.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve liquidation: %v", err)
	}
	if got := numberValue(liquidation.Body.Payload["net_settlement_amount"]); got != -30.0 {
		t.Fatalf("expected net settlement -30, got %v", got)
	}

	reimbursement, err := graph.employeeSpend.CreateReimbursementPaymentFromLiquidation(liquidation.Header.ID, "user_admin")
	if err == nil {
		t.Fatal("expected no reimbursement payment when employee owes company")
	}

	claimPayload = graph.employeeSpend.NormalizePayload("expense_claim", map[string]any{
		"employee_id":       employee.ID,
		"travel_request_id": travelRequest.Header.ID,
		"claim_lines": []map[string]any{
			{"expense_category_code": textValue(category.Values["code"]), "description": "Hotel", "amount": 150.0},
			{"expense_category_code": textValue(category.Values["code"]), "description": "Meals", "amount": 20.0},
		},
	})
	claim, err = graph.documents.Create("expense_claim", orgID, locID, "user_admin", claimPayload)
	if err != nil {
		t.Fatalf("create second expense claim: %v", err)
	}
	claim, err = graph.docActions.Submit(claim.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit second expense claim: %v", err)
	}
	claim, err = graph.docActions.Approve(claim.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve second expense claim: %v", err)
	}
	liquidationPayload = graph.employeeSpend.NormalizePayload("advance_liquidation", map[string]any{
		"employee_id":       employee.ID,
		"travel_request_id": travelRequest.Header.ID,
		"cash_advance_id":   advance.Header.ID,
		"expense_claim_id":  claim.Header.ID,
	})
	liquidation, err = graph.documents.Create("advance_liquidation", orgID, locID, "user_admin", liquidationPayload)
	if err != nil {
		t.Fatalf("create second liquidation: %v", err)
	}
	liquidation, err = graph.docActions.Submit(liquidation.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit second liquidation: %v", err)
	}
	liquidation, err = graph.docActions.Approve(liquidation.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve second liquidation: %v", err)
	}
	if got := numberValue(liquidation.Body.Payload["advance_amount"]); got != 30.0 {
		t.Fatalf("expected remaining advance amount 30, got %v", got)
	}
	if got := numberValue(liquidation.Body.Payload["net_settlement_amount"]); got != 140.0 {
		t.Fatalf("expected net settlement 140, got %v", got)
	}

	reimbursement, err = graph.employeeSpend.CreateReimbursementPaymentFromLiquidation(liquidation.Header.ID, "user_admin")
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

	reloaded := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(reloaded.config, reloaded.identity, reloaded.modules, reloaded.models, reloaded.reporting, reloaded.templates, reloaded.reference, reloaded.search, reloaded.documents, reloaded.workflows, reloaded.policy, nil, testBootstrapAdminPassword); err != nil {
		t.Fatalf("reseed platform kernel: %v", err)
	}
	persistedReimbursement, err := reloaded.documents.Get(reimbursement.Header.ID)
	if err != nil {
		t.Fatalf("get persisted reimbursement payment: %v", err)
	}
	if persistedReimbursement.Header.Status != "paid" {
		t.Fatalf("expected persisted reimbursement payment to stay paid, got %s", persistedReimbursement.Header.Status)
	}
	if got := numberValue(persistedReimbursement.Body.Payload["amount_paid"]); got != 140.0 {
		t.Fatalf("expected persisted reimbursement amount 140, got %v", got)
	}
	if got := textValue(persistedReimbursement.Body.Payload["source_liquidation_id"]); got != liquidation.Header.ID {
		t.Fatalf("expected persisted source_liquidation_id %s, got %q", liquidation.Header.ID, got)
	}
}
