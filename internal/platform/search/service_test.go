package search

import (
	"testing"
	"time"

	"clinic/internal/platform/document"
	"clinic/internal/platform/jobs"
	"clinic/internal/platform/model"
	"clinic/internal/platform/shared"
)

func TestRefreshDocumentProjection(t *testing.T) {
	svc := NewService()
	record := document.Record{
		Header: document.Header{
			ID:             "d1",
			Type:           "generic_request",
			Status:         "submitted",
			Version:        2,
			ETag:           "d1:2",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
			UpdatedAt:      time.Now().UTC(),
			TotalAmount:    shared.Money{Currency: "IDR"},
		},
	}
	svc.RefreshDocument(record)
	items := svc.ListDocuments()
	if len(items) != 1 {
		t.Fatalf("expected 1 projection, got %d", len(items))
	}
	if items[0].Status != "submitted" {
		t.Fatalf("expected submitted status, got %s", items[0].Status)
	}
}

func TestRefreshDocumentProjectionIgnoresStaleVersion(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()
	svc.RefreshDocument(document.Record{
		Header: document.Header{
			ID: "d1", Type: "generic_request", Status: "submitted", Version: 3, ETag: "d1:3", OrganizationID: "org_default", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"},
		},
	})
	svc.RefreshDocument(document.Record{
		Header: document.Header{
			ID: "d1", Type: "generic_request", Status: "draft", Version: 2, ETag: "d1:2", OrganizationID: "org_default", UpdatedAt: now.Add(-time.Minute), TotalAmount: shared.Money{Currency: "IDR"},
		},
	})
	items := svc.ListDocuments()
	if len(items) != 1 || items[0].Version != 3 || items[0].Status != "submitted" {
		t.Fatalf("expected projection to keep newest version, got %+v", items)
	}
}

func TestProjectionConsistencyReport(t *testing.T) {
	svc := NewService()
	now := time.Now().UTC()
	source := []document.Record{
		{Header: document.Header{ID: "d1", Type: "generic_request", Status: "draft", Version: 1, ETag: "d1:1", OrganizationID: "org_default", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}}},
		{Header: document.Header{ID: "d2", Type: "generic_request", Status: "submitted", Version: 2, ETag: "d2:2", OrganizationID: "org_default", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}}},
	}
	svc.RefreshDocument(source[0])
	report := svc.Consistency(source)
	if report.MissingCount != 1 || report.SourceCount != 2 || report.ProjectionCount != 1 {
		t.Fatalf("unexpected consistency report: %+v", report)
	}
}

func TestRegisterIndexAndQueryDocumentSearch(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.requests.search",
		Title:             "Requests",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword", "vector", "hybrid"},
		OrganizationSplit: true,
		QueryFilterFields: []string{"status", "location_id"},
		QuerySortFields:   []string{"status", "title"},
		Fields:            []IndexFieldDefinition{{Key: "status", Path: "header.status", Type: "string", Facet: true}, {Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true}},
		VectorFields:      []VectorFieldDefinition{{Key: "semantic", SourcePaths: []string{"body.payload.title"}, EmbeddingMode: "external", Dimensions: 8}},
	}); err != nil {
		t.Fatalf("register index failed: %v", err)
	}
	record := document.Record{
		Header: document.Header{
			ID:             "d1",
			Type:           "generic_request",
			Status:         "submitted",
			Version:        2,
			ETag:           "d1:2",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
			UpdatedAt:      time.Now().UTC(),
			TotalAmount:    shared.Money{Currency: "IDR"},
		},
		Body: document.Body{Payload: map[string]any{"title": "urgent supply request"}},
	}
	svc.RefreshDocument(record)
	result, err := svc.Query("documents.requests.search", "org_default", "loc_hq", QueryRequest{Mode: "keyword", Query: "supply"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result.Total != 1 || len(result.Hits) != 1 {
		t.Fatalf("expected one hit, got %+v", result)
	}
	vectorResult, err := svc.Query("documents.requests.search", "org_default", "loc_hq", QueryRequest{Mode: "vector", VectorText: "urgent request"})
	if err != nil {
		t.Fatalf("vector query failed: %v", err)
	}
	if vectorResult.Total != 1 {
		t.Fatalf("expected vector hit, got %+v", vectorResult)
	}
}

func TestRefreshModelIndexesModelSearch(t *testing.T) {
	models := model.NewService()
	if err := models.Register(model.Definition{
		Key:         "party",
		DisplayName: "Party",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string"},
			{Key: "status", Type: "string"},
			{Key: "organization_id", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	record, err := models.Create("party", "user_admin", map[string]any{"name": "PT Example", "status": "active", "organization_id": "org_default"})
	if err != nil {
		t.Fatalf("create model failed: %v", err)
	}
	svc := NewService()
	svc.AttachSources(nil, models)
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "masterdata.party.search",
		Title:             "Party Search",
		SourceKind:        "model",
		ModelKey:          "party",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QueryFilterFields: []string{"status"},
		QuerySortFields:   []string{"name"},
		Fields:            []IndexFieldDefinition{{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true}, {Key: "status", Path: "status", Type: "string", Facet: true}},
	}); err != nil {
		t.Fatalf("register model index failed: %v", err)
	}
	svc.RefreshModel(record)
	result, err := svc.Query("masterdata.party.search", "org_default", "", QueryRequest{Query: "example"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected one model hit, got %+v", result)
	}
}

func TestRegisterIndexRejectsInvalidFilterField(t *testing.T) {
	svc := NewService()
	err := svc.RegisterIndex(IndexDefinition{
		Key:               "bad",
		Title:             "Bad",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		QueryFilterFields: []string{"missing"},
		Fields:            []IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string"}},
	})
	if err == nil {
		t.Fatal("expected invalid filter registration to fail")
	}
}

func TestRebuildIndexProjectionAndRefreshModelByID(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.summary.search",
		Title:             "Summary",
		SourceKind:        "projection",
		ProjectionKey:     "document_summary",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QueryFilterFields: []string{"status"},
		QuerySortFields:   []string{"status"},
		Fields:            []IndexFieldDefinition{{Key: "status", Path: "status", Type: "string", Searchable: true}},
	}); err != nil {
		t.Fatalf("register projection index failed: %v", err)
	}
	now := time.Now().UTC()
	svc.RefreshDocument(document.Record{
		Header: document.Header{
			ID: "d1", Type: "generic_request", Status: "submitted", Version: 2, ETag: "d1:2", OrganizationID: "org_default", UpdatedAt: now,
		},
	})
	if _, err := svc.RebuildIndex("documents.summary.search"); err != nil {
		t.Fatalf("rebuild projection index failed: %v", err)
	}
	result, err := svc.Query("documents.summary.search", "org_default", "", QueryRequest{Query: "submitted"})
	if err != nil {
		t.Fatalf("query projection index failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected projection hit, got %+v", result)
	}

	models := model.NewService()
	if err := models.Register(model.Definition{
		Key: "party", DisplayName: "Party", Fields: []model.FieldDefinition{{Key: "name", Type: "string"}, {Key: "organization_id", Type: "string"}},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	record, err := models.Create("party", "user_admin", map[string]any{"name": "PT Refresh", "organization_id": "org_default"})
	if err != nil {
		t.Fatalf("create model failed: %v", err)
	}
	svc.AttachSources(nil, models)
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "party.search",
		Title:             "Party",
		SourceKind:        "model",
		ModelKey:          "party",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"name"},
		Fields:            []IndexFieldDefinition{{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true}},
	}); err != nil {
		t.Fatalf("register model index failed: %v", err)
	}
	if err := svc.RefreshModelByID("party", record.ID); err != nil {
		t.Fatalf("refresh model by id failed: %v", err)
	}
	modelResult, err := svc.Query("party.search", "org_default", "", QueryRequest{Query: "refresh"})
	if err != nil {
		t.Fatalf("query model index failed: %v", err)
	}
	if modelResult.Total != 1 {
		t.Fatalf("expected refreshed model hit, got %+v", modelResult)
	}
}

func TestServiceSettersAndAsyncEnqueue(t *testing.T) {
	svc := NewService()
	svc.SetBackend(NewMemoryBackend())
	svc.SetEmbedder(NewHashEmbedder())
	jobSvc := jobs.NewService()
	svc.AttachJobs(jobSvc)
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.async.search",
		Title:             "Async",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"title"},
		Fields:            []IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true}},
	}); err != nil {
		t.Fatalf("register index failed: %v", err)
	}
	svc.RefreshDocument(document.Record{
		Header: document.Header{
			ID: "d-async", Type: "generic_request", Status: "draft", Version: 1, ETag: "d-async:1", OrganizationID: "org_default", UpdatedAt: time.Now().UTC(),
		},
		Body: document.Body{Payload: map[string]any{"title": "asynchronous"}},
	})
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		result, err := svc.Query("documents.async.search", "org_default", "", QueryRequest{Query: "async"})
		if err == nil && result.Total == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected async indexing to complete, got result=%+v err=%v", result, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
