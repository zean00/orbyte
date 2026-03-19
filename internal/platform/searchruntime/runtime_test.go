package searchruntime

import (
	"context"
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/search"
	"orbyte/internal/platform/securityfields"
)

func TestAttachWiresServicesAndHandlers(t *testing.T) {
	searchSvc := search.NewService()
	docs := document.NewService()
	models := model.NewService()
	jobSvc := jobs.NewService()
	fieldSecurity := securityfields.NewService(nil)
	eventingSvc := eventing.NewService()
	ctx, cancel := context.WithCancel(context.Background())
	jobSvc.Start(ctx)
	defer func() {
		cancel()
		jobSvc.Stop()
	}()

	Attach(searchSvc, docs, models, jobSvc, fieldSecurity, eventingSvc)

	if err := docs.Register(document.Definition{
		Type:          "searchruntime_request",
		DisplayName:   "Request",
		SchemaVersion: "v1",
	}); err != nil {
		t.Fatalf("register document definition failed: %v", err)
	}
	if err := searchSvc.RegisterIndex(search.IndexDefinition{
		Key:          "documents.requests.search",
		Title:        "Requests",
		SourceKind:   "document",
		DocumentType: "searchruntime_request",
		Modes:        []string{"keyword"},
		Fields:       []search.IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true}},
	}); err != nil {
		t.Fatalf("register search index failed: %v", err)
	}

	record, err := docs.Create("searchruntime_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "Projection"})
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	if err := eventingSvc.Record(eventing.Event{
		ID:            "event:doc:update:1",
		Type:          "document.updated",
		AggregateType: "document",
		AggregateID:   record.Header.ID,
		OccurredAt:    record.Header.UpdatedAt,
	}); err != nil {
		t.Fatalf("record event failed: %v", err)
	}
	dispatched, err := eventingSvc.DispatchPending(10)
	if err != nil {
		t.Fatalf("dispatch pending failed: %v", err)
	}
	if dispatched == 0 {
		t.Fatal("expected registered handlers to dispatch")
	}
	result, err := searchSvc.Query("documents.requests.search", "org_default", "loc_hq", search.QueryRequest{})
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		result, err = searchSvc.Query("documents.requests.search", "org_default", "loc_hq", search.QueryRequest{})
		if err == nil && result.Total == 1 {
			break
		}
		if time.Now().After(deadline) {
			if err != nil {
				t.Fatalf("search query failed: %v", err)
			}
			t.Fatalf("expected projected search document, got %+v", result)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAttachIgnoresNilSearchService(t *testing.T) {
	Attach(nil, document.NewService(), model.NewService(), jobs.NewService(), securityfields.NewService(nil), eventing.NewService())
}
