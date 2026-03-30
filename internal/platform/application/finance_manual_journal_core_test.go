package application

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestFinanceManualJournalApproveRequiresDifferentUser(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	service := NewFinanceManualJournalCoreService(docs, nil)
	record, err := docs.Create("ledger_posting", "org-1", "loc-1", "maker", map[string]any{
		"posting_date":        "2099-03-31",
		"currency_code":       "IDR",
		"journal_source_kind": "manual",
		"submitted_by":        "maker",
		"journal_lines": []map[string]any{
			{"account_code": "6100-EXP", "debit": 100.0, "credit": 0.0},
			{"account_code": "2100-ACC", "debit": 0.0, "credit": 100.0},
		},
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	record.Header.Status = "submitted"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save submitted posting: %v", err)
	}
	if err := service.ValidateAction(record, "approve", "maker"); err == nil {
		t.Fatal("expected same-user approval to be rejected")
	}
	if err := service.ValidateAction(record, "approve", "approver"); err != nil {
		t.Fatalf("expected different user approval to pass, got %v", err)
	}
}

func TestCommercialHandleManualJournalActionMetadata(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	commercial := NewCommercialCoreService(docs, nil, nil, nil)
	manual := NewFinanceManualJournalCoreService(docs, nil)
	commercial.SetManualJournals(manual)
	record, err := docs.Create("ledger_posting", "org-1", "loc-1", "maker", map[string]any{
		"posting_date":        "2099-03-31",
		"currency_code":       "IDR",
		"journal_source_kind": "manual",
		"manual_journal_type": "adjusting",
		"journal_lines": []map[string]any{
			{"account_code": "6100-EXP", "debit": 100.0, "credit": 0.0},
			{"account_code": "2100-ACC", "debit": 0.0, "credit": 100.0},
		},
	})
	if err != nil {
		t.Fatalf("create posting: %v", err)
	}
	record.Header.Status = "submitted"
	if err := docs.Save(record); err != nil {
		t.Fatalf("save submitted posting: %v", err)
	}
	if err := commercial.HandleAction(record, "submit", "maker", "month end accrual"); err != nil {
		t.Fatalf("handle submit: %v", err)
	}
	submitted, err := docs.Get(record.Header.ID)
	if err != nil {
		t.Fatalf("get submitted posting: %v", err)
	}
	if got := textValue(submitted.Body.Payload["submitted_by"]); got != "maker" {
		t.Fatalf("expected submitted_by maker, got %q", got)
	}
	if got := textValue(submitted.Body.Payload["submission_note"]); got != "month end accrual" {
		t.Fatalf("expected submission note, got %q", got)
	}
	if submitted.Header.Version <= record.Header.Version {
		t.Fatalf("expected submit metadata update to bump version, before=%d after=%d", record.Header.Version, submitted.Header.Version)
	}
	if submitted.Header.ETag == record.Header.ETag {
		t.Fatalf("expected submit metadata update to bump etag")
	}
	submitted.Header.Status = "posted"
	if err := docs.Save(submitted); err != nil {
		t.Fatalf("save posted posting: %v", err)
	}
	if err := commercial.HandleAction(submitted, "approve", "approver", "approved"); err != nil {
		t.Fatalf("handle approve: %v", err)
	}
	posted, err := docs.Get(record.Header.ID)
	if err != nil {
		t.Fatalf("get posted posting: %v", err)
	}
	if got := textValue(posted.Body.Payload["approved_by"]); got != "approver" {
		t.Fatalf("expected approved_by approver, got %q", got)
	}
	if got := textValue(posted.Body.Payload["approval_note"]); got != "approved" {
		t.Fatalf("expected approval note, got %q", got)
	}
	if got := textValue(posted.Body.Payload["reversal_status"]); got != "available" {
		t.Fatalf("expected available reversal status, got %q", got)
	}
}

func TestFinanceManualJournalCreateCorrectionJournal(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	service := NewFinanceManualJournalCoreService(docs, nil)
	source, err := docs.Create("ledger_posting", "org-1", "loc-1", "maker", map[string]any{
		"posting_date":        "2099-03-31",
		"currency_code":       "IDR",
		"journal_source_kind": "manual",
		"manual_journal_type": "adjusting",
		"journal_lines": []map[string]any{
			{"account_code": "6100-EXP", "debit": 100.0, "credit": 0.0},
			{"account_code": "2100-ACC", "debit": 0.0, "credit": 100.0},
		},
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	source.Header.Status = "posted"
	if err := docs.Save(source); err != nil {
		t.Fatalf("save source: %v", err)
	}
	correction, err := service.CreateCorrectionJournal(source.Header.ID, "maker", "org-1", "loc-1")
	if err != nil {
		t.Fatalf("create correction journal: %v", err)
	}
	if got := textValue(correction.Body.Payload["manual_journal_type"]); got != "correction" {
		t.Fatalf("expected correction manual type, got %q", got)
	}
	if got := textValue(correction.Body.Payload["correction_of_posting_id"]); got != source.Header.ID {
		t.Fatalf("expected correction_of_posting_id %s, got %q", source.Header.ID, got)
	}
	if got := numberValue(correction.Body.Payload["total_amount"]); got != 100 {
		t.Fatalf("expected correction total_amount 100, got %v", got)
	}
	if got := correction.Header.TotalAmount.AmountMinor; got != 10000 {
		t.Fatalf("expected correction header total minor 10000, got %v", got)
	}
}

func TestFinancePeriodEndReverseManualJournalPosting(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinancePeriodEndTestModels(t, models)
	finance := NewFinanceReportingCoreService(docs, models, nil)
	service := NewFinancePeriodEndCoreService(docs, models, finance)
	for _, period := range []map[string]any{
		{"organization_id": "org-1", "location_id": "loc-1", "period_key": "2099-03", "start_date": "2099-03-01", "end_date": "2099-03-31", "status": "open"},
		{"organization_id": "org-1", "location_id": "loc-1", "period_key": "2099-04", "start_date": "2099-04-01", "end_date": "2099-04-30", "status": "open"},
	} {
		if _, err := models.Create("accounting_period", "tester", period); err != nil {
			t.Fatalf("create period: %v", err)
		}
	}
	source, err := docs.Create("ledger_posting", "org-1", "loc-1", "maker", map[string]any{
		"posting_date":        "2099-03-31",
		"currency_code":       "IDR",
		"journal_source_kind": "manual",
		"manual_journal_type": "adjusting",
		"reversal_status":     "available",
		"total_amount":        100.0,
		"journal_lines": []map[string]any{
			{"account_code": "6100-EXP", "debit": 100.0, "credit": 0.0},
			{"account_code": "2100-ACC", "debit": 0.0, "credit": 100.0},
		},
	})
	if err != nil {
		t.Fatalf("create source posting: %v", err)
	}
	source.Header.Status = "posted"
	if err := docs.Save(source); err != nil {
		t.Fatalf("save source posting: %v", err)
	}
	reversal, err := service.ReverseJournalPosting(source.Header.ID, "2099-04-01", "approver", "org-1", "loc-1")
	if err != nil {
		t.Fatalf("reverse manual journal: %v", err)
	}
	if got := textValue(reversal.Body.Payload["journal_source_kind"]); got != "reversal" {
		t.Fatalf("expected reversal kind, got %q", got)
	}
	if got := textValue(reversal.Body.Payload["reversal_of_posting_id"]); got != source.Header.ID {
		t.Fatalf("expected reversal_of_posting_id %s, got %q", source.Header.ID, got)
	}
}
