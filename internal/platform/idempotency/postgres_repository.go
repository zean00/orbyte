package idempotency

import (
	"context"
	"database/sql"
	"encoding/json"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Get(operation, key string) (Record, bool) {
	const query = `
		SELECT operation_key, idempotency_key, actor_id, request_hash, status, response_code,
		       response_json, COALESCE(error_message, ''), created_at, updated_at
		FROM idempotency_records
		WHERE operation_key = $1 AND idempotency_key = $2`
	var (
		record       Record
		responseJSON []byte
		actorID      sql.NullString
	)
	err := r.db.QueryRowContext(context.Background(), query, operation, key).Scan(
		&record.Operation,
		&record.Key,
		&actorID,
		&record.RequestHash,
		&record.Status,
		&record.ResponseCode,
		&responseJSON,
		&record.Error,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return Record{}, false
	}
	if actorID.Valid {
		record.ActorID = actorID.String
	}
	_ = json.Unmarshal(responseJSON, &record.Response)
	return record, true
}

func (r *PostgresRepository) List() []Record {
	const query = `
		SELECT operation_key, idempotency_key, actor_id, request_hash, status, response_code,
		       response_json, COALESCE(error_message, ''), created_at, updated_at
		FROM idempotency_records
		ORDER BY operation_key ASC, idempotency_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Record, 0)
	for rows.Next() {
		var (
			record       Record
			responseJSON []byte
			actorID      sql.NullString
		)
		if err := rows.Scan(&record.Operation, &record.Key, &actorID, &record.RequestHash, &record.Status, &record.ResponseCode, &responseJSON, &record.Error, &record.CreatedAt, &record.UpdatedAt); err != nil {
			continue
		}
		if actorID.Valid {
			record.ActorID = actorID.String
		}
		_ = json.Unmarshal(responseJSON, &record.Response)
		items = append(items, record)
	}
	return items
}

func (r *PostgresRepository) Save(record Record) error {
	responseJSON, err := json.Marshal(record.Response)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO idempotency_records (
			operation_key, idempotency_key, actor_id, request_hash, status, response_code,
			response_json, error_message, created_at, updated_at
		) VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7::jsonb,NULLIF($8,''),$9,$10)
		ON CONFLICT (operation_key, idempotency_key) DO UPDATE SET
			actor_id = EXCLUDED.actor_id,
			request_hash = EXCLUDED.request_hash,
			status = EXCLUDED.status,
			response_code = EXCLUDED.response_code,
			response_json = EXCLUDED.response_json,
			error_message = EXCLUDED.error_message,
			updated_at = EXCLUDED.updated_at`
	_, err = r.db.ExecContext(context.Background(), query,
		record.Operation, record.Key, record.ActorID, record.RequestHash, record.Status, record.ResponseCode,
		string(responseJSON), record.Error, record.CreatedAt, record.UpdatedAt,
	)
	return err
}
