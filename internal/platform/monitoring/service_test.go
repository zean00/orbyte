package monitoring

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/store"
	"orbyte/internal/platform/workflow"
)

func TestSummary(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	auditSvc := audit.NewService()
	actions := application.NewDocumentActions(docs, flows, nil, nil, application.NewMemorySubmitStore(docs, flows, auditSvc, events))
	events.RegisterHandler("document.submitted", eventing.NewDocumentProjectionHandler(docs, searchSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	_, _ = actions.Submit(record.Header.ID, application.ActingContext{ActorID: "user_admin", EffectiveUserID: "user_admin"}, 1, record.Header.ETag)
	_, _ = events.DispatchPending(10)

	svc := NewService(docs, events, flows, searchSvc, obs)
	summary := svc.Summary()
	if summary.Documents.Total == 0 {
		t.Fatal("expected document total")
	}
	if summary.Workflow.OpenTasks == 0 {
		t.Fatal("expected open task")
	}
	if summary.Projections.DocumentSummaries == 0 {
		t.Fatal("expected document projection")
	}
	if summary.Metrics.Counters["domain.events.recorded.total"] == 0 {
		t.Fatal("expected metrics snapshot")
	}

	monitor := store.NewQueryMonitor(obs, store.QueryMonitorOptions{
		SlowThreshold: time.Hour,
		TopOperations: 5,
		SlowQueries:   5,
	})
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = rawDB.Close() }()
	instrumented := store.InstrumentDB(rawDB, monitor, "monitoring", "repository")
	mock.ExpectExec(regexp.QuoteMeta("UPDATE documents SET status = $1 WHERE document_id = $2")).
		WithArgs("submitted", record.Header.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err := instrumented.ExecContext(context.Background(), "UPDATE documents SET status = $1 WHERE document_id = $2", "submitted", record.Header.ID); err != nil {
		t.Fatalf("instrumented exec: %v", err)
	}
	svc.AttachQueryMonitor(monitor)
	summary = svc.Summary()
	if len(summary.Database.TopOperations) != 1 {
		t.Fatal("expected database query snapshot")
	}
	if summary.Database.TopOperations[0].Subsystem != "monitoring" {
		t.Fatalf("unexpected database query operation: %+v", summary.Database.TopOperations[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
