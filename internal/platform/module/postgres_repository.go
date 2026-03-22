package module

import (
	"context"
	"database/sql"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Get(key string) (InstalledModule, bool) {
	const query = `
		SELECT module_key, enabled, updated_at, updated_by
		FROM installed_modules
		WHERE module_key = $1`
	var item InstalledModule
	err := r.db.QueryRowContext(context.Background(), query, key).Scan(&item.Key, &item.Enabled, &item.UpdatedAt, &item.UpdatedBy)
	if err != nil {
		return InstalledModule{}, false
	}
	return item, true
}

func (r *PostgresRepository) List() []InstalledModule {
	const query = `
		SELECT module_key, enabled, updated_at, updated_by
		FROM installed_modules
		ORDER BY module_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]InstalledModule, 0)
	for rows.Next() {
		var item InstalledModule
		if err := rows.Scan(&item.Key, &item.Enabled, &item.UpdatedAt, &item.UpdatedBy); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) Save(item InstalledModule) error {
	const query = `
		INSERT INTO installed_modules (module_key, enabled, updated_at, updated_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (module_key) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by`
	_, err := r.db.ExecContext(context.Background(), query, item.Key, item.Enabled, item.UpdatedAt, item.UpdatedBy)
	return err
}

func (r *PostgresRepository) GetActivation(baseModuleKey, scope, scopeID string) (LocalExtensionActivation, bool) {
	const query = `
		SELECT base_module_key, extension_module_key, scope_type, scope_id, updated_at, updated_by
		FROM module_local_extension_activations
		WHERE base_module_key = $1 AND scope_type = $2 AND scope_id = $3`
	var item LocalExtensionActivation
	err := r.db.QueryRowContext(context.Background(), query, baseModuleKey, scope, scopeID).Scan(
		&item.BaseModuleKey, &item.ExtensionModuleKey, &item.Scope, &item.ScopeID, &item.UpdatedAt, &item.UpdatedBy,
	)
	if err != nil {
		return LocalExtensionActivation{}, false
	}
	return item, true
}

func (r *PostgresRepository) ListActivations() []LocalExtensionActivation {
	const query = `
		SELECT base_module_key, extension_module_key, scope_type, scope_id, updated_at, updated_by
		FROM module_local_extension_activations
		ORDER BY base_module_key ASC, scope_type ASC, scope_id ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]LocalExtensionActivation, 0)
	for rows.Next() {
		var item LocalExtensionActivation
		if err := rows.Scan(&item.BaseModuleKey, &item.ExtensionModuleKey, &item.Scope, &item.ScopeID, &item.UpdatedAt, &item.UpdatedBy); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveActivation(item LocalExtensionActivation) error {
	const query = `
		INSERT INTO module_local_extension_activations (base_module_key, extension_module_key, scope_type, scope_id, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (base_module_key, scope_type, scope_id) DO UPDATE SET
			extension_module_key = EXCLUDED.extension_module_key,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by`
	_, err := r.db.ExecContext(context.Background(), query, item.BaseModuleKey, item.ExtensionModuleKey, item.Scope, item.ScopeID, item.UpdatedAt, item.UpdatedBy)
	return err
}

func (r *PostgresRepository) DeleteActivation(baseModuleKey, scope, scopeID string) error {
	const query = `
		DELETE FROM module_local_extension_activations
		WHERE base_module_key = $1 AND scope_type = $2 AND scope_id = $3`
	_, err := r.db.ExecContext(context.Background(), query, baseModuleKey, scope, scopeID)
	return err
}
