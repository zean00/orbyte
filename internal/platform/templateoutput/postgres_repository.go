package templateoutput

import (
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"orbyte/internal/platform/store"
)

type PostgresRepository struct {
	db store.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return NewPostgresRepositoryWithDB(store.UninstrumentedDB(db))
}

func NewPostgresRepositoryWithDB(db store.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Definitions() []Definition {
	rows, err := r.db.Query(`SELECT template_key, title, COALESCE(title_i18n,'{}'::jsonb), COALESCE(description,''), COALESCE(description_i18n,'{}'::jsonb), COALESCE(owner_module_key,''), target_kind, target_key, renderer_kind, COALESCE(default_format,''), COALESCE(formats_json,'[]'::jsonb), COALESCE(purpose,''), COALESCE(channel,''), COALESCE(related_sources_json,'[]'::jsonb), COALESCE(allowed_scopes_json,'[]'::jsonb), COALESCE(required_permissions_json,'[]'::jsonb), COALESCE(default_body,''), COALESCE(default_style,'') FROM template_output_definitions ORDER BY template_key ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Definition, 0)
	for rows.Next() {
		item, scanErr := scanDefinition(rows)
		if scanErr == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) GetDefinition(key string) (Definition, bool) {
	row := r.db.QueryRow(`SELECT template_key, title, COALESCE(title_i18n,'{}'::jsonb), COALESCE(description,''), COALESCE(description_i18n,'{}'::jsonb), COALESCE(owner_module_key,''), target_kind, target_key, renderer_kind, COALESCE(default_format,''), COALESCE(formats_json,'[]'::jsonb), COALESCE(purpose,''), COALESCE(channel,''), COALESCE(related_sources_json,'[]'::jsonb), COALESCE(allowed_scopes_json,'[]'::jsonb), COALESCE(required_permissions_json,'[]'::jsonb), COALESCE(default_body,''), COALESCE(default_style,'') FROM template_output_definitions WHERE template_key = $1`, key)
	item, err := scanDefinition(row)
	if err != nil {
		return Definition{}, false
	}
	return item, true
}

func (r *PostgresRepository) SaveDefinition(def Definition) error {
	titleI18n, _ := marshalJSON(def.TitleI18n)
	descriptionI18n, _ := marshalJSON(def.DescriptionI18n)
	formats, _ := marshalJSON(def.Formats)
	relatedSources, _ := marshalJSON(def.RelatedSources)
	allowedScopes, _ := marshalJSON(def.AllowedScopes)
	requiredPermissions, _ := marshalJSON(def.RequiredPermissions)
	_, err := r.db.Exec(`INSERT INTO template_output_definitions (template_key, title, title_i18n, description, description_i18n, owner_module_key, target_kind, target_key, renderer_kind, default_format, formats_json, purpose, channel, related_sources_json, allowed_scopes_json, required_permissions_json, default_body, default_style)
VALUES ($1,$2,$3,NULLIF($4,''),$5,NULLIF($6,''),$7,$8,$9,NULLIF($10,''),$11,NULLIF($12,''),NULLIF($13,''),$14,$15,$16,NULLIF($17,''),NULLIF($18,''))
ON CONFLICT (template_key) DO UPDATE SET title=EXCLUDED.title, title_i18n=EXCLUDED.title_i18n, description=EXCLUDED.description, description_i18n=EXCLUDED.description_i18n, owner_module_key=EXCLUDED.owner_module_key, target_kind=EXCLUDED.target_kind, target_key=EXCLUDED.target_key, renderer_kind=EXCLUDED.renderer_kind, default_format=EXCLUDED.default_format, formats_json=EXCLUDED.formats_json, purpose=EXCLUDED.purpose, channel=EXCLUDED.channel, related_sources_json=EXCLUDED.related_sources_json, allowed_scopes_json=EXCLUDED.allowed_scopes_json, required_permissions_json=EXCLUDED.required_permissions_json, default_body=EXCLUDED.default_body, default_style=EXCLUDED.default_style`,
		def.Key, def.Title, titleI18n, def.Description, descriptionI18n, def.OwnerModuleKey, def.TargetKind, def.TargetKey, def.RendererKind, def.DefaultFormat, formats, def.Purpose, def.Channel, relatedSources, allowedScopes, requiredPermissions, def.DefaultBody, def.DefaultStyle)
	return err
}

func (r *PostgresRepository) DeleteDefinition(key string) error {
	_, err := r.db.Exec(`DELETE FROM template_output_definitions WHERE template_key = $1`, key)
	return err
}

func (r *PostgresRepository) Versions(templateKey string) []Version {
	rows, err := r.db.Query(`SELECT template_key, version_no, status, renderer_kind, body, COALESCE(style,''), COALESCE(change_note,''), COALESCE(cloned_from_version,0), last_previewed_at, COALESCE(last_render_status,''), COALESCE(last_render_error,''), last_rendered_at, updated_at, COALESCE(updated_by,''), published_at, COALESCE(published_by,'') FROM template_output_versions WHERE template_key = $1 ORDER BY version_no ASC`, templateKey)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		var item Version
		_ = rows.Scan(&item.TemplateKey, &item.Version, &item.Status, &item.RendererKind, &item.Body, &item.Style, &item.ChangeNote, &item.ClonedFromVersion, &item.LastPreviewedAt, &item.LastRenderStatus, &item.LastRenderError, &item.LastRenderedAt, &item.UpdatedAt, &item.UpdatedBy, &item.PublishedAt, &item.PublishedBy)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) ListVersions() []Version {
	rows, err := r.db.Query(`SELECT template_key, version_no, status, renderer_kind, body, COALESCE(style,''), COALESCE(change_note,''), COALESCE(cloned_from_version,0), last_previewed_at, COALESCE(last_render_status,''), COALESCE(last_render_error,''), last_rendered_at, updated_at, COALESCE(updated_by,''), published_at, COALESCE(published_by,'') FROM template_output_versions ORDER BY template_key ASC, version_no ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		var item Version
		_ = rows.Scan(&item.TemplateKey, &item.Version, &item.Status, &item.RendererKind, &item.Body, &item.Style, &item.ChangeNote, &item.ClonedFromVersion, &item.LastPreviewedAt, &item.LastRenderStatus, &item.LastRenderError, &item.LastRenderedAt, &item.UpdatedAt, &item.UpdatedBy, &item.PublishedAt, &item.PublishedBy)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveVersion(version Version) error {
	_, err := r.db.Exec(`INSERT INTO template_output_versions (template_key, version_no, status, renderer_kind, body, style, change_note, cloned_from_version, last_previewed_at, last_render_status, last_render_error, last_rendered_at, updated_at, updated_by, published_at, published_by)
VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,0),$9,NULLIF($10,''),NULLIF($11,''),$12,$13,NULLIF($14,''),$15,NULLIF($16,''))
ON CONFLICT (template_key, version_no) DO UPDATE SET status=EXCLUDED.status, renderer_kind=EXCLUDED.renderer_kind, body=EXCLUDED.body, style=EXCLUDED.style, change_note=EXCLUDED.change_note, cloned_from_version=EXCLUDED.cloned_from_version, last_previewed_at=EXCLUDED.last_previewed_at, last_render_status=EXCLUDED.last_render_status, last_render_error=EXCLUDED.last_render_error, last_rendered_at=EXCLUDED.last_rendered_at, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by, published_at=EXCLUDED.published_at, published_by=EXCLUDED.published_by`,
		version.TemplateKey, version.Version, version.Status, version.RendererKind, version.Body, version.Style, version.ChangeNote, version.ClonedFromVersion, nullTime(version.LastPreviewedAt), version.LastRenderStatus, version.LastRenderError, nullTime(version.LastRenderedAt), version.UpdatedAt, version.UpdatedBy, nullTime(version.PublishedAt), version.PublishedBy)
	return err
}

func (r *PostgresRepository) DeleteVersions(templateKey string) error {
	_, err := r.db.Exec(`DELETE FROM template_output_versions WHERE template_key = $1`, templateKey)
	return err
}

func (r *PostgresRepository) Bindings() []Binding {
	rows, err := r.db.Query(`SELECT binding_id, template_key, scope_type, COALESCE(scope_id,''), target_kind, target_key, COALESCE(purpose,''), COALESCE(channel,''), is_default, is_official, updated_at, COALESCE(updated_by,'') FROM template_output_bindings ORDER BY updated_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Binding, 0)
	for rows.Next() {
		var item Binding
		_ = rows.Scan(&item.ID, &item.TemplateKey, &item.ScopeType, &item.ScopeID, &item.TargetKind, &item.TargetKey, &item.Purpose, &item.Channel, &item.IsDefault, &item.IsOfficial, &item.UpdatedAt, &item.UpdatedBy)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (r *PostgresRepository) SaveBinding(binding Binding) error {
	_, err := r.db.Exec(`INSERT INTO template_output_bindings (binding_id, template_key, scope_type, scope_id, target_kind, target_key, purpose, channel, is_default, is_official, updated_at, updated_by)
VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),NULLIF($8,''),$9,$10,$11,NULLIF($12,''))
ON CONFLICT (binding_id) DO UPDATE SET template_key=EXCLUDED.template_key, scope_type=EXCLUDED.scope_type, scope_id=EXCLUDED.scope_id, target_kind=EXCLUDED.target_kind, target_key=EXCLUDED.target_key, purpose=EXCLUDED.purpose, channel=EXCLUDED.channel, is_default=EXCLUDED.is_default, is_official=EXCLUDED.is_official, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by`,
		binding.ID, binding.TemplateKey, binding.ScopeType, binding.ScopeID, binding.TargetKind, binding.TargetKey, binding.Purpose, binding.Channel, binding.IsDefault, binding.IsOfficial, binding.UpdatedAt, binding.UpdatedBy)
	return err
}

func (r *PostgresRepository) DeleteBinding(bindingID string) error {
	_, err := r.db.Exec(`DELETE FROM template_output_bindings WHERE binding_id = $1`, bindingID)
	return err
}

func (r *PostgresRepository) Fixtures(templateKey, targetKind string) []TemplateFixture {
	rows, err := r.db.Query(`SELECT fixture_key, COALESCE(name,''), target_kind, COALESCE(template_key,''), COALESCE(source_type,''), COALESCE(payload_json,'{}'::jsonb), updated_at, COALESCE(updated_by,'') FROM template_output_fixtures WHERE ($1 = '' OR target_kind = $1) AND ($2 = '' OR COALESCE(template_key,'') = $2) ORDER BY target_kind ASC, fixture_key ASC`, targetKind, templateKey)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]TemplateFixture, 0)
	for rows.Next() {
		var item TemplateFixture
		var payload []byte
		_ = rows.Scan(&item.FixtureKey, &item.Name, &item.TargetKind, &item.TemplateKey, &item.SourceType, &payload, &item.UpdatedAt, &item.UpdatedBy)
		_ = unmarshalJSON(payload, &item.Payload)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveFixture(fixture TemplateFixture) error {
	payload, _ := marshalJSON(fixture.Payload)
	_, err := r.db.Exec(`INSERT INTO template_output_fixtures (fixture_key, name, target_kind, template_key, source_type, payload_json, updated_at, updated_by)
VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7,NULLIF($8,''))
ON CONFLICT (fixture_key) DO UPDATE SET name=EXCLUDED.name, target_kind=EXCLUDED.target_kind, template_key=EXCLUDED.template_key, source_type=EXCLUDED.source_type, payload_json=EXCLUDED.payload_json, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by`,
		fixture.FixtureKey, fixture.Name, fixture.TargetKind, fixture.TemplateKey, fixture.SourceType, payload, fixture.UpdatedAt, fixture.UpdatedBy)
	return err
}

func (r *PostgresRepository) DeleteFixtures(templateKey string) error {
	_, err := r.db.Exec(`DELETE FROM template_output_fixtures WHERE COALESCE(template_key, '') = $1`, templateKey)
	return err
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func marshalJSON(value any) ([]byte, error) {
	if value == nil {
		return []byte(`{}`), nil
	}
	return json.Marshal(value)
}

func unmarshalJSON(raw []byte, target any) error {
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	return json.Unmarshal(raw, target)
}

type definitionScanner interface {
	Scan(dest ...any) error
}

func scanDefinition(scanner definitionScanner) (Definition, error) {
	var item Definition
	var titleI18n []byte
	var descriptionI18n []byte
	var formats []byte
	var relatedSources []byte
	var allowedScopes []byte
	var requiredPermissions []byte
	if err := scanner.Scan(&item.Key, &item.Title, &titleI18n, &item.Description, &descriptionI18n, &item.OwnerModuleKey, &item.TargetKind, &item.TargetKey, &item.RendererKind, &item.DefaultFormat, &formats, &item.Purpose, &item.Channel, &relatedSources, &allowedScopes, &requiredPermissions, &item.DefaultBody, &item.DefaultStyle); err != nil {
		return Definition{}, err
	}
	_ = unmarshalJSON(titleI18n, &item.TitleI18n)
	_ = unmarshalJSON(descriptionI18n, &item.DescriptionI18n)
	_ = unmarshalJSON(formats, &item.Formats)
	_ = unmarshalJSON(relatedSources, &item.RelatedSources)
	_ = unmarshalJSON(allowedScopes, &item.AllowedScopes)
	_ = unmarshalJSON(requiredPermissions, &item.RequiredPermissions)
	return item, nil
}
