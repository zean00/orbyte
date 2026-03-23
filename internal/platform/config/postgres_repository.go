package config

import (
	"context"
	"database/sql"
	"encoding/json"

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

func (r *PostgresRepository) Get(key, scope, scopeID string) (Entry, bool) {
	const query = `
		SELECT configuration_key, module_key, category_key, scope_type, scope_id, value_json, updated_at, updated_by, COALESCE(description, '')
		FROM configuration_entries
		WHERE configuration_key = $1 AND scope_type = $2 AND scope_id = $3`
	var (
		entry     Entry
		valueJSON []byte
	)
	err := r.db.QueryRowContext(context.Background(), query, key, scope, scopeID).Scan(
		&entry.Key,
		&entry.ModuleKey,
		&entry.Category,
		&entry.Scope,
		&entry.ScopeID,
		&valueJSON,
		&entry.UpdatedAt,
		&entry.UpdatedBy,
		&entry.Description,
	)
	if err != nil {
		return Entry{}, false
	}
	_ = json.Unmarshal(valueJSON, &entry.Value)
	return entry, true
}

func (r *PostgresRepository) List() []Entry {
	const query = `
		SELECT configuration_key, module_key, category_key, scope_type, scope_id, value_json, updated_at, updated_by, COALESCE(description, '')
		FROM configuration_entries
		ORDER BY configuration_key ASC, scope_type ASC, scope_id ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Entry, 0)
	for rows.Next() {
		var (
			entry     Entry
			valueJSON []byte
		)
		if err := rows.Scan(&entry.Key, &entry.ModuleKey, &entry.Category, &entry.Scope, &entry.ScopeID, &valueJSON, &entry.UpdatedAt, &entry.UpdatedBy, &entry.Description); err != nil {
			continue
		}
		_ = json.Unmarshal(valueJSON, &entry.Value)
		items = append(items, entry)
	}
	return items
}

func (r *PostgresRepository) Save(entry Entry) error {
	valueJSON, err := json.Marshal(entry.Value)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO configuration_entries (
			configuration_key, module_key, category_key, scope_type, scope_id, value_json, updated_at, updated_by, description
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, NULLIF($9, ''))
		ON CONFLICT (configuration_key, scope_type, scope_id) DO UPDATE SET
			module_key = EXCLUDED.module_key,
			category_key = EXCLUDED.category_key,
			value_json = EXCLUDED.value_json,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by,
			description = EXCLUDED.description`
	_, err = r.db.ExecContext(context.Background(), query, entry.Key, entry.ModuleKey, entry.Category, entry.Scope, entry.ScopeID, string(valueJSON), entry.UpdatedAt, entry.UpdatedBy, entry.Description)
	return err
}
