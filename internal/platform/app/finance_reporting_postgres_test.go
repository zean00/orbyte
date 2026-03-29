package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/store"
)

func TestFinanceReportingPostgresReportsAndClose(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed finance reporting test")
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

	suffix := time.Now().UTC().Format("20060102150405")
	orgID := "org_default"
	locID := "loc_hq"
	actorID := "user_admin"
	postingDate := "2099-03-20"
	fromDate := "2099-03-01"
	toDate := "2099-03-31"
	_, _ = graph.models.Create("finance_account", actorID, map[string]any{"code": "1100-AR-" + suffix, "name": "Accounts Receivable", "account_type": "asset", "report_group": "accounts_receivable", "normal_balance": "debit", "status": "active"})
	_, _ = graph.models.Create("finance_account", actorID, map[string]any{"code": "4000-REV-" + suffix, "name": "Revenue", "account_type": "revenue", "report_group": "revenue", "normal_balance": "credit", "status": "active"})
	_, _ = graph.models.Create("finance_account", actorID, map[string]any{"code": "2100-VATOUT-" + suffix, "name": "VAT Output", "account_type": "liability", "report_group": "tax_output", "normal_balance": "credit", "status": "active"})

	posting, err := graph.documents.Create("ledger_posting", orgID, locID, actorID, map[string]any{
		"source_document_type": "invoice",
		"source_document_id":   "invoice-" + suffix,
		"posting_date":         postingDate,
		"currency_code":        "IDR",
		"posting_rule_key":     "invoice_issue_default",
		"total_amount":         111.0,
		"journal_lines": []map[string]any{
			{"account_code": "1100-AR-" + suffix, "account_name": "Accounts Receivable", "description": "AR", "debit": 111.0, "credit": 0.0},
			{"account_code": "4000-REV-" + suffix, "account_name": "Revenue", "description": "Revenue", "debit": 0.0, "credit": 100.0},
			{"account_code": "2100-VATOUT-" + suffix, "account_name": "VAT Output", "description": "Tax Payable", "debit": 0.0, "credit": 11.0},
		},
	})
	if err != nil {
		t.Fatalf("create ledger posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := graph.documents.Save(posting); err != nil {
		t.Fatalf("save ledger posting: %v", err)
	}

	tb := graph.financeReporting.TrialBalance(orgID, locID, fromDate, toDate)
	var matchedRows int
	var matchedDebit float64
	var matchedCredit float64
	for _, row := range tb.Rows {
		if row.AccountCode != "1100-AR-"+suffix && row.AccountCode != "4000-REV-"+suffix && row.AccountCode != "2100-VATOUT-"+suffix {
			continue
		}
		matchedRows++
		matchedDebit += row.Debit
		matchedCredit += row.Credit
	}
	if matchedRows != 3 {
		t.Fatalf("expected 3 matching rows, got %d in %+v", matchedRows, tb.Rows)
	}
	if matchedDebit != 111.0 || matchedCredit != 111.0 {
		t.Fatalf("unexpected matched trial balance totals: debit=%.2f credit=%.2f", matchedDebit, matchedCredit)
	}
	pnl := graph.financeReporting.ProfitAndLoss(orgID, locID, fromDate, toDate)
	var matchedRevenue float64
	for _, section := range pnl.Sections {
		for _, row := range section.Rows {
			if row.AccountCode == "4000-REV-"+suffix {
				matchedRevenue += row.Amount
			}
		}
	}
	if matchedRevenue != 100.0 {
		t.Fatalf("expected matched revenue 100, got %.2f", matchedRevenue)
	}

	period, err := graph.models.Create("accounting_period", actorID, map[string]any{
		"organization_id": orgID,
		"location_id":     locID,
		"period_key":      "2099-03-" + suffix,
		"start_date":      fromDate,
		"end_date":        toDate,
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create accounting period: %v", err)
	}
	if _, err := graph.financeReporting.CloseAccountingPeriod(period.ID, actorID, orgID, locID); err != nil {
		t.Fatalf("close accounting period: %v", err)
	}
	if err := graph.financeReporting.ValidatePostingDateOpen(orgID, locID, "2099-03-21"); err == nil {
		t.Fatal("expected posting date in closed period to be rejected")
	}
}
