package document

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"

	"orbyte/internal/platform/shared"
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

func (r *PostgresRepository) SaveDefinition(def Definition) error {
	const query = `
		INSERT INTO document_definitions (
			document_type, display_name, schema_version, workflow_key, numbering_key, owner_module_key, allowed_link_types_json, allowed_attachment_types_json
		) VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7, $8)`

	allowedLinkTypes, err := json.Marshal(def.AllowedLinkTypes)
	if err != nil {
		return shared.Validation("invalid allowed link types")
	}
	allowedAttachmentTypes, err := json.Marshal(def.AllowedAttachmentTypes)
	if err != nil {
		return shared.Validation("invalid allowed attachment types")
	}

	_, err = r.db.ExecContext(context.Background(), query,
		def.Type,
		def.DisplayName,
		def.SchemaVersion,
		def.WorkflowKey,
		def.NumberingKey,
		def.OwnerModuleKey,
		allowedLinkTypes,
		allowedAttachmentTypes,
	)
	if err != nil {
		return shared.Conflict("document definition already exists")
	}
	return nil
}

func (r *PostgresRepository) GetDefinition(documentType string) (Definition, bool) {
	const query = `
		SELECT document_type, display_name, schema_version, COALESCE(workflow_key, ''), COALESCE(numbering_key, ''), COALESCE(owner_module_key, ''),
		       COALESCE(allowed_link_types_json, '[]'::jsonb), COALESCE(allowed_attachment_types_json, '[]'::jsonb)
		FROM document_definitions
		WHERE document_type = $1`

	var def Definition
	var allowedLinkTypesJSON []byte
	var allowedAttachmentTypesJSON []byte
	err := r.db.QueryRowContext(context.Background(), query, documentType).Scan(
		&def.Type,
		&def.DisplayName,
		&def.SchemaVersion,
		&def.WorkflowKey,
		&def.NumberingKey,
		&def.OwnerModuleKey,
		&allowedLinkTypesJSON,
		&allowedAttachmentTypesJSON,
	)
	if err != nil {
		return Definition{}, false
	}
	_ = json.Unmarshal(allowedLinkTypesJSON, &def.AllowedLinkTypes)
	_ = json.Unmarshal(allowedAttachmentTypesJSON, &def.AllowedAttachmentTypes)
	return def, true
}

func (r *PostgresRepository) ListDefinitions() []Definition {
	const query = `
		SELECT document_type, display_name, schema_version, COALESCE(workflow_key, ''), COALESCE(numbering_key, ''), COALESCE(owner_module_key, ''),
		       COALESCE(allowed_link_types_json, '[]'::jsonb), COALESCE(allowed_attachment_types_json, '[]'::jsonb)
		FROM document_definitions
		ORDER BY document_type ASC`

	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	defs := make([]Definition, 0)
	for rows.Next() {
		var def Definition
		var allowedLinkTypesJSON []byte
		var allowedAttachmentTypesJSON []byte
		if err := rows.Scan(&def.Type, &def.DisplayName, &def.SchemaVersion, &def.WorkflowKey, &def.NumberingKey, &def.OwnerModuleKey, &allowedLinkTypesJSON, &allowedAttachmentTypesJSON); err != nil {
			continue
		}
		_ = json.Unmarshal(allowedLinkTypesJSON, &def.AllowedLinkTypes)
		_ = json.Unmarshal(allowedAttachmentTypesJSON, &def.AllowedAttachmentTypes)
		defs = append(defs, def)
	}
	return defs
}

func (r *PostgresRepository) DeleteRecord(documentID string) error {
	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, query := range []string{
		`DELETE FROM document_lines WHERE document_id = $1`,
		`DELETE FROM document_links WHERE document_id = $1`,
		`DELETE FROM document_attachments WHERE document_id = $1`,
		`DELETE FROM document_records WHERE document_id = $1`,
	} {
		if _, err := tx.ExecContext(context.Background(), query, documentID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *PostgresRepository) SaveExtensionDefinition(def ExtensionDefinition) error {
	const query = `
		INSERT INTO document_extension_definitions (
			document_type, module_key, display_name, schema_version, read_permission_key, write_permission_key
		) VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))`
	_, err := r.db.ExecContext(context.Background(), query, def.DocumentType, def.ModuleKey, def.DisplayName, def.SchemaVersion, def.ReadPermissionKey, def.WritePermissionKey)
	if err != nil {
		return shared.Conflict("document extension definition already exists")
	}
	return nil
}

func (r *PostgresRepository) ListExtensionDefinitions(documentType string) []ExtensionDefinition {
	const query = `
		SELECT document_type, module_key, display_name, schema_version, COALESCE(read_permission_key, ''), COALESCE(write_permission_key, '')
		FROM document_extension_definitions
		WHERE document_type = $1
		ORDER BY module_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query, documentType)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ExtensionDefinition, 0)
	for rows.Next() {
		var item ExtensionDefinition
		if err := rows.Scan(&item.DocumentType, &item.ModuleKey, &item.DisplayName, &item.SchemaVersion, &item.ReadPermissionKey, &item.WritePermissionKey); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveRecord(record Record) error {
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

	_, err = r.db.ExecContext(context.Background(), query,
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

func (r *PostgresRepository) GetRecord(documentID string) (Record, bool) {
	const query = `
		SELECT document_id, document_type, status, version, etag, organization_id,
			COALESCE(location_id, ''), COALESCE(number, ''), created_by, created_at,
			updated_by, updated_at, COALESCE(submitted_by, ''), submitted_at,
			schema_version, payload_json, COALESCE(content_hash, ''), total_amount_minor,
			COALESCE(total_amount_currency, ''), COALESCE(metadata_json, '{}'::jsonb)
		FROM document_records
		WHERE document_id = $1`

	var (
		record           Record
		payload          []byte
		metadata         []byte
		submittedAt      sql.NullTime
		totalAmountMinor int64
	)

	err := r.db.QueryRowContext(context.Background(), query, documentID).Scan(
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
	if err != nil {
		return Record{}, false
	}
	if submittedAt.Valid {
		record.Header.SubmittedAt = submittedAt.Time
	}
	record.Header.TotalAmount.AmountMinor = totalAmountMinor
	record.Body.DocumentID = record.Header.ID
	if err := json.Unmarshal(payload, &record.Body.Payload); err != nil {
		return Record{}, false
	}
	_ = json.Unmarshal(metadata, &record.Header.Metadata)
	record.Lines = r.ListLines(documentID)
	record.Links = r.ListLinks(documentID)
	record.Attachments = r.ListAttachments(documentID)
	return record, true
}

func (r *PostgresRepository) ListRecords() []Record {
	const query = `
		SELECT document_id, document_type, status, version, etag, organization_id,
			COALESCE(location_id, ''), COALESCE(number, ''), created_by, created_at,
			updated_by, updated_at, COALESCE(submitted_by, ''), submitted_at,
			schema_version, payload_json, COALESCE(content_hash, ''), total_amount_minor,
			COALESCE(total_amount_currency, ''), COALESCE(metadata_json, '{}'::jsonb)
		FROM document_records`

	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	records := make([]Record, 0)
	for rows.Next() {
		var (
			record           Record
			payload          []byte
			metadata         []byte
			submittedAt      sql.NullTime
			totalAmountMinor int64
		)
		if err := rows.Scan(
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
		); err != nil {
			continue
		}
		if submittedAt.Valid {
			record.Header.SubmittedAt = submittedAt.Time
		}
		record.Header.TotalAmount.AmountMinor = totalAmountMinor
		record.Body.DocumentID = record.Header.ID
		if err := json.Unmarshal(payload, &record.Body.Payload); err != nil {
			continue
		}
		_ = json.Unmarshal(metadata, &record.Header.Metadata)
		record.Lines = r.ListLines(record.Header.ID)
		record.Links = r.ListLinks(record.Header.ID)
		record.Attachments = r.ListAttachments(record.Header.ID)
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Header.CreatedAt.Before(records[j].Header.CreatedAt)
	})
	return records
}

func nullableTime(t interface{ IsZero() bool }) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func (r *PostgresRepository) SaveLines(documentID string, lines []Line) error {
	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(context.Background(), `DELETE FROM document_lines WHERE document_id = $1`, documentID); err != nil {
		return err
	}
	for _, line := range lines {
		payload, err := json.Marshal(line.Payload)
		if err != nil {
			return shared.Validation("invalid document line payload")
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO document_lines (
				document_line_id, document_id, line_no, line_type, line_schema_ref, payload_json, amount_minor, amount_currency
			) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8)`,
			line.ID, documentID, line.LineNo, line.LineType, line.SchemaRef, payload, line.Amount.AmountMinor, line.Amount.Currency,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *PostgresRepository) ListLines(documentID string) []Line {
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT document_line_id, document_id, line_no, line_type, COALESCE(line_schema_ref, ''), payload_json, amount_minor, COALESCE(amount_currency, '')
		FROM document_lines
		WHERE document_id = $1
		ORDER BY line_no ASC`, documentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Line, 0)
	for rows.Next() {
		var item Line
		var payload []byte
		if err := rows.Scan(&item.ID, &item.DocumentID, &item.LineNo, &item.LineType, &item.SchemaRef, &payload, &item.Amount.AmountMinor, &item.Amount.Currency); err != nil {
			continue
		}
		_ = json.Unmarshal(payload, &item.Payload)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveLinks(documentID string, links []Link) error {
	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(context.Background(), `DELETE FROM document_links WHERE document_id = $1`, documentID); err != nil {
		return err
	}
	for _, link := range links {
		metadata, err := json.Marshal(link.Metadata)
		if err != nil {
			return shared.Validation("invalid document link metadata")
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO document_links (
				link_id, document_id, linked_document_id, link_type, metadata_json, created_at
			) VALUES ($1, $2, $3, $4, $5, $6)`,
			link.ID, documentID, link.LinkedDocumentID, link.LinkType, metadata, link.CreatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *PostgresRepository) ListLinks(documentID string) []Link {
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT link_id, document_id, linked_document_id, link_type, metadata_json, created_at
		FROM document_links
		WHERE document_id = $1
		ORDER BY created_at ASC`, documentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Link, 0)
	for rows.Next() {
		var item Link
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.DocumentID, &item.LinkedDocumentID, &item.LinkType, &metadata, &item.CreatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(metadata, &item.Metadata)
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) SaveAttachments(documentID string, attachments []Attachment) error {
	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(context.Background(), `DELETE FROM document_attachments WHERE document_id = $1`, documentID); err != nil {
		return err
	}
	for _, attachment := range attachments {
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO document_attachments (
				attachment_id, document_id, attachment_type, file_name, content_type, storage_key, size_bytes, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			attachment.ID, documentID, attachment.AttachmentType, attachment.FileName, attachment.ContentType, attachment.StorageKey, attachment.SizeBytes, attachment.CreatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *PostgresRepository) ListAttachments(documentID string) []Attachment {
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT attachment_id, document_id, attachment_type, file_name, content_type, storage_key, size_bytes, created_at
		FROM document_attachments
		WHERE document_id = $1
		ORDER BY created_at ASC`, documentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Attachment, 0)
	for rows.Next() {
		var item Attachment
		if err := rows.Scan(&item.ID, &item.DocumentID, &item.AttachmentType, &item.FileName, &item.ContentType, &item.StorageKey, &item.SizeBytes, &item.CreatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}
