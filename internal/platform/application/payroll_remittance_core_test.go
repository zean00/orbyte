package application

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestPayrollRemittanceGenerateLiabilitiesFromProcessedRun(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	registerPayrollRemittanceTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	registerPayrollRemittanceTestDocuments(t, docs)
	service := NewPayrollRemittanceCoreService(docs, models)

	treasuryAccount, err := models.Create("treasury_account", "user_admin", map[string]any{
		"code":            "TR-MAIN",
		"name":            "Main Treasury",
		"gl_account_code": "1010-BANK",
	})
	if err != nil {
		t.Fatalf("create treasury account: %v", err)
	}
	authority, err := models.Create("remittance_authority", "user_admin", map[string]any{
		"code":                        "TAX",
		"name":                        "Tax Office",
		"default_treasury_account_id": treasuryAccount.ID,
		"payment_method_code":         "BANK",
		"status":                      "active",
	})
	if err != nil {
		t.Fatalf("create remittance authority: %v", err)
	}
	withholding, err := models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "WHT",
		"name":                    "Withholding",
		"obligation_class":        "withholding",
		"liability_account_code":  "2310-WHT",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create withholding obligation: %v", err)
	}
	employeeContrib, err := models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "EMP-CONTRIB",
		"name":                    "Employee Contribution",
		"obligation_class":        "employee_contribution",
		"liability_account_code":  "2311-EMP-CONTRIB",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create employee contribution obligation: %v", err)
	}
	employerContrib, err := models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "ER-CONTRIB",
		"name":                    "Employer Contribution",
		"obligation_class":        "employer_contribution",
		"liability_account_code":  "2312-ER-CONTRIB",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create employer contribution obligation: %v", err)
	}
	if _, err := models.Create("remittance_schedule_rule", "user_admin", map[string]any{
		"remittance_authority_id":   authority.ID,
		"due_days_after_period_end": 10,
		"status":                    "active",
	}); err != nil {
		t.Fatalf("create schedule rule: %v", err)
	}
	if _, err := models.Create("payroll_remittance_profile", "user_admin", map[string]any{
		"organization_id":                          "org_default",
		"location_id":                              "loc_hq",
		"payroll_tax_rule_id":                      "tax_rule_1",
		"payroll_contribution_rule_id":             "contrib_rule_1",
		"remittance_authority_id":                  authority.ID,
		"withholding_obligation_type_id":           withholding.ID,
		"employee_contribution_obligation_type_id": employeeContrib.ID,
		"employer_contribution_obligation_type_id": employerContrib.ID,
		"default_treasury_account_id":              treasuryAccount.ID,
		"payment_method_code":                      "BANK",
		"status":                                   "active",
	}); err != nil {
		t.Fatalf("create remittance profile: %v", err)
	}

	run, err := docs.Create("payroll_run", "org_default", "loc_hq", "user_admin", map[string]any{
		"payroll_period_id":    "period_1",
		"period_end_date":      "2099-10-31",
		"pay_date":             "2099-11-05",
		"currency_code":        "IDR",
		"payable_account_code": "2105-PAYROLL",
		"payroll_lines": []map[string]any{{
			"employee_id":                  "emp_1",
			"organization_id":              "org_default",
			"location_id":                  "loc_hq",
			"currency_code":                "IDR",
			"treasury_account_id":          "treasury_account-1",
			"payment_method_code":          "BANK",
			"tax_rule_id":                  "tax_rule_1",
			"contribution_rule_id":         "contrib_rule_1",
			"tax_withholding_total":        10.0,
			"employee_contributions_total": 5.0,
			"employer_contributions_total": 7.5,
		}},
	})
	if err != nil {
		t.Fatalf("create payroll run: %v", err)
	}
	run.Header.Status = "processed"
	if err := docs.Save(run); err != nil {
		t.Fatalf("save payroll run: %v", err)
	}

	liabilities, posting, err := service.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate liabilities: %v", err)
	}
	if len(liabilities) != 3 {
		t.Fatalf("expected 3 liabilities, got %d", len(liabilities))
	}
	if posting.Header.Type != "ledger_posting" || posting.Header.Status != "posted" {
		t.Fatalf("expected posted ledger posting, got %s/%s", posting.Header.Type, posting.Header.Status)
	}
	for _, liability := range liabilities {
		if liability.Header.Status != "open" {
			t.Fatalf("expected liability status open, got %s", liability.Header.Status)
		}
		if textValue(liability.Body.Payload["generation_posting_id"]) != posting.Header.ID {
			t.Fatalf("expected generation posting %s, got %q", posting.Header.ID, textValue(liability.Body.Payload["generation_posting_id"]))
		}
		if numberValue(liability.Body.Payload["outstanding_amount"]) <= 0 {
			t.Fatalf("expected outstanding amount on liability %s", liability.Header.ID)
		}
	}
}

func TestPayrollRemittanceGenerateLiabilitiesFromProcessedRunIsIdempotent(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	registerPayrollRemittanceTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	registerPayrollRemittanceTestDocuments(t, docs)
	service := NewPayrollRemittanceCoreService(docs, models)

	mustCreateRemittanceSetup(t, models)
	run := mustCreateProcessedRemittancePayrollRun(t, docs)

	firstLiabilities, firstPosting, err := service.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("first liability generation: %v", err)
	}
	secondLiabilities, secondPosting, err := service.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("second liability generation: %v", err)
	}
	if len(firstLiabilities) != len(secondLiabilities) {
		t.Fatalf("expected same liability count, got %d and %d", len(firstLiabilities), len(secondLiabilities))
	}
	if firstPosting.Header.ID != secondPosting.Header.ID {
		t.Fatalf("expected same posting id, got %s and %s", firstPosting.Header.ID, secondPosting.Header.ID)
	}
}

func TestPayrollRemittanceCreatePaymentFromApprovedBatch(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	registerPayrollRemittanceTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	registerPayrollRemittanceTestDocuments(t, docs)
	service := NewPayrollRemittanceCoreService(docs, models)

	mustCreateRemittanceSetup(t, models)
	run := mustCreateProcessedRemittancePayrollRun(t, docs)
	liabilities, _, err := service.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate liabilities: %v", err)
	}
	liabilityIDs := make([]string, 0, len(liabilities))
	for _, liability := range liabilities {
		liabilityIDs = append(liabilityIDs, liability.Header.ID)
	}
	batchPayload := service.NormalizePayload("payroll_remittance_batch", map[string]any{
		"liability_ids": liabilityIDs,
	})
	batch, err := docs.Create("payroll_remittance_batch", "org_default", "loc_hq", "user_admin", batchPayload)
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	batch.Header.Status = "approved"
	if err := docs.Save(batch); err != nil {
		t.Fatalf("save batch: %v", err)
	}

	payment, posting, err := service.CreatePaymentFromBatch(batch.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create payment from batch: %v", err)
	}
	if payment.Header.Type != "payroll_remittance_payment" || payment.Header.Status != "paid" {
		t.Fatalf("expected paid remittance payment, got %s/%s", payment.Header.Type, payment.Header.Status)
	}
	if posting.Header.Type != "ledger_posting" || posting.Header.Status != "posted" {
		t.Fatalf("expected posted ledger posting, got %s/%s", posting.Header.Type, posting.Header.Status)
	}
	for _, liabilityID := range liabilityIDs {
		liability, err := docs.Get(liabilityID)
		if err != nil {
			t.Fatalf("get liability %s: %v", liabilityID, err)
		}
		if liability.Header.Status != "paid" {
			t.Fatalf("expected liability %s paid, got %s", liabilityID, liability.Header.Status)
		}
		if numberValue(liability.Body.Payload["outstanding_amount"]) != 0 {
			t.Fatalf("expected liability %s outstanding 0, got %v", liabilityID, liability.Body.Payload["outstanding_amount"])
		}
	}
}

func TestPayrollRemittanceCreatePaymentFromApprovedBatchRejectsDuplicate(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	registerPayrollRemittanceTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	registerPayrollRemittanceTestDocuments(t, docs)
	service := NewPayrollRemittanceCoreService(docs, models)

	mustCreateRemittanceSetup(t, models)
	run := mustCreateProcessedRemittancePayrollRun(t, docs)
	liabilities, _, err := service.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate liabilities: %v", err)
	}
	liabilityIDs := make([]string, 0, len(liabilities))
	for _, liability := range liabilities {
		liabilityIDs = append(liabilityIDs, liability.Header.ID)
	}
	batch, err := docs.Create("payroll_remittance_batch", "org_default", "loc_hq", "user_admin", service.NormalizePayload("payroll_remittance_batch", map[string]any{
		"liability_ids": liabilityIDs,
	}))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	batch.Header.Status = "approved"
	if err := docs.Save(batch); err != nil {
		t.Fatalf("save batch: %v", err)
	}
	if _, _, err := service.CreatePaymentFromBatch(batch.Header.ID, "user_admin"); err != nil {
		t.Fatalf("first payment generation: %v", err)
	}
	if _, _, err := service.CreatePaymentFromBatch(batch.Header.ID, "user_admin"); err == nil {
		t.Fatal("expected duplicate remittance payment generation to be rejected")
	}
}

func TestPayrollRemittanceCreatePaymentFromApprovedBatchAcceptsTreasuryCodeReference(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	registerPayrollRemittanceTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	registerPayrollRemittanceTestDocuments(t, docs)
	service := NewPayrollRemittanceCoreService(docs, models)

	mustCreateRemittanceSetup(t, models)
	run := mustCreateProcessedRemittancePayrollRun(t, docs)
	liabilities, _, err := service.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate liabilities: %v", err)
	}
	liabilityIDs := make([]string, 0, len(liabilities))
	for _, liability := range liabilities {
		payload := cloneMap(liability.Body.Payload)
		payload["treasury_account_id"] = "TR-MAIN"
		liability.Body.Payload = document.NormalizePayload(payload)
		if err := docs.Save(liability); err != nil {
			t.Fatalf("save liability with code treasury ref: %v", err)
		}
		liabilityIDs = append(liabilityIDs, liability.Header.ID)
	}
	batch, err := docs.Create("payroll_remittance_batch", "org_default", "loc_hq", "user_admin", service.NormalizePayload("payroll_remittance_batch", map[string]any{
		"liability_ids": liabilityIDs,
	}))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	batch.Header.Status = "approved"
	if err := docs.Save(batch); err != nil {
		t.Fatalf("save batch: %v", err)
	}

	_, posting, err := service.CreatePaymentFromBatch(batch.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create payment from batch with treasury code ref: %v", err)
	}
	lines := recordList(posting.Body.Payload["journal_lines"])
	foundBankLine := false
	for _, line := range lines {
		if textValue(line["account_code"]) == "1010-BANK" {
			foundBankLine = true
			break
		}
	}
	if !foundBankLine {
		t.Fatal("expected remittance posting to resolve treasury GL account from treasury code reference")
	}
}

func TestPayrollRemittanceCreatePaymentFromApprovedBatchRejectsIncompatibleLiabilities(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	registerPayrollRemittanceTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	registerPayrollRemittanceTestDocuments(t, docs)
	service := NewPayrollRemittanceCoreService(docs, models)

	mustCreateRemittanceSetup(t, models)
	run := mustCreateProcessedRemittancePayrollRun(t, docs)
	liabilities, _, err := service.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate liabilities: %v", err)
	}
	if len(liabilities) == 0 {
		t.Fatal("expected liabilities")
	}
	otherAuthority, err := models.Create("remittance_authority", "user_admin", map[string]any{
		"code":                        "AUTH-OTHER",
		"name":                        "Other Authority",
		"default_treasury_account_id": "treasury_account-1",
		"payment_method_code":         "BANK",
		"status":                      "active",
	})
	if err != nil {
		t.Fatalf("create other authority: %v", err)
	}
	incompatible, err := docs.Create("payroll_remittance_liability", "org_default", "loc_hq", "user_admin", service.NormalizePayload("payroll_remittance_liability", map[string]any{
		"source_payroll_run_id":         run.Header.ID,
		"remittance_authority_id":       otherAuthority.ID,
		"remittance_obligation_type_id": textValue(liabilities[0].Body.Payload["remittance_obligation_type_id"]),
		"currency_code":                 "IDR",
		"treasury_account_id":           "treasury_account-1",
		"payment_method_code":           "BANK",
		"liability_account_code":        "2310-WHT",
		"due_date":                      "2099-11-07",
		"amount":                        1.0,
		"total_amount":                  1.0,
		"outstanding_amount":            1.0,
	}))
	if err != nil {
		t.Fatalf("create incompatible liability: %v", err)
	}
	incompatible.Header.Status = "open"
	if err := docs.Save(incompatible); err != nil {
		t.Fatalf("save incompatible liability: %v", err)
	}
	batch, err := docs.Create("payroll_remittance_batch", "org_default", "loc_hq", "user_admin", service.NormalizePayload("payroll_remittance_batch", map[string]any{
		"liability_ids": []string{liabilities[0].Header.ID, incompatible.Header.ID},
	}))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	batch.Header.Status = "approved"
	if err := docs.Save(batch); err != nil {
		t.Fatalf("save batch: %v", err)
	}
	if _, _, err := service.CreatePaymentFromBatch(batch.Header.ID, "user_admin"); err == nil {
		t.Fatal("expected incompatible liability batch to be rejected")
	}
}

func TestPayrollRemittanceCreatePaymentFromBatchRejectsUnknownTreasuryAccount(t *testing.T) {
	models := model.NewService()
	registerEmployeePayrollTestModels(t, models)
	registerPayrollRemittanceTestModels(t, models)
	docs := document.NewService()
	registerEmployeePayrollTestDocuments(t, docs)
	registerPayrollRemittanceTestDocuments(t, docs)
	service := NewPayrollRemittanceCoreService(docs, models)

	mustCreateRemittanceSetup(t, models)
	run := mustCreateProcessedRemittancePayrollRun(t, docs)
	liabilities, _, err := service.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate liabilities: %v", err)
	}
	if len(liabilities) == 0 {
		t.Fatal("expected liabilities")
	}
	payload := cloneMap(liabilities[0].Body.Payload)
	payload["treasury_account_id"] = "missing_treasury"
	liabilities[0].Body.Payload = document.NormalizePayload(payload)
	if err := docs.Save(liabilities[0]); err != nil {
		t.Fatalf("save liability: %v", err)
	}
	batch, err := docs.Create("payroll_remittance_batch", "org_default", "loc_hq", "user_admin", service.NormalizePayload("payroll_remittance_batch", map[string]any{
		"liability_ids": []string{liabilities[0].Header.ID},
	}))
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	batch.Header.Status = "approved"
	if err := docs.Save(batch); err != nil {
		t.Fatalf("save batch: %v", err)
	}
	if _, _, err := service.CreatePaymentFromBatch(batch.Header.ID, "user_admin"); err == nil {
		t.Fatal("expected unknown treasury account to be rejected")
	}
}

func registerPayrollRemittanceTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{Key: "remittance_authority", DisplayName: "Remittance Authority", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "default_treasury_account_id", Type: "string"}, {Key: "payment_method_code", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "remittance_obligation_type", DisplayName: "Remittance Obligation Type", DefaultSort: "code", Fields: []model.FieldDefinition{{Key: "remittance_authority_id", Type: "string"}, {Key: "code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "obligation_class", Type: "string"}, {Key: "liability_account_code", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "remittance_schedule_rule", DisplayName: "Remittance Schedule Rule", DefaultSort: "remittance_authority_id", Fields: []model.FieldDefinition{{Key: "remittance_authority_id", Type: "string"}, {Key: "remittance_obligation_type_id", Type: "string"}, {Key: "due_days_after_period_end", Type: "number"}, {Key: "due_day_of_month", Type: "number"}, {Key: "status", Type: "string"}}},
		{Key: "payroll_remittance_profile", DisplayName: "Payroll Remittance Profile", DefaultSort: "remittance_authority_id", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "payroll_tax_rule_id", Type: "string"}, {Key: "payroll_contribution_rule_id", Type: "string"}, {Key: "remittance_authority_id", Type: "string"}, {Key: "withholding_obligation_type_id", Type: "string"}, {Key: "employee_contribution_obligation_type_id", Type: "string"}, {Key: "employer_contribution_obligation_type_id", Type: "string"}, {Key: "default_treasury_account_id", Type: "string"}, {Key: "payment_method_code", Type: "string"}, {Key: "status", Type: "string"}}},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}

func registerPayrollRemittanceTestDocuments(t *testing.T, docs *document.Service) {
	t.Helper()
	defs := []document.Definition{
		{Type: "payroll_remittance_liability", DisplayName: "Payroll Remittance Liability", SchemaVersion: "v1", WorkflowKey: "payroll_remittance_liability_flow", NumberingKey: "payroll_remittance_liability_number"},
		{Type: "payroll_remittance_batch", DisplayName: "Payroll Remittance Batch", SchemaVersion: "v1", WorkflowKey: "payroll_remittance_batch_flow", NumberingKey: "payroll_remittance_batch_number"},
		{Type: "payroll_remittance_payment", DisplayName: "Payroll Remittance Payment", SchemaVersion: "v1", WorkflowKey: "payroll_remittance_payment_flow", NumberingKey: "payroll_remittance_payment_number"},
		{Type: "payroll_remittance_adjustment", DisplayName: "Payroll Remittance Adjustment", SchemaVersion: "v1", WorkflowKey: "payroll_remittance_adjustment_flow", NumberingKey: "payroll_remittance_adjustment_number"},
	}
	for _, def := range defs {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document %s: %v", def.Type, err)
		}
	}
}

func mustCreateRemittanceSetup(t *testing.T, models *model.Service) {
	t.Helper()
	treasuryAccount, err := models.Create("treasury_account", "user_admin", map[string]any{
		"code":            "TR-MAIN",
		"name":            "Main Treasury",
		"gl_account_code": "1010-BANK",
	})
	if err != nil {
		t.Fatalf("create treasury account: %v", err)
	}
	authority, err := models.Create("remittance_authority", "user_admin", map[string]any{
		"code":                        "AUTH",
		"name":                        "Authority",
		"default_treasury_account_id": treasuryAccount.ID,
		"payment_method_code":         "BANK",
		"status":                      "active",
	})
	if err != nil {
		t.Fatalf("create authority: %v", err)
	}
	withholding, err := models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "WHT",
		"name":                    "Withholding",
		"obligation_class":        "withholding",
		"liability_account_code":  "2310-WHT",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create withholding obligation: %v", err)
	}
	employeeContrib, err := models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "EMP",
		"name":                    "Employee Contribution",
		"obligation_class":        "employee_contribution",
		"liability_account_code":  "2311-EMP",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create employee contribution obligation: %v", err)
	}
	employerContrib, err := models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "ER",
		"name":                    "Employer Contribution",
		"obligation_class":        "employer_contribution",
		"liability_account_code":  "2312-ER",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create employer contribution obligation: %v", err)
	}
	if _, err := models.Create("remittance_schedule_rule", "user_admin", map[string]any{
		"remittance_authority_id":   authority.ID,
		"due_days_after_period_end": 7,
		"status":                    "active",
	}); err != nil {
		t.Fatalf("create schedule rule: %v", err)
	}
	if _, err := models.Create("payroll_remittance_profile", "user_admin", map[string]any{
		"organization_id":                          "org_default",
		"location_id":                              "loc_hq",
		"payroll_tax_rule_id":                      "tax_rule_1",
		"payroll_contribution_rule_id":             "contrib_rule_1",
		"remittance_authority_id":                  authority.ID,
		"withholding_obligation_type_id":           withholding.ID,
		"employee_contribution_obligation_type_id": employeeContrib.ID,
		"employer_contribution_obligation_type_id": employerContrib.ID,
		"default_treasury_account_id":              treasuryAccount.ID,
		"payment_method_code":                      "BANK",
		"status":                                   "active",
	}); err != nil {
		t.Fatalf("create remittance profile: %v", err)
	}
}

func mustCreateProcessedRemittancePayrollRun(t *testing.T, docs *document.Service) document.Record {
	t.Helper()
	run, err := docs.Create("payroll_run", "org_default", "loc_hq", "user_admin", map[string]any{
		"payroll_period_id":    "period_1",
		"period_end_date":      "2099-10-31",
		"pay_date":             "2099-11-05",
		"currency_code":        "IDR",
		"payable_account_code": "2105-PAYROLL",
		"payroll_lines": []map[string]any{{
			"employee_id":                  "emp_1",
			"organization_id":              "org_default",
			"location_id":                  "loc_hq",
			"currency_code":                "IDR",
			"treasury_account_id":          "treasury_account-1",
			"payment_method_code":          "BANK",
			"tax_rule_id":                  "tax_rule_1",
			"contribution_rule_id":         "contrib_rule_1",
			"tax_withholding_total":        10.0,
			"employee_contributions_total": 5.0,
			"employer_contributions_total": 7.5,
		}},
	})
	if err != nil {
		t.Fatalf("create payroll run: %v", err)
	}
	run.Header.Status = "processed"
	if err := docs.Save(run); err != nil {
		t.Fatalf("save payroll run: %v", err)
	}
	return run
}
