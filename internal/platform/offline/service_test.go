package offline

import (
	"testing"
	"time"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/module"
	"orbyte/internal/platform/reference"
	"orbyte/internal/platform/search"
)

func TestBootstrapRememberAndCapabilityFilters(t *testing.T) {
	modules := module.NewService()
	if err := modules.Register(module.Manifest{
		Key:                "offline.test",
		Name:               "Offline Test",
		Version:            "1.0.0",
		DomainFamily:       "platform",
		OwnedDocumentTypes: []string{"generic_request"},
		ReferenceTypes: []reference.TypeDefinition{
			{Key: "request_status", DisplayName: "Request Status"},
		},
		Documents: []document.Definition{
			{Type: "generic_request", DisplayName: "Request", SchemaVersion: "v1"},
		},
		Models: []model.Definition{
			{Key: "party", DisplayName: "Party", Version: "v1", Fields: []model.FieldDefinition{{Key: "name", Type: "string"}}},
		},
		SearchIndexes: []search.IndexDefinition{
			{Key: "documents.requests.search", Title: "Requests", SourceKind: "document", DocumentType: "generic_request", Modes: []string{"keyword"}, Fields: []search.IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string"}}},
		},
		Offline: module.OfflineDefinition{
			Projections: []module.OfflineProjectionDefinition{
				{IndexKey: "documents.requests.search", Title: "Requests", RequiredPermissions: []string{"document.list"}, DefaultFilters: []string{"status=draft"}, DefaultIncludeFields: []string{"title"}},
			},
			Documents: []module.OfflineDocumentDefinition{
				{Type: "generic_request", Title: "Request", CreatePermissionKey: "document.create", UpdatePermissionKey: "document.update_draft"},
			},
			Models: []module.OfflineModelDefinition{
				{ModelKey: "party", Title: "Party", CreatePermissionKey: "party.create", UpdatePermissionKey: "party.update"},
			},
		},
	}, "test"); err != nil {
		t.Fatalf("register module failed: %v", err)
	}
	svc := NewService(modules, nil, nil)

	bootstrap := svc.Bootstrap()
	if len(bootstrap.References) != 0 || len(bootstrap.Projections) != 1 || len(bootstrap.Documents) != 1 || len(bootstrap.Models) != 1 {
		t.Fatalf("unexpected bootstrap payload: %+v", bootstrap)
	}

	result := SyncResultItem{IdempotencyKey: "idem-1", Status: "accepted", Kind: "document", Operation: "create", TargetID: "doc_1"}
	svc.RememberSyncResult("idem-1", result)
	got, ok := svc.SyncResult("idem-1")
	if !ok || got.TargetID != "doc_1" {
		t.Fatalf("expected remembered sync result, got ok=%v result=%+v", ok, got)
	}

	allowedDocs := FilterDocumentCapabilities(bootstrap.Documents, func(perms []string) bool { return len(perms) == 2 })
	if len(allowedDocs) != 1 {
		t.Fatalf("expected document capability, got %+v", allowedDocs)
	}
	allowedModels := FilterModelCapabilities(bootstrap.Models, func(perms []string) bool { return len(perms) == 2 })
	if len(allowedModels) != 1 {
		t.Fatalf("expected model capability, got %+v", allowedModels)
	}
}

func TestProjectionNormalizationAndSignatures(t *testing.T) {
	req := normalizeProjectionQuery(module.OfflineProjectionDefinition{
		DefaultFilters:       []string{"status=draft", "location_id=loc_hq"},
		DefaultIncludeFields: []string{"title"},
	}, search.QueryRequest{})
	if req.Filters["status"] != "draft" || req.Filters["location_id"] != "loc_hq" {
		t.Fatalf("expected default filters, got %+v", req.Filters)
	}
	if len(req.IncludeFields) != 1 || req.IncludeFields[0] != "title" || req.Page != 1 || req.PageSize != 50 {
		t.Fatalf("unexpected normalized query: %+v", req)
	}

	checksum1, version1 := packageSignature(map[string]any{"a": 1})
	checksum2, version2 := packageSignature(map[string]any{"a": 1})
	if checksum1 == "" || version1 == "" || checksum1 != checksum2 || version1 != version2 {
		t.Fatalf("expected deterministic package signature, got %q %q %q %q", checksum1, version1, checksum2, version2)
	}

	parsed := parseDefaultFilters([]string{"status=draft", "bad", " location_id = loc_hq "})
	if len(parsed) != 2 || parsed["location_id"] != "loc_hq" {
		t.Fatalf("unexpected parsed filters: %+v", parsed)
	}

	nilBootstrap := (*Service)(nil).Bootstrap()
	if nilBootstrap.GeneratedAt.IsZero() {
		t.Fatal("expected generated_at for nil bootstrap")
	}
}

func TestSyncResultEmptyKeyAndBootstrapTimestamp(t *testing.T) {
	svc := NewService(nil, nil, nil)
	svc.RememberSyncResult("", SyncResultItem{Status: "accepted"})
	if _, ok := svc.SyncResult(""); ok {
		t.Fatal("expected empty key lookup to fail")
	}
	boot := svc.Bootstrap()
	if time.Since(boot.GeneratedAt) > time.Minute {
		t.Fatalf("expected recent bootstrap timestamp, got %s", boot.GeneratedAt)
	}
}

func TestReferenceAndProjectionPackages(t *testing.T) {
	modules := module.NewService()
	if err := modules.Register(module.Manifest{
		Key:                "offline.package.test",
		Name:               "Offline Package Test",
		Version:            "1.0.0",
		DomainFamily:       "platform",
		OwnedDocumentTypes: []string{"generic_request"},
		ReferenceTypes: []reference.TypeDefinition{
			{Key: "request_status", DisplayName: "Request Status"},
		},
		Documents: []document.Definition{
			{Type: "generic_request", DisplayName: "Request", SchemaVersion: "v1"},
		},
		SearchIndexes: []search.IndexDefinition{
			{Key: "documents.requests.search", Title: "Requests", SourceKind: "document", DocumentType: "generic_request", Modes: []string{"keyword"}, QueryFilterFields: []string{"title"}, Fields: []search.IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true}}},
		},
		Offline: module.OfflineDefinition{
			References: []module.OfflineReferenceDefinition{
				{TypeKey: "request_status", Title: "Request Status", RequiredPermissions: []string{"reference.read"}},
			},
			Projections: []module.OfflineProjectionDefinition{
				{IndexKey: "documents.requests.search", Title: "Requests", DefaultFilters: []string{"title=Offline Ready"}, DefaultIncludeFields: []string{"title"}},
			},
		},
	}, "test"); err != nil {
		t.Fatalf("register module failed: %v", err)
	}

	referenceSvc := reference.NewService()
	if err := referenceSvc.RegisterType(reference.TypeDefinition{Key: "request_status", DisplayName: "Request Status"}); err != nil {
		t.Fatalf("register reference type failed: %v", err)
	}
	if err := referenceSvc.UpsertRecord(reference.Record{
		TypeKey:     "request_status",
		Key:         "draft",
		DisplayName: "Draft",
		Scope:       "deployment",
		Value:       map[string]any{"code": "draft"},
	}); err != nil {
		t.Fatalf("upsert reference record failed: %v", err)
	}

	searchSvc := search.NewService()
	if err := searchSvc.RegisterIndex(search.IndexDefinition{
		Key:               "documents.requests.search",
		Title:             "Requests",
		SourceKind:        "document",
		DocumentType:      "generic_request",
		Modes:             []string{"keyword"},
		QueryFilterFields: []string{"title"},
		Fields:            []search.IndexFieldDefinition{{Key: "title", Path: "body.payload.title", Type: "string", Searchable: true}},
	}); err != nil {
		t.Fatalf("register search index failed: %v", err)
	}
	searchSvc.RefreshDocument(document.Record{
		Header: document.Header{
			ID:             "doc-1",
			Type:           "generic_request",
			Status:         "draft",
			Version:        1,
			ETag:           "doc-1:1",
			OrganizationID: "org_default",
			LocationID:     "loc_hq",
			UpdatedAt:      time.Now().UTC(),
		},
		Body: document.Body{Payload: map[string]any{"title": "Offline Ready"}},
	})

	svc := NewService(modules, referenceSvc, searchSvc)
	refPkg, err := svc.ReferencePackage("request_status", "org_default", "loc_hq", time.Time{})
	if err != nil {
		t.Fatalf("reference package failed: %v", err)
	}
	if refPkg.PackageKey != "reference:request_status" || len(refPkg.ResolvedSet.Items) != 1 || refPkg.Checksum == "" || refPkg.Version == "" {
		t.Fatalf("unexpected reference package: %+v", refPkg)
	}

	projPkg, err := svc.ProjectionPackage("documents.requests.search", "org_default", "loc_hq", search.QueryRequest{})
	if err != nil {
		t.Fatalf("projection package failed: %v", err)
	}
	if projPkg.PackageKey != "projection:documents.requests.search" || projPkg.Result.Total != 1 {
		t.Fatalf("unexpected projection package: %+v", projPkg)
	}
	if projPkg.Query.Filters["title"] != "Offline Ready" {
		t.Fatalf("expected default title filter, got %+v", projPkg.Query.Filters)
	}
}

func TestReferenceAndProjectionCapabilityFilters(t *testing.T) {
	references := FilterReferenceCapabilities([]module.OfflineReferenceDefinition{
		{TypeKey: "b", RequiredPermissions: []string{"reference.read"}},
		{TypeKey: "a", RequiredPermissions: []string{"reference.manage"}},
	}, func(perms []string) bool { return len(perms) == 1 && perms[0] == "reference.read" })
	if len(references) != 1 || references[0].TypeKey != "b" {
		t.Fatalf("unexpected filtered references: %+v", references)
	}

	projections := FilterProjectionCapabilities([]module.OfflineProjectionDefinition{
		{IndexKey: "z", RequiredPermissions: []string{"search.manage"}},
		{IndexKey: "a", RequiredPermissions: []string{"search.query"}},
	}, func(perms []string) bool { return len(perms) == 1 && perms[0] == "search.query" })
	if len(projections) != 1 || projections[0].IndexKey != "a" {
		t.Fatalf("unexpected filtered projections: %+v", projections)
	}
}
