package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/application"
	"orbyte/internal/platform/store"
)

func TestFinanceManualJournalPostgresWorkflowAndReversal(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed finance manual journal test")
	}
	postgres, err := store.OpenFromEnv()
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer func() { _ = postgres.Close() }()

	graph := constructServiceGraph(postgres, nil)
	if err := seedPlatformKernel(graph.config, graph.identity, graph.modules, graph.models, graph.reporting, graph.templates, graph.reference, graph.search, graph.documents, graph.workflows, graph.policy, nil, testBootstrapAdminPassword); err != nil {
		t.Fatalf("seed platform kernel: %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405")
	orgID := "org_default"
	locID := "loc_hq"
	if _, err := graph.models.Create("accounting_period", "user_admin", map[string]any{
		"organization_id": orgID,
		"location_id":     locID,
		"period_key":      "2099-10-" + suffix,
		"start_date":      "2099-10-01",
		"end_date":        "2099-10-31",
		"status":          "open",
	}); err != nil {
		t.Fatalf("create source period: %v", err)
	}
	if _, err := graph.models.Create("accounting_period", "user_admin", map[string]any{
		"organization_id": orgID,
		"location_id":     locID,
		"period_key":      "2099-11-" + suffix,
		"start_date":      "2099-11-01",
		"end_date":        "2099-11-30",
		"status":          "open",
	}); err != nil {
		t.Fatalf("create reversal period: %v", err)
	}
	approverUser, err := graph.identity.CreateUser("finance-approver-"+suffix, testBootstrapAdminPassword, locID, "role_admin", "location", locID)
	if err != nil {
		t.Fatalf("create approver user: %v", err)
	}

	payload := graph.commercialCore.NormalizePayload("ledger_posting", map[string]any{
		"posting_date":        "2099-10-31",
		"currency_code":       "IDR",
		"journal_source_kind": "manual",
		"manual_journal_type": "adjusting",
		"supporting_reference": "MJ-" + suffix,
		"journal_lines": []map[string]any{
			{"account_code": "6100-MJ-" + suffix, "description": "Expense", "debit": 250.0, "credit": 0.0},
			{"account_code": "2100-MJ-" + suffix, "description": "Accrual", "debit": 0.0, "credit": 250.0},
		},
	})
	record, err := graph.documents.Create("ledger_posting", orgID, locID, "user_admin", payload)
	if err != nil {
		t.Fatalf("create manual journal: %v", err)
	}
	record, err = graph.docActions.Submit(record.Header.ID, application.ActingContext{ActorID: "user_admin"}, 0, "")
	if err != nil {
		t.Fatalf("submit manual journal: %v", err)
	}
	if err := graph.commercialCore.HandleAction(record, "submit", "user_admin", "month end adjustment"); err != nil {
		t.Fatalf("handle submit metadata: %v", err)
	}
	if err := graph.commercialCore.ValidateAction(record, "approve", "user_admin"); err == nil {
		t.Fatal("expected same-user manual journal approval to fail")
	}
	if err := graph.commercialCore.ValidateAction(record, "approve", approverUser.ID); err != nil {
		t.Fatalf("validate approval: %v", err)
	}
	record, err = graph.docActions.Approve(record.Header.ID, application.ActingContext{ActorID: approverUser.ID}, 0, "")
	if err != nil {
		t.Fatalf("approve manual journal: %v", err)
	}
	if err := graph.commercialCore.HandleAction(record, "approve", approverUser.ID, "approved"); err != nil {
		t.Fatalf("handle approve metadata: %v", err)
	}
	posted, err := graph.documents.Get(record.Header.ID)
	if err != nil {
		t.Fatalf("get posted journal: %v", err)
	}
	if posted.Header.Status != "posted" {
		t.Fatalf("expected posted status, got %s", posted.Header.Status)
	}
	if got := posted.Body.Payload["approved_by"]; got != approverUser.ID {
		t.Fatalf("expected approved_by %s, got %#v", approverUser.ID, got)
	}

	reversal, err := graph.financePeriodEnd.ReverseJournalPosting(posted.Header.ID, "2099-11-01", approverUser.ID, orgID, locID)
	if err != nil {
		t.Fatalf("reverse manual journal: %v", err)
	}
	if reversal.Header.Type != "ledger_posting" {
		t.Fatalf("expected ledger reversal, got %s", reversal.Header.Type)
	}
	if got := reversal.Body.Payload["reversal_of_posting_id"]; got != posted.Header.ID {
		t.Fatalf("expected reversal_of_posting_id %s, got %#v", posted.Header.ID, got)
	}
}
