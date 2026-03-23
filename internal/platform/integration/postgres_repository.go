package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
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

func (r *PostgresRepository) SaveSystem(system ExternalSystem) error {
	settings, _ := json.Marshal(system.Settings)
	const query = `
		INSERT INTO integration_systems (
			system_key, name, status, adapter_key, connector_key, description, settings_json, created_at, updated_at
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9)
		ON CONFLICT (system_key) DO UPDATE SET
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			adapter_key = EXCLUDED.adapter_key,
			connector_key = EXCLUDED.connector_key,
			description = EXCLUDED.description,
			settings_json = EXCLUDED.settings_json,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, system.Key, system.Name, system.Status, system.Adapter, system.Connector, system.Description, settings, system.CreatedAt, system.UpdatedAt)
	return err
}

func (r *PostgresRepository) ListSystems() []ExternalSystem {
	const query = `SELECT system_key, name, status, adapter_key, COALESCE(connector_key,''), COALESCE(description,''), settings_json, created_at, updated_at FROM integration_systems ORDER BY system_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ExternalSystem, 0)
	for rows.Next() {
		var item ExternalSystem
		var settings []byte
		if err := rows.Scan(&item.Key, &item.Name, &item.Status, &item.Adapter, &item.Connector, &item.Description, &settings, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(settings, &item.Settings)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) GetSystem(key string) (ExternalSystem, bool) {
	const query = `SELECT system_key, name, status, adapter_key, COALESCE(connector_key,''), COALESCE(description,''), settings_json, created_at, updated_at FROM integration_systems WHERE system_key = $1`
	var item ExternalSystem
	var settings []byte
	if err := r.db.QueryRowContext(context.Background(), query, key).Scan(&item.Key, &item.Name, &item.Status, &item.Adapter, &item.Connector, &item.Description, &settings, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return ExternalSystem{}, false
	}
	_ = json.Unmarshal(settings, &item.Settings)
	return item, true
}

func (r *PostgresRepository) SaveEndpoint(endpoint Endpoint) error {
	settings, _ := json.Marshal(endpoint.Settings)
	const query = `
		INSERT INTO integration_endpoints (
			endpoint_key, system_key, name, direction, mode, status, connector_key, description, settings_json, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11)
		ON CONFLICT (endpoint_key) DO UPDATE SET
			system_key = EXCLUDED.system_key,
			name = EXCLUDED.name,
			direction = EXCLUDED.direction,
			mode = EXCLUDED.mode,
			status = EXCLUDED.status,
			connector_key = EXCLUDED.connector_key,
			description = EXCLUDED.description,
			settings_json = EXCLUDED.settings_json,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, endpoint.Key, endpoint.SystemKey, endpoint.Name, endpoint.Direction, endpoint.Mode, endpoint.Status, endpoint.Connector, endpoint.Description, settings, endpoint.CreatedAt, endpoint.UpdatedAt)
	return err
}

func (r *PostgresRepository) ListEndpoints() []Endpoint {
	const query = `SELECT endpoint_key, system_key, name, direction, mode, status, connector_key, COALESCE(description,''), settings_json, created_at, updated_at FROM integration_endpoints ORDER BY endpoint_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Endpoint, 0)
	for rows.Next() {
		var item Endpoint
		var settings []byte
		if err := rows.Scan(&item.Key, &item.SystemKey, &item.Name, &item.Direction, &item.Mode, &item.Status, &item.Connector, &item.Description, &settings, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(settings, &item.Settings)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) GetEndpoint(key string) (Endpoint, bool) {
	const query = `SELECT endpoint_key, system_key, name, direction, mode, status, connector_key, COALESCE(description,''), settings_json, created_at, updated_at FROM integration_endpoints WHERE endpoint_key = $1`
	var item Endpoint
	var settings []byte
	if err := r.db.QueryRowContext(context.Background(), query, key).Scan(&item.Key, &item.SystemKey, &item.Name, &item.Direction, &item.Mode, &item.Status, &item.Connector, &item.Description, &settings, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Endpoint{}, false
	}
	_ = json.Unmarshal(settings, &item.Settings)
	return item, true
}

func (r *PostgresRepository) SaveContract(contract Contract) error {
	schema, _ := json.Marshal(contract.Schema)
	const query = `
		INSERT INTO integration_contracts (
			contract_key, version, name, direction, intent, status, description, schema_ref, schema_json, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),$9,$10,$11)
		ON CONFLICT (contract_key, version) DO UPDATE SET
			name = EXCLUDED.name,
			direction = EXCLUDED.direction,
			intent = EXCLUDED.intent,
			status = EXCLUDED.status,
			description = EXCLUDED.description,
			schema_ref = EXCLUDED.schema_ref,
			schema_json = EXCLUDED.schema_json,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, contract.Key, contract.Version, contract.Name, contract.Direction, contract.Intent, contract.Status, contract.Description, contract.SchemaRef, schema, contract.CreatedAt, contract.UpdatedAt)
	return err
}

func (r *PostgresRepository) ListContracts() []Contract {
	const query = `SELECT contract_key, version, name, direction, intent, status, COALESCE(description,''), COALESCE(schema_ref,''), schema_json, created_at, updated_at FROM integration_contracts ORDER BY contract_key ASC, version ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Contract, 0)
	for rows.Next() {
		var item Contract
		var schema []byte
		if err := rows.Scan(&item.Key, &item.Version, &item.Name, &item.Direction, &item.Intent, &item.Status, &item.Description, &item.SchemaRef, &schema, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(schema, &item.Schema)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) GetContract(key string, version int) (Contract, bool) {
	const query = `SELECT contract_key, version, name, direction, intent, status, COALESCE(description,''), COALESCE(schema_ref,''), schema_json, created_at, updated_at FROM integration_contracts WHERE contract_key = $1 AND version = $2`
	var item Contract
	var schema []byte
	if err := r.db.QueryRowContext(context.Background(), query, key, version).Scan(&item.Key, &item.Version, &item.Name, &item.Direction, &item.Intent, &item.Status, &item.Description, &item.SchemaRef, &schema, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Contract{}, false
	}
	_ = json.Unmarshal(schema, &item.Schema)
	return item, true
}

func (r *PostgresRepository) SaveMapping(mapping Mapping) error {
	rule, _ := json.Marshal(mapping.Rule)
	const query = `
		INSERT INTO integration_mappings (
			mapping_key, system_key, endpoint_key, contract_key, contract_version, direction, status, rule_json, created_at, updated_at
		) VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (mapping_key) DO UPDATE SET
			system_key = EXCLUDED.system_key,
			endpoint_key = EXCLUDED.endpoint_key,
			contract_key = EXCLUDED.contract_key,
			contract_version = EXCLUDED.contract_version,
			direction = EXCLUDED.direction,
			status = EXCLUDED.status,
			rule_json = EXCLUDED.rule_json,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, mapping.Key, mapping.SystemKey, mapping.EndpointKey, mapping.ContractKey, mapping.ContractVersion, mapping.Direction, mapping.Status, rule, mapping.CreatedAt, mapping.UpdatedAt)
	return err
}

func (r *PostgresRepository) ListMappings() []Mapping {
	const query = `SELECT mapping_key, system_key, COALESCE(endpoint_key,''), contract_key, contract_version, direction, status, rule_json, created_at, updated_at FROM integration_mappings ORDER BY mapping_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Mapping, 0)
	for rows.Next() {
		var item Mapping
		var rule []byte
		if err := rows.Scan(&item.Key, &item.SystemKey, &item.EndpointKey, &item.ContractKey, &item.ContractVersion, &item.Direction, &item.Status, &rule, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(rule, &item.Rule)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveSubmission(record SubmissionRecord) error {
	payload, _ := json.Marshal(record.Payload)
	result, _ := json.Marshal(record.Result)
	const query = `
		INSERT INTO integration_submissions (
			submission_id, external_system_key, endpoint_key, contract_key, contract_version, intent, mode, idempotency_key,
			operation_type, status, document_id, correlation_id, external_reference, attempt_count, last_error,
			payload_json, result_json, processed_at, created_at, updated_at
		) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9,$10,NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),$14,NULLIF($15,''),$16,$17,$18,$19,$20)
		ON CONFLICT (submission_id) DO UPDATE SET
			endpoint_key = EXCLUDED.endpoint_key,
			contract_key = EXCLUDED.contract_key,
			contract_version = EXCLUDED.contract_version,
			intent = EXCLUDED.intent,
			mode = EXCLUDED.mode,
			idempotency_key = EXCLUDED.idempotency_key,
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
		record.ID, record.ExternalSystemKey, record.EndpointKey, record.ContractKey, record.ContractVersion, record.Intent, record.Mode, record.IdempotencyKey,
		record.OperationType, record.Status, record.DocumentID, record.CorrelationID, record.ExternalReference, record.AttemptCount, record.LastError, payload, result, nullableTime(record.ProcessedAt), record.CreatedAt, record.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) ListSubmissions() []SubmissionRecord {
	const query = `SELECT submission_id, external_system_key, COALESCE(endpoint_key,''), COALESCE(contract_key,''), contract_version, COALESCE(intent,''), COALESCE(mode,''), COALESCE(idempotency_key,''), operation_type, status, COALESCE(document_id,''), COALESCE(correlation_id,''), COALESCE(external_reference,''), attempt_count, COALESCE(last_error,''), payload_json, result_json, processed_at, created_at, updated_at FROM integration_submissions`
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
		if err := rows.Scan(&item.ID, &item.ExternalSystemKey, &item.EndpointKey, &item.ContractKey, &item.ContractVersion, &item.Intent, &item.Mode, &item.IdempotencyKey, &item.OperationType, &item.Status, &item.DocumentID, &item.CorrelationID, &item.ExternalReference, &item.AttemptCount, &item.LastError, &payload, &result, &processed, &item.CreatedAt, &item.UpdatedAt); err != nil {
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
	const query = `SELECT submission_id, external_system_key, COALESCE(endpoint_key,''), COALESCE(contract_key,''), contract_version, COALESCE(intent,''), COALESCE(mode,''), COALESCE(idempotency_key,''), operation_type, status, COALESCE(document_id,''), COALESCE(correlation_id,''), COALESCE(external_reference,''), attempt_count, COALESCE(last_error,''), payload_json, result_json, processed_at, created_at, updated_at FROM integration_submissions WHERE submission_id = $1`
	var item SubmissionRecord
	var payload []byte
	var result []byte
	var processed sql.NullTime
	if err := r.db.QueryRowContext(context.Background(), query, id).Scan(&item.ID, &item.ExternalSystemKey, &item.EndpointKey, &item.ContractKey, &item.ContractVersion, &item.Intent, &item.Mode, &item.IdempotencyKey, &item.OperationType, &item.Status, &item.DocumentID, &item.CorrelationID, &item.ExternalReference, &item.AttemptCount, &item.LastError, &payload, &result, &processed, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return SubmissionRecord{}, false
	}
	if processed.Valid {
		item.ProcessedAt = processed.Time
	}
	_ = json.Unmarshal(payload, &item.Payload)
	_ = json.Unmarshal(result, &item.Result)
	return item, true
}

func (r *PostgresRepository) FindSubmissionByIdempotency(externalSystemKey, endpointKey, contractKey, idempotencyKey string) (SubmissionRecord, bool) {
	const query = `SELECT submission_id, external_system_key, COALESCE(endpoint_key,''), COALESCE(contract_key,''), contract_version, COALESCE(intent,''), COALESCE(mode,''), COALESCE(idempotency_key,''), operation_type, status, COALESCE(document_id,''), COALESCE(correlation_id,''), COALESCE(external_reference,''), attempt_count, COALESCE(last_error,''), payload_json, result_json, processed_at, created_at, updated_at FROM integration_submissions WHERE external_system_key = $1 AND COALESCE(endpoint_key,'') = $2 AND COALESCE(contract_key,'') = $3 AND COALESCE(idempotency_key,'') = $4 ORDER BY created_at ASC LIMIT 1`
	var item SubmissionRecord
	var payload []byte
	var result []byte
	var processed sql.NullTime
	if err := r.db.QueryRowContext(context.Background(), query, externalSystemKey, endpointKey, contractKey, idempotencyKey).Scan(&item.ID, &item.ExternalSystemKey, &item.EndpointKey, &item.ContractKey, &item.ContractVersion, &item.Intent, &item.Mode, &item.IdempotencyKey, &item.OperationType, &item.Status, &item.DocumentID, &item.CorrelationID, &item.ExternalReference, &item.AttemptCount, &item.LastError, &payload, &result, &processed, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return SubmissionRecord{}, false
	}
	if processed.Valid {
		item.ProcessedAt = processed.Time
	}
	_ = json.Unmarshal(payload, &item.Payload)
	_ = json.Unmarshal(result, &item.Result)
	return item, true
}

func (r *PostgresRepository) SaveSubmissionAttempt(attempt SubmissionAttempt) error {
	request, _ := json.Marshal(attempt.Request)
	response, _ := json.Marshal(attempt.Response)
	const query = `
		INSERT INTO integration_submission_attempts (
			attempt_id, submission_id, attempt_number, status, error_code, error_message, request_json, response_json, occurred_at
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9)
		ON CONFLICT (attempt_id) DO UPDATE SET
			status = EXCLUDED.status,
			error_code = EXCLUDED.error_code,
			error_message = EXCLUDED.error_message,
			request_json = EXCLUDED.request_json,
			response_json = EXCLUDED.response_json,
			occurred_at = EXCLUDED.occurred_at`
	_, err := r.db.ExecContext(context.Background(), query, attempt.ID, attempt.SubmissionID, attempt.Attempt, attempt.Status, attempt.ErrorCode, attempt.ErrorMessage, request, response, attempt.OccurredAt)
	return err
}

func (r *PostgresRepository) ListSubmissionAttempts(submissionID string) []SubmissionAttempt {
	const query = `SELECT attempt_id, submission_id, attempt_number, status, COALESCE(error_code,''), COALESCE(error_message,''), request_json, response_json, occurred_at FROM integration_submission_attempts WHERE submission_id = $1 ORDER BY attempt_number ASC, attempt_id ASC`
	rows, err := r.db.QueryContext(context.Background(), query, submissionID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]SubmissionAttempt, 0)
	for rows.Next() {
		var item SubmissionAttempt
		var request []byte
		var response []byte
		if err := rows.Scan(&item.ID, &item.SubmissionID, &item.Attempt, &item.Status, &item.ErrorCode, &item.ErrorMessage, &request, &response, &item.OccurredAt); err != nil {
			continue
		}
		_ = json.Unmarshal(request, &item.Request)
		_ = json.Unmarshal(response, &item.Response)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveDeadLetter(record DeadLetterRecord) error {
	payload, _ := json.Marshal(record.Payload)
	const query = `
		INSERT INTO integration_dead_letters (
			dead_letter_id, submission_id, external_system_key, endpoint_key, contract_key, contract_version, intent, status, last_error, payload_json, created_at, updated_at
		) VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,NULLIF($7,''),$8,NULLIF($9,''),$10,$11,$12)
		ON CONFLICT (dead_letter_id) DO UPDATE SET
			status = EXCLUDED.status,
			last_error = EXCLUDED.last_error,
			payload_json = EXCLUDED.payload_json,
			updated_at = EXCLUDED.updated_at`
	_, err := r.db.ExecContext(context.Background(), query, record.ID, record.SubmissionID, record.ExternalSystemKey, record.EndpointKey, record.ContractKey, record.ContractVersion, record.Intent, record.Status, record.LastError, payload, record.CreatedAt, record.UpdatedAt)
	return err
}

func (r *PostgresRepository) ListDeadLetters() []DeadLetterRecord {
	const query = `SELECT dead_letter_id, submission_id, external_system_key, COALESCE(endpoint_key,''), COALESCE(contract_key,''), contract_version, COALESCE(intent,''), status, COALESCE(last_error,''), payload_json, created_at, updated_at FROM integration_dead_letters ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]DeadLetterRecord, 0)
	for rows.Next() {
		var item DeadLetterRecord
		var payload []byte
		if err := rows.Scan(&item.ID, &item.SubmissionID, &item.ExternalSystemKey, &item.EndpointKey, &item.ContractKey, &item.ContractVersion, &item.Intent, &item.Status, &item.LastError, &payload, &item.CreatedAt, &item.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(payload, &item.Payload)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) GetDeadLetter(id string) (DeadLetterRecord, bool) {
	const query = `SELECT dead_letter_id, submission_id, external_system_key, COALESCE(endpoint_key,''), COALESCE(contract_key,''), contract_version, COALESCE(intent,''), status, COALESCE(last_error,''), payload_json, created_at, updated_at FROM integration_dead_letters WHERE dead_letter_id = $1`
	var item DeadLetterRecord
	var payload []byte
	if err := r.db.QueryRowContext(context.Background(), query, id).Scan(&item.ID, &item.SubmissionID, &item.ExternalSystemKey, &item.EndpointKey, &item.ContractKey, &item.ContractVersion, &item.Intent, &item.Status, &item.LastError, &payload, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return DeadLetterRecord{}, false
	}
	_ = json.Unmarshal(payload, &item.Payload)
	return item, true
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
