package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/store"
)

func TestPayrollRemittancePostgresValidation(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed remittance test")
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
	party := ensurePartyRecord(t, graph.models, "user_admin", "party_remittance_"+suffix, "Remittance Party "+suffix)
	ensureLocationRecord(t, graph.models, "user_admin", locID)
	unitID := "ou_remittance_" + suffix
	departmentID := "dept_remittance_" + suffix
	costCenterID := "cc_remittance_" + suffix
	ensureOrganizationUnitRecord(t, graph.models, "user_admin", unitID, orgID, locID)
	ensureDepartmentRecord(t, graph.models, "user_admin", departmentID, orgID, locID, unitID)
	ensureCostCenterRecord(t, graph.models, "user_admin", costCenterID, orgID, locID, unitID, departmentID)
	approverUser, err := graph.identity.CreateUser("remittance-approver-"+suffix, testBootstrapAdminPassword, locID, "role_admin", "location", locID)
	if err != nil {
		t.Fatalf("create approver user: %v", err)
	}

	treasuryAccount, err := graph.models.Create("treasury_account", "user_admin", map[string]any{
		"account_code":    "TR-REM-" + suffix,
		"name":            "Remittance Treasury",
		"organization_id": orgID,
		"location_id":     locID,
		"currency_code":   "IDR",
		"gl_account_code": "1010-BANK-REM",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create treasury account: %v", err)
	}

	employee, err := graph.models.Create("employee_profile", "user_admin", map[string]any{
		"party_id":          party.ID,
		"user_id":           "user_admin",
		"employee_code":     "EMP-REM-" + suffix,
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
		"organization_unit_id": unitID,
		"department_id":        departmentID,
		"cost_center_id":       costCenterID,
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
	taxRule, err := graph.models.Create("payroll_tax_rule", "user_admin", map[string]any{
		"code":                  "TAX-" + suffix,
		"name":                  "Payroll Tax",
		"organization_id":       orgID,
		"location_id":           locID,
		"employee_rate_percent": 10.0,
		"status":                "active",
	})
	if err != nil {
		t.Fatalf("create tax rule: %v", err)
	}
	contribRule, err := graph.models.Create("payroll_contribution_rule", "user_admin", map[string]any{
		"code":                  "CONTRIB-" + suffix,
		"name":                  "Payroll Contribution",
		"organization_id":       orgID,
		"location_id":           locID,
		"employee_rate_percent": 5.0,
		"employer_rate_percent": 7.5,
		"status":                "active",
	})
	if err != nil {
		t.Fatalf("create contribution rule: %v", err)
	}
	component, err := graph.models.Create("pay_component", "user_admin", map[string]any{
		"code":            "BASIC-" + suffix,
		"name":            "Basic Salary",
		"component_class": "earning",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create pay component: %v", err)
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
	if _, err := graph.models.Create("salary_structure_line", "user_admin", map[string]any{
		"salary_structure_id": structure.ID,
		"component_code":      textValue(component.Values["code"]),
		"sequence":            1,
		"formula_key":         "fixed_amount",
		"fixed_amount":        1000.0,
		"status":              "active",
	}); err != nil {
		t.Fatalf("create salary structure line: %v", err)
	}
	if _, err := graph.models.Create("employee_payroll_profile", "user_admin", map[string]any{
		"employee_id":          employee.ID,
		"salary_structure_id":  structure.ID,
		"currency_code":        "IDR",
		"payment_method_code":  "BANK",
		"treasury_account_id":  treasuryAccount.ID,
		"tax_rule_id":          taxRule.ID,
		"contribution_rule_id": contribRule.ID,
		"status":               "active",
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
		"pay_date":        "2099-11-05",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create payroll period: %v", err)
	}

	authority, err := graph.models.Create("remittance_authority", "user_admin", map[string]any{
		"code":                        "AUTH-" + suffix,
		"name":                        "Tax Authority",
		"organization_id":             orgID,
		"location_id":                 locID,
		"default_currency_code":       "IDR",
		"default_treasury_account_id": treasuryAccount.ID,
		"payment_method_code":         "BANK",
		"status":                      "active",
	})
	if err != nil {
		t.Fatalf("create remittance authority: %v", err)
	}
	withholding, err := graph.models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "WHT-" + suffix,
		"name":                    "Withholding",
		"obligation_class":        "withholding",
		"liability_account_code":  "2310-WHT",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create withholding obligation: %v", err)
	}
	employeeContribution, err := graph.models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "EC-" + suffix,
		"name":                    "Employee Contribution",
		"obligation_class":        "employee_contribution",
		"liability_account_code":  "2311-EMP",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create employee contribution obligation: %v", err)
	}
	employerContribution, err := graph.models.Create("remittance_obligation_type", "user_admin", map[string]any{
		"remittance_authority_id": authority.ID,
		"code":                    "ER-" + suffix,
		"name":                    "Employer Contribution",
		"obligation_class":        "employer_contribution",
		"liability_account_code":  "2312-ER",
		"status":                  "active",
	})
	if err != nil {
		t.Fatalf("create employer contribution obligation: %v", err)
	}
	if _, err := graph.models.Create("remittance_schedule_rule", "user_admin", map[string]any{
		"remittance_authority_id":   authority.ID,
		"due_days_after_period_end": 7,
		"status":                    "active",
	}); err != nil {
		t.Fatalf("create remittance schedule rule: %v", err)
	}
	if _, err := graph.models.Create("payroll_remittance_profile", "user_admin", map[string]any{
		"organization_id":                          orgID,
		"location_id":                              locID,
		"payroll_tax_rule_id":                      taxRule.ID,
		"payroll_contribution_rule_id":             contribRule.ID,
		"remittance_authority_id":                  authority.ID,
		"withholding_obligation_type_id":           withholding.ID,
		"employee_contribution_obligation_type_id": employeeContribution.ID,
		"employer_contribution_obligation_type_id": employerContribution.ID,
		"default_treasury_account_id":              treasuryAccount.ID,
		"payment_method_code":                      "BANK",
		"status":                                   "active",
	}); err != nil {
		t.Fatalf("create payroll remittance profile: %v", err)
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

	liabilities, generationPosting, err := graph.payrollRemittance.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("generate remittance liabilities: %v", err)
	}
	if len(liabilities) != 3 {
		t.Fatalf("expected 3 remittance liabilities, got %d", len(liabilities))
	}
	liabilityIDs := make([]string, 0, len(liabilities))
	for _, liability := range liabilities {
		liabilityIDs = append(liabilityIDs, liability.Header.ID)
	}

	batchPayload := graph.payrollRemittance.NormalizePayload("payroll_remittance_batch", map[string]any{
		"liability_ids": liabilityIDs,
	})
	batch, err := graph.documents.Create("payroll_remittance_batch", orgID, locID, "user_admin", batchPayload)
	if err != nil {
		t.Fatalf("create remittance batch: %v", err)
	}
	batch, err = graph.docActions.Submit(batch.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit remittance batch: %v", err)
	}
	batch, err = graph.docActions.Approve(batch.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve remittance batch: %v", err)
	}
	if batch.Header.Status != "approved" {
		t.Fatalf("expected remittance batch status approved, got %s", batch.Header.Status)
	}

	payment, paymentPosting, err := graph.payrollRemittance.CreatePaymentFromBatch(batch.Header.ID, "user_admin")
	if err != nil {
		t.Fatalf("create remittance payment from batch: %v", err)
	}
	if payment.Header.Status != "paid" {
		t.Fatalf("expected remittance payment status paid, got %s", payment.Header.Status)
	}

	reloaded := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(reloaded.config, reloaded.identity, reloaded.modules, reloaded.models, reloaded.reporting, reloaded.templates, reloaded.reference, reloaded.search, reloaded.documents, reloaded.workflows, reloaded.policy, nil, testBootstrapAdminPassword); err != nil {
		t.Fatalf("reseed platform kernel: %v", err)
	}
	persistedPayment, err := reloaded.documents.Get(payment.Header.ID)
	if err != nil {
		t.Fatalf("get persisted remittance payment: %v", err)
	}
	if persistedPayment.Header.Status != "paid" {
		t.Fatalf("expected persisted remittance payment status paid, got %s", persistedPayment.Header.Status)
	}
	if got := textValue(persistedPayment.Body.Payload["posted_ledger_id"]); got != paymentPosting.Header.ID {
		t.Fatalf("expected persisted payment ledger %s, got %q", paymentPosting.Header.ID, got)
	}
	for _, liabilityID := range liabilityIDs {
		persistedLiability, err := reloaded.documents.Get(liabilityID)
		if err != nil {
			t.Fatalf("get persisted liability %s: %v", liabilityID, err)
		}
		if persistedLiability.Header.Status != "paid" {
			t.Fatalf("expected persisted liability %s paid, got %s", liabilityID, persistedLiability.Header.Status)
		}
	}
	persistedPosting, err := reloaded.documents.Get(generationPosting.Header.ID)
	if err != nil {
		t.Fatalf("get generation posting: %v", err)
	}
	if persistedPosting.Header.Status != "posted" {
		t.Fatalf("expected generation posting posted, got %s", persistedPosting.Header.Status)
	}
	if _, _, err := reloaded.payrollRemittance.GenerateLiabilitiesFromPayrollRun(run.Header.ID, "user_admin"); err != nil {
		t.Fatalf("expected remittance generation retry to remain idempotent, got %v", err)
	}
}
