package templateoutput

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositoryVersionsAndBindings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO template_output_versions (template_key, version_no, status, renderer_kind, body, style, change_note, cloned_from_version, last_previewed_at, last_render_status, last_render_error, last_rendered_at, updated_at, updated_by, published_at, published_by)")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveVersion(Version{
		TemplateKey:  "documents.generic_request.default",
		Version:      1,
		Status:       "draft",
		RendererKind: "html",
		Body:         "<p>draft</p>",
		Style:        "body{}",
		UpdatedAt:    now,
		UpdatedBy:    "user_admin",
	}); err != nil {
		t.Fatalf("save version failed: %v", err)
	}

	versionRows := sqlmock.NewRows([]string{"template_key", "version_no", "status", "renderer_kind", "body", "style", "change_note", "cloned_from_version", "last_previewed_at", "last_render_status", "last_render_error", "last_rendered_at", "updated_at", "updated_by", "published_at", "published_by"}).
		AddRow("documents.generic_request.default", 1, "draft", "html", "<p>draft</p>", "body{}", "", 0, nil, "", "", nil, now, "user_admin", nil, "")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT template_key, version_no, status, renderer_kind, body, COALESCE(style,''), COALESCE(change_note,''), COALESCE(cloned_from_version,0), last_previewed_at, COALESCE(last_render_status,''), COALESCE(last_render_error,''), last_rendered_at, updated_at, COALESCE(updated_by,''), published_at, COALESCE(published_by,'') FROM template_output_versions WHERE template_key = $1 ORDER BY version_no ASC")).
		WillReturnRows(versionRows)
	versions := repo.Versions("documents.generic_request.default")
	if len(versions) != 1 || versions[0].Version != 1 {
		t.Fatalf("unexpected versions: %+v", versions)
	}

	listRows := sqlmock.NewRows([]string{"template_key", "version_no", "status", "renderer_kind", "body", "style", "change_note", "cloned_from_version", "last_previewed_at", "last_render_status", "last_render_error", "last_rendered_at", "updated_at", "updated_by", "published_at", "published_by"}).
		AddRow("documents.generic_request.default", 1, "draft", "html", "<p>draft</p>", "body{}", "", 0, nil, "", "", nil, now, "user_admin", nil, "")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT template_key, version_no, status, renderer_kind, body, COALESCE(style,''), COALESCE(change_note,''), COALESCE(cloned_from_version,0), last_previewed_at, COALESCE(last_render_status,''), COALESCE(last_render_error,''), last_rendered_at, updated_at, COALESCE(updated_by,''), published_at, COALESCE(published_by,'') FROM template_output_versions ORDER BY template_key ASC, version_no ASC")).
		WillReturnRows(listRows)
	if items := repo.ListVersions(); len(items) != 1 || items[0].TemplateKey != "documents.generic_request.default" {
		t.Fatalf("unexpected listed versions: %+v", items)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO template_output_bindings (binding_id, template_key, scope_type, scope_id, target_kind, target_key, purpose, channel, is_default, is_official, updated_at, updated_by)")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveBinding(Binding{
		ID:          "bind-1",
		TemplateKey: "documents.generic_request.default",
		ScopeType:   "location",
		ScopeID:     "loc_hq",
		TargetKind:  "document",
		TargetKey:   "generic_request",
		Purpose:     "official",
		Channel:     "print",
		UpdatedAt:   now,
		UpdatedBy:   "user_admin",
	}); err != nil {
		t.Fatalf("save binding failed: %v", err)
	}

	bindingRows := sqlmock.NewRows([]string{"binding_id", "template_key", "scope_type", "scope_id", "target_kind", "target_key", "purpose", "channel", "is_default", "is_official", "updated_at", "updated_by"}).
		AddRow("bind-1", "documents.generic_request.default", "location", "loc_hq", "document", "generic_request", "official", "print", false, false, now, "user_admin")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT binding_id, template_key, scope_type, COALESCE(scope_id,''), target_kind, target_key, COALESCE(purpose,''), COALESCE(channel,''), is_default, is_official, updated_at, COALESCE(updated_by,'') FROM template_output_bindings ORDER BY updated_at DESC")).
		WillReturnRows(bindingRows)
	bindings := repo.Bindings()
	if len(bindings) != 1 || bindings[0].ID != "bind-1" {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO template_output_fixtures (fixture_key, name, target_kind, template_key, source_type, payload_json, updated_at, updated_by)")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveFixture(TemplateFixture{
		FixtureKey:  "fixture-1",
		Name:        "Sample",
		TargetKind:  "document",
		TemplateKey: "documents.generic_request.default",
		SourceType:  "sample",
		Payload:     map[string]any{"header": map[string]any{"number": "SAMPLE-1"}},
		UpdatedAt:   now,
		UpdatedBy:   "user_admin",
	}); err != nil {
		t.Fatalf("save fixture failed: %v", err)
	}

	fixtureRows := sqlmock.NewRows([]string{"fixture_key", "name", "target_kind", "template_key", "source_type", "payload_json", "updated_at", "updated_by"}).
		AddRow("fixture-1", "Sample", "document", "documents.generic_request.default", "sample", []byte(`{"header":{"number":"SAMPLE-1"}}`), now, "user_admin")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT fixture_key, COALESCE(name,''), target_kind, COALESCE(template_key,''), COALESCE(source_type,''), COALESCE(payload_json,'{}'::jsonb), updated_at, COALESCE(updated_by,'') FROM template_output_fixtures WHERE ($1 = '' OR target_kind = $1) AND ($2 = '' OR COALESCE(template_key,'') = $2) ORDER BY target_kind ASC, fixture_key ASC")).
		WithArgs("document", "documents.generic_request.default").
		WillReturnRows(fixtureRows)
	fixtures := repo.Fixtures("documents.generic_request.default", "document")
	if len(fixtures) != 1 || fixtures[0].FixtureKey != "fixture-1" {
		t.Fatalf("unexpected fixtures: %+v", fixtures)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestNullTime(t *testing.T) {
	if value := nullTime(time.Time{}); value != nil {
		t.Fatalf("expected nil nullTime for zero time, got %+v", value)
	}
	now := time.Now().UTC()
	if value := nullTime(now); value == nil {
		t.Fatal("expected non-nil nullTime for populated time")
	}
}
