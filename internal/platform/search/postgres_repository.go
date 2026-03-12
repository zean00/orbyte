package search

import (
	"context"
	"database/sql"
	"sort"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) SaveDocument(summary DocumentSummary) error {
	const query = `
		INSERT INTO search_document_summaries (
			document_id, document_type, status, version, etag, organization_id, location_id, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
		ON CONFLICT (document_id) DO UPDATE SET
			document_type = EXCLUDED.document_type,
			status = EXCLUDED.status,
			version = EXCLUDED.version,
			etag = EXCLUDED.etag,
			organization_id = EXCLUDED.organization_id,
			location_id = EXCLUDED.location_id,
			updated_at = EXCLUDED.updated_at
		WHERE EXCLUDED.version > search_document_summaries.version
		   OR (EXCLUDED.version = search_document_summaries.version AND EXCLUDED.updated_at > search_document_summaries.updated_at)`
	_, err := r.db.ExecContext(context.Background(), query,
		summary.DocumentID,
		summary.DocumentType,
		summary.Status,
		summary.Version,
		summary.ETag,
		summary.OrganizationID,
		summary.LocationID,
		summary.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) ListDocuments() []DocumentSummary {
	const query = `
		SELECT document_id, document_type, status, version, etag, organization_id, COALESCE(location_id, ''), updated_at
		FROM search_document_summaries`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]DocumentSummary, 0)
	for rows.Next() {
		var item DocumentSummary
		if err := rows.Scan(&item.DocumentID, &item.DocumentType, &item.Status, &item.Version, &item.ETag, &item.OrganizationID, &item.LocationID, &item.UpdatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.Before(items[j].UpdatedAt)
	})
	return items
}
