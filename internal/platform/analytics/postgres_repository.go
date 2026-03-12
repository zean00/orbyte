package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) SaveDimensions(dimensions DimensionBundle) error {
	for _, item := range dimensions.DocumentTypes {
		const query = `INSERT INTO analytics_document_type_dim (document_type, display_name) VALUES ($1,$2) ON CONFLICT (document_type) DO UPDATE SET display_name = EXCLUDED.display_name`
		if _, err := r.db.ExecContext(context.Background(), query, item.DocumentType, item.DisplayName); err != nil {
			return err
		}
	}
	for _, item := range dimensions.Locations {
		const query = `INSERT INTO analytics_location_dim (location_id, display_name) VALUES ($1,$2) ON CONFLICT (location_id) DO UPDATE SET display_name = EXCLUDED.display_name`
		if _, err := r.db.ExecContext(context.Background(), query, item.LocationID, item.Name); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) SaveReportDefinition(def ReportDefinition) error {
	const query = `INSERT INTO analytics_report_definitions (report_id, name, dimension, format, window_key, location_id, document_type, delivery_channel, delivery_target, schedule_key, next_run_at, enabled) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,$11,$12)`
	_, err := r.db.ExecContext(context.Background(), query, def.ID, def.Name, def.Dimension, def.Format, def.Window, def.LocationID, def.DocumentType, def.DeliveryChannel, def.DeliveryTarget, def.Schedule, def.NextRunAt, def.Enabled)
	return err
}

func (r *PostgresRepository) ListReportDefinitions() []ReportDefinition {
	const query = `SELECT report_id, name, dimension, format, window_key, COALESCE(location_id,''), COALESCE(document_type,''), COALESCE(delivery_channel,''), COALESCE(delivery_target,''), schedule_key, next_run_at, enabled FROM analytics_report_definitions ORDER BY next_run_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ReportDefinition, 0)
	for rows.Next() {
		var item ReportDefinition
		if err := rows.Scan(&item.ID, &item.Name, &item.Dimension, &item.Format, &item.Window, &item.LocationID, &item.DocumentType, &item.DeliveryChannel, &item.DeliveryTarget, &item.Schedule, &item.NextRunAt, &item.Enabled); err == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) UpdateReportDefinition(def ReportDefinition) error {
	const query = `UPDATE analytics_report_definitions SET name=$1, dimension=$2, format=$3, window_key=$4, location_id=NULLIF($5,''), document_type=NULLIF($6,''), delivery_channel=NULLIF($7,''), delivery_target=NULLIF($8,''), schedule_key=$9, next_run_at=$10, enabled=$11 WHERE report_id=$12`
	_, err := r.db.ExecContext(context.Background(), query, def.Name, def.Dimension, def.Format, def.Window, def.LocationID, def.DocumentType, def.DeliveryChannel, def.DeliveryTarget, def.Schedule, def.NextRunAt, def.Enabled, def.ID)
	return err
}

func (r *PostgresRepository) SaveReportRun(run ReportRun) error {
	const query = `INSERT INTO analytics_report_runs (report_run_id, report_id, format, status, generated_at, row_count) VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := r.db.ExecContext(context.Background(), query, run.ID, run.ReportID, run.Format, run.Status, run.GeneratedAt, run.RowCount)
	return err
}

func (r *PostgresRepository) ListReportRuns() []ReportRun {
	const query = `SELECT report_run_id, report_id, format, status, generated_at, row_count FROM analytics_report_runs ORDER BY generated_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ReportRun, 0)
	for rows.Next() {
		var item ReportRun
		if err := rows.Scan(&item.ID, &item.ReportID, &item.Format, &item.Status, &item.GeneratedAt, &item.RowCount); err == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) SaveReportArtifact(artifact ReportArtifact) error {
	const query = `INSERT INTO analytics_report_artifacts (artifact_id, report_id, report_run_id, file_name, content_type, size_bytes, content_bytes, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := r.db.ExecContext(context.Background(), query, artifact.ID, artifact.ReportID, artifact.ReportRunID, artifact.FileName, artifact.ContentType, artifact.SizeBytes, artifact.Content, artifact.CreatedAt)
	return err
}

func (r *PostgresRepository) ListReportArtifacts() []ReportArtifact {
	const query = `SELECT artifact_id, report_id, report_run_id, file_name, content_type, size_bytes, created_at FROM analytics_report_artifacts ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ReportArtifact, 0)
	for rows.Next() {
		var item ReportArtifact
		if err := rows.Scan(&item.ID, &item.ReportID, &item.ReportRunID, &item.FileName, &item.ContentType, &item.SizeBytes, &item.CreatedAt); err == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) GetReportArtifact(id string) (ReportArtifact, bool) {
	const query = `SELECT artifact_id, report_id, report_run_id, file_name, content_type, size_bytes, content_bytes, created_at FROM analytics_report_artifacts WHERE artifact_id = $1`
	var item ReportArtifact
	err := r.db.QueryRowContext(context.Background(), query, id).Scan(&item.ID, &item.ReportID, &item.ReportRunID, &item.FileName, &item.ContentType, &item.SizeBytes, &item.Content, &item.CreatedAt)
	if err != nil {
		return ReportArtifact{}, false
	}
	return item, true
}

func (r *PostgresRepository) SaveReportDelivery(delivery ReportDelivery) error {
	const query = `INSERT INTO analytics_report_deliveries (delivery_id, artifact_id, channel, recipient, status, attempt_count, last_error, created_at, delivered_at) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),$8,$9)`
	_, err := r.db.ExecContext(context.Background(), query, delivery.ID, delivery.ArtifactID, delivery.Channel, delivery.Recipient, delivery.Status, delivery.AttemptCount, delivery.LastError, delivery.CreatedAt, nullableReportTime(delivery.DeliveredAt))
	return err
}

func (r *PostgresRepository) ListReportDeliveries() []ReportDelivery {
	const query = `SELECT delivery_id, artifact_id, channel, COALESCE(recipient,''), status, attempt_count, COALESCE(last_error,''), created_at, delivered_at FROM analytics_report_deliveries ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ReportDelivery, 0)
	for rows.Next() {
		var item ReportDelivery
		var deliveredAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.ArtifactID, &item.Channel, &item.Recipient, &item.Status, &item.AttemptCount, &item.LastError, &item.CreatedAt, &deliveredAt); err == nil {
			if deliveredAt.Valid {
				item.DeliveredAt = deliveredAt.Time
			}
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) SaveReportDeliveryDeadLetter(record ReportDeliveryDeadLetter) error {
	const query = `INSERT INTO analytics_report_delivery_dead_letters (dead_letter_id, artifact_id, channel, recipient, reason, attempt_count, created_at) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7)`
	_, err := r.db.ExecContext(context.Background(), query, record.ID, record.ArtifactID, record.Channel, record.Recipient, record.Reason, record.AttemptCount, record.CreatedAt)
	return err
}

func (r *PostgresRepository) ListReportDeliveryDeadLetters() []ReportDeliveryDeadLetter {
	const query = `SELECT dead_letter_id, artifact_id, channel, COALESCE(recipient,''), reason, attempt_count, created_at FROM analytics_report_delivery_dead_letters ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(context.Background(), query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ReportDeliveryDeadLetter, 0)
	for rows.Next() {
		var item ReportDeliveryDeadLetter
		if err := rows.Scan(&item.ID, &item.ArtifactID, &item.Channel, &item.Recipient, &item.Reason, &item.AttemptCount, &item.CreatedAt); err == nil {
			items = append(items, item)
		}
	}
	return items
}

func (r *PostgresRepository) SaveSnapshot(snapshot Snapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	const query = `INSERT INTO analytics_snapshots (snapshot_id, generated_at, window_key, payload_json) VALUES ($1, $2, $3, $4)`
	_, err = r.db.ExecContext(context.Background(), query, snapshot.ID, snapshot.GeneratedAt, snapshot.Window, payload)
	return err
}

func (r *PostgresRepository) ListSnapshots() []Snapshot {
	const query = `SELECT payload_json FROM analytics_snapshots ORDER BY generated_at ASC`
	return r.listByQuery(query)
}

func (r *PostgresRepository) QuerySnapshots(query Query) []Snapshot {
	stmt := `SELECT payload_json FROM analytics_snapshots`
	args := make([]any, 0, 4)
	clauses := make([]string, 0, 3)
	if query.Window != "" {
		args = append(args, query.Window)
		clauses = append(clauses, `window_key = $1`)
	}
	if !query.From.IsZero() {
		args = append(args, query.From)
		clauses = append(clauses, placeholderClause("generated_at >= ", len(args)))
	}
	if !query.To.IsZero() {
		args = append(args, query.To)
		clauses = append(clauses, placeholderClause("generated_at <= ", len(args)))
	}
	if len(clauses) > 0 {
		stmt += ` WHERE ` + joinClauses(clauses)
	}
	stmt += ` ORDER BY generated_at ASC`
	if query.Limit > 0 {
		args = append(args, query.Limit)
		stmt += placeholderClause(" LIMIT ", len(args))
	}
	return r.listByQuery(stmt, args...)
}

func (r *PostgresRepository) ListRecent(limit int) []Snapshot {
	if limit <= 0 {
		return r.ListSnapshots()
	}
	const query = `SELECT payload_json FROM analytics_snapshots ORDER BY generated_at DESC LIMIT $1`
	items := r.listByQuery(query, limit)
	sort.Slice(items, func(i, j int) bool { return items[i].GeneratedAt.Before(items[j].GeneratedAt) })
	return items
}

func (r *PostgresRepository) DeleteOlderThan(cutoff time.Time) error {
	const query = `DELETE FROM analytics_snapshots WHERE generated_at < $1`
	_, err := r.db.ExecContext(context.Background(), query, cutoff)
	return err
}

func (r *PostgresRepository) UpsertRollup(rollup Rollup) error {
	payload, err := json.Marshal(rollup)
	if err != nil {
		return err
	}
	const query = `
		INSERT INTO analytics_rollups (
			rollup_id, granularity, bucket_start, bucket_end, snapshot_count, payload_json
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (rollup_id) DO UPDATE SET
			granularity = EXCLUDED.granularity,
			bucket_start = EXCLUDED.bucket_start,
			bucket_end = EXCLUDED.bucket_end,
			snapshot_count = EXCLUDED.snapshot_count,
			payload_json = EXCLUDED.payload_json`
	_, err = r.db.ExecContext(context.Background(), query, rollup.ID, rollup.Granularity, rollup.BucketStart, rollup.BucketEnd, rollup.SnapshotCount, payload)
	return err
}

func (r *PostgresRepository) ListRollups(granularity string, limit int) []Rollup {
	return r.QueryRollups(granularity, time.Time{}, time.Time{}, limit)
}

func (r *PostgresRepository) QueryRollups(granularity string, from, to time.Time, limit int) []Rollup {
	query := `SELECT payload_json FROM analytics_rollups`
	args := make([]any, 0, 4)
	clauses := make([]string, 0, 3)
	if granularity != "" {
		args = append(args, granularity)
		clauses = append(clauses, `granularity = $1`)
	}
	if !from.IsZero() {
		args = append(args, from)
		clauses = append(clauses, placeholderClause("bucket_start >= ", len(args)))
	}
	if !to.IsZero() {
		args = append(args, to)
		clauses = append(clauses, placeholderClause("bucket_start <= ", len(args)))
	}
	if len(clauses) > 0 {
		query += ` WHERE ` + joinClauses(clauses)
	}
	query += ` ORDER BY bucket_start ASC`
	if limit > 0 {
		if len(args) == 0 {
			query += ` LIMIT $1`
		} else {
			query += ` LIMIT $2`
		}
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Rollup, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			continue
		}
		var item Rollup
		if err := json.Unmarshal(payload, &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].BucketStart.Before(items[j].BucketStart) })
	return items
}

func (r *PostgresRepository) SaveFacts(facts FactBundle) error {
	for _, fact := range facts.Documents {
		const query = `INSERT INTO analytics_document_facts (snapshot_id, captured_at, location_id, document_type, created_count, draft_count, submitted_count, approved_count, rejected_count, cancelled_count) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,$9,$10)`
		if _, err := r.db.ExecContext(context.Background(), query, fact.SnapshotID, fact.CapturedAt, fact.LocationID, fact.DocumentType, fact.Created, fact.Draft, fact.Submitted, fact.Approved, fact.Rejected, fact.Cancelled); err != nil {
			return err
		}
	}
	for _, fact := range facts.Workflow {
		const query = `INSERT INTO analytics_workflow_facts (snapshot_id, captured_at, pending_approvals, open_tasks, completed_tasks) VALUES ($1,$2,$3,$4,$5)`
		if _, err := r.db.ExecContext(context.Background(), query, fact.SnapshotID, fact.CapturedAt, fact.PendingApprovals, fact.OpenTasks, fact.CompletedTasks); err != nil {
			return err
		}
	}
	for _, fact := range facts.Reliability {
		const query = `INSERT INTO analytics_reliability_facts (snapshot_id, captured_at, outbox_pending, dead_letters, dispatch_success, dispatch_retries) VALUES ($1,$2,$3,$4,$5,$6)`
		if _, err := r.db.ExecContext(context.Background(), query, fact.SnapshotID, fact.CapturedAt, fact.OutboxPending, fact.DeadLetters, fact.DispatchSuccess, fact.DispatchRetries); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) QueryFacts(query FactQuery) FactBundle {
	bundle := FactBundle{}
	args := []any{}
	clauses := []string{}
	if !query.From.IsZero() {
		args = append(args, query.From)
		clauses = append(clauses, placeholderClause("captured_at >= ", len(args)))
	}
	if !query.To.IsZero() {
		args = append(args, query.To)
		clauses = append(clauses, placeholderClause("captured_at <= ", len(args)))
	}
	if query.LocationID != "" {
		args = append(args, query.LocationID)
		clauses = append(clauses, placeholderClause("location_id = ", len(args)))
	}
	if query.DocumentType != "" {
		args = append(args, query.DocumentType)
		clauses = append(clauses, placeholderClause("document_type = ", len(args)))
	}
	docQuery := `SELECT snapshot_id, captured_at, COALESCE(location_id,''), COALESCE(document_type,''), created_count, draft_count, submitted_count, approved_count, rejected_count, cancelled_count FROM analytics_document_facts`
	if len(clauses) > 0 {
		docQuery += ` WHERE ` + joinClauses(clauses)
	}
	rows, err := r.db.QueryContext(context.Background(), docQuery, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item DocumentFact
			if err := rows.Scan(&item.SnapshotID, &item.CapturedAt, &item.LocationID, &item.DocumentType, &item.Created, &item.Draft, &item.Submitted, &item.Approved, &item.Rejected, &item.Cancelled); err == nil {
				bundle.Documents = append(bundle.Documents, item)
			}
		}
	}
	wfRows, err := r.db.QueryContext(context.Background(), `SELECT snapshot_id, captured_at, pending_approvals, open_tasks, completed_tasks FROM analytics_workflow_facts`)
	if err == nil {
		defer wfRows.Close()
		for wfRows.Next() {
			var item WorkflowFact
			if err := wfRows.Scan(&item.SnapshotID, &item.CapturedAt, &item.PendingApprovals, &item.OpenTasks, &item.CompletedTasks); err == nil {
				bundle.Workflow = append(bundle.Workflow, item)
			}
		}
	}
	relRows, err := r.db.QueryContext(context.Background(), `SELECT snapshot_id, captured_at, outbox_pending, dead_letters, dispatch_success, dispatch_retries FROM analytics_reliability_facts`)
	if err == nil {
		defer relRows.Close()
		for relRows.Next() {
			var item ReliabilityFact
			if err := relRows.Scan(&item.SnapshotID, &item.CapturedAt, &item.OutboxPending, &item.DeadLetters, &item.DispatchSuccess, &item.DispatchRetries); err == nil {
				bundle.Reliability = append(bundle.Reliability, item)
			}
		}
	}
	return bundle
}

func (r *PostgresRepository) ReportingBreakdown(query FactQuery, dimension string) []ReportingRow {
	baseArgs := []any{}
	clauses := []string{}
	if !query.From.IsZero() {
		baseArgs = append(baseArgs, query.From)
		clauses = append(clauses, placeholderClause("f.captured_at >= ", len(baseArgs)))
	}
	if !query.To.IsZero() {
		baseArgs = append(baseArgs, query.To)
		clauses = append(clauses, placeholderClause("f.captured_at <= ", len(baseArgs)))
	}
	if query.LocationID != "" {
		baseArgs = append(baseArgs, query.LocationID)
		clauses = append(clauses, placeholderClause("f.location_id = ", len(baseArgs)))
	}
	if query.DocumentType != "" {
		baseArgs = append(baseArgs, query.DocumentType)
		clauses = append(clauses, placeholderClause("f.document_type = ", len(baseArgs)))
	}

	querySQL := `
		SELECT COALESCE(f.document_type,''), COALESCE(dt.display_name, COALESCE(f.document_type,'')),
			SUM(f.created_count), SUM(f.draft_count), SUM(f.submitted_count), SUM(f.approved_count), SUM(f.rejected_count), SUM(f.cancelled_count)
		FROM analytics_document_facts f
		LEFT JOIN analytics_document_type_dim dt ON dt.document_type = f.document_type`
	labelField := `COALESCE(dt.display_name, COALESCE(f.document_type,''))`
	keyField := `COALESCE(f.document_type,'')`
	if dimension == "location" {
		querySQL = `
			SELECT COALESCE(f.location_id,''), COALESCE(ld.display_name, COALESCE(f.location_id,'')),
				SUM(f.created_count), SUM(f.draft_count), SUM(f.submitted_count), SUM(f.approved_count), SUM(f.rejected_count), SUM(f.cancelled_count)
			FROM analytics_document_facts f
			LEFT JOIN analytics_location_dim ld ON ld.location_id = f.location_id`
		labelField = `COALESCE(ld.display_name, COALESCE(f.location_id,''))`
		keyField = `COALESCE(f.location_id,'')`
	} else {
		dimension = "document_type"
	}
	if len(clauses) > 0 {
		querySQL += ` WHERE ` + joinClauses(clauses)
	}
	querySQL += ` GROUP BY ` + keyField + `, ` + labelField + ` ORDER BY ` + keyField
	rows, err := r.db.QueryContext(context.Background(), querySQL, baseArgs...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]ReportingRow, 0)
	for rows.Next() {
		var item ReportingRow
		item.DimensionType = dimension
		if err := rows.Scan(&item.DimensionKey, &item.Label, &item.Created, &item.Draft, &item.Submitted, &item.Approved, &item.Rejected, &item.Cancelled); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (r *PostgresRepository) listByQuery(query string, args ...any) []Snapshot {
	rows, err := r.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]Snapshot, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			continue
		}
		var item Snapshot
		if err := json.Unmarshal(payload, &item); err != nil {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GeneratedAt.Before(items[j].GeneratedAt) })
	return items
}

func joinClauses(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	result := clauses[0]
	for i := 1; i < len(clauses); i++ {
		result += ` AND ` + clauses[i]
	}
	return result
}

func placeholderClause(prefix string, idx int) string {
	return prefix + `$` + strconv.Itoa(idx)
}

func _unusedFmt() string { return fmt.Sprintf("") }

func nullableReportTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func (r *PostgresRepository) CleanupReportData(cutoff time.Time) error {
	queries := []string{
		`DELETE FROM analytics_report_delivery_dead_letters WHERE created_at < $1`,
		`DELETE FROM analytics_report_deliveries WHERE created_at < $1`,
		`DELETE FROM analytics_report_artifacts WHERE created_at < $1`,
		`DELETE FROM analytics_report_runs WHERE generated_at < $1`,
	}
	for _, query := range queries {
		if _, err := r.db.ExecContext(context.Background(), query, cutoff); err != nil {
			return err
		}
	}
	return nil
}
