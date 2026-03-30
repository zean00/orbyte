package application

import (
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
)

func TestFinancePeriodEndGenerateRunsAndClose(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinancePeriodEndTestModels(t, models)
	finance := NewFinanceReportingCoreService(docs, models, nil)
	service := NewFinancePeriodEndCoreService(docs, models, finance)

	period, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "loc-1",
		"period_key":      "2099-03",
		"start_date":      "2099-03-01",
		"end_date":        "2099-03-31",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create period: %v", err)
	}
	if _, err := models.Create("journal_template", "tester", map[string]any{
		"organization_id":            "org-1",
		"location_id":                "loc-1",
		"code":                       "ACCRUAL-RENT",
		"name":                       "Rent Accrual",
		"journal_kind":               "accrual",
		"cadence":                    "monthly",
		"currency_code":              "IDR",
		"required_for_period_close":  true,
		"journal_lines": []map[string]any{
			{"account_code": "6100-RENT", "description": "Rent Expense", "debit": 1000.0, "credit": 0.0},
			{"account_code": "2105-ACCRUAL", "description": "Accrual", "debit": 0.0, "credit": 1000.0},
		},
		"status": "active",
	}); err != nil {
		t.Fatalf("create template: %v", err)
	}

	pack, err := service.GenerateJournalRuns(period.ID, "tester", "org-1", "loc-1")
	if err != nil {
		t.Fatalf("generate journal runs: %v", err)
	}
	if len(pack.JournalRuns) != 1 {
		t.Fatalf("expected 1 journal run, got %d", len(pack.JournalRuns))
	}
	if pack.JournalRuns[0].Status != "generated" {
		t.Fatalf("expected generated status, got %q", pack.JournalRuns[0].Status)
	}
	if pack.Ready {
		t.Fatal("expected close pack to be blocked before posting journals/checklist")
	}
	posting, err := docs.Get(pack.JournalRuns[0].PostingID)
	if err != nil {
		t.Fatalf("get generated posting: %v", err)
	}
	posting.Header.Status = "posted"
	if err := docs.Save(posting); err != nil {
		t.Fatalf("save posted journal: %v", err)
	}
	if err := service.HandleApprovedLedgerPosting(posting, "tester"); err != nil {
		t.Fatalf("handle approved ledger posting: %v", err)
	}
	closePack, err := service.ClosePack(period.ID, "tester", "org-1", "loc-1")
	if err != nil {
		t.Fatalf("reload close pack: %v", err)
	}
	for _, task := range closePack.Tasks {
		if task.TaskType == "checklist" {
			if _, err := service.CompleteTask(task.ID, "tester", "org-1", "loc-1"); err != nil {
				t.Fatalf("complete task %s: %v", task.TaskCode, err)
			}
		}
	}
	closed, err := service.CloseAccountingPeriod(period.ID, "tester", "org-1", "loc-1")
	if err != nil {
		t.Fatalf("close accounting period: %v", err)
	}
	if got := closed.Values["status"]; got != "closed" {
		t.Fatalf("expected closed period, got %#v", got)
	}
}

func TestFinancePeriodEndReverseAccrualPosting(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinancePeriodEndTestModels(t, models)
	finance := NewFinanceReportingCoreService(docs, models, nil)
	service := NewFinancePeriodEndCoreService(docs, models, finance)

	if _, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "loc-1",
		"period_key":      "2099-03",
		"start_date":      "2099-03-01",
		"end_date":        "2099-03-31",
		"status":          "open",
	}); err != nil {
		t.Fatalf("create source period: %v", err)
	}
	if _, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "loc-1",
		"period_key":      "2099-04",
		"start_date":      "2099-04-01",
		"end_date":        "2099-04-30",
		"status":          "open",
	}); err != nil {
		t.Fatalf("create next period: %v", err)
	}
	source, err := docs.Create("ledger_posting", "org-1", "loc-1", "tester", map[string]any{
		"source_document_type": "journal_template",
		"source_document_id":   "tmpl-1",
		"posting_date":         "2099-03-31",
		"currency_code":        "IDR",
		"posting_rule_key":     "period_end_accrual",
		"journal_source_kind":  "accrual",
		"reversal_status":      "available",
		"total_amount":         1000.0,
		"journal_lines": []map[string]any{
			{"account_code": "6100-RENT", "description": "Rent Expense", "debit": 1000.0, "credit": 0.0},
			{"account_code": "2105-ACCRUAL", "description": "Accrual", "debit": 0.0, "credit": 1000.0},
		},
	})
	if err != nil {
		t.Fatalf("create source posting: %v", err)
	}
	source.Header.Status = "posted"
	if err := docs.Save(source); err != nil {
		t.Fatalf("save source posting: %v", err)
	}
	reversal, err := service.ReverseAccrualPosting(source.Header.ID, "2099-04-01", "tester", "org-1", "loc-1")
	if err != nil {
		t.Fatalf("reverse accrual posting: %v", err)
	}
	if got := reversal.Body.Payload["journal_source_kind"]; got != "reversal" {
		t.Fatalf("expected reversal journal source kind, got %#v", got)
	}
	updatedSource, err := docs.Get(source.Header.ID)
	if err != nil {
		t.Fatalf("get updated source: %v", err)
	}
	if got := updatedSource.Body.Payload["reversal_status"]; got != "generated" {
		t.Fatalf("expected generated reversal status, got %#v", got)
	}
	reversal.Header.Status = "posted"
	if err := docs.Save(reversal); err != nil {
		t.Fatalf("save reversal posted: %v", err)
	}
	if err := service.HandleApprovedLedgerPosting(reversal, "tester"); err != nil {
		t.Fatalf("handle approved reversal: %v", err)
	}
	finalSource, err := docs.Get(source.Header.ID)
	if err != nil {
		t.Fatalf("get final source: %v", err)
	}
	if got := finalSource.Body.Payload["reversal_status"]; got != "reversed" {
		t.Fatalf("expected reversed status, got %#v", got)
	}
}

func TestFinancePeriodEndReadClosePackDoesNotMutateState(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinancePeriodEndTestModels(t, models)
	finance := NewFinanceReportingCoreService(docs, models, nil)
	service := NewFinancePeriodEndCoreService(docs, models, finance)

	period, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "loc-1",
		"period_key":      "2099-05",
		"start_date":      "2099-05-01",
		"end_date":        "2099-05-31",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create period: %v", err)
	}
	if _, err := models.Create("journal_template", "tester", map[string]any{
		"organization_id":           "org-1",
		"location_id":               "loc-1",
		"code":                      "ACCRUAL-READONLY",
		"name":                      "Readonly Accrual",
		"journal_kind":              "accrual",
		"cadence":                   "monthly",
		"currency_code":             "IDR",
		"required_for_period_close": true,
		"journal_lines": []map[string]any{
			{"account_code": "6100-READONLY", "description": "Expense", "debit": 100.0, "credit": 0.0},
			{"account_code": "2105-READONLY", "description": "Accrual", "debit": 0.0, "credit": 100.0},
		},
		"status": "active",
	}); err != nil {
		t.Fatalf("create template: %v", err)
	}

	tasksBefore, _, err := models.List("accounting_period_task", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list tasks before: %v", err)
	}
	runsBefore, _, err := models.List("journal_run", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list runs before: %v", err)
	}
	pack, err := service.ReadClosePack(period.ID, "org-1", "loc-1")
	if err != nil {
		t.Fatalf("read close pack: %v", err)
	}
	if pack.Ready {
		t.Fatal("expected read-only pack to show pending blockers")
	}
	if len(pack.Tasks) == 0 {
		t.Fatal("expected synthetic close-pack tasks in read-only view")
	}
	tasksAfter, _, err := models.List("accounting_period_task", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list tasks after: %v", err)
	}
	runsAfter, _, err := models.List("journal_run", model.Query{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list runs after: %v", err)
	}
	if len(tasksAfter) != len(tasksBefore) {
		t.Fatalf("expected read-only close pack not to create tasks, got %d -> %d", len(tasksBefore), len(tasksAfter))
	}
	if len(runsAfter) != len(runsBefore) {
		t.Fatalf("expected read-only close pack not to create runs, got %d -> %d", len(runsBefore), len(runsAfter))
	}
}

func TestFinancePeriodEndReverseAccrualPostingRejectsUnpostedSource(t *testing.T) {
	docs := document.NewService()
	if err := docs.Register(document.Definition{Type: "ledger_posting", DisplayName: "Ledger Posting", SchemaVersion: "v1", AllowedLinkTypes: []string{"posting_for"}}); err != nil {
		t.Fatalf("register ledger_posting: %v", err)
	}
	models := model.NewService()
	registerFinancePeriodEndTestModels(t, models)
	finance := NewFinanceReportingCoreService(docs, models, nil)
	service := NewFinancePeriodEndCoreService(docs, models, finance)

	if _, err := models.Create("accounting_period", "tester", map[string]any{
		"organization_id": "org-1",
		"location_id":     "loc-1",
		"period_key":      "2099-07",
		"start_date":      "2099-07-01",
		"end_date":        "2099-07-31",
		"status":          "open",
	}); err != nil {
		t.Fatalf("create period: %v", err)
	}
	source, err := docs.Create("ledger_posting", "org-1", "loc-1", "tester", map[string]any{
		"source_document_type": "journal_template",
		"source_document_id":   "tmpl-draft",
		"posting_date":         "2099-07-31",
		"currency_code":        "IDR",
		"posting_rule_key":     "period_end_accrual",
		"journal_source_kind":  "accrual",
		"reversal_status":      "available",
		"total_amount":         250.0,
		"journal_lines": []map[string]any{
			{"account_code": "6100-DRAFT", "description": "Expense", "debit": 250.0, "credit": 0.0},
			{"account_code": "2105-DRAFT", "description": "Accrual", "debit": 0.0, "credit": 250.0},
		},
	})
	if err != nil {
		t.Fatalf("create draft source: %v", err)
	}
	if _, err := service.ReverseAccrualPosting(source.Header.ID, "2099-08-01", "tester", "org-1", "loc-1"); err == nil {
		t.Fatal("expected reversal of unposted accrual to fail")
	}
	refreshed, err := docs.Get(source.Header.ID)
	if err != nil {
		t.Fatalf("get draft source: %v", err)
	}
	if got := refreshed.Body.Payload["reversed_by_posting_id"]; textValue(got) != "" {
		t.Fatalf("expected draft source not to be marked reversed, got %#v", got)
	}
}

func registerFinancePeriodEndTestModels(t *testing.T, models *model.Service) {
	t.Helper()
	defs := []model.Definition{
		{
			Key:         "accounting_period",
			DisplayName: "Accounting Period",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "period_key", Type: "string", Required: true},
				{Key: "start_date", Type: "string", Required: true},
				{Key: "end_date", Type: "string", Required: true},
				{Key: "status", Type: "string"},
				{Key: "closed_at", Type: "string"},
				{Key: "closed_by", Type: "string"},
			},
		},
		{
			Key:         "journal_template",
			DisplayName: "Journal Template",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string", Required: true},
				{Key: "location_id", Type: "string"},
				{Key: "code", Type: "string", Required: true},
				{Key: "name", Type: "string", Required: true},
				{Key: "journal_kind", Type: "string"},
				{Key: "cadence", Type: "string"},
				{Key: "currency_code", Type: "string"},
				{Key: "description_template", Type: "string"},
				{Key: "required_for_period_close", Type: "bool"},
				{Key: "journal_lines", Type: "object"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "journal_run",
			DisplayName: "Journal Run",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "accounting_period_id", Type: "string"},
				{Key: "period_key", Type: "string"},
				{Key: "journal_template_id", Type: "string"},
				{Key: "template_code", Type: "string"},
				{Key: "template_name", Type: "string"},
				{Key: "cadence", Type: "string"},
				{Key: "journal_kind", Type: "string"},
				{Key: "posting_date", Type: "string"},
				{Key: "generated_posting_id", Type: "string"},
				{Key: "generated_posting_number", Type: "string"},
				{Key: "generated_posting_status", Type: "string"},
				{Key: "reversal_status", Type: "string"},
				{Key: "status", Type: "string"},
			},
		},
		{
			Key:         "accounting_period_task",
			DisplayName: "Accounting Period Task",
			Fields: []model.FieldDefinition{
				{Key: "organization_id", Type: "string"},
				{Key: "location_id", Type: "string"},
				{Key: "accounting_period_id", Type: "string"},
				{Key: "period_key", Type: "string"},
				{Key: "task_code", Type: "string"},
				{Key: "label", Type: "string"},
				{Key: "task_type", Type: "string"},
				{Key: "required", Type: "bool"},
				{Key: "journal_template_id", Type: "string"},
				{Key: "journal_run_id", Type: "string"},
				{Key: "posting_id", Type: "string"},
				{Key: "posting_number", Type: "string"},
				{Key: "status", Type: "string"},
				{Key: "completed_at", Type: "string"},
				{Key: "completed_by", Type: "string"},
				{Key: "note", Type: "string"},
			},
		},
	}
	for _, def := range defs {
		if err := models.Register(def); err != nil {
			t.Fatalf("register %s: %v", def.Key, err)
		}
	}
}
