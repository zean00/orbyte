package monitoring

import (
	"testing"

	application "orbyte/internal/platform/application"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/observability"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/workflow"
)

func TestSummary(t *testing.T) {
	docs := document.NewService()
	flows := workflow.NewService()
	obs := observability.NewService()
	events := eventing.NewServiceWithRepository(eventing.NewMemoryRepository(), obs, nil)
	searchSvc := search.NewService()
	auditSvc := audit.NewService()
	actions := application.NewDocumentActions(docs, flows, nil, application.NewMemorySubmitStore(docs, flows, auditSvc, events))
	events.RegisterHandler("document.submitted", eventing.NewDocumentProjectionHandler(docs, searchSvc))

	record, _ := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "x"})
	_, _ = actions.Submit(record.Header.ID, "user_admin", 1, record.Header.ETag)
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
}
