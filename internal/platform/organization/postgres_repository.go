package organization

import (
	"context"
	"database/sql"

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

func (r *PostgresRepository) Root() Organization {
	const query = `
		SELECT organization_id, organization_key, name, status, created_at, updated_at
		FROM organizations
		ORDER BY created_at ASC
		LIMIT 1`

	var item Organization
	err := r.db.QueryRowContext(context.Background(), query).Scan(
		&item.ID,
		&item.Key,
		&item.Name,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Organization{}
	}
	return item
}

func (r *PostgresRepository) Locations() []Location {
	const query = `
		SELECT location_id, organization_id, location_key, name, location_type, status, COALESCE(parent_location_id, ''), created_at, updated_at
		FROM locations
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	locations := make([]Location, 0)
	for rows.Next() {
		var item Location
		if err := rows.Scan(
			&item.ID,
			&item.OrganizationID,
			&item.Key,
			&item.Name,
			&item.Type,
			&item.Status,
			&item.ParentID,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			continue
		}
		locations = append(locations, item)
	}
	return locations
}

func (r *PostgresRepository) OperatingUnits() []OperatingUnit {
	const query = `
		SELECT operating_unit_id, organization_id, COALESCE(location_id, ''), operating_unit_key, name, status, created_at, updated_at
		FROM operating_units
		ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]OperatingUnit, 0)
	for rows.Next() {
		var item OperatingUnit
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.LocationID, &item.Key, &item.Name, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveOperatingUnit(unit OperatingUnit) error {
	const query = `
		INSERT INTO operating_units (
			operating_unit_id, organization_id, location_id, operating_unit_key, name, status, created_at, updated_at
		) VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8)
		ON CONFLICT (operating_unit_id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			location_id = EXCLUDED.location_id,
			operating_unit_key = EXCLUDED.operating_unit_key,
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, unit.ID, unit.OrganizationID, unit.LocationID, unit.Key, unit.Name, unit.Status, unit.CreatedAt, unit.UpdatedAt)
	return err
}
