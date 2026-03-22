package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/jobs"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/policy"
	"orbyte/internal/platform/securityfields"
	"orbyte/internal/platform/shared"
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

func TestDocumentSearchHonorsSearchVisibleWithoutHidingAPIField(t *testing.T) {
	svc := NewService()
	policies := policy.NewService()
	if err := policies.Register(policy.HookDefinition{Key: "documents.fields.profile", Kind: "security", Target: "document_fields", AllowedScopes: []string{"deployment"}, DefaultRule: map[string]any{"fields": map[string]any{}}}); err != nil {
		t.Fatalf("register policy hook failed: %v", err)
	}
	if err := policies.SetEvaluator("documents.fields.profile", func(req policy.Request) policy.Decision {
		return policy.Decision{Allowed: true, Output: map[string]any{
			"fields": map[string]any{
				"patient_code": map[string]any{"visible": true, "search_visible": false},
			},
		}}
	}); err != nil {
		t.Fatalf("set evaluator failed: %v", err)
	}
	svc.AttachFieldSecurity(securityfields.NewService(policies))
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.requests.secure",
		Title:             "Requests Secure",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"title"},
		Fields: []IndexFieldDefinition{
			{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true},
			{Key: "patient_code", Path: "body.payload.patient_code", Type: "string", Searchable: true},
		},
	}); err != nil {
		t.Fatalf("register index failed: %v", err)
	}
	record := document.Record{
		Header: document.Header{
			ID:             "d-secure",
			Type:           "generic_request",
			Status:         "submitted",
			Version:        1,
			ETag:           "d-secure:1",
			OrganizationID: "org_default",
			UpdatedAt:      time.Now().UTC(),
			TotalAmount:    shared.Money{Currency: "IDR"},
		},
		Body: document.Body{Payload: map[string]any{
			"title":        "visible title",
			"patient_code": "PC-777",
		}},
	}
	svc.RefreshDocument(record)
	result, err := svc.Query("documents.requests.secure", "org_default", "", QueryRequest{Query: "PC-777"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result.Total != 0 {
		t.Fatalf("expected search_visible=false field to stay out of search index, got %+v", result)
	}
	titleResult, err := svc.Query("documents.requests.secure", "org_default", "", QueryRequest{Query: "visible"})
	if err != nil {
		t.Fatalf("title query failed: %v", err)
	}
	if titleResult.Total != 1 {
		t.Fatalf("expected title field to remain searchable, got %+v", titleResult)
	}
	if _, ok := titleResult.Hits[0].Fields["patient_code"]; ok {
		t.Fatalf("expected search hit to omit patient_code, got %+v", titleResult.Hits[0].Fields)
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

func TestRefreshModelSearchAppliesFieldSecurity(t *testing.T) {
	models := model.NewService()
	if err := models.Register(model.Definition{
		Key:         "party",
		DisplayName: "Party",
		Fields: []model.FieldDefinition{
			{Key: "name", Type: "string"},
			{Key: "email", Type: "string", Sensitive: true, DefaultMask: "partial_email"},
			{Key: "organization_id", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	record, err := models.Create("party", "user_admin", map[string]any{
		"name":            "PT Secure",
		"email":           "secure@example.com",
		"organization_id": "org_default",
	})
	if err != nil {
		t.Fatalf("create model failed: %v", err)
	}
	policies := policy.NewService()
	if err := policies.Register(policy.HookDefinition{Key: "models.fields.profile", Kind: "security", Target: "model_fields", AllowedScopes: []string{"deployment"}, DefaultRule: map[string]any{"fields": map[string]any{}}}); err != nil {
		t.Fatalf("register policy hook failed: %v", err)
	}
	fieldSecurity := securityfields.NewService(policies)

	svc := NewService()
	svc.AttachSources(nil, models)
	svc.AttachFieldSecurity(fieldSecurity)
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "masterdata.party.secure",
		Title:             "Party Secure Search",
		SourceKind:        "model",
		ModelKey:          "party",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"name"},
		Fields: []IndexFieldDefinition{
			{Key: "name", Path: "name", Type: "string", Searchable: true, Sort: true},
			{Key: "email", Path: "email", Type: "string", Searchable: true},
		},
	}); err != nil {
		t.Fatalf("register model index failed: %v", err)
	}

	svc.RefreshModel(record)
	result, err := svc.Query("masterdata.party.secure", "org_default", "", QueryRequest{Query: "secure"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected one secured model hit, got %+v", result)
	}
	if _, ok := result.Hits[0].Fields["email"]; ok {
		t.Fatalf("expected sensitive email to be excluded from search hit, got %+v", result.Hits[0].Fields)
	}
}

func TestIndexRuntimeConsistencyRepairAndSchemaLifecycle(t *testing.T) {
	docs := document.NewService()
	recordA, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "First"})
	if err != nil {
		t.Fatalf("create document A failed: %v", err)
	}
	recordB, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "Second"})
	if err != nil {
		t.Fatalf("create document B failed: %v", err)
	}

	svc := NewService()
	svc.AttachSources(docs, nil)
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.requests.runtime",
		Title:             "Requests Runtime",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword", "vector", "hybrid"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"title"},
		Fields:            []IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true}},
		VectorFields:      []VectorFieldDefinition{{Key: "semantic", SourcePaths: []string{"body.payload.title"}, EmbeddingMode: "external", Dimensions: 8}},
	}); err != nil {
		t.Fatalf("register index failed: %v", err)
	}

	svc.RefreshDocument(recordA)
	report, err := svc.ConsistencyReport("documents.requests.runtime")
	if err != nil {
		t.Fatalf("consistency report failed: %v", err)
	}
	if report.MissingCount != 1 || report.Status != "missing_records" {
		t.Fatalf("expected missing record report, got %+v", report)
	}
	runtime, ok := svc.IndexRuntime("documents.requests.runtime")
	if !ok {
		t.Fatal("expected index runtime")
	}
	if runtime.RuntimeStatus != "degraded" || !runtime.BackendCapabilities.Hybrid {
		t.Fatalf("expected degraded hybrid-capable runtime, got %+v", runtime)
	}

	repairResult, err := svc.RepairIndex("documents.requests.runtime", "repair_missing", "")
	if err != nil {
		t.Fatalf("repair failed: %v", err)
	}
	if repairResult["repaired"].(int) != 1 {
		t.Fatalf("expected one repaired record, got %+v", repairResult)
	}
	report, err = svc.ConsistencyReport("documents.requests.runtime")
	if err != nil {
		t.Fatalf("consistency report after repair failed: %v", err)
	}
	if report.MissingCount != 0 || report.Status != "ok" {
		t.Fatalf("expected healthy report after repair, got %+v", report)
	}

	runtime, err = svc.PlanIndexSchemaVersion("documents.requests.runtime", "v2")
	if err != nil {
		t.Fatalf("plan schema version failed: %v", err)
	}
	if runtime.CandidateSchemaVersion != "v2" || runtime.LifecycleState != "cutover_pending" {
		t.Fatalf("expected candidate version planned, got %+v", runtime)
	}
	runtime, err = svc.BuildCandidateIndex("documents.requests.runtime")
	if err != nil {
		t.Fatalf("build candidate failed: %v", err)
	}
	if runtime.LifecycleState != "validating" || runtime.RuntimeStatus != "candidate_built" {
		t.Fatalf("expected built candidate runtime, got %+v", runtime)
	}
	runtime, err = svc.ActivateCandidateIndex("documents.requests.runtime")
	if err != nil {
		t.Fatalf("activate candidate failed: %v", err)
	}
	if runtime.ActiveSchemaVersion != "v2" || runtime.CandidateSchemaVersion != "" || runtime.LifecycleState != "active" {
		t.Fatalf("expected activated candidate version, got %+v", runtime)
	}

	if recordB.Header.ID == "" {
		t.Fatal("expected second document id")
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
	ctx, cancel := context.WithCancel(context.Background())
	jobSvc.Start(ctx)
	defer func() {
		cancel()
		jobSvc.Stop()
	}()
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

func TestRefreshDocumentFallsBackWhenJobEnqueueFails(t *testing.T) {
	svc := NewService()
	svc.AttachSources(document.NewService(), nil)
	svc.SetBackend(NewMemoryBackend())
	svc.AttachJobs(jobs.NewServiceWithRepository(failingJobRepository{}))
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.sync-fallback.search",
		Title:             "SyncFallback",
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
		Header: document.Header{ID: "d-fallback", Type: "generic_request", Status: "draft", Version: 1, ETag: "d-fallback:1", OrganizationID: "org_default", UpdatedAt: time.Now().UTC()},
		Body:   document.Body{Payload: map[string]any{"title": "fallback path"}},
	})
	result, err := svc.Query("documents.sync-fallback.search", "org_default", "", QueryRequest{Query: "fallback"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected fallback indexing to complete, got %+v", result)
	}
}

func TestIndexRuntimesProjectionStatusesAndRebuildWrappers(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "z.documents.search",
		Title:             "Documents",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"title"},
		Fields:            []IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true}},
	}); err != nil {
		t.Fatalf("register document index failed: %v", err)
	}
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "a.summary.search",
		Title:             "Summary",
		SourceKind:        "projection",
		ProjectionKey:     "document_summary",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"status"},
		Fields:            []IndexFieldDefinition{{Key: "status", Path: "status", Type: "string", Searchable: true, Sort: true}},
	}); err != nil {
		t.Fatalf("register projection index failed: %v", err)
	}

	recordA := document.Record{
		Header: document.Header{ID: "d1", Type: "generic_request", Status: "submitted", Version: 1, ETag: "d1:1", OrganizationID: "org_default", UpdatedAt: time.Now().UTC()},
		Body:   document.Body{Payload: map[string]any{"title": "first"}},
	}
	recordB := document.Record{
		Header: document.Header{ID: "d2", Type: "generic_request", Status: "approved", Version: 1, ETag: "d2:1", OrganizationID: "org_default", UpdatedAt: time.Now().UTC()},
		Body:   document.Body{Payload: map[string]any{"title": "second"}},
	}

	svc.RebuildDocument(recordA)
	svc.RebuildAll([]document.Record{recordB})

	runtimes := svc.IndexRuntimes()
	if len(runtimes) != 2 || runtimes[0].IndexKey != "a.summary.search" || runtimes[1].IndexKey != "z.documents.search" {
		t.Fatalf("expected sorted runtimes, got %+v", runtimes)
	}

	statuses := svc.ProjectionStatuses([]document.Record{recordA, recordB})
	if len(statuses) == 0 || statuses[0].ProjectionKey != "document_summary" {
		t.Fatalf("expected projection status for document summary, got %+v", statuses)
	}
	if statuses[0].ProjectionCount != 2 || statuses[0].MissingCount != 0 {
		t.Fatalf("unexpected projection status values: %+v", statuses[0])
	}
}

func TestRebuildFailurePathsAndCandidateBuildFailure(t *testing.T) {
	svc := NewService()
	svc.SetBackend(failingSearchBackend{upsertErr: errors.New("upsert failed")})
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.fail.search",
		Title:             "Documents Fail",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"title"},
		Fields:            []IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true}},
	}); err != nil {
		t.Fatalf("register document index failed: %v", err)
	}
	svc.AttachSources(document.NewService(), nil)
	record, err := svc.documents.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "boom"})
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	if _, err := svc.RebuildIndex("documents.fail.search"); err == nil {
		t.Fatal("expected rebuild to fail")
	}
	runtime, ok := svc.IndexRuntime("documents.fail.search")
	if !ok || runtime.RuntimeStatus != "failed" || runtime.LastError == "" {
		t.Fatalf("expected failed runtime after rebuild, got %+v", runtime)
	}
	statuses := svc.repo.ListProjectionStatuses()
	if len(statuses) == 0 || statuses[0].LastRefreshStatus != "failed" || statuses[0].LastError == "" {
		t.Fatalf("expected failed projection status, got %+v", statuses)
	}

	if _, err := svc.PlanIndexSchemaVersion("documents.fail.search", "v2"); err != nil {
		t.Fatalf("plan schema version failed: %v", err)
	}
	if _, err := svc.BuildCandidateIndex("documents.fail.search"); err == nil {
		t.Fatal("expected candidate build to fail")
	}
	runtime, _ = svc.IndexRuntime("documents.fail.search")
	if runtime.RuntimeStatus != "failed" || runtime.LastFailureAt.IsZero() {
		t.Fatalf("expected failed candidate runtime, got %+v", runtime)
	}

	if record.Header.ID == "" {
		t.Fatal("expected created document id")
	}
}

func TestQueryVectorAndModelRefreshValidationFailures(t *testing.T) {
	svc := NewService()
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.keyword.only",
		Title:             "Keyword Only",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword"},
		OrganizationSplit: true,
		QuerySortFields:   []string{"title"},
		Fields:            []IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true, Sort: true}},
	}); err != nil {
		t.Fatalf("register keyword-only index failed: %v", err)
	}
	if _, err := svc.Query("documents.keyword.only", "org_default", "", QueryRequest{Mode: "vector", VectorText: "urgent"}); err == nil {
		t.Fatal("expected vector query without vector field to fail")
	}

	svc = NewService()
	svc.SetEmbedder(failingEmbedder{})
	if err := svc.RegisterIndex(IndexDefinition{
		Key:               "documents.vector.fail",
		Title:             "Vector Fail",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"vector"},
		OrganizationSplit: true,
		VectorFields:      []VectorFieldDefinition{{Key: "semantic", SourcePaths: []string{"body.payload.title"}, EmbeddingMode: "external", Dimensions: 4}},
	}); err != nil {
		t.Fatalf("register vector index failed: %v", err)
	}
	if _, err := svc.Query("documents.vector.fail", "org_default", "", QueryRequest{Mode: "vector", VectorText: "urgent"}); err == nil {
		t.Fatal("expected embedder failure to surface")
	}
	if err := svc.RefreshModelByID("party", "missing"); err == nil {
		t.Fatal("expected model refresh without model source to fail")
	}
}

type failingJobRepository struct{}

type failingSearchBackend struct {
	upsertErr error
	searchErr error
}

func (f failingSearchBackend) EnsureIndex(IndexDefinition, string) error { return nil }

func (f failingSearchBackend) Upsert(IndexDefinition, string, IndexedRecord) error {
	return f.upsertErr
}

func (f failingSearchBackend) Delete(IndexDefinition, string, string) error { return nil }

func (f failingSearchBackend) Search(IndexDefinition, string, QueryRequest) (QueryResult, error) {
	return QueryResult{}, f.searchErr
}

func (f failingSearchBackend) List(IndexDefinition, string) ([]IndexedRecord, error) { return nil, nil }

func (f failingSearchBackend) Capabilities(IndexDefinition) BackendCapabilities {
	return BackendCapabilities{Keyword: true, Vector: true, Hybrid: true, BackendKind: "failing"}
}

type failingEmbedder struct{}

func (failingEmbedder) Embed([]string, int) ([][]float32, error) {
	return nil, errors.New("embed failed")
}

func (failingJobRepository) Enqueue(job jobs.Job) (jobs.Job, bool, error) {
	return jobs.Job{}, false, errors.New("enqueue failed")
}

func (failingJobRepository) Get(string) (jobs.Job, bool) { return jobs.Job{}, false }

func (failingJobRepository) List() []jobs.Job { return nil }

func (failingJobRepository) ClaimPending(time.Time, time.Duration, int) []jobs.Job { return nil }

func (failingJobRepository) RenewLease(string, time.Time, time.Duration) error { return nil }

func (failingJobRepository) MarkSucceeded(string, map[string]any, time.Time) error { return nil }

func (failingJobRepository) MarkFailed(string, string, string, time.Time) error { return nil }

func (failingJobRepository) Requeue(string, time.Time) error { return nil }
