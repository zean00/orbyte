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
