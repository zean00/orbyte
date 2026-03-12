package organization

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
