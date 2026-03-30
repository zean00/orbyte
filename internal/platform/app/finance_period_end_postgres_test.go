package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/store"
)

func TestFinancePeriodEndPostgresGenerationAndClose(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed finance period-end test")
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
	period, err := graph.models.Create("accounting_period", actorID, map[string]any{
		"organization_id": orgID,
		"location_id":     locID,
		"period_key":      "2099-06-" + suffix,
		"start_date":      "2099-06-01",
		"end_date":        "2099-06-30",
		"status":          "open",
	})
	if err != nil {
		t.Fatalf("create accounting period: %v", err)
	}
	template, err := graph.models.Create("journal_template", actorID, map[string]any{
		"organization_id":           orgID,
		"location_id":               locID,
		"code":                      "ACCRUAL-POSTGRES-" + suffix,
		"name":                      "Postgres Accrual",
		"journal_kind":              "accrual",
		"cadence":                   "monthly",
		"currency_code":             "IDR",
		"required_for_period_close": true,
		"journal_lines": []map[string]any{
			{"account_code": "6100-EXP-" + suffix, "description": "Expense", "debit": 500.0, "credit": 0.0},
			{"account_code": "2105-ACCRUAL-" + suffix, "description": "Accrual", "debit": 0.0, "credit": 500.0},
		},
		"status": "active",
	})
	if err != nil {
		t.Fatalf("create journal template: %v", err)
	}
	pack, err := graph.financePeriodEnd.GenerateJournalRuns(period.ID, actorID, orgID, locID)
	if err != nil {
		t.Fatalf("generate journal runs: %v", err)
	}
	foundTemplateRun := false
	targetPostingID := ""
	for _, run := range pack.JournalRuns {
		if run.TemplateID == template.ID {
			foundTemplateRun = true
			targetPostingID = run.PostingID
		}
		posting, err := graph.documents.Get(run.PostingID)
		if err != nil {
			t.Fatalf("get generated posting: %v", err)
		}
		posting.Header.Status = "posted"
		if err := graph.documents.Save(posting); err != nil {
			t.Fatalf("save posting: %v", err)
		}
		if err := graph.financePeriodEnd.HandleApprovedLedgerPosting(posting, actorID); err != nil {
			t.Fatalf("handle approved posting: %v", err)
		}
	}
	if !foundTemplateRun {
		t.Fatalf("expected generated run for template %s, got %#v", template.ID, pack.JournalRuns)
	}
	refreshed, err := graph.financePeriodEnd.ClosePack(period.ID, actorID, orgID, locID)
	if err != nil {
		t.Fatalf("load close pack: %v", err)
	}
	for _, task := range refreshed.Tasks {
		if task.TaskType == "checklist" {
			if _, err := graph.financePeriodEnd.CompleteTask(task.ID, actorID, orgID, locID); err != nil {
				t.Fatalf("complete task %s: %v", task.TaskCode, err)
			}
		}
	}
	closed, err := graph.financePeriodEnd.CloseAccountingPeriod(period.ID, actorID, orgID, locID)
	if err != nil {
		t.Fatalf("close accounting period: %v", err)
	}
	if closed.Values["status"] != "closed" {
		t.Fatalf("expected closed period, got %#v", closed.Values["status"])
	}
	reversal, err := graph.financePeriodEnd.ReverseAccrualPosting(targetPostingID, "2099-07-01", actorID, orgID, locID)
	if err != nil {
		t.Fatalf("reverse accrual posting: %v", err)
	}
	if reversal.Header.Type != "ledger_posting" {
		t.Fatalf("expected ledger_posting reversal, got %s", reversal.Header.Type)
	}
}
