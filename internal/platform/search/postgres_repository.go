package search

import (
	"context"
	"database/sql"
	"sort"
	"time"
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

func (r *PostgresRepository) SaveProjectionStatus(status ProjectionStatus) error {
	const query = `
		INSERT INTO search_projection_status (
			projection_key, last_refresh_status, last_success_at, last_failure_at, last_error,
			last_rebuild_started_at, last_rebuild_finished_at, last_rebuild_count,
			source_count, projection_count, stale_count, missing_count
		) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (projection_key) DO UPDATE SET
			last_refresh_status = EXCLUDED.last_refresh_status,
			last_success_at = EXCLUDED.last_success_at,
			last_failure_at = EXCLUDED.last_failure_at,
			last_error = EXCLUDED.last_error,
			last_rebuild_started_at = EXCLUDED.last_rebuild_started_at,
			last_rebuild_finished_at = EXCLUDED.last_rebuild_finished_at,
			last_rebuild_count = EXCLUDED.last_rebuild_count,
			source_count = EXCLUDED.source_count,
			projection_count = EXCLUDED.projection_count,
			stale_count = EXCLUDED.stale_count,
			missing_count = EXCLUDED.missing_count`
	_, err := r.db.ExecContext(context.Background(), query,
		status.ProjectionKey, status.LastRefreshStatus, nullableTime(status.LastSuccessAt), nullableTime(status.LastFailureAt), status.LastError,
		nullableTime(status.LastRebuildStartedAt), nullableTime(status.LastRebuildFinishedAt), status.LastRebuildCount,
		status.SourceCount, status.ProjectionCount, status.StaleCount, status.MissingCount,
	)
	return err
}

func (r *PostgresRepository) ListProjectionStatuses() []ProjectionStatus {
	const query = `
		SELECT projection_key, last_refresh_status, last_success_at, last_failure_at, COALESCE(last_error,''),
		       last_rebuild_started_at, last_rebuild_finished_at, last_rebuild_count,
		       source_count, projection_count, stale_count, missing_count
		FROM search_projection_status
		ORDER BY projection_key ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ProjectionStatus, 0)
	for rows.Next() {
		var item ProjectionStatus
		var lastSuccess, lastFailure, rebuildStarted, rebuildFinished sql.NullTime
		if err := rows.Scan(&item.ProjectionKey, &item.LastRefreshStatus, &lastSuccess, &lastFailure, &item.LastError, &rebuildStarted, &rebuildFinished, &item.LastRebuildCount, &item.SourceCount, &item.ProjectionCount, &item.StaleCount, &item.MissingCount); err != nil {
			continue
		}
		if lastSuccess.Valid {
			item.LastSuccessAt = lastSuccess.Time
		}
		if lastFailure.Valid {
			item.LastFailureAt = lastFailure.Time
		}
		if rebuildStarted.Valid {
			item.LastRebuildStartedAt = rebuildStarted.Time
		}
		if rebuildFinished.Valid {
			item.LastRebuildFinishedAt = rebuildFinished.Time
		}
		items = append(items, item)
	}
	return items
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
