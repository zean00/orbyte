package application

import (
	"testing"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestFinanceReportingTrialBalanceAndStatements(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	cfg := config.NewService()
	registerFinanceReportingTestModels(t, models)
	registerLedgerPostingDefinition(t, docs)
	svc := NewFinanceReportingCoreService(docs, models, cfg)

	_, _ = models.Create("finance_account", "tester", map[string]any{"code": "1100-AR", "name": "Accounts Receivable", "account_type": "asset", "report_group": "accounts_receivable", "normal_balance": "debit", "status": "active"})
	_, _ = models.Create("finance_account", "tester", map[string]any{"code": "4000-REV", "name": "Revenue", "account_type": "revenue", "report_group": "revenue", "normal_balance": "credit", "status": "active"})
	_, _ = models.Create("finance_account", "tester", map[string]any{"code": "2100-VATOUT", "name": "VAT Output", "account_type": "liability", "report_group": "tax_output", "normal_balance": "credit", "status": "active"})

	posting, err := docs.Create("ledger_posting", "org-1", "loc-1", "tester", map[string]any{
		"source_document_type": "invoice",
		"source_document_id":   "inv-1",
		"posting_date":         "2026-03-15",
		"currency_code":        "IDR",
		"posting_rule_key":     "invoice_issue_default",
		"total_amount":         111.0,
		"journal_lines": []map[string]any{
			{"account_code": "1100-AR", "account_name": "Accounts Receivable", "description": "AR", "debit": 111.0, "credit": 0.0},
			{"account_code": "4000-REV", "account_name": "Revenue", "description": "Revenue", "debit": 0.0, "credit": 100.0},
			{"account_code": "2100-VATOUT", "account_name": "VAT Output", "description": "Tax Payable", "debit": 0.0, "credit": 11.0},
		},
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := docs.Save(posting); err != nil {
		t.Fatalf("save posting: %v", err)
	}

	tb := svc.TrialBalance("org-1", "loc-1", "2026-03-01", "2026-03-31")
	if len(tb.Rows) != 3 {
		t.Fatalf("expected 3 trial balance rows, got %d", len(tb.Rows))
	}
	if tb.Totals["debit"] != 111.0 || tb.Totals["credit"] != 111.0 {
		t.Fatalf("unexpected trial balance totals: %+v", tb.Totals)
	}

	pnl := svc.ProfitAndLoss("org-1", "loc-1", "2026-03-01", "2026-03-31")
	if pnl.NetProfit != 100.0 {
		t.Fatalf("expected net profit 100, got %.2f", pnl.NetProfit)
	}

	bs := svc.BalanceSheet("org-1", "loc-1", "2026-03-31")
	if len(bs.Sections) != 3 {
		t.Fatalf("expected 3 balance sheet sections, got %d", len(bs.Sections))
	}

	tax := svc.TaxSummary("org-1", "loc-1", "2026-03-01", "2026-03-31")
	if tax.Totals["output_tax"] != 11.0 {
		t.Fatalf("expected output tax 11, got %.2f", tax.Totals["output_tax"])
	}
}

func TestFinanceReportingValidatePostingDateOpen(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	cfg := config.NewService()
	registerFinanceReportingTestModels(t, models)
	svc := NewFinanceReportingCoreService(docs, models, cfg)

	_, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "loc-1",
		"period_key":      "2026-03",
		"start_date":      "2026-03-01",
		"end_date":        "2026-03-31",
		"status":          "closed",
	})
	if err != nil {
		t.Fatalf("create period: %v", err)
	}
	if err := svc.ValidatePostingDateOpen("org-1", "loc-1", "2026-03-15"); err == nil {
		t.Fatalf("expected closed period validation error")
	}
	if err := svc.ValidatePostingDateOpen("org-1", "loc-1", "2026-04-01"); err != nil {
		t.Fatalf("expected open posting date, got %v", err)
	}
}

func TestFinanceReportingValidatePostingDateOpenHonorsOrgWideClosedPeriods(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	cfg := config.NewService()
	registerFinanceReportingTestModels(t, models)
	svc := NewFinanceReportingCoreService(docs, models, cfg)

	_, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "",
		"period_key":      "2026-03",
		"start_date":      "2026-03-01",
		"end_date":        "2026-03-31",
		"status":          "closed",
	})
	if err != nil {
		t.Fatalf("create period: %v", err)
	}
	if err := svc.ValidatePostingDateOpen("org-1", "loc-9", "2026-03-15"); err == nil {
		t.Fatalf("expected org-wide closed period validation error")
	}
}

func TestFinanceReportingCloseAccountingPeriodRequiresScopeMatch(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	cfg := config.NewService()
	registerFinanceReportingTestModels(t, models)
	svc := NewFinanceReportingCoreService(docs, models, cfg)

	period, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "loc-1",
		"period_key":      "2026-03",
		"start_date":      "2026-03-01",
		"end_date":        "2026-03-31",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create period: %v", err)
	}
	if _, err := svc.CloseAccountingPeriod(period.ID, "tester", "org-1", "loc-2"); err == nil {
		t.Fatalf("expected cross-location close to be forbidden")
	}
	if _, err := svc.CloseAccountingPeriod(period.ID, "tester", "org-2", ""); err == nil {
		t.Fatalf("expected cross-organization close to be forbidden")
	}
	if _, err := svc.CloseAccountingPeriod(period.ID, "tester", "org-1", "loc-1"); err != nil {
		t.Fatalf("expected scoped close to succeed, got %v", err)
	}
}

func registerFinanceReportingTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	for _, def := range []model.Definition{
		{
			Key:                 "finance_account",
			DisplayName:         "Finance Account",
			Version:             "v1",
			CreatePermissionKey: "finance_account.create",
			ListPermissionKey:   "finance_account.list",
			ReadPermissionKey:   "finance_account.read",
			UpdatePermissionKey: "finance_account.update",
			DefaultSort:         "code",
			Fields: []model.FieldDefinition{
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "account_type", Type: "string"},
				{Key: "report_group", Type: "string"},
				{Key: "normal_balance", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:                 "accounting_period",
			DisplayName:         "Accounting Period",
			Version:             "v1",
			CreatePermissionKey: "accounting_period.create",
			ListPermissionKey:   "accounting_period.list",
			ReadPermissionKey:   "accounting_period.read",
			UpdatePermissionKey: "accounting_period.update",
			DefaultSort:         "start_date",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "period_key", Type: "string", Required: true},
				{Key: "start_date", Type: "string", Required: true},
				{Key: "end_date", Type: "string", Required: true},
				{Key: "status", Type: "string"},
			},
		},
	} {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}

func registerLedgerPostingDefinition(t *testing.T, docs *document.Service) {
	t.Helper()
	if err := docs.Register(document.Definition{
		Type:           "ledger_posting",
		DisplayName:    "Ledger Posting",
		SchemaVersion:  "v1",
		AllowedLinkTypes: []string{"posting_for"},
	}); err != nil {
		t.Fatalf("register ledger posting: %v", err)
	}
}
