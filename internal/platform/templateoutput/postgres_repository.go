package templateoutput

import (
	"database/sql"
	"sort"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Versions(templateKey string) []Version {
	rows, err := r.db.Query(`SELECT template_key, version_no, status, renderer_kind, body, COALESCE(style,''), updated_at, COALESCE(updated_by,''), published_at, COALESCE(published_by,'') FROM template_output_versions WHERE template_key = $1 ORDER BY version_no ASC`, templateKey)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		var item Version
		_ = rows.Scan(&item.TemplateKey, &item.Version, &item.Status, &item.RendererKind, &item.Body, &item.Style, &item.UpdatedAt, &item.UpdatedBy, &item.PublishedAt, &item.PublishedBy)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) ListVersions() []Version {
	rows, err := r.db.Query(`SELECT template_key, version_no, status, renderer_kind, body, COALESCE(style,''), updated_at, COALESCE(updated_by,''), published_at, COALESCE(published_by,'') FROM template_output_versions ORDER BY template_key ASC, version_no ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		var item Version
		_ = rows.Scan(&item.TemplateKey, &item.Version, &item.Status, &item.RendererKind, &item.Body, &item.Style, &item.UpdatedAt, &item.UpdatedBy, &item.PublishedAt, &item.PublishedBy)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveVersion(version Version) error {
	_, err := r.db.Exec(`INSERT INTO template_output_versions (template_key, version_no, status, renderer_kind, body, style, updated_at, updated_by, published_at, published_by)
VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,NULLIF($8,''),$9,NULLIF($10,''))
ON CONFLICT (template_key, version_no) DO UPDATE SET status=EXCLUDED.status, renderer_kind=EXCLUDED.renderer_kind, body=EXCLUDED.body, style=EXCLUDED.style, updated_at=EXCLUDED.updated_at, updated_by=EXCLUDED.updated_by, published_at=EXCLUDED.published_at, published_by=EXCLUDED.published_by`,
		version.TemplateKey, version.Version, version.Status, version.RendererKind, version.Body, version.Style, version.UpdatedAt, version.UpdatedBy, nullTime(version.PublishedAt), version.PublishedBy)
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

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
