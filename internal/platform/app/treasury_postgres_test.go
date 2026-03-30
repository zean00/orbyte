package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/model"
	"orbyte/internal/platform/store"
)

func TestTreasuryPostgresReconciliationAndTransfer(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed treasury test")
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
	account, err := graph.models.Create("treasury_account", "user_admin", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"account_code":    "BANK-" + suffix,
		"name":            "Main Bank " + suffix,
		"treasury_type":   "bank",
		"currency_code":   "IDR",
		"gl_account_code": "1010-BANK-" + suffix,
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create treasury account: %v", err)
	}
	posting, err := graph.documents.Create("ledger_posting", "org_default", "loc_hq", "user_admin", map[string]any{
		"posting_date":  "2099-10-31",
		"currency_code": "IDR",
		"journal_lines": []map[string]any{
			{"account_code": "1010-BANK-" + suffix, "debit": 125.0, "credit": 0.0},
			{"account_code": "1100-AR", "debit": 0.0, "credit": 125.0},
		},
		"total_amount": 125.0,
	})
	if err != nil {
		t.Fatalf("create treasury posting: %v", err)
	}
	posting.Header.Status = "posted"
	posting.Header.TotalAmount.AmountMinor = 12500
	if err := graph.documents.Save(posting); err != nil {
		t.Fatalf("save treasury posting: %v", err)
	}

	statementResult, err := graph.treasuryCore.CreateManualStatement("org_default", "loc_hq", account.ID, "user_admin", map[string]any{
		"statement_number": "STMT-" + suffix,
		"statement_date":   "2099-10-31",
		"from_date":        "2099-10-01",
		"to_date":          "2099-10-31",
		"opening_balance":  0.0,
		"lines": []map[string]any{
			{"statement_date": "2099-10-31", "reference": "DEP-" + suffix, "description": "Customer Deposit", "credit_amount": 125.0},
		},
	})
	if err != nil {
		t.Fatalf("create statement: %v", err)
	}
	statement := statementResult["statement"].(model.Record)
	reconciliation, err := graph.treasuryCore.SyncBankReconciliation("org_default", "loc_hq", statement.ID, "user_admin")
	if err != nil {
		t.Fatalf("sync reconciliation: %v", err)
	}
	lines, _, err := graph.models.List("bank_statement_line", model.Query{Page: 1, PageSize: 10, Filters: map[string]string{"bank_statement_id": statement.ID}})
	if err != nil || len(lines) != 1 {
		t.Fatalf("list statement lines: %v len=%d", err, len(lines))
	}
	if _, err := graph.treasuryCore.MatchStatementLine(reconciliation.ID, lines[0].ID, "user_admin", map[string]any{
		"source_type": "ledger_posting",
		"source_id":   posting.Header.ID,
		"amount":      125.0,
	}); err != nil {
		t.Fatalf("match statement line: %v", err)
	}
	if _, err := graph.treasuryCore.ApproveBankReconciliation(reconciliation.ID, "user_admin"); err != nil {
		t.Fatalf("approve reconciliation: %v", err)
	}
	position := graph.treasuryCore.CashPositionReport("org_default", "loc_hq", "2099-10-31")
	if len(position.Rows) == 0 {
		t.Fatal("expected cash position rows")
	}

	petty, err := graph.models.Create("treasury_account", "user_admin", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_hq",
		"account_code":    "PETTY-" + suffix,
		"name":            "Petty Cash " + suffix,
		"treasury_type":   "petty_cash",
		"currency_code":   "IDR",
		"gl_account_code": "1000-CASH-" + suffix,
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create petty cash account: %v", err)
	}
	transfer, err := graph.treasuryCore.CreateTransfer("org_default", "loc_hq", "user_admin", map[string]any{
		"transfer_date":            "2099-10-31",
		"from_treasury_account_id": petty.ID,
		"to_treasury_account_id":   account.ID,
		"amount":                   25.0,
		"reference":                "XFER-" + suffix,
	})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	result, err := graph.treasuryCore.ApproveTransfer(transfer.ID, "user_admin")
	if err != nil {
		t.Fatalf("approve transfer: %v", err)
	}
	updated := result["transfer"].(model.Record)
	if got := updated.Values["posting_id"]; got == "" {
		t.Fatal("expected posting id on transfer")
	}
	register := graph.treasuryCore.TransferRegister("org_default", "loc_hq", "2099-10-31", "")
	if len(register.Rows) != 1 {
		t.Fatalf("expected 1 transfer row, got %d", len(register.Rows))
	}
}
