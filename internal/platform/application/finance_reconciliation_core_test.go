package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestARAgingUsesExtendedBuckets(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceReportingTestModels(t, models)
	mustRegisterFinanceReconciliationDocumentTypes(t, docs)
	reporting := NewFinanceReportingCoreService(docs, models, config.NewService())
	svc := NewFinanceReconciliationCoreService(docs, models, reporting)

	createAgingInvoice(t, docs, "INV-AR-30", "2026-03-01", "2026-03-16", 100, "1100-AR", "party-1", "Party One")
	createAgingInvoice(t, docs, "INV-AR-75", "2026-02-01", "2026-01-15", 200, "1100-AR", "party-1", "Party One")
	createAgingInvoice(t, docs, "INV-AR-95", "2025-12-01", "2025-12-12", 300, "1100-AR", "party-2", "Party Two")

	report := svc.ARAging("org_default", "loc_hq", "2026-03-31", "", "")
	if report.Totals["overdue_1_30"] != 100 {
		t.Fatalf("expected overdue_1_30 100, got %+v", report.Totals)
	}
	if report.Totals["overdue_61_90"] != 200 {
		t.Fatalf("expected overdue_61_90 200, got %+v", report.Totals)
	}
	if report.Totals["overdue_91_up"] != 300 {
		t.Fatalf("expected overdue_91_up 300, got %+v", report.Totals)
	}
	if len(report.Groups) != 2 {
		t.Fatalf("expected 2 party groups, got %d", len(report.Groups))
	}
}

func TestAPAgingUsesDocumentBalances(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceReportingTestModels(t, models)
	mustRegisterFinanceReconciliationDocumentTypes(t, docs)
	reporting := NewFinanceReportingCoreService(docs, models, config.NewService())
	svc := NewFinanceReconciliationCoreService(docs, models, reporting)

	createAgingBill(t, docs, "BILL-AP-20", "2026-03-01", "2026-03-11", 150, "2000-AP", "vendor-1", "Vendor One")
	createAgingBill(t, docs, "BILL-AP-70", "2026-02-01", "2026-01-20", 250, "2000-AP", "vendor-1", "Vendor One")

	report := svc.APAging("org_default", "loc_hq", "2026-03-31", "", "")
	if report.Totals["overdue_1_30"] != 150 {
		t.Fatalf("expected overdue_1_30 150, got %+v", report.Totals)
	}
	if report.Totals["overdue_61_90"] != 250 {
		t.Fatalf("expected overdue_61_90 250, got %+v", report.Totals)
	}
}

func TestARReconciliationMatchesSubledgerAndGL(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceReportingTestModels(t, models)
	mustRegisterFinanceReconciliationDocumentTypes(t, docs)
	if _, err := models.Create("finance_account", "tester", map[string]any{"code": "1100-AR", "name": "Accounts Receivable", "account_type": "asset", "report_group": "accounts_receivable", "normal_balance": "debit", "status": "active"}); err != nil {
		t.Fatalf("create finance account: %v", err)
	}
	reporting := NewFinanceReportingCoreService(docs, models, config.NewService())
	svc := NewFinanceReconciliationCoreService(docs, models, reporting)

	createAgingInvoice(t, docs, "INV-REC-1", "2026-03-01", "2026-03-15", 220, "1100-AR", "party-1", "Party One")
	postLedger(t, docs, "invoice", "inv-1", "2026-03-01", []map[string]any{
		{"account_code": "1100-AR", "account_name": "Accounts Receivable", "description": "AR", "debit": 220.0, "credit": 0.0},
		{"account_code": "4000-REV", "account_name": "Revenue", "description": "Revenue", "debit": 0.0, "credit": 220.0},
	})

	report := svc.ARReconciliation("org_default", "loc_hq", "2026-03-31", "", "")
	if report.SubledgerTotal != 220 || report.GLTotal != 220 || report.Difference != 0 {
		t.Fatalf("unexpected reconciliation totals: %+v", report)
	}
	if len(report.Mismatches) != 0 {
		t.Fatalf("expected no mismatches, got %+v", report.Mismatches)
	}
}

func TestAPReconciliationFlagsMismatch(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceReportingTestModels(t, models)
	mustRegisterFinanceReconciliationDocumentTypes(t, docs)
	if _, err := models.Create("finance_account", "tester", map[string]any{"code": "2000-AP", "name": "Accounts Payable", "account_type": "liability", "report_group": "accounts_payable", "normal_balance": "credit", "status": "active"}); err != nil {
		t.Fatalf("create finance account: %v", err)
	}
	reporting := NewFinanceReportingCoreService(docs, models, config.NewService())
	svc := NewFinanceReconciliationCoreService(docs, models, reporting)

	createAgingBill(t, docs, "BILL-REC-1", "2026-03-01", "2026-03-15", 300, "2000-AP", "vendor-1", "Vendor One")
	postLedger(t, docs, "vendor_bill", "bill-1", "2026-03-01", []map[string]any{
		{"account_code": "2000-AP", "account_name": "Accounts Payable", "description": "AP", "debit": 0.0, "credit": 250.0},
		{"account_code": "5000-EXP", "account_name": "Expense", "description": "Expense", "debit": 250.0, "credit": 0.0},
	})

	report := svc.APReconciliation("org_default", "loc_hq", "2026-03-31", "", "")
	if report.Difference != 50 {
		t.Fatalf("expected difference 50, got %+v", report)
	}
	if len(report.Mismatches) == 0 {
		t.Fatalf("expected mismatch rows, got %+v", report)
	}
}

func TestARReconciliationScopesGLByPartyFilter(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceReportingTestModels(t, models)
	mustRegisterFinanceReconciliationDocumentTypes(t, docs)
	if _, err := models.Create("finance_account", "tester", map[string]any{"code": "1100-AR", "name": "Accounts Receivable", "account_type": "asset", "report_group": "accounts_receivable", "normal_balance": "debit", "status": "active"}); err != nil {
		t.Fatalf("create finance account: %v", err)
	}
	reporting := NewFinanceReportingCoreService(docs, models, config.NewService())
	svc := NewFinanceReconciliationCoreService(docs, models, reporting)

	invoiceA := createAgingInvoice(t, docs, "INV-PARTY-A", "2026-03-01", "2026-03-15", 100, "1100-AR", "party-a", "Party A")
	invoiceB := createAgingInvoice(t, docs, "INV-PARTY-B", "2026-03-01", "2026-03-15", 200, "1100-AR", "party-b", "Party B")
	postLedger(t, docs, "invoice", invoiceA.Header.ID, "2026-03-01", []map[string]any{
		{"account_code": "1100-AR", "account_name": "Accounts Receivable", "description": "AR", "debit": 100.0, "credit": 0.0},
		{"account_code": "4000-REV", "account_name": "Revenue", "description": "Revenue", "debit": 0.0, "credit": 100.0},
	})
	postLedger(t, docs, "invoice", invoiceB.Header.ID, "2026-03-01", []map[string]any{
		{"account_code": "1100-AR", "account_name": "Accounts Receivable", "description": "AR", "debit": 200.0, "credit": 0.0},
		{"account_code": "4000-REV", "account_name": "Revenue", "description": "Revenue", "debit": 0.0, "credit": 200.0},
	})

	report := svc.ARReconciliation("org_default", "loc_hq", "2026-03-31", "party-a", "")
	if report.SubledgerTotal != 100 || report.GLTotal != 100 || report.Difference != 0 {
		t.Fatalf("expected party-filtered reconciliation to balance for party-a, got %+v", report)
	}
	if len(report.Items) != 1 || report.Items[0].CounterpartyID != "party-a" {
		t.Fatalf("expected only party-a item, got %+v", report.Items)
	}
}

func mustRegisterFinanceReconciliationDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "invoice", DisplayName: "Invoice", SchemaVersion: "v1"},
		{Type: "vendor_bill", DisplayName: "Vendor Bill", SchemaVersion: "v1"},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1"},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}

func createAgingInvoice(t *testing.T, docs *document.Service, number, invoiceDate, dueDate string, balance float64, accountCode, partyID, partyName string) document.Record {
	t.Helper()
	record, err := docs.Create("invoice", "org_default", "loc_hq", "user_admin", map[string]any{
		"party_id":                partyID,
		"party_name":              partyName,
		"invoice_date":            invoiceDate,
		"due_date":                dueDate,
		"total_amount":            balance,
		"paid_amount":             0.0,
		"credited_amount":         0.0,
		"refunded_amount":         0.0,
		"balance_due_amount":      balance,
		"receivable_account_code": accountCode,
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	record.Header.Number = number
	record.Header.Status = "issued"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	return record
}

func createAgingBill(t *testing.T, docs *document.Service, number, billDate, dueDate string, balance float64, accountCode, vendorID, vendorName string) document.Record {
	t.Helper()
	record, err := docs.Create("vendor_bill", "org_default", "loc_hq", "user_admin", map[string]any{
		"vendor_id":             vendorID,
		"vendor_name":           vendorName,
		"bill_date":             billDate,
		"due_date":              dueDate,
		"total_amount":          balance,
		"paid_amount":           0.0,
		"credited_amount":       0.0,
		"balance_due_amount":    balance,
		"payable_account_code":  accountCode,
	})
	if err != nil {
		t.Fatalf("create vendor bill: %v", err)
	}
	record.Header.Number = number
	record.Header.Status = "issued"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save vendor bill: %v", err)
	}
	return record
}

func postLedger(t *testing.T, docs *document.Service, sourceType, sourceID, postingDate string, lines []map[string]any) {
	t.Helper()
	record, err := docs.Create("ledger_posting", "org_default", "loc_hq", "user_admin", map[string]any{
		"source_document_type": sourceType,
		"source_document_id":   sourceID,
		"posting_date":         postingDate,
		"currency_code":        "USD",
		"posting_rule_key":     sourceType + "_posting",
		"total_amount":         0.0,
		"journal_lines":        lines,
	})
	if err != nil {
		t.Fatalf("create ledger posting: %v", err)
	}
	record.Header.Status = "posted"
	record.Header.Number = "GL-" + time.Now().UTC().Format("150405.000")
	if err := docs.Save(record); err != nil {
		t.Fatalf("save ledger posting: %v", err)
	}
}
