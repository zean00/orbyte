package featureflags

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (r *PostgresRepository) SaveDefinition(def Definition) error {
	allowedScopes, _ := json.Marshal(def.AllowedScopes)
	const query = `
		INSERT INTO feature_flag_definitions (
			flag_key, module_key, description, allowed_scopes_json, default_state, created_at, updated_at
		) VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7)
		ON CONFLICT (flag_key) DO UPDATE SET
			module_key = EXCLUDED.module_key,
			description = EXCLUDED.description,
			allowed_scopes_json = EXCLUDED.allowed_scopes_json,
			default_state = EXCLUDED.default_state,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, def.Key, def.ModuleKey, def.Description, allowedScopes, def.DefaultState, def.CreatedAt, def.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetDefinition(key string) (Definition, bool) {
	const query = `SELECT flag_key, module_key, COALESCE(description,''), allowed_scopes_json, default_state, created_at, updated_at FROM feature_flag_definitions WHERE flag_key = $1`
	var item Definition
	var scopes []byte
	if err := r.db.QueryRowContext(context.Background(), query, key).Scan(&item.Key, &item.ModuleKey, &item.Description, &scopes, &item.DefaultState, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Definition{}, false
	}
	_ = json.Unmarshal(scopes, &item.AllowedScopes)
	return item, true
}

func (r *PostgresRepository) ListDefinitions() []Definition {
	const query = `SELECT flag_key, module_key, COALESCE(description,''), allowed_scopes_json, default_state, created_at, updated_at FROM feature_flag_definitions ORDER BY flag_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Definition, 0)
	for rows.Next() {
		var item Definition
		var scopes []byte
		if err := rows.Scan(&item.Key, &item.ModuleKey, &item.Description, &scopes, &item.DefaultState, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(scopes, &item.AllowedScopes)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveValue(value Value) error {
	const query = `
		INSERT INTO feature_flag_values (
			flag_key, scope_type, scope_id, enabled, status, updated_at, updated_by, effective_from, effective_to
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (flag_key, scope_type, scope_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by,
			effective_from = EXCLUDED.effective_from,
			effective_to = EXCLUDED.effective_to`
	_, err := r.db.ExecContext(context.Background(), query,
		value.FlagKey, value.Scope, value.ScopeID, value.Enabled, value.Status, value.UpdatedAt, value.UpdatedBy,
		nullableTime(value.EffectiveFrom), nullableTime(value.EffectiveTo),
	)
	return err
}

func (r *PostgresRepository) GetValue(flagKey, scope, scopeID string) (Value, bool) {
	const query = `SELECT flag_key, scope_type, scope_id, enabled, status, updated_at, updated_by, effective_from, effective_to FROM feature_flag_values WHERE flag_key = $1 AND scope_type = $2 AND scope_id = $3`
	var (
		item          Value
		effectiveFrom sql.NullTime
		effectiveTo   sql.NullTime
	)
	if err := r.db.QueryRowContext(context.Background(), query, flagKey, scope, scopeID).Scan(&item.FlagKey, &item.Scope, &item.ScopeID, &item.Enabled, &item.Status, &item.UpdatedAt, &item.UpdatedBy, &effectiveFrom, &effectiveTo); err != nil {
		return Value{}, false
	}
	if effectiveFrom.Valid {
		item.EffectiveFrom = effectiveFrom.Time
	}
	if effectiveTo.Valid {
		item.EffectiveTo = effectiveTo.Time
	}
	return item, true
}

func (r *PostgresRepository) ListValues() []Value {
	const query = `SELECT flag_key, scope_type, scope_id, enabled, status, updated_at, updated_by, effective_from, effective_to FROM feature_flag_values ORDER BY flag_key ASC, scope_type ASC, scope_id ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Value, 0)
	for rows.Next() {
		var (
			item          Value
			effectiveFrom sql.NullTime
			effectiveTo   sql.NullTime
		)
		if err := rows.Scan(&item.FlagKey, &item.Scope, &item.ScopeID, &item.Enabled, &item.Status, &item.UpdatedAt, &item.UpdatedBy, &effectiveFrom, &effectiveTo); err != nil {
			continue
		}
		if effectiveFrom.Valid {
			item.EffectiveFrom = effectiveFrom.Time
		}
		if effectiveTo.Valid {
			item.EffectiveTo = effectiveTo.Time
		}
		items = append(items, item)
	}
	return items
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
