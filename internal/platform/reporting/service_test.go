package reporting

import (
	"testing"

	"clinic/internal/platform/document"
	"clinic/internal/platform/model"
	"clinic/internal/platform/search"
)

func TestExecuteModelDataset(t *testing.T) {
	models := model.NewService()
	_ = models.Register(model.Definition{Key: "party", DisplayName: "Party", Version: "v1", Fields: []model.FieldDefinition{{Key: "name", Type: "string"}, {Key: "status", Type: "string"}, {Key: "credit_limit", Type: "number"}}})
	_, _ = models.Create("party", "u1", map[string]any{"name": "Alice", "status": "active", "credit_limit": 100})
	_, _ = models.Create("party", "u1", map[string]any{"name": "Bob", "status": "inactive", "credit_limit": 50})
	_, _ = models.Create("party", "u1", map[string]any{"name": "Charlie", "status": "active", "credit_limit": 150})

	svc := NewService(models)
	if err := svc.Register(DatasetDefinition{
		Key:        "party.summary",
		Title:      "Party Summary",
		SourceKind: "model",
		ModelKey:   "party",
		Dimensions: []DimensionDefinition{{Key: "by_status", Label: "By Status", Path: "status"}},
		Measures: []MeasureDefinition{
			{Key: "total", Label: "Total", Kind: "count"},
			{Key: "credit_total", Label: "Credit Total", Kind: "sum", Path: "credit_limit"},
			{Key: "credit_avg", Label: "Credit Average", Kind: "avg", Path: "credit_limit"},
		},
	}); err != nil {
		t.Fatalf("register dataset failed: %v", err)
	}
	result, err := svc.Execute("party.summary")
	if err != nil {
		t.Fatalf("execute dataset failed: %v", err)
	}
	summary := result["summary"].(map[string]any)
	if summary["total"].(int) != 3 {
		t.Fatalf("expected total count, got %+v", result)
	}
	if summary["credit_total"].(float64) != 300 {
		t.Fatalf("expected sum aggregation, got %+v", result)
	}
	groups := result["groups"].([]map[string]any)
	if len(groups) != 2 {
		t.Fatalf("expected grouped rows, got %+v", result)
	}
}

func TestExecuteAdHocSourceSupportsDocumentsAndProjections(t *testing.T) {
	models := model.NewService()
	docs := document.NewService()
	searchSvc := search.NewService()
	recordA, err := docs.Create("generic_request", "org_default", "loc_hq", "u1", map[string]any{"title": "A"})
	if err != nil {
		t.Fatalf("create document A failed: %v", err)
	}
	recordA.Header.Status = "submitted"
	recordA.Header.UpdatedAt = recordA.Header.CreatedAt.Add(1)
	if err := docs.Save(recordA); err != nil {
		t.Fatalf("save document A failed: %v", err)
	}
	recordB, err := docs.Create("generic_request", "org_default", "loc_branch", "u1", map[string]any{"title": "B"})
	if err != nil {
		t.Fatalf("create document B failed: %v", err)
	}
	searchSvc.RefreshDocument(recordA)
	searchSvc.RefreshDocument(recordB)

	svc := NewService(models)
	svc.AttachDocumentSources(docs, searchSvc)

	docResult, err := svc.ExecuteAdHocSource("documents", model.Query{Page: 1, PageSize: 100}, QueryRequest{
		Dimensions: []string{"header.status"},
		Measures:   []string{"count"},
		GroupBy:    []string{"header.status"},
	})
	if err != nil {
		t.Fatalf("execute ad hoc documents failed: %v", err)
	}
	docGroups := docResult["groups"].([]map[string]any)
	if len(docGroups) != 2 {
		t.Fatalf("expected 2 document groups, got %+v", docResult)
	}

	projResult, err := svc.ExecuteAdHocSource("document_projections", model.Query{Page: 1, PageSize: 100}, QueryRequest{
		Dimensions: []string{"status"},
		Measures:   []string{"count"},
		GroupBy:    []string{"status"},
	})
	if err != nil {
		t.Fatalf("execute ad hoc projections failed: %v", err)
	}
	if projResult["total"].(int) != 2 {
		t.Fatalf("expected 2 projected rows, got %+v", projResult)
	}
}

func TestDefinitionsExecuteQueryAndAdHocModelErrors(t *testing.T) {
	models := model.NewService()
	_ = models.Register(model.Definition{Key: "party", DisplayName: "Party", Version: "v1", DefaultSort: "name", Fields: []model.FieldDefinition{{Key: "name", Type: "string"}, {Key: "credit_limit", Type: "number"}}})
	_, _ = models.Create("party", "u1", map[string]any{"name": "Bob", "credit_limit": 50})
	_, _ = models.Create("party", "u1", map[string]any{"name": "Alice", "credit_limit": 100})

	svc := NewService(models)
	if err := svc.Register(DatasetDefinition{
		Key:        "party.query",
		Title:      "Party Query",
		SourceKind: "model",
		ModelKey:   "party",
		Dimensions: []DimensionDefinition{{Key: "name", Label: "Name", Path: "name"}},
		Measures: []MeasureDefinition{
			{Key: "credit_total", Label: "Credit Total", Kind: "sum", Path: "credit_limit"},
			{Key: "credit_min", Label: "Credit Min", Kind: "min", Path: "credit_limit"},
			{Key: "credit_max", Label: "Credit Max", Kind: "max", Path: "credit_limit"},
		},
	}); err != nil {
		t.Fatalf("register dataset failed: %v", err)
	}
	if len(svc.Definitions()) != 1 {
		t.Fatalf("expected dataset definitions, got %+v", svc.Definitions())
	}

	result, err := svc.ExecuteQuery("party.query", model.Query{SortKey: "name", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("execute query failed: %v", err)
	}
	rows := result["rows"].([]map[string]any)
	if rows[0]["name"] != "Alice" {
		t.Fatalf("expected sorted query rows, got %+v", rows)
	}
	summary := result["summary"].(map[string]any)
	if summary["credit_min"].(float64) != 50 || summary["credit_max"].(float64) != 100 {
		t.Fatalf("expected min/max summary, got %+v", summary)
	}

	adhoc, err := svc.ExecuteAdHocModel(model.Query{Page: 1, PageSize: 10}, QueryRequest{
		ModelKey:   "party",
		Dimensions: []string{"name"},
		Measures:   []string{"sum:credit_limit"},
		GroupBy:    []string{"name"},
	})
	if err != nil {
		t.Fatalf("execute ad hoc model failed: %v", err)
	}
	if adhoc["dataset_key"].(string) == "" {
		t.Fatalf("expected adhoc dataset key, got %+v", adhoc)
	}

	if _, err := svc.ExecuteAdHocModel(model.Query{}, QueryRequest{}); err == nil {
		t.Fatal("expected missing model key error")
	}
	if _, err := svc.ExecuteAdHocSource("unknown", model.Query{}, QueryRequest{}); err == nil {
		t.Fatal("expected unknown source error")
	}
	if _, err := svc.ExecuteAdHocModel(model.Query{}, QueryRequest{
		ModelKey:   "party",
		Dimensions: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
	}); err == nil {
		t.Fatal("expected bounded dimensions validation")
	}
}

func TestExecuteViewSupportsDynamicSelectionAndSorting(t *testing.T) {
	models := model.NewService()
	_ = models.Register(model.Definition{Key: "party", DisplayName: "Party", Version: "v1", Fields: []model.FieldDefinition{{Key: "name", Type: "string"}, {Key: "status", Type: "string"}, {Key: "credit_limit", Type: "number"}}})
	_, _ = models.Create("party", "u1", map[string]any{"name": "Alice", "status": "active", "credit_limit": 100})
	_, _ = models.Create("party", "u1", map[string]any{"name": "Bob", "status": "active", "credit_limit": 150})
	_, _ = models.Create("party", "u1", map[string]any{"name": "Carol", "status": "inactive", "credit_limit": 50})

	svc := NewService(models)
	_ = svc.Register(DatasetDefinition{
		Key:        "party.query",
		Title:      "Party Query",
		SourceKind: "model",
		ModelKey:   "party",
		Dimensions: []DimensionDefinition{{Key: "status", Label: "Status", Path: "status"}},
		Measures: []MeasureDefinition{
			{Key: "total", Label: "Total", Kind: "count"},
			{Key: "credit_total", Label: "Credit Total", Kind: "sum", Path: "credit_limit"},
			{Key: "credit_max", Label: "Credit Max", Kind: "max", Path: "credit_limit"},
		},
	})
	result, err := svc.ExecuteView("party.query", model.Query{Page: 1, PageSize: 1000}, QueryRequest{
		GroupBy: []string{"status"},
		SortBy:  "credit_total",
		Desc:    true,
		Limit:   1,
	})
	if err != nil {
		t.Fatalf("execute view failed: %v", err)
	}
	groups := result["groups"].([]map[string]any)
	if len(groups) != 1 {
		t.Fatalf("expected limited grouped result, got %+v", result)
	}
	if groups[0]["status"] != "active" || groups[0]["credit_total"].(float64) != 250 {
		t.Fatalf("expected sorted active group first, got %+v", groups[0])
	}
}
