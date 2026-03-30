package application

import (
	"testing"
	"time"

	"orbyte/internal/platform/config"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestFinanceAssetCreateFixedAssetAndPreview(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinanceAssetTestModels(t, models)
	svc := NewFinanceAssetCoreService(docs, models, config.NewService(), NewFinanceReportingCoreService(docs, models, config.NewService()))

	result, err := svc.CreateFixedAsset("org-1", "loc-1", "tester", map[string]any{
		"code":          "FA-LAPTOP-01",
		"name":          "Office Laptop",
		"basis_amount":  1200.0,
		"salvage_amount": 0.0,
		"method":        "straight_line",
		"cadence":       "monthly",
		"total_periods": 12,
		"acquisition_date": "2099-10-03",
	})
	if err != nil {
		t.Fatalf("create fixed asset: %v", err)
	}
	asset := result["asset"].(model.Record)
	if got := textValue(asset.Values["linked_journal_template_id"]); got == "" {
		t.Fatal("expected linked journal template")
	}
	preview, err := svc.FixedAssetPreview(asset.ID, "org-1", "loc-1")
	if err != nil {
		t.Fatalf("preview fixed asset: %v", err)
	}
	if preview.NextPostingDate != "2099-10-31" {
		t.Fatalf("expected next posting 2099-10-31, got %q", preview.NextPostingDate)
	}
	if preview.NextAmount != 100 {
		t.Fatalf("expected next amount 100, got %v", preview.NextAmount)
	}
	template, err := models.Get("journal_template", textValue(asset.Values["linked_journal_template_id"]))
	if err != nil {
		t.Fatalf("get journal template: %v", err)
	}
	if got := textValue(template.Values["source_model_type"]); got != "fixed_asset_schedule" {
		t.Fatalf("expected fixed_asset_schedule source type, got %q", got)
	}
	if got := textValue(template.Values["next_due_date"]); got != "2099-10-31" {
		t.Fatalf("expected next_due_date 2099-10-31, got %q", got)
	}
}

func TestFinanceAssetPostingApprovalAndCancelAdvanceSchedule(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinanceAssetTestModels(t, models)
	cfg := config.NewService()
	finance := NewFinanceReportingCoreService(docs, models, cfg)
	periodEnd := NewFinancePeriodEndCoreService(docs, models, finance)
	svc := NewFinanceAssetCoreService(docs, models, cfg, finance)

	result, err := svc.CreateFixedAsset("org-1", "loc-1", "tester", map[string]any{
		"code":             "FA-MACHINE-01",
		"name":             "Machine",
		"basis_amount":     1000.0,
		"method":           "declining_balance",
		"declining_rate_percent": 40.0,
		"cadence":          "monthly",
		"total_periods":    5,
		"acquisition_date": "2099-10-01",
	})
	if err != nil {
		t.Fatalf("create fixed asset: %v", err)
	}
	asset := result["asset"].(model.Record)
	if _, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "loc-1",
		"period_key":      "2099-10",
		"start_date":      "2099-10-01",
		"end_date":        "2099-10-31",
		"status":          "open",
	}); err != nil {
		t.Fatalf("create period: %v", err)
	}
	periods, _, err := models.List("accounting_period", model.Query{Filters: map[string]string{"period_key": "2099-10"}, Page: 1, PageSize: 1})
	if err != nil || len(periods) == 0 {
		t.Fatalf("list periods: %v", err)
	}
	pack, err := periodEnd.GenerateJournalRuns(periods[0].ID, "tester", "org-1", "loc-1")
	if err != nil {
		t.Fatalf("generate runs: %v", err)
	}
	if len(pack.JournalRuns) != 1 {
		t.Fatalf("expected 1 journal run, got %d", len(pack.JournalRuns))
	}
	posting, err := docs.Get(pack.JournalRuns[0].PostingID)
	if err != nil {
		t.Fatalf("get posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := docs.Save(posting); err != nil {
		t.Fatalf("save posting: %v", err)
	}
	if err := periodEnd.HandleApprovedLedgerPosting(posting, "tester"); err != nil {
		t.Fatalf("period end approve: %v", err)
	}
	if err := svc.HandleApprovedLedgerPosting(posting, "tester"); err != nil {
		t.Fatalf("asset approve: %v", err)
	}
	preview, err := svc.FixedAssetPreview(asset.ID, "org-1", "loc-1")
	if err != nil {
		t.Fatalf("preview after approve: %v", err)
	}
	if preview.PeriodsBooked != 1 {
		t.Fatalf("expected 1 booked period, got %d", preview.PeriodsBooked)
	}
	if preview.BookToDate != 400 {
		t.Fatalf("expected booked amount 400, got %v", preview.BookToDate)
	}
	if preview.RemainingAmount != 600 {
		t.Fatalf("expected remaining amount 600, got %v", preview.RemainingAmount)
	}
	if preview.NextPostingDate != "2099-11-30" {
		t.Fatalf("expected next posting 2099-11-30, got %q", preview.NextPostingDate)
	}
	posting.Header.Status = "cancelled"
	if err := docs.Save(posting); err != nil {
		t.Fatalf("save cancelled posting: %v", err)
	}
	if err := svc.HandleCanceledLedgerPosting(posting, "tester"); err != nil {
		t.Fatalf("asset cancel: %v", err)
	}
	preview, err = svc.FixedAssetPreview(asset.ID, "org-1", "loc-1")
	if err != nil {
		t.Fatalf("preview after cancel: %v", err)
	}
	if preview.PeriodsBooked != 0 || preview.BookToDate != 0 || preview.RemainingAmount != 1000 {
		t.Fatalf("expected schedule reset after cancel, got %+v", preview)
	}
	if preview.NextPostingDate != "2099-10-31" {
		t.Fatalf("expected next posting reset to 2099-10-31, got %q", preview.NextPostingDate)
	}
}

func TestFinanceAssetCreatePrepaidFromVendorBillLine(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	if err := docs.Register(document.Definition{Type: "vendor_bill", DisplayName: "Vendor Bill", SchemaVersion: "v1"}); err != nil {
		t.Fatalf("register vendor_bill: %v", err)
	}
	models := model.NewService()
	registerFinanceAssetTestModels(t, models)
	svc := NewFinanceAssetCoreService(docs, models, config.NewService(), NewFinanceReportingCoreService(docs, models, config.NewService()))

	bill, err := docs.Create("vendor_bill", "org-1", "loc-1", "tester", map[string]any{
		"bill_date": "2099-10-10",
		"lines": []map[string]any{
			{"description": "Annual Insurance", "line_subtotal": 1200.0, "line_total": 1320.0},
		},
	})
	if err != nil {
		t.Fatalf("create vendor bill: %v", err)
	}
	result, err := svc.CreatePrepaidFromVendorBill(bill.Header.ID, 0, "org-1", "loc-1", "tester", map[string]any{
		"code":          "PRE-INS-01",
		"name":          "Insurance Prepaid",
		"method":        "straight_line",
		"cadence":       "monthly",
		"total_periods": 12,
	})
	if err != nil {
		t.Fatalf("create prepaid from vendor bill: %v", err)
	}
	prepaid := result["asset"].(model.Record)
	if got := numberValue(prepaid.Values["basis_amount"]); got != 1200 {
		t.Fatalf("expected basis 1200, got %v", got)
	}
	if got := textValue(prepaid.Values["source_vendor_bill_id"]); got != bill.Header.ID {
		t.Fatalf("expected source vendor bill id %q, got %q", bill.Header.ID, got)
	}
	preview, err := svc.PrepaidPreview(prepaid.ID, "org-1", "loc-1")
	if err != nil {
		t.Fatalf("preview prepaid: %v", err)
	}
	if preview.NextAmount != 100 {
		t.Fatalf("expected next amortization amount 100, got %v", preview.NextAmount)
	}
}

func TestFinanceAssetCancelOlderPostingRecomputesSchedule(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinanceAssetTestModels(t, models)
	cfg := config.NewService()
	finance := NewFinanceReportingCoreService(docs, models, cfg)
	svc := NewFinanceAssetCoreService(docs, models, cfg, finance)

	result, err := svc.CreateFixedAsset("org-1", "loc-1", "tester", map[string]any{
		"code":             "FA-OLDER-CANCEL",
		"name":             "Older Cancel Asset",
		"basis_amount":     1200.0,
		"method":           "straight_line",
		"cadence":          "monthly",
		"total_periods":    12,
		"acquisition_date": "2099-10-01",
	})
	if err != nil {
		t.Fatalf("create fixed asset: %v", err)
	}
	asset := result["asset"].(model.Record)
	templateID := textValue(asset.Values["linked_journal_template_id"])
	oct := mustCreatePostedJournalRun(t, docs, models, templateID, "2099-10-31", 100, time.Date(2099, 10, 31, 1, 0, 0, 0, time.UTC))
	nov := mustCreatePostedJournalRun(t, docs, models, templateID, "2099-11-30", 100, time.Date(2099, 11, 30, 1, 0, 0, 0, time.UTC))

	if err := svc.HandleApprovedLedgerPosting(oct, "tester"); err != nil {
		t.Fatalf("approve oct posting: %v", err)
	}
	if err := svc.HandleApprovedLedgerPosting(nov, "tester"); err != nil {
		t.Fatalf("approve nov posting: %v", err)
	}
	preview, err := svc.FixedAssetPreview(asset.ID, "org-1", "loc-1")
	if err != nil {
		t.Fatalf("preview after approvals: %v", err)
	}
	if preview.PeriodsBooked != 2 || preview.BookToDate != 200 || preview.RemainingAmount != 1000 {
		t.Fatalf("unexpected schedule after approvals: %+v", preview)
	}

	oct.Header.Status = "cancelled"
	if err := docs.Save(oct); err != nil {
		t.Fatalf("save cancelled oct posting: %v", err)
	}
	if err := svc.HandleCanceledLedgerPosting(oct, "tester"); err != nil {
		t.Fatalf("cancel older posting: %v", err)
	}
	preview, err = svc.FixedAssetPreview(asset.ID, "org-1", "loc-1")
	if err != nil {
		t.Fatalf("preview after older cancel: %v", err)
	}
	if preview.PeriodsBooked != 1 || preview.BookToDate != 100 || preview.RemainingAmount != 1100 {
		t.Fatalf("expected recomputed schedule after older cancel, got %+v", preview)
	}
	if preview.NextPostingDate != "2099-12-31" {
		t.Fatalf("expected next posting 2099-12-31 after older cancel, got %q", preview.NextPostingDate)
	}
}

func TestFinanceAssetValidateActionRejectsOutOfSequenceApproval(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinanceAssetTestModels(t, models)
	cfg := config.NewService()
	finance := NewFinanceReportingCoreService(docs, models, cfg)
	svc := NewFinanceAssetCoreService(docs, models, cfg, finance)

	result, err := svc.CreatePrepaidExpense("org-1", "loc-1", "tester", map[string]any{
		"code":                   "PRE-OUT-SEQ",
		"name":                   "Out Sequence Prepaid",
		"basis_amount":           600.0,
		"recognition_start_date": "2099-10-01",
		"method":                 "straight_line",
		"cadence":                "monthly",
		"total_periods":          6,
	})
	if err != nil {
		t.Fatalf("create prepaid: %v", err)
	}
	prepaid := result["asset"].(model.Record)
	templateID := textValue(prepaid.Values["linked_journal_template_id"])
	nov := mustCreatePostedJournalRun(t, docs, models, templateID, "2099-11-30", 100, time.Date(2099, 11, 30, 1, 0, 0, 0, time.UTC))
	if err := svc.HandleApprovedLedgerPosting(nov, "tester"); err != nil {
		t.Fatalf("approve nov posting: %v", err)
	}
	octDraft := mustCreateDraftJournalRun(t, docs, models, templateID, "2099-10-31", 100)
	if err := svc.ValidateAction(octDraft, "approve"); err == nil {
		t.Fatal("expected out-of-sequence approval to be rejected")
	}
}

func registerFinanceAssetTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	registerFinancePeriodEndTestModels(t, models)
	defs := []model.Definition{
		{
			Key:         "fixed_asset",
			DisplayName: "Fixed Asset",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "status", Type: "string"},
				{Key: "acquisition_date", Type: "string"},
				{Key: "capitalization_date", Type: "string"},
				{Key: "basis_amount", Type: "number"},
				{Key: "salvage_amount", Type: "number"},
				{Key: "method", Type: "string"},
				{Key: "declining_rate_percent", Type: "number"},
				{Key: "cadence", Type: "string"},
				{Key: "total_periods", Type: "int"},
				{Key: "periods_booked", Type: "int"},
				{Key: "booked_amount", Type: "number"},
				{Key: "remaining_amount", Type: "number"},
				{Key: "next_posting_date", Type: "string"},
				{Key: "asset_account_code", Type: "string"},
				{Key: "accumulated_depreciation_account_code", Type: "string"},
				{Key: "depreciation_expense_account_code", Type: "string"},
				{Key: "schedule_id", Type: "string"},
				{Key: "linked_journal_template_id", Type: "string"},
				{Key: "source_vendor_bill_id", Type: "string"},
				{Key: "source_vendor_bill_number", Type: "string"},
				{Key: "source_vendor_bill_line_index", Type: "int"},
			},
		},
		{
			Key:         "fixed_asset_schedule",
			DisplayName: "Fixed Asset Schedule",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "fixed_asset_id", Type: "string", Required: true},
				{Key: "status", Type: "string"},
				{Key: "start_date", Type: "string"},
				{Key: "method", Type: "string"},
				{Key: "declining_rate_percent", Type: "number"},
				{Key: "cadence", Type: "string"},
				{Key: "basis_amount", Type: "number"},
				{Key: "salvage_amount", Type: "number"},
				{Key: "total_periods", Type: "int"},
				{Key: "periods_booked", Type: "int"},
				{Key: "booked_amount", Type: "number"},
				{Key: "remaining_amount", Type: "number"},
				{Key: "next_posting_date", Type: "string"},
				{Key: "last_posting_id", Type: "string"},
				{Key: "last_posting_date", Type: "string"},
				{Key: "last_posting_amount", Type: "number"},
				{Key: "asset_account_code", Type: "string"},
				{Key: "accumulated_depreciation_account_code", Type: "string"},
				{Key: "depreciation_expense_account_code", Type: "string"},
				{Key: "linked_journal_template_id", Type: "string"},
			},
		},
		{
			Key:         "prepaid_expense",
			DisplayName: "Prepaid Expense",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "status", Type: "string"},
				{Key: "recognition_start_date", Type: "string"},
				{Key: "basis_amount", Type: "number"},
				{Key: "method", Type: "string"},
				{Key: "declining_rate_percent", Type: "number"},
				{Key: "cadence", Type: "string"},
				{Key: "total_periods", Type: "int"},
				{Key: "periods_booked", Type: "int"},
				{Key: "booked_amount", Type: "number"},
				{Key: "remaining_amount", Type: "number"},
				{Key: "next_posting_date", Type: "string"},
				{Key: "prepaid_asset_account_code", Type: "string"},
				{Key: "expense_account_code", Type: "string"},
				{Key: "schedule_id", Type: "string"},
				{Key: "linked_journal_template_id", Type: "string"},
				{Key: "source_vendor_bill_id", Type: "string"},
				{Key: "source_vendor_bill_number", Type: "string"},
				{Key: "source_vendor_bill_line_index", Type: "int"},
			},
		},
		{
			Key:         "prepaid_schedule",
			DisplayName: "Prepaid Schedule",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "prepaid_expense_id", Type: "string", Required: true},
				{Key: "status", Type: "string"},
				{Key: "start_date", Type: "string"},
				{Key: "method", Type: "string"},
				{Key: "declining_rate_percent", Type: "number"},
				{Key: "cadence", Type: "string"},
				{Key: "basis_amount", Type: "number"},
				{Key: "total_periods", Type: "int"},
				{Key: "periods_booked", Type: "int"},
				{Key: "booked_amount", Type: "number"},
				{Key: "remaining_amount", Type: "number"},
				{Key: "next_posting_date", Type: "string"},
				{Key: "last_posting_id", Type: "string"},
				{Key: "last_posting_date", Type: "string"},
				{Key: "last_posting_amount", Type: "number"},
				{Key: "prepaid_asset_account_code", Type: "string"},
				{Key: "expense_account_code", Type: "string"},
				{Key: "linked_journal_template_id", Type: "string"},
			},
		},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil {
			t.Fatalf("register %s: %v", def.Key, err)
		}
	}
}

func mustCreatePostedJournalRun(t *testing.T, docs *document.Service, models *model.Service, templateID, postingDate string, amount float64, createdAt time.Time) document.Record {
	t.Helper()
	record := mustCreateDraftJournalRun(t, docs, models, templateID, postingDate, amount)
	record.Header.Status = "posted"
	record.Header.CreatedAt = createdAt
	record.Header.UpdatedAt = createdAt
	if err := docs.Save(record); err != nil {
		t.Fatalf("save posted journal: %v", err)
	}
	return record
}

func mustCreateDraftJournalRun(t *testing.T, docs *document.Service, models *model.Service, templateID, postingDate string, amount float64) document.Record {
	t.Helper()
	template, err := models.Get("journal_template", templateID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	record, err := docs.Create("ledger_posting", textValue(template.Values["organization_id"]), textValue(template.Values["location_id"]), "tester", map[string]any{
		"source_document_type": "journal_template",
		"source_document_id":   templateID,
		"posting_date":         postingDate,
		"currency_code":        "IDR",
		"posting_rule_key":     "period_end_" + textValue(template.Values["journal_kind"]),
		"journal_source_kind":  textValue(template.Values["journal_kind"]),
		"journal_template_id":  templateID,
		"total_amount":         amount,
		"journal_lines": []map[string]any{
			{"account_code": "6100-T", "description": "Debit", "debit": amount, "credit": 0.0},
			{"account_code": "1500-T", "description": "Credit", "debit": 0.0, "credit": amount},
		},
	})
	if err != nil {
		t.Fatalf("create draft journal: %v", err)
	}
	if _, err := models.Create("journal_run", "tester", map[string]any{
		"organization_id":      textValue(template.Values["organization_id"]),
		"location_id":          textValue(template.Values["location_id"]),
		"accounting_period_id": "period-"+postingDate,
		"period_key":           postingDate[:7],
		"journal_template_id":  templateID,
		"template_code":        textValue(template.Values["code"]),
		"template_name":        textValue(template.Values["name"]),
		"generated_posting_id": record.Header.ID,
		"status":               "generated",
	}); err != nil {
		t.Fatalf("create journal run: %v", err)
	}
	return record
}
