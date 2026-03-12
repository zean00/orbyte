package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) SaveSystem(system ExternalSystem) error {
	settings, _ := json.Marshal(system.Settings)
	const query = `
		INSERT INTO integration_systems (
			system_key, name, status, adapter_key, description, settings_json, created_at, updated_at
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
		ON CONFLICT (system_key) DO UPDATE SET
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			adapter_key = EXCLUDED.adapter_key,
			description = EXCLUDED.description,
			settings_json = EXCLUDED.settings_json,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, system.Key, system.Name, system.Status, system.Adapter, system.Description, settings, system.CreatedAt, system.UpdatedAt)
	return err
}

func (r *PostgresRepository) ListSystems() []ExternalSystem {
	const query = `SELECT system_key, name, status, adapter_key, COALESCE(description,''), settings_json, created_at, updated_at FROM integration_systems ORDER BY system_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ExternalSystem, 0)
	for rows.Next() {
		var item ExternalSystem
		var settings []byte
		if err := rows.Scan(&item.Key, &item.Name, &item.Status, &item.Adapter, &item.Description, &settings, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(settings, &item.Settings)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) GetSystem(key string) (ExternalSystem, bool) {
	const query = `SELECT system_key, name, status, adapter_key, COALESCE(description,''), settings_json, created_at, updated_at FROM integration_systems WHERE system_key = $1`
	var item ExternalSystem
	var settings []byte
	if err := r.db.QueryRowContext(context.Background(), query, key).Scan(&item.Key, &item.Name, &item.Status, &item.Adapter, &item.Description, &settings, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return ExternalSystem{}, false
	}
	_ = json.Unmarshal(settings, &item.Settings)
	return item, true
}

func (r *PostgresRepository) SaveSubmission(record SubmissionRecord) error {
	payload, _ := json.Marshal(record.Payload)
	result, _ := json.Marshal(record.Result)
	const query = `
		INSERT INTO integration_submissions (
			submission_id, external_system_key, operation_type, status, document_id, correlation_id,
			external_reference, attempt_count, last_error, payload_json, result_json, processed_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),$8,NULLIF($9,''),$10,$11,$12,$13,$14)
		ON CONFLICT (submission_id) DO UPDATE SET
			status = EXCLUDED.status,
			document_id = EXCLUDED.document_id,
			correlation_id = EXCLUDED.correlation_id,
			external_reference = EXCLUDED.external_reference,
			attempt_count = EXCLUDED.attempt_count,
			last_error = EXCLUDED.last_error,
			payload_json = EXCLUDED.payload_json,
			result_json = EXCLUDED.result_json,
			processed_at = EXCLUDED.processed_at,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query,
		record.ID, record.ExternalSystemKey, record.OperationType, record.Status, record.DocumentID, record.CorrelationID,
		record.ExternalReference, record.AttemptCount, record.LastError, payload, result, nullableTime(record.ProcessedAt), record.CreatedAt, record.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) ListSubmissions() []SubmissionRecord {
	const query = `SELECT submission_id, external_system_key, operation_type, status, COALESCE(document_id,''), COALESCE(correlation_id,''), COALESCE(external_reference,''), attempt_count, COALESCE(last_error,''), payload_json, result_json, processed_at, created_at, updated_at FROM integration_submissions`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]SubmissionRecord, 0)
	for rows.Next() {
		var item SubmissionRecord
		var payload []byte
		var result []byte
		var processed sql.NullTime
		if err := rows.Scan(&item.ID, &item.ExternalSystemKey, &item.OperationType, &item.Status, &item.DocumentID, &item.CorrelationID, &item.ExternalReference, &item.AttemptCount, &item.LastError, &payload, &result, &processed, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		if processed.Valid {
			item.ProcessedAt = processed.Time
		}
		_ = json.Unmarshal(payload, &item.Payload)
		_ = json.Unmarshal(result, &item.Result)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (r *PostgresRepository) GetSubmission(id string) (SubmissionRecord, bool) {
	const query = `SELECT submission_id, external_system_key, operation_type, status, COALESCE(document_id,''), COALESCE(correlation_id,''), COALESCE(external_reference,''), attempt_count, COALESCE(last_error,''), payload_json, result_json, processed_at, created_at, updated_at FROM integration_submissions WHERE submission_id = $1`
	var item SubmissionRecord
	var payload []byte
	var result []byte
	var processed sql.NullTime
	if err := r.db.QueryRowContext(context.Background(), query, id).Scan(&item.ID, &item.ExternalSystemKey, &item.OperationType, &item.Status, &item.DocumentID, &item.CorrelationID, &item.ExternalReference, &item.AttemptCount, &item.LastError, &payload, &result, &processed, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return SubmissionRecord{}, false
	}
	if processed.Valid {
		item.ProcessedAt = processed.Time
	}
	_ = json.Unmarshal(payload, &item.Payload)
	_ = json.Unmarshal(result, &item.Result)
	return item, true
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
