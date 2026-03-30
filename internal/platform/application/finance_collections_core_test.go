package application

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestFinanceCollectionsGenerateARStatementRun(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceCollectionsTestDocumentTypes(t, docs)
	registerFinanceCollectionsTestModels(t, models)
	reporting := NewFinanceReportingCoreService(docs, models, nil)
	recon := NewFinanceReconciliationCoreService(docs, models, reporting)
	svc := NewFinanceCollectionsCoreService(docs, models, recon, NewCommercialCoreService(docs, nil, models, nil), NewProcurementCoreService(docs, nil, models, nil), reporting)

	invoice, err := docs.Create("invoice", "org_default", "loc_hq", "user_admin", map[string]any{
		"party_id":                "party-a",
		"party_name":              "Party A",
		"invoice_date":            "2099-04-01",
		"due_date":                "2099-04-15",
		"total_amount":            220.0,
		"paid_amount":             100.0,
		"credited_amount":         0.0,
		"refunded_amount":         0.0,
		"writeoff_amount":         0.0,
		"balance_due_amount":      120.0,
		"receivable_account_code": "1100-AR",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Number = "INV-AR-1"
	invoice.Header.Status = "partially_paid"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	record, err := svc.GenerateARStatementRun("org_default", "loc_hq", "party-a", "2099-04-30", "user_admin")
	if err != nil {
		t.Fatalf("generate statement run: %v", err)
	}
	if record.ModelKey != "party_statement_run" {
		t.Fatalf("expected party_statement_run, got %s", record.ModelKey)
	}
	if got := numberValue(record.Values["open_amount_total"]); got != 120.0 {
		t.Fatalf("expected open amount 120, got %v", got)
	}
}

func TestFinanceCollectionsApplySettlementException(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceCollectionsTestDocumentTypes(t, docs)
	registerFinanceCollectionsTestModels(t, models)
	reporting := NewFinanceReportingCoreService(docs, models, nil)
	recon := NewFinanceReconciliationCoreService(docs, models, reporting)
	commercial := NewCommercialCoreService(docs, nil, models, nil)
	svc := NewFinanceCollectionsCoreService(docs, models, recon, commercial, NewProcurementCoreService(docs, nil, models, nil), reporting)

	invoice, err := docs.Create("invoice", "org_default", "loc_hq", "user_admin", map[string]any{
		"party_id":                "party-a",
		"party_name":              "Party A",
		"invoice_date":            "2099-05-01",
		"due_date":                "2099-05-15",
		"total_amount":            220.0,
		"paid_amount":             0.0,
		"credited_amount":         0.0,
		"refunded_amount":         0.0,
		"writeoff_amount":         0.0,
		"balance_due_amount":      220.0,
		"receivable_account_code": "1100-AR",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-ALLOC-1"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	payment, err := docs.Create("payment_receipt", "org_default", "loc_hq", "user_admin", map[string]any{
		"party_id":                "party-a",
		"party_name":              "Party A",
		"receipt_date":            "2099-05-10",
		"amount_received":         220.0,
		"refunded_amount":         0.0,
		"unapplied_amount":        220.0,
		"receivable_account_code": "1100-AR",
		"allocations":             []map[string]any{},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	payment.Header.Status = "received"
	payment.Header.Number = "PAY-1"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save payment: %v", err)
	}

	exception, err := models.Create("settlement_exception", "user_admin", map[string]any{
		"organization_id":       "org_default",
		"location_id":           "loc_hq",
		"source_key":            "ar|unapplied_cash|1",
		"kind":                  "ar",
		"exception_type":        "unapplied_cash",
		"as_of_date":            "2099-05-31",
		"counterparty_id":       "party-a",
		"counterparty_name":     "Party A",
		"source_payment_id":     payment.Header.ID,
		"source_payment_number": payment.Header.Number,
		"unapplied_amount":      220.0,
		"status":                "open",
	})
	if err != nil {
		t.Fatalf("create exception: %v", err)
	}

	updated, err := svc.ApplySettlementException(exception.ID, invoice.Header.ID, 220.0, "user_admin", "org_default", "loc_hq")
	if err != nil {
		t.Fatalf("apply settlement exception: %v", err)
	}
	if updated.Values["status"] != "applied" {
		t.Fatalf("expected applied status, got %#v", updated.Values["status"])
	}
	reloadedInvoice, err := docs.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if got := numberValue(reloadedInvoice.Body.Payload["balance_due_amount"]); got != 0.0 {
		t.Fatalf("expected invoice balance 0, got %v", got)
	}
}

func TestFinanceCollectionsWriteOffSettlementException(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceCollectionsTestDocumentTypes(t, docs)
	registerFinanceCollectionsTestModels(t, models)
	reporting := NewFinanceReportingCoreService(docs, models, nil)
	recon := NewFinanceReconciliationCoreService(docs, models, reporting)
	svc := NewFinanceCollectionsCoreService(docs, models, recon, NewCommercialCoreService(docs, nil, models, nil), NewProcurementCoreService(docs, nil, models, nil), reporting)

	invoice, err := docs.Create("invoice", "org_default", "loc_hq", "user_admin", map[string]any{
		"party_id":                "party-writeoff",
		"party_name":              "Writeoff Party",
		"invoice_date":            "2099-06-01",
		"due_date":                "2099-06-10",
		"total_amount":            95.0,
		"paid_amount":             0.0,
		"credited_amount":         0.0,
		"refunded_amount":         0.0,
		"writeoff_amount":         0.0,
		"balance_due_amount":      95.0,
		"receivable_account_code": "1100-AR",
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-WRITE-1"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	exception, err := models.Create("settlement_exception", "user_admin", map[string]any{
		"organization_id":        "org_default",
		"location_id":            "loc_hq",
		"source_key":             "ar|write_off_candidate|1",
		"kind":                   "ar",
		"exception_type":         "write_off_candidate",
		"as_of_date":             "2099-06-30",
		"counterparty_id":        "party-writeoff",
		"counterparty_name":      "Writeoff Party",
		"source_document_id":     invoice.Header.ID,
		"source_document_number": invoice.Header.Number,
		"account_code":           "1100-AR",
		"open_amount":            95.0,
		"status":                 "open",
	})
	if err != nil {
		t.Fatalf("create exception: %v", err)
	}

	posting, err := svc.WriteOffSettlementException(exception.ID, "2099-06-30", 95.0, "user_admin", "org_default", "loc_hq")
	if err != nil {
		t.Fatalf("write off exception: %v", err)
	}
	if posting.Header.Type != "ledger_posting" || posting.Header.Status != "posted" {
		t.Fatalf("expected posted ledger posting, got %s/%s", posting.Header.Type, posting.Header.Status)
	}
	reloadedInvoice, err := docs.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if got := numberValue(reloadedInvoice.Body.Payload["writeoff_amount"]); got != 95.0 {
		t.Fatalf("expected writeoff 95, got %v", got)
	}
	if got := numberValue(reloadedInvoice.Body.Payload["balance_due_amount"]); got != 0.0 {
		t.Fatalf("expected balance 0, got %v", got)
	}
}

func TestFinanceCollectionsSyncSettlementExceptionsPreservesOtherKinds(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	registerFinanceCollectionsTestDocumentTypes(t, docs)
	registerFinanceCollectionsTestModels(t, models)
	reporting := NewFinanceReportingCoreService(docs, models, nil)
	recon := NewFinanceReconciliationCoreService(docs, models, reporting)
	svc := NewFinanceCollectionsCoreService(docs, models, recon, NewCommercialCoreService(docs, nil, models, nil), NewProcurementCoreService(docs, nil, models, nil), reporting)

	if _, err := models.Create("settlement_exception", "user_admin", map[string]any{
		"organization_id":   "org_default",
		"location_id":       "loc_hq",
		"source_key":        "ap|overpayment|seed",
		"kind":              "ap",
		"exception_type":    "overpayment",
		"as_of_date":        "2099-05-31",
		"counterparty_id":   "vendor-a",
		"counterparty_name": "Vendor A",
		"open_amount":       45.0,
		"status":            "open",
	}); err != nil {
		t.Fatalf("seed ap exception: %v", err)
	}

	invoice, err := docs.Create("invoice", "org_default", "loc_hq", "user_admin", map[string]any{
		"party_id":           "party-ar",
		"party_name":         "Party AR",
		"invoice_date":       "2099-05-01",
		"due_date":           "2099-05-05",
		"total_amount":       100.0,
		"paid_amount":        0.0,
		"balance_due_amount": 100.0,
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-AR-1"
	if err := docs.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	payment, err := docs.Create("payment_receipt", "org_default", "loc_hq", "user_admin", map[string]any{
		"party_id":                "party-ar",
		"party_name":              "Party AR",
		"receipt_date":            "2099-05-10",
		"amount_received":         30.0,
		"unapplied_amount":        30.0,
		"receivable_account_code": "1100-AR",
		"allocations":             []map[string]any{},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	payment.Header.Status = "received"
	payment.Header.Number = "PAY-AR-1"
	if err := docs.Save(payment); err != nil {
		t.Fatalf("save payment: %v", err)
	}

	report, err := svc.SyncSettlementExceptions("org_default", "loc_hq", "2099-05-31", "ar", "user_admin")
	if err != nil {
		t.Fatalf("sync settlement exceptions: %v", err)
	}
	if len(report.Items) == 0 {
		t.Fatal("expected ar exceptions after sync")
	}

	apItems, _, err := models.List("settlement_exception", model.Query{
		Page: 1, PageSize: 20, Filters: map[string]string{
			"organization_id": "org_default",
			"location_id":     "loc_hq",
			"as_of_date":      "2099-05-31",
			"kind":            "ap",
		},
	})
	if err != nil {
		t.Fatalf("list ap exceptions: %v", err)
	}
	if len(apItems) != 1 {
		t.Fatalf("expected 1 ap exception, got %d", len(apItems))
	}
	if got := textValue(apItems[0].Values["status"]); got != "open" {
		t.Fatalf("expected ap exception to remain open, got %s", got)
	}
}

func registerFinanceCollectionsTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{Key: "party_statement_run", DisplayName: "Party Statement Run", Version: "v1", CreatePermissionKey: "party_statement_run.create", ListPermissionKey: "party_statement_run.list", ReadPermissionKey: "party_statement_run.read", UpdatePermissionKey: "party_statement_run.update", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string", Required: true}, {Key: "location_id", Type: "string"}, {Key: "party_id", Type: "string", Required: true}, {Key: "as_of_date", Type: "string", Required: true}}},
		{Key: "vendor_statement_run", DisplayName: "Vendor Statement Run", Version: "v1", CreatePermissionKey: "vendor_statement_run.create", ListPermissionKey: "vendor_statement_run.list", ReadPermissionKey: "vendor_statement_run.read", UpdatePermissionKey: "vendor_statement_run.update", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string", Required: true}, {Key: "location_id", Type: "string"}, {Key: "vendor_id", Type: "string", Required: true}, {Key: "as_of_date", Type: "string", Required: true}}},
		{Key: "collection_case", DisplayName: "Collection Case", Version: "v1", CreatePermissionKey: "collection_case.create", ListPermissionKey: "collection_case.list", ReadPermissionKey: "collection_case.read", UpdatePermissionKey: "collection_case.update", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string", Required: true}, {Key: "location_id", Type: "string"}, {Key: "kind", Type: "string", Required: true}, {Key: "counterparty_id", Type: "string", Required: true}}},
		{Key: "settlement_exception", DisplayName: "Settlement Exception", Version: "v1", CreatePermissionKey: "settlement_exception.create", ListPermissionKey: "settlement_exception.list", ReadPermissionKey: "settlement_exception.read", UpdatePermissionKey: "settlement_exception.update", Fields: []model.FieldDefinition{{Key: "organization_id", Type: "string", Required: true}, {Key: "location_id", Type: "string"}, {Key: "source_key", Type: "string", Required: true}, {Key: "kind", Type: "string", Required: true}, {Key: "exception_type", Type: "string", Required: true}, {Key: "as_of_date", Type: "string", Required: true}, {Key: "status", Type: "string"}}},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil {
			t.Fatalf("register model %s: %v", def.Key, err)
		}
	}
}

func registerFinanceCollectionsTestDocumentTypes(t *testing.T, docs *document.Service) {
	t.Helper()
	for _, def := range []document.Definition{
		{Type: "invoice", DisplayName: "Invoice", SchemaVersion: "v1", AllowedLinkTypes: []string{"payment_for", "posting_for"}},
		{Type: "payment_receipt", DisplayName: "Payment Receipt", SchemaVersion: "v1", AllowedLinkTypes: []string{"payment_for", "posting_for"}},
		{Type: "vendor_bill", DisplayName: "Vendor Bill", SchemaVersion: "v1", AllowedLinkTypes: []string{"payment_for", "posting_for"}},
		{Type: "payment_out", DisplayName: "Payment Out", SchemaVersion: "v1", AllowedLinkTypes: []string{"payment_for", "posting_for"}},
		{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}},
	} {
		if err := docs.Register(def); err != nil {
			t.Fatalf("register document definition %s: %v", def.Type, err)
		}
	}
}
