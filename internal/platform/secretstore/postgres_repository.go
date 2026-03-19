package secretstore

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

func (r *PostgresRepository) Get(ref string) (Secret, bool) {
	const query = `SELECT secret_ref, name, secret_value, status, created_at, updated_at FROM secret_store WHERE secret_ref = $1`
	var secret Secret
	if err := r.db.QueryRowContext(context.Background(), query, ref).Scan(&secret.Ref, &secret.Name, &secret.Value, &secret.Status, &secret.CreatedAt, &secret.UpdatedAt); err != nil {
		return Secret{}, false
	}
	return secret, true
}

func (r *PostgresRepository) List() []Secret {
	const query = `SELECT secret_ref, name, '' AS secret_value, status, created_at, updated_at FROM secret_store ORDER BY secret_ref ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Secret, 0)
	for rows.Next() {
		var secret Secret
		if err := rows.Scan(&secret.Ref, &secret.Name, &secret.Value, &secret.Status, &secret.CreatedAt, &secret.UpdatedAt); err != nil {
			continue
		}
		items = append(items, secret)
	}
	return items
}

func (r *PostgresRepository) Save(secret Secret) error {
	const query = `
		INSERT INTO secret_store (secret_ref, name, secret_value, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (secret_ref) DO UPDATE SET
			name = EXCLUDED.name,
			secret_value = EXCLUDED.secret_value,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, secret.Ref, secret.Name, secret.Value, secret.Status, secret.CreatedAt, secret.UpdatedAt)
	return err
}
