package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/store"
)

func TestFinanceReconciliationPostgresAgingAndReconciliation(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed finance reconciliation test")
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
	asOfDate := "2099-06-30"
	if _, err := graph.models.Create("finance_account", actorID, map[string]any{"code": "1100-AR-REC-" + suffix, "name": "Accounts Receivable", "account_type": "asset", "report_group": "accounts_receivable", "normal_balance": "debit", "status": "active"}); err != nil {
		t.Fatalf("create ar finance account: %v", err)
	}

	invoice, err := graph.documents.Create("invoice", orgID, locID, actorID, map[string]any{
		"party_id":                "party-rec-" + suffix,
		"party_name":              "Reconciliation Party " + suffix,
		"invoice_date":            "2099-06-01",
		"due_date":                "2099-06-15",
		"total_amount":            220.0,
		"paid_amount":             0.0,
		"credited_amount":         0.0,
		"refunded_amount":         0.0,
		"balance_due_amount":      220.0,
		"receivable_account_code": "1100-AR-REC-" + suffix,
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Number = "INV-REC-" + suffix
	invoice.Header.Status = "issued"
	if err := graph.documents.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	posting, err := graph.documents.Create("ledger_posting", orgID, locID, actorID, map[string]any{
		"source_document_type": "invoice",
		"source_document_id":   invoice.Header.ID,
		"posting_date":         "2099-06-01",
		"currency_code":        "USD",
		"posting_rule_key":     "invoice_issue_default",
		"total_amount":         220.0,
		"journal_lines": []map[string]any{
			{"account_code": "1100-AR-REC-" + suffix, "account_name": "Accounts Receivable", "description": "AR", "debit": 220.0, "credit": 0.0},
			{"account_code": "4000-REV-REC-" + suffix, "account_name": "Revenue", "description": "Revenue", "debit": 0.0, "credit": 220.0},
		},
	})
	if err != nil {
		t.Fatalf("create ledger posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := graph.documents.Save(posting); err != nil {
		t.Fatalf("save ledger posting: %v", err)
	}

	aging := graph.financeReconcile.ARAging(orgID, locID, asOfDate, "party-rec-"+suffix, "")
	if aging.Totals["overdue_1_30"] != 220.0 {
		t.Fatalf("expected ar aging overdue_1_30 220, got %+v", aging.Totals)
	}
	recon := graph.financeReconcile.ARReconciliation(orgID, locID, asOfDate, "party-rec-"+suffix, "1100-AR-REC-"+suffix)
	if recon.SubledgerTotal != 220.0 || recon.GLTotal != 220.0 || recon.Difference != 0.0 {
		t.Fatalf("unexpected reconciliation totals: %+v", recon)
	}
}
