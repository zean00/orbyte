package application

import (
	"context"
	"database/sql"
	"encoding/json"

	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/store"
	"orbyte/internal/platform/workflow"
)

type UnitOfWork interface {
	GetDocument(documentID string) (document.Record, error)
	CreateDocument(record document.Record) error
	UpdateDocument(previousVersion int, record document.Record) error
	GetModelRecord(modelKey, recordID string) (model.Record, error)
	ListModelRecords(modelKey string) ([]model.Record, error)
	CreateModelRecord(record model.Record) error
	UpdateModelRecord(expectedVersion int, record model.Record) error
	DeleteModelRecord(modelKey, recordID string) error
	GetModelDefinition(modelKey string) (model.Definition, error)
	SaveAudit(event audit.Event) error
	SaveDomainEvent(event eventing.Event) error
	SaveOutbox(record eventing.OutboxRecord) error
	ApplyWorkflowMutation(mutation workflow.Mutation) error
}

type TransactionManager interface {
	WithinTx(ctx context.Context, fn func(UnitOfWork) error) error
}

type MemoryTransactionManager struct {
	documents *document.Service
	models    *model.Service
	audit     *audit.Service
	eventing  *eventing.Service
	workflows *workflow.Service
}

func NewMemoryTransactionManager(documents *document.Service, models *model.Service, workflows *workflow.Service, auditSvc *audit.Service, eventingSvc *eventing.Service) *MemoryTransactionManager {
	return &MemoryTransactionManager{documents: documents, models: models, audit: auditSvc, eventing: eventingSvc, workflows: workflows}
}

func (m *MemoryTransactionManager) WithinTx(_ context.Context, fn func(UnitOfWork) error) error {
	return fn(&memoryUnitOfWork{documents: m.documents, models: m.models, audit: m.audit, eventing: m.eventing, workflows: m.workflows})
}

type PostgresTransactionManager struct {
	txm store.TransactionManager
}

func NewPostgresTransactionManager(db *sql.DB) *PostgresTransactionManager {
	return NewPostgresTransactionManagerWithDB(store.UninstrumentedDB(db))
}

func NewPostgresTransactionManagerWithDB(db store.DB) *PostgresTransactionManager {
	return &PostgresTransactionManager{txm: store.NewPostgresTransactionManagerWithDB(db)}
}

func (m *PostgresTransactionManager) WithinTx(ctx context.Context, fn func(UnitOfWork) error) error {
	return m.txm.WithinTx(ctx, func(tx store.Tx) error {
		return fn(&postgresUnitOfWork{tx: tx})
	})
}

type memoryUnitOfWork struct {
	documents *document.Service
	models    *model.Service
	audit     *audit.Service
	eventing  *eventing.Service
	workflows *workflow.Service
}

func (u *memoryUnitOfWork) GetDocument(documentID string) (document.Record, error) {
	return u.documents.Get(documentID)
}

func (u *memoryUnitOfWork) CreateDocument(record document.Record) error {
	return u.documents.Save(record)
}

func (u *memoryUnitOfWork) UpdateDocument(previousVersion int, record document.Record) error {
	current, err := u.documents.Get(record.Header.ID)
	if err != nil {
		return err
	}
	if current.Header.Version != previousVersion {
		return shared.Conflict("document version mismatch")
	}
	return u.documents.Save(record)
}

func (u *memoryUnitOfWork) GetModelRecord(modelKey, recordID string) (model.Record, error) {
	return u.models.Get(modelKey, recordID)
}

func (u *memoryUnitOfWork) ListModelRecords(modelKey string) ([]model.Record, error) {
	items, _, err := u.models.List(modelKey, model.Query{Page: 1, PageSize: 1000})
	return items, err
}

func (u *memoryUnitOfWork) CreateModelRecord(record model.Record) error {
	return u.models.WithRepository(u.modelsRepository()).WithRawRecordSave(record)
}

func (u *memoryUnitOfWork) UpdateModelRecord(expectedVersion int, record model.Record) error {
	current, err := u.models.Get(record.ModelKey, record.ID)
	if err != nil {
		return err
	}
	if expectedVersion > 0 && current.Version != expectedVersion {
		return shared.Conflict("record version mismatch")
	}
	return u.models.WithRepository(u.modelsRepository()).WithRawRecordSave(record)
}

func (u *memoryUnitOfWork) DeleteModelRecord(modelKey, recordID string) error {
	return u.modelsRepository().DeleteRecord(modelKey, recordID)
}

func (u *memoryUnitOfWork) GetModelDefinition(modelKey string) (model.Definition, error) {
	def, ok := u.models.Definition(modelKey)
	if !ok {
		return model.Definition{}, shared.NotFound("model definition not found")
	}
	return def, nil
}

func (u *memoryUnitOfWork) SaveAudit(event audit.Event) error {
	return u.audit.Record(event)
}

func (u *memoryUnitOfWork) SaveDomainEvent(event eventing.Event) error {
	return u.eventing.Record(event)
}

func (u *memoryUnitOfWork) SaveOutbox(_ eventing.OutboxRecord) error {
	return nil
}

func (u *memoryUnitOfWork) modelsRepository() model.Repository {
	return u.models.Repository()
}

func (u *memoryUnitOfWork) ApplyWorkflowMutation(mutation workflow.Mutation) error {
	return u.workflows.ApplyMutation(mutation)
}

type postgresUnitOfWork struct {
	tx store.Tx
}

func (u *postgresUnitOfWork) GetDocument(documentID string) (document.Record, error) {
	record, ok, err := getDocumentRecordTx(u.tx, documentID)
	if err != nil {
		return document.Record{}, err
	}
	if !ok {
		return document.Record{}, shared.NotFound("document not found")
	}
	return record, nil
}

func (u *postgresUnitOfWork) CreateDocument(record document.Record) error {
	return saveDocumentRecordUpsertTx(u.tx, record)
}

func (u *postgresUnitOfWork) UpdateDocument(previousVersion int, record document.Record) error {
	return updateDocumentRecordTx(u.tx, previousVersion, record)
}

func (u *postgresUnitOfWork) GetModelRecord(modelKey, recordID string) (model.Record, error) {
	repo := model.NewTxRepositoryWithTx(u.tx)
	record, ok := repo.GetRecord(modelKey, recordID)
	if !ok {
		return model.Record{}, shared.NotFound("record not found")
	}
	return record, nil
}

func (u *postgresUnitOfWork) ListModelRecords(modelKey string) ([]model.Record, error) {
	return model.NewTxRepositoryWithTx(u.tx).ListRecords(modelKey), nil
}

func (u *postgresUnitOfWork) CreateModelRecord(record model.Record) error {
	return model.NewTxRepositoryWithTx(u.tx).SaveRecord(record)
}

func (u *postgresUnitOfWork) UpdateModelRecord(expectedVersion int, record model.Record) error {
	current, err := u.GetModelRecord(record.ModelKey, record.ID)
	if err != nil {
		return err
	}
	if expectedVersion > 0 && current.Version != expectedVersion {
		return shared.Conflict("record version mismatch")
	}
	return model.NewTxRepositoryWithTx(u.tx).SaveRecord(record)
}

func (u *postgresUnitOfWork) DeleteModelRecord(modelKey, recordID string) error {
	return model.NewTxRepositoryWithTx(u.tx).DeleteRecord(modelKey, recordID)
}

func (u *postgresUnitOfWork) GetModelDefinition(modelKey string) (model.Definition, error) {
	def, ok := model.NewTxRepositoryWithTx(u.tx).GetDefinition(modelKey)
	if !ok {
		return model.Definition{}, shared.NotFound("model definition not found")
	}
	return def, nil
}

func (u *postgresUnitOfWork) SaveAudit(event audit.Event) error {
	return saveAuditEventTx(u.tx, event)
}

func (u *postgresUnitOfWork) SaveDomainEvent(event eventing.Event) error {
	return saveDomainEventTx(u.tx, event)
}

func (u *postgresUnitOfWork) SaveOutbox(record eventing.OutboxRecord) error {
	return saveOutboxRecordTx(u.tx, record)
}

func (u *postgresUnitOfWork) ApplyWorkflowMutation(mutation workflow.Mutation) error {
	return applyWorkflowMutationTx(u.tx, mutation)
}

func getDocumentRecordTx(tx store.Tx, documentID string) (document.Record, bool, error) {
	const query = `
		SELECT document_id, document_type, status, version, etag, organization_id,
			COALESCE(location_id, ''), COALESCE(number, ''), created_by, created_at,
			updated_by, updated_at, COALESCE(submitted_by, ''), submitted_at,
			schema_version, payload_json, COALESCE(content_hash, ''), total_amount_minor,
			COALESCE(total_amount_currency, ''), COALESCE(metadata_json, '{}'::jsonb)
		FROM document_records
		WHERE document_id = $1`
	var (
		record           document.Record
		payload          []byte
		metadata         []byte
		submittedAt      sql.NullTime
		totalAmountMinor int64
	)
	err := tx.QueryRowContext(context.Background(), query, documentID).Scan(
		&record.Header.ID,
		&record.Header.Type,
		&record.Header.Status,
		&record.Header.Version,
		&record.Header.ETag,
		&record.Header.OrganizationID,
		&record.Header.LocationID,
		&record.Header.Number,
		&record.Header.CreatedBy,
		&record.Header.CreatedAt,
		&record.Header.UpdatedBy,
		&record.Header.UpdatedAt,
		&record.Header.SubmittedBy,
		&submittedAt,
		&record.Body.SchemaVersion,
		&payload,
		&record.Body.ContentHash,
		&totalAmountMinor,
		&record.Header.TotalAmount.Currency,
		&metadata,
	)
	if err == sql.ErrNoRows {
		return document.Record{}, false, nil
	}
	if err != nil {
		return document.Record{}, false, err
	}
	if submittedAt.Valid {
		record.Header.SubmittedAt = submittedAt.Time
	}
	record.Header.TotalAmount.AmountMinor = totalAmountMinor
	record.Body.DocumentID = record.Header.ID
	if err := json.Unmarshal(payload, &record.Body.Payload); err != nil {
		return document.Record{}, false, err
	}
	_ = json.Unmarshal(metadata, &record.Header.Metadata)
	record.Lines = listDocumentLinesTx(tx, documentID)
	record.Links = listDocumentLinksTx(tx, documentID)
	record.Attachments = listDocumentAttachmentsTx(tx, documentID)
	return record, true, nil
}

func saveDocumentRecordUpsertTx(tx store.Tx, record document.Record) error {
	payload, err := json.Marshal(record.Body.Payload)
	if err != nil {
		return shared.Validation("invalid document payload")
	}
	metadata, err := json.Marshal(record.Header.Metadata)
	if err != nil {
		return shared.Validation("invalid document metadata")
	}
	const query = `
		INSERT INTO document_records (
			document_id, document_type, status, version, etag, organization_id, location_id, number,
			created_by, created_at, updated_by, updated_at, submitted_by, submitted_at,
			schema_version, payload_json, content_hash, total_amount_minor, total_amount_currency, metadata_json
		) VALUES (
			$1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''),
			$9, $10, $11, $12, NULLIF($13, ''), $14,
			$15, $16, NULLIF($17, ''), $18, $19, $20
		)
		ON CONFLICT (document_id) DO UPDATE SET
			status = EXCLUDED.status,
			version = EXCLUDED.version,
			etag = EXCLUDED.etag,
			location_id = EXCLUDED.location_id,
			number = EXCLUDED.number,
			updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at,
			submitted_by = EXCLUDED.submitted_by,
			submitted_at = EXCLUDED.submitted_at,
			schema_version = EXCLUDED.schema_version,
			payload_json = EXCLUDED.payload_json,
			content_hash = EXCLUDED.content_hash,
			total_amount_minor = EXCLUDED.total_amount_minor,
			total_amount_currency = EXCLUDED.total_amount_currency,
			metadata_json = EXCLUDED.metadata_json`
	_, err = tx.ExecContext(context.Background(), query,
		record.Header.ID,
		record.Header.Type,
		record.Header.Status,
		record.Header.Version,
		record.Header.ETag,
		record.Header.OrganizationID,
		record.Header.LocationID,
		record.Header.Number,
		record.Header.CreatedBy,
		record.Header.CreatedAt,
		record.Header.UpdatedBy,
		record.Header.UpdatedAt,
		record.Header.SubmittedBy,
		nullableTime(record.Header.SubmittedAt),
		record.Body.SchemaVersion,
		payload,
		record.Body.ContentHash,
		record.Header.TotalAmount.AmountMinor,
		record.Header.TotalAmount.Currency,
		metadata,
	)
	return err
}

func updateDocumentRecordTx(tx store.Tx, previousVersion int, record document.Record) error {
	payload, err := json.Marshal(record.Body.Payload)
	if err != nil {
		return shared.Validation("invalid document payload")
	}
	metadata, err := json.Marshal(record.Header.Metadata)
	if err != nil {
		return shared.Validation("invalid document metadata")
	}
	const query = `
		UPDATE document_records
		SET status = $1,
			version = $2,
			etag = $3,
			location_id = NULLIF($4, ''),
			number = NULLIF($5, ''),
			updated_by = $6,
			updated_at = $7,
			submitted_by = NULLIF($8, ''),
			submitted_at = $9,
			schema_version = $10,
			payload_json = $11,
			content_hash = NULLIF($12, ''),
			total_amount_minor = $13,
			total_amount_currency = $14,
			metadata_json = $15
		WHERE document_id = $16 AND version = $17`
	result, err := tx.ExecContext(context.Background(), query,
		record.Header.Status,
		record.Header.Version,
		record.Header.ETag,
		record.Header.LocationID,
		record.Header.Number,
		record.Header.UpdatedBy,
		record.Header.UpdatedAt,
		record.Header.SubmittedBy,
		nullableTime(record.Header.SubmittedAt),
		record.Body.SchemaVersion,
		payload,
		record.Body.ContentHash,
		record.Header.TotalAmount.AmountMinor,
		record.Header.TotalAmount.Currency,
		metadata,
		record.Header.ID,
		previousVersion,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return shared.Conflict("document version mismatch")
	}
	return nil
}

func listDocumentLinesTx(tx store.Tx, documentID string) []document.Line {
	const query = `
		SELECT document_line_id, line_no, line_type, COALESCE(line_schema_ref, ''),
		       COALESCE(payload_json, '{}'::jsonb), amount_minor, COALESCE(amount_currency, '')
		FROM document_lines
		WHERE document_id = $1
		ORDER BY line_no ASC`
	rows, err := tx.QueryContext(context.Background(), query, documentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]document.Line, 0)
	for rows.Next() {
		var (
			item    document.Line
			payload []byte
		)
		if err := rows.Scan(&item.ID, &item.LineNo, &item.LineType, &item.SchemaRef, &payload, &item.Amount.AmountMinor, &item.Amount.Currency); err != nil {
			continue
		}
		item.DocumentID = documentID
		_ = json.Unmarshal(payload, &item.Payload)
		items = append(items, item)
	}
	return items
}

func listDocumentLinksTx(tx store.Tx, documentID string) []document.Link {
	const query = `
		SELECT link_id, linked_document_id, link_type, COALESCE(metadata_json, '{}'::jsonb), created_at
		FROM document_links
		WHERE document_id = $1
		ORDER BY created_at ASC`
	rows, err := tx.QueryContext(context.Background(), query, documentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]document.Link, 0)
	for rows.Next() {
		var (
			item     document.Link
			metadata []byte
		)
		if err := rows.Scan(&item.ID, &item.LinkedDocumentID, &item.LinkType, &metadata, &item.CreatedAt); err != nil {
			continue
		}
		item.DocumentID = documentID
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	return items
}

func listDocumentAttachmentsTx(tx store.Tx, documentID string) []document.Attachment {
	const query = `
		SELECT attachment_id, attachment_type, file_name, content_type, storage_key, size_bytes, created_at
		FROM document_attachments
		WHERE document_id = $1
		ORDER BY created_at ASC`
	rows, err := tx.QueryContext(context.Background(), query, documentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]document.Attachment, 0)
	for rows.Next() {
		var item document.Attachment
		if err := rows.Scan(&item.ID, &item.AttachmentType, &item.FileName, &item.ContentType, &item.StorageKey, &item.SizeBytes, &item.CreatedAt); err != nil {
			continue
		}
		item.DocumentID = documentID
		items = append(items, item)
	}
	return items
}
