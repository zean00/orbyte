package app

import (
	"os"
	"strings"
	"testing"
	"time"

	"orbyte/internal/platform/document"
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

	template, err := graph.models.Create("bank_import_template", "user_admin", map[string]any{
		"organization_id":     "org_default",
		"location_id":         "loc_hq",
		"treasury_account_id": account.ID,
		"template_code":       "CSV-" + suffix,
		"name":                "Treasury Import " + suffix,
		"header_row_index":    0,
		"date_column":         "Txn Date",
		"reference_column":    "Ref",
		"description_column":  "Desc",
		"debit_column":        "Debit",
		"credit_column":       "Credit",
		"balance_column":      "Balance",
		"date_format":         "02/01/2006",
		"sign_convention":     "credit_minus_debit",
		"status":              "active",
	})
	if err != nil {
		t.Fatalf("create import template: %v", err)
	}
	imported, err := graph.treasuryCore.ImportStatementCSV("org_default", "loc_hq", account.ID, "user_admin", map[string]any{
		"bank_import_template_id": template.ID,
		"statement_number":        "STMT-CSV-" + suffix,
		"statement_date":          "2099-11-01",
		"source_file_name":        "statement.csv",
	}, "Txn Date,Ref,Desc,Debit,Credit,Balance\n01/11/2099,FEE-"+suffix+",Monthly bank fee,10.00,0,115.00\n")
	if err != nil {
		t.Fatalf("import statement csv: %v", err)
	}
	importedStatement := imported["statement"].(model.Record)
	importedRecon, err := graph.treasuryCore.SyncBankReconciliation("org_default", "loc_hq", importedStatement.ID, "user_admin")
	if err != nil {
		t.Fatalf("sync imported reconciliation: %v", err)
	}
	_ = importedRecon
	exceptions := graph.treasuryCore.ExceptionReport("org_default", "loc_hq", "2099-11-01", "open")
	var feeException model.Record
	for _, item := range exceptions.Items {
		if textValue(item.Values["bank_statement_id"]) == importedStatement.ID && textValue(item.Values["exception_kind"]) == "bank_fee_candidate" {
			feeException = item
			break
		}
	}
	if feeException.ID == "" {
		t.Fatalf("expected bank fee candidate exception for imported statement, got %+v", exceptions.Items)
	}
	journalResult, err := graph.treasuryCore.CreateExceptionJournal(feeException.ID, "user_admin", map[string]any{"posting_date": "2099-11-01"})
	if err != nil {
		t.Fatalf("create exception journal: %v", err)
	}
	if record := journalResult["record"].(document.Record); strings.TrimSpace(textValue(record.Body.Payload["journal_source_kind"])) != "manual" {
		t.Fatalf("expected manual draft journal, got %+v", record.Body.Payload)
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
	foundTransfer := false
	for _, row := range register.Rows {
		if row.TransferID == updated.ID {
			foundTransfer = true
			break
		}
	}
	if !foundTransfer {
		t.Fatalf("expected transfer register to include %s, got %+v", updated.ID, register.Rows)
	}
}
