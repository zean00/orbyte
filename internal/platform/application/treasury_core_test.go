package application

import (
	"strings"
	"testing"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestTreasuryStatementReconciliationAndMatch(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1"}); err != nil {
		t.Fatalf("register ledger posting: %v", err)
	}
	models := model.NewService()
	registerTreasuryTestModels(t, models)
	finance := NewFinanceReportingCoreService(docs, models, config.NewService())
	svc := NewTreasuryCoreService(docs, models, config.NewService(), finance, nil)

	account, err := models.Create("treasury_account", "user_admin", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_main",
		"account_code":    "BANK-001",
		"name":            "Main Bank",
		"treasury_type":   "bank",
		"gl_account_code": "1010-BANK",
		"currency_code":   "IDR",
		"status":          "active",
	})
	if err != nil {
		t.Fatalf("create treasury account: %v", err)
	}
	posting, err := docs.Create("ledger_posting", "org_default", "loc_main", "user_admin", map[string]any{
		"posting_date":        "2099-10-31",
		"currency_code":       "IDR",
		"journal_source_kind": "system",
		"journal_lines": []map[string]any{
			{"account_code": "1010-BANK", "debit": 100.0, "credit": 0.0},
			{"account_code": "1100-AR", "debit": 0.0, "credit": 100.0},
		},
		"total_amount": 100.0,
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	posting.Header.Status = "posted"
	posting.Header.TotalAmount.AmountMinor = 10000
	if err := docs.Save(posting); err != nil {
		t.Fatalf("save posting: %v", err)
	}

	result, err := svc.CreateManualStatement("org_default", "loc_main", account.ID, "user_admin", map[string]any{
		"statement_number": "STMT-001",
		"statement_date":   "2099-10-31",
		"from_date":        "2099-10-01",
		"to_date":          "2099-10-31",
		"opening_balance":  0.0,
		"lines": []map[string]any{
			{"statement_date": "2099-10-31", "reference": "DEP-1", "description": "Deposit", "credit_amount": 100.0},
		},
	})
	if err != nil {
		t.Fatalf("create manual statement: %v", err)
	}
	statement := result["statement"].(model.Record)
	reconciliation, err := svc.SyncBankReconciliation("org_default", "loc_main", statement.ID, "user_admin")
	if err != nil {
		t.Fatalf("sync reconciliation: %v", err)
	}
	if got := numberValue(reconciliation.Values["difference_amount"]); got != 0 {
		t.Fatalf("expected reconciliation difference 0, got %v", got)
	}
	exceptions := svc.ExceptionReport("org_default", "loc_main", "2099-10-31", "open")
	if len(exceptions.Items) != 1 {
		t.Fatalf("expected 1 open treasury exception before match, got %d", len(exceptions.Items))
	}
	lines, _, err := models.List("bank_statement_line", model.Query{Page: 1, PageSize: 10, Filters: map[string]string{"bank_statement_id": statement.ID}})
	if err != nil || len(lines) != 1 {
		t.Fatalf("list statement lines: %v len=%d", err, len(lines))
	}
	if _, err := svc.MatchStatementLine(reconciliation.ID, lines[0].ID, "user_admin", map[string]any{
		"source_type": "ledger_posting",
		"source_id":   posting.Header.ID,
		"amount":      100.0,
	}); err != nil {
		t.Fatalf("match statement line: %v", err)
	}
	exceptions = svc.ExceptionReport("org_default", "loc_main", "2099-10-31", "open")
	if len(exceptions.Items) != 0 {
		t.Fatalf("expected no open treasury exceptions after match, got %d", len(exceptions.Items))
	}
	report := svc.BankReconciliation("org_default", "loc_main", statement.ID)
	if len(report.Lines) != 1 || report.Lines[0].MatchStatus != "matched" {
		t.Fatalf("expected matched line, got %+v", report.Lines)
	}
}

func TestTreasuryTransferPostingAndRegister(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1"}); err != nil {
		t.Fatalf("register ledger posting: %v", err)
	}
	models := model.NewService()
	registerTreasuryTestModels(t, models)
	svc := NewTreasuryCoreService(docs, models, config.NewService(), NewFinanceReportingCoreService(docs, models, config.NewService()), nil)

	from, _ := models.Create("treasury_account", "user_admin", map[string]any{"organization_id": "org_default", "location_id": "loc_main", "account_code": "CASH", "name": "Cash Drawer", "treasury_type": "petty_cash", "gl_account_code": "1000-CASH"})
	to, _ := models.Create("treasury_account", "user_admin", map[string]any{"organization_id": "org_default", "location_id": "loc_main", "account_code": "BANK", "name": "Main Bank", "treasury_type": "bank", "gl_account_code": "1010-BANK"})
	transfer, err := svc.CreateTransfer("org_default", "loc_main", "user_admin", map[string]any{
		"transfer_date":            "2099-10-31",
		"from_treasury_account_id": from.ID,
		"to_treasury_account_id":   to.ID,
		"amount":                   55.0,
		"reference":                "DEP-55",
	})
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	result, err := svc.ApproveTransfer(transfer.ID, "manager_1")
	if err != nil {
		t.Fatalf("approve transfer: %v", err)
	}
	updated := result["transfer"].(model.Record)
	if got := textValue(updated.Values["status"]); got != "posted" {
		t.Fatalf("expected posted transfer, got %q", got)
	}
	if got := textValue(updated.Values["posting_id"]); got == "" {
		t.Fatal("expected posting id on transfer")
	}
	register := svc.TransferRegister("org_default", "loc_main", "2099-10-31", "")
	if len(register.Rows) != 1 || register.Rows[0].Amount != 55 {
		t.Fatalf("unexpected transfer register: %+v", register.Rows)
	}
	if _, err := svc.ApproveTransfer(transfer.ID, "manager_1"); err == nil {
		t.Fatal("expected duplicate transfer approval to fail")
	}
}

func TestTreasuryCashPositionAndClearingReport(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1"}); err != nil {
		t.Fatalf("register ledger posting: %v", err)
	}
	if err := docs.Register(document.Definition{Type: "payment_receipt", DisplayName: "Payment Receipt", SchemaVersion: "v1"}); err != nil {
		t.Fatalf("register payment receipt: %v", err)
	}
	models := model.NewService()
	mustRegisterRetailFinanceTestModels(t, models)
	registerTreasuryTestModels(t, models)
	cfg := config.NewService()
	finance := NewFinanceReportingCoreService(docs, models, cfg)
	retail := NewRetailFinanceCoreService(docs, models, cfg, finance)
	svc := NewTreasuryCoreService(docs, models, cfg, finance, retail)

	if _, err := models.Create("treasury_account", "user_admin", map[string]any{
		"organization_id": "org_default",
		"location_id":     "loc_main",
		"account_code":    "BANK-001",
		"name":            "Main Bank",
		"treasury_type":   "bank",
		"gl_account_code": "1010-BANK",
		"currency_code":   "IDR",
		"status":          "active",
	}); err != nil {
		t.Fatalf("create treasury account: %v", err)
	}
	posting, _ := docs.Create("ledger_posting", "org_default", "loc_main", "user_admin", map[string]any{
		"posting_date":  "2099-10-31",
		"currency_code": "IDR",
		"journal_lines": []map[string]any{
			{"account_code": "1010-BANK", "debit": 80.0, "credit": 0.0},
			{"account_code": "1100-AR", "debit": 0.0, "credit": 80.0},
		},
		"total_amount": 80.0,
	})
	posting.Header.Status = "posted"
	posting.Header.TotalAmount.AmountMinor = 8000
	_ = docs.Save(posting)
	payment, err := docs.Create("payment_receipt", "org_default", "loc_main", "user_admin", map[string]any{
		"payment_date":          "2099-10-31",
		"clearing_account_code": "1010-BANK",
		"unapplied_amount":      25.0,
	})
	if err != nil {
		t.Fatalf("create payment receipt: %v", err)
	}
	payment.Header.Status = "received"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save payment receipt: %v", err)
	}
	if _, err := models.Create("pos_tender_settlement", "user_admin", map[string]any{
		"organization_id":       "org_default",
		"location_id":           "loc_main",
		"reconciliation_id":     "recon-1",
		"tender_type_code":      "CARD",
		"tender_kind":           "card",
		"clearing_account_code": "1015-CARD",
		"expected_amount":       40.0,
		"settled_amount":        10.0,
		"difference_amount":     30.0,
		"status":                "partially_settled",
	}); err != nil {
		t.Fatalf("create pos settlement: %v", err)
	}
	position := svc.CashPositionReport("org_default", "loc_main", "2099-10-31")
	if len(position.Rows) != 1 || position.Rows[0].BookBalance != 80 {
		t.Fatalf("unexpected cash position: %+v", position.Rows)
	}
	clearing := svc.ClearingBalanceReport("org_default", "loc_main", "2099-10-31")
	if len(clearing.Rows) != 2 {
		t.Fatalf("expected 2 clearing rows, got %d", len(clearing.Rows))
	}
}

func TestTreasuryApproveBankReconciliationBlocksDifferencesAndOpenExceptions(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerTreasuryTestModels(t, models)
	svc := NewTreasuryCoreService(docs, models, config.NewService(), NewFinanceReportingCoreService(docs, models, config.NewService()), nil)

	reconciliation, err := models.Create("bank_reconciliation", "user_admin", map[string]any{
		"organization_id":   "org_default",
		"location_id":       "loc_main",
		"treasury_account_id": "acct_1",
		"bank_statement_id": "stmt_1",
		"difference_amount": 15.0,
		"status":            "draft",
	})
	if err != nil {
		t.Fatalf("create reconciliation: %v", err)
	}
	if _, err := svc.ApproveBankReconciliation(reconciliation.ID, "manager_1"); err == nil {
		t.Fatal("expected approval with differences to fail")
	}
	updated, err := models.Update("bank_reconciliation", reconciliation.ID, "user_admin", mergeModelValues(reconciliation.Values, map[string]any{"difference_amount": 0.0}), reconciliation.Version)
	if err != nil {
		t.Fatalf("update reconciliation: %v", err)
	}
	if _, err := models.Create("treasury_exception", "user_admin", map[string]any{
		"organization_id":   "org_default",
		"location_id":       "loc_main",
		"bank_statement_id": "stmt_1",
		"exception_kind":    "unmatched_statement_line",
		"amount":            15.0,
		"status":            "open",
	}); err != nil {
		t.Fatalf("create exception: %v", err)
	}
	if _, err := svc.ApproveBankReconciliation(updated.ID, "manager_1"); err == nil {
		t.Fatal("expected approval with open exception to fail")
	}
}

func registerTreasuryTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{Key: "treasury_account", DisplayName: "Treasury Account", Version: "v1", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "account_code", Type: "string"}, {Key: "name", Type: "string"}, {Key: "treasury_type", Type: "string"}, {Key: "currency_code", Type: "string"}, {Key: "gl_account_code", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "bank_statement", DisplayName: "Bank Statement", Version: "v1", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "treasury_account_id", Type: "string"}, {Key: "statement_number", Type: "string"}, {Key: "statement_date", Type: "string"}, {Key: "from_date", Type: "string"}, {Key: "to_date", Type: "string"}, {Key: "opening_balance", Type: "number"}, {Key: "closing_balance", Type: "number"}, {Key: "import_method", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "bank_statement_line", DisplayName: "Bank Statement Line", Version: "v1", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "bank_statement_id", Type: "string"}, {Key: "treasury_account_id", Type: "string"}, {Key: "statement_date", Type: "string"}, {Key: "value_date", Type: "string"}, {Key: "reference", Type: "string"}, {Key: "description", Type: "string"}, {Key: "debit_amount", Type: "number"}, {Key: "credit_amount", Type: "number"}, {Key: "signed_amount", Type: "number"}, {Key: "matched_amount", Type: "number"}, {Key: "remaining_amount", Type: "number"}, {Key: "match_status", Type: "string"}, {Key: "matched_source_type", Type: "string"}, {Key: "matched_source_id", Type: "string"}}},
		{Key: "bank_reconciliation", DisplayName: "Bank Reconciliation", Version: "v1", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "treasury_account_id", Type: "string"}, {Key: "bank_statement_id", Type: "string"}, {Key: "reconciliation_date", Type: "string"}, {Key: "book_balance", Type: "number"}, {Key: "statement_balance", Type: "number"}, {Key: "matched_amount", Type: "number"}, {Key: "outstanding_book_amount", Type: "number"}, {Key: "difference_amount", Type: "number"}, {Key: "status", Type: "string"}, {Key: "approved_by", Type: "string"}, {Key: "approved_at", Type: "string"}}},
		{Key: "bank_reconciliation_match", DisplayName: "Bank Reconciliation Match", Version: "v1", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "bank_reconciliation_id", Type: "string"}, {Key: "bank_statement_line_id", Type: "string"}, {Key: "matched_source_type", Type: "string"}, {Key: "matched_source_id", Type: "string"}, {Key: "matched_amount", Type: "number"}, {Key: "match_kind", Type: "string"}}},
		{Key: "treasury_transfer", DisplayName: "Treasury Transfer", Version: "v1", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "transfer_date", Type: "string"}, {Key: "from_treasury_account_id", Type: "string"}, {Key: "to_treasury_account_id", Type: "string"}, {Key: "from_account_code", Type: "string"}, {Key: "to_account_code", Type: "string"}, {Key: "amount", Type: "number"}, {Key: "reference", Type: "string"}, {Key: "posting_id", Type: "string"}, {Key: "status", Type: "string"}}},
		{Key: "treasury_exception", DisplayName: "Treasury Exception", Version: "v1", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string"}, {Key: "location_id", Type: "string"}, {Key: "treasury_account_id", Type: "string"}, {Key: "bank_statement_id", Type: "string"}, {Key: "bank_statement_line_id", Type: "string"}, {Key: "exception_kind", Type: "string"}, {Key: "statement_date", Type: "string"}, {Key: "description", Type: "string"}, {Key: "amount", Type: "number"}, {Key: "status", Type: "string"}, {Key: "note", Type: "string"}, {Key: "resolved_at", Type: "string"}, {Key: "resolved_by", Type: "string"}}},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil && !strings.Contains(err.Error(), "already registered") {
			t.Fatalf("register treasury model %s: %v", def.Key, err)
		}
	}
}
