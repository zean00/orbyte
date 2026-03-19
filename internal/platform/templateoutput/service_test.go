package templateoutput

import (
	"strings"
	"testing"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	platformmodule "orbyte/internal/platform/module"
	"orbyte/internal/platform/reporting"
)

func TestServiceRendersDocumentTemplateAndPublishesDraft(t *testing.T) {
	docs := document.NewService()
	record, err := docs.Create("generic_request", "org_default", "loc_hq", "user_admin", map[string]any{"title": "Need approval"})
	if err != nil {
		t.Fatalf("create document failed: %v", err)
	}
	reportingSvc := reporting.NewService(model.NewService())
	svc := NewService(docs, reportingSvc)
	if err := svc.RegisterDefinition(Definition{
		Key:          "documents.generic_request.default",
		Title:        "Generic Request Print",
		TargetKind:   "document",
		TargetKey:    "generic_request",
		RendererKind: "html",
		DefaultBody:  `<h1>{{ .document.Header.Number }}</h1><p>{{ index .document.Body.Payload "title" }}</p>`,
		DefaultStyle: `body{font-family:Arial}`,
	}); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}

	version, err := svc.SaveDraft("documents.generic_request.default", `<h1>{{ .document.Header.Type }}</h1><p>{{ index .document.Body.Payload "title" }}</p>`, "", "user_admin")
	if err != nil {
		t.Fatalf("save draft failed: %v", err)
	}
	if version.Status != "draft" {
		t.Fatalf("expected draft status, got %+v", version)
	}
	if _, err := svc.Publish("documents.generic_request.default", version.Version, "user_admin"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	output, err := svc.Render(RenderRequest{
		TemplateKey: "documents.generic_request.default",
		TargetKind:  "document",
		TargetID:    record.Header.ID,
		Format:      "html",
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(output.HTML, "generic_request") || !strings.Contains(output.HTML, "Need approval") {
		t.Fatalf("expected rendered html to include document data, got %s", output.HTML)
	}
}

func TestServiceRendersVisualReportTemplate(t *testing.T) {
	models := model.NewService()
	if err := models.Register(model.Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields: []model.FieldDefinition{
			{Key: "name", Label: "Name", Type: "string"},
			{Key: "category", Label: "Category", Type: "string"},
		},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	if _, err := models.Create("party", "user_admin", map[string]any{"name": "Alpha", "category": "customer"}); err != nil {
		t.Fatalf("create record failed: %v", err)
	}
	reportingSvc := reporting.NewService(models)
	if err := reportingSvc.Register(reporting.DatasetDefinition{
		Key:        "party_reporting",
		Title:      "Party Reporting",
		SourceKind: "model",
		ModelKey:   "party",
		Dimensions: []reporting.DimensionDefinition{{Key: "category", Label: "Category", Path: "category"}},
		Measures:   []reporting.MeasureDefinition{{Key: "total", Label: "Total", Kind: "count"}},
	}); err != nil {
		t.Fatalf("register dataset failed: %v", err)
	}
	svc := NewService(document.NewService(), reportingSvc)
	if err := svc.RegisterDefinition(Definition{
		Key:          "reporting.party.default",
		Title:        "Party Print",
		TargetKind:   "report",
		TargetKey:    "party_reporting",
		RendererKind: "visual",
		DefaultBody:  `{"schema_version":"visual-grid/v1","title":"Party Reporting","settings":{"paper_preset":"a4","orientation":"portrait","density":"comfortable"},"sections":[{"id":"header","title":"Summary","rows":[{"columns":[{"span":6,"blocks":[{"type":"text","text":"Party Reporting","font_size":"xl","emphasis":"strong"}]},{"span":6,"blocks":[{"type":"field","label":"Total Rows","path":"report.total","align":"right"}]}]}]},{"id":"body","title":"Rows","rows":[{"columns":[{"span":12,"blocks":[{"type":"table","rows_path":"report.rows","columns":[{"label":"Dimension","path":"dimension_key"},{"label":"Total","path":"total"}]}]}]}]}]}`,
	}); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}
	output, err := svc.Render(RenderRequest{TemplateKey: "reporting.party.default", TargetKind: "report", TargetKey: "party_reporting", Format: "html"})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(output.HTML, "Party Reporting") || !strings.Contains(output.HTML, "template-table") {
		t.Fatalf("expected visual report html, got %s", output.HTML)
	}
}

func TestServiceRendersVisualSampleDocumentWithoutTargetID(t *testing.T) {
	svc := NewService(document.NewService(), reporting.NewService(model.NewService()))
	if err := svc.RegisterDefinition(Definition{
		Key:          "documents.receipt.visual",
		Title:        "Receipt",
		TargetKind:   "document",
		TargetKey:    "receipt",
		RendererKind: "visual",
		DefaultBody:  `{"schema_version":"visual-grid/v1","title":"Receipt","settings":{"paper_preset":"receipt-80","density":"compact"},"sections":[{"id":"header","title":"Header","rows":[{"columns":[{"span":12,"blocks":[{"type":"text","text":"Store Receipt","font_size":"xl","emphasis":"strong","align":"center"},{"type":"field","label":"Number","path":"document.header.number","align":"center"}]}]}]},{"id":"body","title":"Items","rows":[{"columns":[{"span":12,"blocks":[{"type":"table","rows_path":"document.lines","columns":[{"label":"Name","path":"payload.name"},{"label":"Amount","path":"amount"}]},{"type":"totals","label":"Grand Total","rows_path":"document.lines","path":"amount"}]}]}]}]}`,
	}); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}
	output, err := svc.Render(RenderRequest{TemplateKey: "documents.receipt.visual", TargetKind: "document", Format: "html", Sample: true})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(output.HTML, "Store Receipt") || !strings.Contains(output.HTML, "SAMPLE-0001") || !strings.Contains(output.HTML, "Grand Total") {
		t.Fatalf("expected sample receipt html, got %s", output.HTML)
	}
}

func TestServiceRendersPDFOutput(t *testing.T) {
	svc := NewService(document.NewService(), reporting.NewService(model.NewService()))
	if err := svc.RegisterDefinition(Definition{
		Key:          "documents.receipt.pdf",
		Title:        "Receipt PDF",
		TargetKind:   "document",
		TargetKey:    "receipt",
		RendererKind: "visual",
		DefaultBody:  `{"schema_version":"visual-grid/v1","title":"Receipt","settings":{"paper_preset":"receipt-80","density":"compact"},"sections":[{"id":"body","title":"Body","rows":[{"columns":[{"span":12,"blocks":[{"type":"text","text":"Receipt PDF","font_size":"xl"},{"type":"field","label":"Number","path":"document.header.number"}]}]}]}]}`,
	}); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}
	output, err := svc.Render(RenderRequest{TemplateKey: "documents.receipt.pdf", TargetKind: "document", Format: "pdf", Sample: true})
	if err != nil {
		t.Fatalf("render pdf failed: %v", err)
	}
	if output.ContentType != "application/pdf" {
		t.Fatalf("expected pdf content type, got %s", output.ContentType)
	}
	if len(output.Bytes) < 16 || !strings.HasPrefix(string(output.Bytes[:4]), "%PDF") {
		t.Fatalf("expected pdf bytes, got %q", string(output.Bytes))
	}
}

func TestServiceSaveBindingReusesSignatureAndScopeResolutionPrefersLocation(t *testing.T) {
	svc := NewService(document.NewService(), reporting.NewService(model.NewService()))
	for _, def := range []Definition{
		{Key: "documents.generic_request.default", Title: "Default", TargetKind: "document", TargetKey: "generic_request", RendererKind: "html", DefaultBody: `<p>default</p>`},
		{Key: "documents.generic_request.location", Title: "Location", TargetKind: "document", TargetKey: "generic_request", RendererKind: "html", DefaultBody: `<p>location</p>`},
	} {
		if err := svc.RegisterDefinition(def); err != nil {
			t.Fatalf("register definition failed: %v", err)
		}
	}

	first, err := svc.SaveBinding(Binding{
		TemplateKey: "documents.generic_request.default",
		ScopeType:   "location",
		ScopeID:     "loc_hq",
		TargetKind:  "document",
		TargetKey:   "generic_request",
		Purpose:     "official",
		Channel:     "print",
		UpdatedBy:   "user_admin",
	})
	if err != nil {
		t.Fatalf("save binding failed: %v", err)
	}
	second, err := svc.SaveBinding(Binding{
		TemplateKey: "documents.generic_request.location",
		ScopeType:   "location",
		ScopeID:     "loc_hq",
		TargetKind:  "document",
		TargetKey:   "generic_request",
		Purpose:     "official",
		Channel:     "print",
		UpdatedBy:   "user_admin",
	})
	if err != nil {
		t.Fatalf("replace binding failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same binding id to be reused, got %q and %q", first.ID, second.ID)
	}
	if len(svc.Bindings()) != 1 {
		t.Fatalf("expected single effective binding, got %d", len(svc.Bindings()))
	}

	resolved, _, err := svc.Resolve(RenderRequest{
		TargetKind:     "document",
		TargetKey:      "generic_request",
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
		Purpose:        "official",
		Channel:        "print",
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.Key != "documents.generic_request.location" {
		t.Fatalf("expected location binding to win, got %q", resolved.Key)
	}
}

func TestServiceHelpersAndModuleConversion(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewServiceWithRepository(repo, document.NewService(), reporting.NewService(model.NewService()))
	def := FromModule(platformmodule.TemplateDefinition{
		Key:                 "documents.generic_request.official",
		Title:               "Official Request",
		TargetKind:          "document",
		TargetKey:           "generic_request",
		RendererKind:        "html",
		DefaultFormat:       "html",
		Formats:             []string{"html", "pdf"},
		Purpose:             "official",
		Channel:             "print",
		AllowedScopes:       []string{"deployment", "location"},
		RequiredPermissions: []string{"template.read"},
		DefaultBody:         "<p>official</p>",
		DefaultStyle:        "body{}",
	}, "documents")
	if def.OwnerModuleKey != "documents" || def.Key != "documents.generic_request.official" {
		t.Fatalf("unexpected converted definition: %+v", def)
	}
	if err := svc.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}
	if len(svc.Definitions()) != 1 {
		t.Fatalf("expected one definition, got %+v", svc.Definitions())
	}

	draft, err := svc.SaveDraft(def.Key, "", "", "user_admin")
	if err != nil {
		t.Fatalf("save draft failed: %v", err)
	}
	if draft.Body != def.DefaultBody || draft.Style != def.DefaultStyle {
		t.Fatalf("expected default body/style to be used, got %+v", draft)
	}
	if versions := svc.Versions(def.Key); len(versions) != 1 {
		t.Fatalf("expected one stored version, got %+v", versions)
	}

	if _, err := svc.Publish(def.Key, draft.Version, "user_admin"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if !svc.HasTemplate("document", "generic_request", "official", "print", "deployment", "") {
		t.Fatal("expected template to resolve after publish")
	}
}

func TestTemplateVisualHelpers(t *testing.T) {
	sections := defaultSections()
	if len(sections) != 3 || sections[0].ID != "header" || sections[2].ID != "footer" {
		t.Fatalf("unexpected default sections: %+v", sections)
	}

	report := sampleReport(Definition{Title: "Summary"}, "report.key")
	if report["key"] != "report.key" || report["title"] != "Summary" {
		t.Fatalf("unexpected sample report: %+v", report)
	}
	rows := normalizeSlice(report["rows"])
	if len(rows) != 3 {
		t.Fatalf("unexpected normalized rows: %+v", rows)
	}

	ctx := map[string]any{
		"visible": true,
		"title":   "hello",
		"rows":    []any{map[string]any{"x": 1}},
		"empty":   "",
	}
	if !visualVisible(ctx, "visible") || !visualVisible(ctx, "title") || !visualVisible(ctx, "rows") {
		t.Fatalf("expected visible fields to evaluate true")
	}
	if visualVisible(ctx, "empty") || visualVisible(ctx, "missing") {
		t.Fatalf("expected empty and missing fields to evaluate false")
	}
	if !visualVisible(ctx, "") {
		t.Fatal("expected empty visible_if to default true")
	}
	if maxInt(3, 7) != 7 {
		t.Fatalf("expected maxInt to return larger value")
	}
}
