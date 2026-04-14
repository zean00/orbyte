package app

import (
	"os"
	"testing"
	"time"

	"orbyte/internal/platform/store"
)

func TestFinanceCollectionsPostgresStatementsAndWriteoff(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is required for postgres-backed finance collections test")
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
	actorID := "user_admin"
	party := ensurePartyRecord(t, graph.models, actorID, "party-fincol-"+suffix, "Finance Collections Party "+suffix)

	invoice, err := graph.documents.Create("invoice", orgID, locID, actorID, map[string]any{
		"party_id":                party.ID,
		"party_name":              "Finance Collections Party " + suffix,
		"invoice_date":            "2099-07-01",
		"due_date":                "2099-07-10",
		"total_amount":            90.0,
		"paid_amount":             0.0,
		"credited_amount":         0.0,
		"refunded_amount":         0.0,
		"writeoff_amount":         0.0,
		"balance_due_amount":      90.0,
		"receivable_account_code": "1100-AR-COLL-" + suffix,
	})
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	invoice.Header.Status = "issued"
	invoice.Header.Number = "INV-COLL-" + suffix
	if err := graph.documents.Save(invoice); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	statementRun, err := graph.financeCollections.GenerateARStatementRun(orgID, locID, party.ID, "2099-07-31", actorID)
	if err != nil {
		t.Fatalf("generate ar statement run: %v", err)
	}
	if statementRun.ModelKey != "party_statement_run" {
		t.Fatalf("expected party_statement_run, got %s", statementRun.ModelKey)
	}

	report, err := graph.financeCollections.SyncSettlementExceptions(orgID, locID, "2099-07-31", "ar", actorID)
	if err != nil {
		t.Fatalf("sync settlement exceptions: %v", err)
	}
	if len(report.Items) == 0 {
		t.Fatal("expected synced settlement exception items")
	}

	var writeoffID string
	for _, item := range report.Items {
		if item.ExceptionType == "write_off_candidate" && item.SourceDocumentID == invoice.Header.ID {
			writeoffID = item.ID
			break
		}
	}
	if writeoffID == "" {
		t.Fatalf("expected write-off candidate for invoice %s, got %+v", invoice.Header.ID, report.Items)
	}

	posting, err := graph.financeCollections.WriteOffSettlementException(writeoffID, "2099-07-31", 90.0, actorID, orgID, locID)
	if err != nil {
		t.Fatalf("write off settlement exception: %v", err)
	}
	if posting.Header.Status != "posted" {
		t.Fatalf("expected posted write-off posting, got %s", posting.Header.Status)
	}

	reloadedInvoice, err := graph.documents.Get(invoice.Header.ID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if got := reloadedInvoice.Body.Payload["balance_due_amount"]; got != 0.0 {
		t.Fatalf("expected closed balance, got %#v", got)
	}
}
