package templateoutput

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jung-kurt/gofpdf"
	"golang.org/x/net/html"

	"orbyte/internal/platform/document"
	"orbyte/internal/platform/model"
	platformmodule "orbyte/internal/platform/module"
	"orbyte/internal/platform/reporting"
)

func TestServicePersistsTemplateDefinitionsAcrossServiceInstances(t *testing.T) {
	repo := NewMemoryRepository()
	reportingSvc := reporting.NewService(model.NewService())
	svc := NewServiceWithRepository(repo, document.NewService(), reportingSvc)
	def := Definition{
		Key:          "documents.invoice.print",
		Title:        "Invoice Print",
		TargetKind:   "document",
		TargetKey:    "invoice",
		RendererKind: "visual",
		DefaultBody:  `{"schema_version":"visual-grid/v1","title":"Invoice Print","sections":[]}`,
	}
	if err := svc.RegisterDefinition(def); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}
	updated, err := svc.SaveDefinition(Definition{
		Key:          def.Key,
		Title:        `Invoice "Official" Print`,
		TargetKind:   def.TargetKind,
		TargetKey:    def.TargetKey,
		RendererKind: def.RendererKind,
		DefaultBody:  def.DefaultBody,
	})
	if err != nil {
		t.Fatalf("save definition failed: %v", err)
	}

	reloaded := NewServiceWithRepository(repo, document.NewService(), reportingSvc)
	got, ok := reloaded.Definition(def.Key)
	if !ok {
		t.Fatalf("expected definition %q to reload from repository", def.Key)
	}
	if got.Title != updated.Title {
		t.Fatalf("expected persisted title %q, got %q", updated.Title, got.Title)
	}
	if len(reloaded.Definitions()) != 1 {
		t.Fatalf("expected one persisted definition, got %+v", reloaded.Definitions())
	}
}

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

func TestServiceRejectsUnknownExplicitFixtureKey(t *testing.T) {
	svc := NewService(document.NewService(), reporting.NewService(model.NewService()))
	if err := svc.RegisterDefinition(Definition{
		Key:          "documents.receipt.fixture-check",
		Title:        "Fixture Check",
		TargetKind:   "document",
		TargetKey:    "receipt",
		RendererKind: "html",
		DefaultBody:  `<p>{{ .document.header.number }}</p>`,
	}); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}
	if _, err := svc.Render(RenderRequest{
		TemplateKey: "documents.receipt.fixture-check",
		TargetKind:  "document",
		Format:      "html",
		Sample:      true,
		FixtureKey:  "missing-fixture",
	}); err == nil || !strings.Contains(err.Error(), "fixture not found") {
		t.Fatalf("expected explicit missing fixture to fail, got %v", err)
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

func TestServicePreviewDuplicateResetAndFixtures(t *testing.T) {
	svc := NewService(document.NewService(), reporting.NewService(model.NewService()))
	if err := svc.RegisterDefinition(Definition{
		Key:          "documents.generic_request.preview",
		Title:        "Preview Request",
		TargetKind:   "document",
		TargetKey:    "generic_request",
		RendererKind: "visual",
		DefaultBody:  `{"schema_version":"visual-grid/v1","title":"Preview Request","sections":[{"id":"body","rows":[{"columns":[{"span":12,"blocks":[{"type":"text","text":"Preview Request"},{"type":"field","label":"Number","path":"document.header.number"}]}]}]}]}`,
	}); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}
	draft, err := svc.SaveDraftWithOptions("documents.generic_request.preview", `{"schema_version":"visual-grid/v1","title":"Draft Preview","sections":[{"id":"body","rows":[{"columns":[{"span":12,"blocks":[{"type":"text","text":"Draft Preview"},{"type":"field","label":"Number","path":"document.header.number"},{"type":"image","label":"Logo"}]}]}]}]}`, "", "user_admin", "draft note", 0)
	if err != nil {
		t.Fatalf("save draft failed: %v", err)
	}
	if _, err := svc.Publish("documents.generic_request.preview", draft.Version, "user_admin"); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	dup, err := svc.DuplicateDraft("documents.generic_request.preview", draft.Version, "user_admin")
	if err != nil {
		t.Fatalf("duplicate draft failed: %v", err)
	}
	if dup.Status != "draft" || dup.ClonedFromVersion != draft.Version {
		t.Fatalf("expected duplicated draft metadata, got %+v", dup)
	}
	reset, err := svc.ResetDraftToPublished("documents.generic_request.preview", "user_admin")
	if err != nil {
		t.Fatalf("reset draft failed: %v", err)
	}
	if reset.ClonedFromVersion != draft.Version {
		t.Fatalf("expected reset draft to reference published version, got %+v", reset)
	}
	fixture, err := svc.SaveFixture(TemplateFixture{
		Name:        "Preview Fixture",
		TargetKind:  "document",
		TemplateKey: "documents.generic_request.preview",
		SourceType:  "sample",
		Payload: map[string]any{
			"header": map[string]any{"number": "FIX-001"},
			"body":   map[string]any{"payload": map[string]any{"title": "Fixture"}},
			"lines":  []any{},
		},
		UpdatedBy: "user_admin",
	})
	if err != nil {
		t.Fatalf("save fixture failed: %v", err)
	}
	preview, err := svc.Preview(RenderRequest{
		TemplateKey: "documents.generic_request.preview",
		TargetKind:  "document",
		TargetKey:   "generic_request",
		Draft:       true,
		FixtureKey:  fixture.FixtureKey,
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if preview.DataSource != "fixture" || preview.Mode != "draft" {
		t.Fatalf("unexpected preview metadata: %+v", preview)
	}
	if len(preview.Outputs) != 3 {
		t.Fatalf("expected html/pdf/print outputs, got %+v", preview.Outputs)
	}
	if !strings.Contains(preview.Outputs[0].HTML, "FIX-001") {
		t.Fatalf("expected fixture-backed preview html, got %s", preview.Outputs[0].HTML)
	}
	debug, err := svc.ResolveBindingDebug(RenderRequest{TemplateKey: "documents.generic_request.preview", TargetKind: "document", TargetKey: "generic_request", Draft: true})
	if err != nil {
		t.Fatalf("binding debug failed: %v", err)
	}
	if debug.DefinitionKey != "documents.generic_request.preview" || debug.Version == 0 {
		t.Fatalf("unexpected binding debug: %+v", debug)
	}
	compare, err := svc.CompareVersions("documents.generic_request.preview", draft.Version, reset.Version)
	if err != nil {
		t.Fatalf("compare versions failed: %v", err)
	}
	if !compare.HasDifferences && draft.Body != reset.Body {
		t.Fatalf("expected compare to report differences: %+v", compare)
	}
}

func TestServiceValidateAndBindingDebugBranches(t *testing.T) {
	svc := NewService(document.NewService(), reporting.NewService(model.NewService()))
	if issues := svc.Validate(RenderRequest{TemplateKey: "missing"}); len(issues) != 1 || issues[0].Code != "template_definition_not_found" {
		t.Fatalf("expected missing template validation issue, got %+v", issues)
	}
	if err := svc.RegisterDefinition(Definition{
		Key:          "documents.generic_request.validate",
		Title:        "Validate Request",
		TargetKind:   "document",
		TargetKey:    "generic_request",
		RendererKind: "html",
		DefaultBody:  `<p>{{ .document.Header.Type }}</p>`,
	}); err != nil {
		t.Fatalf("register definition failed: %v", err)
	}

	issues := svc.Validate(RenderRequest{
		TemplateKey:  "documents.generic_request.validate",
		RendererKind: "html",
		Body:         `{{ if }}`,
	})
	if len(issues) == 0 || issues[0].Code != "html_template_invalid" {
		t.Fatalf("expected html template validation failure, got %+v", issues)
	}

	if issues := svc.validateVersion(Definition{Key: "invalid"}, Version{RendererKind: "markdown", Body: "hello"}); len(issues) == 0 || issues[0].Code != "renderer_kind_invalid" {
		t.Fatalf("expected invalid renderer issue, got %+v", issues)
	}

	debug, err := svc.ResolveBindingDebug(RenderRequest{
		TargetKind:     "document",
		TargetKey:      "generic_request",
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
	})
	if err != nil {
		t.Fatalf("resolve binding debug failed: %v", err)
	}
	if debug.DefinitionKey != "documents.generic_request.validate" || debug.Version != 1 {
		t.Fatalf("expected definition fallback in binding debug, got %+v", debug)
	}
	if len(debug.ScopePath) == 0 {
		t.Fatalf("expected scope path to be recorded, got %+v", debug)
	}

	if _, err := svc.ResolveBindingDebug(RenderRequest{TargetKind: "document", TargetKey: "unknown"}); err == nil {
		t.Fatal("expected unresolved binding debug to fail")
	}
}

func TestTemplateValidationHelpersAndFixtureBranches(t *testing.T) {
	visual := VisualTemplate{
		SchemaVersion: "visual-grid/v2",
		Sections: []VisualSection{
			{
				ID: "empty",
			},
			{
				ID: "body",
				Rows: []VisualRow{
					{},
					{Columns: []VisualCell{
						{Span: 13, Blocks: []VisualBlock{{Type: "field"}}},
						{Span: 6, Blocks: []VisualBlock{{Type: "table"}, {Type: "totals", RowsPath: "rows"}, {Type: "mystery"}}},
					}},
				},
			},
		},
	}
	issues := validateVisualTemplate(visual)
	if len(filterIssues(issues, "error")) == 0 {
		t.Fatalf("expected visual validation errors, got %+v", issues)
	}
	if len(filterIssues(issues, "warning")) == 0 {
		t.Fatalf("expected visual validation warnings, got %+v", issues)
	}
	if !strings.Contains(joinIssueMessages(issues), "column span must be between 1 and 12") {
		t.Fatalf("expected joined issue message to include error text, got %+v", issues)
	}

	svc := NewService(document.NewService(), reporting.NewService(model.NewService()))
	if _, err := svc.SaveFixture(TemplateFixture{}); err == nil {
		t.Fatal("expected fixture target kind validation error")
	}
	if _, err := svc.SaveFixture(TemplateFixture{TemplateKey: "missing"}); err == nil {
		t.Fatal("expected fixture save for missing definition to fail")
	}
	if _, err := svc.SaveBinding(Binding{TemplateKey: "missing"}); err == nil {
		t.Fatal("expected binding save for missing definition to fail")
	}
	if _, err := svc.CompareVersions("missing", 1, 2); err == nil {
		t.Fatal("expected compare versions to fail for missing versions")
	}
}

func TestVisualTemplateLayoutFieldsRenderAndValidate(t *testing.T) {
	visual := VisualTemplate{
		SchemaVersion: "visual-grid/v1",
		Title:         "Promo Sheet",
		Sections: []VisualSection{
			{
				ID:    "body",
				Title: "Body",
				Rows: []VisualRow{
					{
						ID:            "body-row-1",
						Width:         "72%",
						Height:        "180px",
						AlignX:        "center",
						AlignY:        "stretch",
						ContentAlignX: "center",
						ContentAlignY: "stretch",
						Columns: []VisualCell{
							{
								ID:            "body-cell-1",
								Span:          6,
								Width:         "50%",
								Height:        "160px",
								AlignX:        "end",
								AlignY:        "center",
								ContentAlignX: "center",
								ContentAlignY: "end",
								Blocks:        []VisualBlock{{Type: "text", Text: "Hello world"}},
							},
						},
					},
				},
			},
		},
	}

	issues := validateVisualTemplate(visual)
	if len(filterIssues(issues, "error")) != 0 {
		t.Fatalf("expected layout fields to validate, got %+v", issues)
	}

	body, err := json.Marshal(visual)
	if err != nil {
		t.Fatalf("marshal visual template failed: %v", err)
	}
	html, err := renderVisualTemplate(Version{RendererKind: "visual", Body: string(body)}, map[string]any{})
	if err != nil {
		t.Fatalf("renderVisualTemplate failed: %v", err)
	}
	if !strings.Contains(html, `class="template-row-shell"`) {
		t.Fatalf("expected row shell wrapper in html, got %s", html)
	}
	if !strings.Contains(html, `width:72%`) || !strings.Contains(html, `min-height:180px`) {
		t.Fatalf("expected row layout styles in html, got %s", html)
	}
	if !strings.Contains(html, `width:50%`) || !strings.Contains(html, `justify-self:end`) || !strings.Contains(html, `min-height:160px`) {
		t.Fatalf("expected cell layout styles in html, got %s", html)
	}
}

func TestVisualTemplateWidthColumnsSkipSpanErrorsAndStretchDoesNotLeakToJustifyContent(t *testing.T) {
	visual := VisualTemplate{
		SchemaVersion: "visual-grid/v1",
		Sections: []VisualSection{
			{
				ID: "body",
				Rows: []VisualRow{
					{
						ID:     "body-row-1",
						AlignX: "stretch",
						Columns: []VisualCell{
							{
								ID:            "body-cell-1",
								Span:          0,
								Width:         "48%",
								ContentAlignY: "stretch",
								Blocks:        []VisualBlock{{Type: "text", Text: "Width-based column"}},
							},
						},
					},
				},
			},
		},
	}

	issues := validateVisualTemplate(visual)
	for _, item := range issues {
		if item.Code == "visual_span_invalid" {
			t.Fatalf("did not expect span validation error for explicit-width column, got %+v", issues)
		}
	}

	body, err := json.Marshal(visual)
	if err != nil {
		t.Fatalf("marshal visual template failed: %v", err)
	}
	html, err := renderVisualTemplate(Version{RendererKind: "visual", Body: string(body)}, map[string]any{})
	if err != nil {
		t.Fatalf("renderVisualTemplate failed: %v", err)
	}
	if strings.Contains(html, "justify-content:stretch") {
		t.Fatalf("did not expect invalid stretch justify-content in html, got %s", html)
	}
}

func TestTemplatePDFAndHTMLHelperPaths(t *testing.T) {
	lines := htmlToPDFLines(`<style>p{}</style><h1>Title</h1><p>Hello<br>world</p><table><tr><td>A</td></tr></table>`)
	if len(lines) == 0 || lines[0] != "Title" {
		t.Fatalf("expected html to pdf lines to retain readable content, got %+v", lines)
	}

	doc, err := html.Parse(strings.NewReader(`<dl><dt>Label</dt><dd>Value</dd></dl><table><tr><th>H1</th><th>H2</th></tr><tr><td>A</td><td>B</td></tr></table><ul><li>First</li></ul><ol><li>Second</li></ol>`))
	if err != nil {
		t.Fatalf("parse html failed: %v", err)
	}
	pdfBytes, err := pdfFromHTML(Version{RendererKind: "html"}, `<h1>Title</h1><p>Paragraph</p><dl><dt>Term</dt><dd>Definition</dd></dl><table><tr><th>A</th></tr><tr><td>B</td></tr></table><ul><li>Bullet</li></ul>`)
	if err != nil {
		t.Fatalf("pdfFromHTML failed: %v", err)
	}
	if len(pdfBytes) < 4 || !strings.HasPrefix(string(pdfBytes[:4]), "%PDF") {
		t.Fatalf("expected pdf bytes, got %q", string(pdfBytes))
	}

	rows := extractHTMLTableRows(doc)
	if len(rows) != 2 || !rows[0].Header || rows[1].Cells[0] != "A" {
		t.Fatalf("unexpected extracted table rows: %+v", rows)
	}

	if text := collectNodeText(doc); !strings.Contains(text, "Label") || !strings.Contains(text, "Second") {
		t.Fatalf("expected collected node text, got %q", text)
	}

	pdf := newTestPDF()
	renderDefinitionList(pdf, 100, doc)
	renderHTMLTablePDF(pdf, 100, doc)
	renderHTMLListPDF(pdf, 100, doc, false)
	renderHTMLListPDF(pdf, 100, doc, true)
	renderHTMLNodesToPDF(pdf, doc, 10, 10, 10, 10)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("pdf output failed: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected rendered helper pdf output")
	}
}

func newTestPDF() *gofpdf.Fpdf {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	return pdf
}
