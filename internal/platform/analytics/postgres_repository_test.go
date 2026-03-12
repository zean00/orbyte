package analytics

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositorySaveAndListSnapshots(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	snap := Snapshot{ID: "s1", GeneratedAt: now, Window: "current_state", Metrics: map[string]float64{"x": 1}}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_snapshots (snapshot_id, generated_at, window_key, payload_json) VALUES ($1, $2, $3, $4)")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveSnapshot(snap); err != nil {
		t.Fatalf("save snapshot failed: %v", err)
	}
	rows := sqlmock.NewRows([]string{"payload_json"}).AddRow([]byte(`{"id":"s1","generated_at":"` + now.Format(time.RFC3339Nano) + `","window":"current_state","documents":{"created":0,"draft":0,"submitted":0,"approved":0,"rejected":0,"cancelled":0},"workflow":{"open_tasks":0,"pending_approvals":0,"completed_tasks":0,"approval_rate":0,"rejection_rate":0},"reliability":{"outbox_pending":0,"outbox_dead_letters":0,"dispatch_success":0,"dispatch_retries":0,"dead_letter_rate":0},"coverage":{"document_summaries":0,"projection_coverage":0,"audit_events":0},"metrics":{}}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT payload_json FROM analytics_snapshots ORDER BY generated_at ASC")).WillReturnRows(rows)
	items := repo.ListSnapshots()
	if len(items) != 1 || items[0].ID != "s1" {
		t.Fatal("expected listed analytics snapshot")
	}
	queryRows := sqlmock.NewRows([]string{"payload_json"}).AddRow([]byte(`{"id":"s1","generated_at":"` + now.Format(time.RFC3339Nano) + `","window":"current_state","documents":{"created":0,"draft":0,"submitted":0,"approved":0,"rejected":0,"cancelled":0},"segments":{"by_document_type":{},"by_location":{}},"workflow":{"open_tasks":0,"pending_approvals":0,"completed_tasks":0,"approval_rate":0,"rejection_rate":0},"reliability":{"outbox_pending":0,"outbox_dead_letters":0,"dispatch_success":0,"dispatch_retries":0,"dead_letter_rate":0},"coverage":{"document_summaries":0,"projection_coverage":0,"audit_events":0},"metrics":{}}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT payload_json FROM analytics_snapshots WHERE window_key = $1 AND generated_at >= $2 AND generated_at <= $3 ORDER BY generated_at ASC LIMIT $4")).WithArgs("current_state", now.Add(-time.Hour), now.Add(time.Hour), 5).WillReturnRows(queryRows)
	if len(repo.QuerySnapshots(Query{Window: "current_state", From: now.Add(-time.Hour), To: now.Add(time.Hour), Limit: 5})) != 1 {
		t.Fatal("expected queried analytics snapshot")
	}
	rows = sqlmock.NewRows([]string{"payload_json"}).AddRow([]byte(`{"id":"s1","generated_at":"` + now.Format(time.RFC3339Nano) + `","window":"current_state","documents":{"created":0,"draft":0,"submitted":0,"approved":0,"rejected":0,"cancelled":0},"segments":{"by_document_type":{},"by_location":{}},"workflow":{"open_tasks":0,"pending_approvals":0,"completed_tasks":0,"approval_rate":0,"rejection_rate":0},"reliability":{"outbox_pending":0,"outbox_dead_letters":0,"dispatch_success":0,"dispatch_retries":0,"dead_letter_rate":0},"coverage":{"document_summaries":0,"projection_coverage":0,"audit_events":0},"metrics":{}}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT payload_json FROM analytics_snapshots ORDER BY generated_at DESC LIMIT $1")).WithArgs(1).WillReturnRows(rows)
	if len(repo.ListRecent(1)) != 1 {
		t.Fatal("expected recent analytics snapshot")
	}
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM analytics_snapshots WHERE generated_at < $1")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.DeleteOlderThan(now); err != nil {
		t.Fatalf("delete older snapshots failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_rollups (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.UpsertRollup(Rollup{ID: "daily:1", Granularity: "daily", BucketStart: now, BucketEnd: now.Add(24 * time.Hour), SnapshotCount: 1}); err != nil {
		t.Fatalf("upsert rollup failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_document_type_dim (document_type, display_name) VALUES ($1,$2) ON CONFLICT (document_type) DO UPDATE SET display_name = EXCLUDED.display_name")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_location_dim (location_id, display_name) VALUES ($1,$2) ON CONFLICT (location_id) DO UPDATE SET display_name = EXCLUDED.display_name")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveDimensions(DimensionBundle{DocumentTypes: []DocumentTypeDimension{{DocumentType: "generic_request", DisplayName: "Generic Request"}}, Locations: []LocationDimension{{LocationID: "loc_hq", Name: "Head Office"}}}); err != nil {
		t.Fatalf("save dimensions failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_report_definitions (report_id, name, dimension, format, window_key, location_id, document_type, delivery_channel, delivery_target, schedule_key, next_run_at, enabled) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,$11,$12)")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveReportDefinition(ReportDefinition{ID: "r1", Name: "Daily", Dimension: "document_type", Format: "csv", Window: "current_state", DeliveryChannel: "download", Schedule: "daily", NextRunAt: now, Enabled: true}); err != nil {
		t.Fatalf("save report definition failed: %v", err)
	}
	defRows := sqlmock.NewRows([]string{"report_id", "name", "dimension", "format", "window_key", "location_id", "document_type", "delivery_channel", "delivery_target", "schedule_key", "next_run_at", "enabled"}).AddRow("r1", "Daily", "document_type", "csv", "current_state", "", "", "download", "", "daily", now, true)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT report_id, name, dimension, format, window_key, COALESCE(location_id,''), COALESCE(document_type,''), COALESCE(delivery_channel,''), COALESCE(delivery_target,''), schedule_key, next_run_at, enabled FROM analytics_report_definitions ORDER BY next_run_at ASC")).WillReturnRows(defRows)
	if len(repo.ListReportDefinitions()) != 1 {
		t.Fatal("expected report definitions")
	}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE analytics_report_definitions SET name=$1, dimension=$2, format=$3, window_key=$4, location_id=NULLIF($5,''), document_type=NULLIF($6,''), delivery_channel=NULLIF($7,''), delivery_target=NULLIF($8,''), schedule_key=$9, next_run_at=$10, enabled=$11 WHERE report_id=$12")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.UpdateReportDefinition(ReportDefinition{ID: "r1", Name: "Daily", Dimension: "document_type", Format: "csv", Window: "current_state", DeliveryChannel: "download", Schedule: "daily", NextRunAt: now, Enabled: true}); err != nil {
		t.Fatalf("update report definition failed: %v", err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_report_runs (report_run_id, report_id, format, status, generated_at, row_count) VALUES ($1,$2,$3,$4,$5,$6)")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveReportRun(ReportRun{ID: "run1", ReportID: "r1", Format: "csv", Status: "generated", GeneratedAt: now, RowCount: 1}); err != nil {
		t.Fatalf("save report run failed: %v", err)
	}
	runRows := sqlmock.NewRows([]string{"report_run_id", "report_id", "format", "status", "generated_at", "row_count"}).AddRow("run1", "r1", "csv", "generated", now, 1)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT report_run_id, report_id, format, status, generated_at, row_count FROM analytics_report_runs ORDER BY generated_at ASC")).WillReturnRows(runRows)
	if len(repo.ListReportRuns()) != 1 {
		t.Fatal("expected report runs")
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_report_artifacts (artifact_id, report_id, report_run_id, file_name, content_type, size_bytes, content_bytes, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveReportArtifact(ReportArtifact{ID: "a1", ReportID: "r1", ReportRunID: "run1", FileName: "x.csv", ContentType: "text/csv", SizeBytes: 3, Content: []byte("abc"), CreatedAt: now}); err != nil {
		t.Fatalf("save report artifact failed: %v", err)
	}
	artifactRows := sqlmock.NewRows([]string{"artifact_id", "report_id", "report_run_id", "file_name", "content_type", "size_bytes", "created_at"}).AddRow("a1", "r1", "run1", "x.csv", "text/csv", 3, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT artifact_id, report_id, report_run_id, file_name, content_type, size_bytes, created_at FROM analytics_report_artifacts ORDER BY created_at ASC")).WillReturnRows(artifactRows)
	if len(repo.ListReportArtifacts()) != 1 {
		t.Fatal("expected report artifacts")
	}
	artifactGetRows := sqlmock.NewRows([]string{"artifact_id", "report_id", "report_run_id", "file_name", "content_type", "size_bytes", "content_bytes", "created_at"}).AddRow("a1", "r1", "run1", "x.csv", "text/csv", 3, []byte("abc"), now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT artifact_id, report_id, report_run_id, file_name, content_type, size_bytes, content_bytes, created_at FROM analytics_report_artifacts WHERE artifact_id = $1")).WithArgs("a1").WillReturnRows(artifactGetRows)
	if _, ok := repo.GetReportArtifact("a1"); !ok {
		t.Fatal("expected report artifact lookup")
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_report_deliveries (delivery_id, artifact_id, channel, recipient, status, attempt_count, last_error, created_at, delivered_at) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''),$8,$9)")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveReportDelivery(ReportDelivery{ID: "d1", ArtifactID: "a1", Channel: "download", Status: "delivered", AttemptCount: 1, CreatedAt: now, DeliveredAt: now}); err != nil {
		t.Fatalf("save report delivery failed: %v", err)
	}
	deliveryRows := sqlmock.NewRows([]string{"delivery_id", "artifact_id", "channel", "recipient", "status", "attempt_count", "last_error", "created_at", "delivered_at"}).AddRow("d1", "a1", "download", "", "delivered", 1, "", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT delivery_id, artifact_id, channel, COALESCE(recipient,''), status, attempt_count, COALESCE(last_error,''), created_at, delivered_at FROM analytics_report_deliveries ORDER BY created_at ASC")).WillReturnRows(deliveryRows)
	if len(repo.ListReportDeliveries()) != 1 {
		t.Fatal("expected report deliveries")
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_report_delivery_dead_letters (dead_letter_id, artifact_id, channel, recipient, reason, attempt_count, created_at) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7)")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveReportDeliveryDeadLetter(ReportDeliveryDeadLetter{ID: "ddl1", ArtifactID: "a1", Channel: "webhook", Recipient: "https://example.com", Reason: "boom", AttemptCount: 3, CreatedAt: now}); err != nil {
		t.Fatalf("save report delivery dead letter failed: %v", err)
	}
	deadRows := sqlmock.NewRows([]string{"dead_letter_id", "artifact_id", "channel", "recipient", "reason", "attempt_count", "created_at"}).AddRow("ddl1", "a1", "webhook", "https://example.com", "boom", 3, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT dead_letter_id, artifact_id, channel, COALESCE(recipient,''), reason, attempt_count, created_at FROM analytics_report_delivery_dead_letters ORDER BY created_at ASC")).WillReturnRows(deadRows)
	if len(repo.ListReportDeliveryDeadLetters()) != 1 {
		t.Fatal("expected report delivery dead letters")
	}
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM analytics_report_delivery_dead_letters WHERE created_at < $1")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM analytics_report_deliveries WHERE created_at < $1")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM analytics_report_artifacts WHERE created_at < $1")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM analytics_report_runs WHERE generated_at < $1")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.CleanupReportData(now); err != nil {
		t.Fatalf("cleanup report data failed: %v", err)
	}
	rollupRows := sqlmock.NewRows([]string{"payload_json"}).AddRow([]byte(`{"id":"daily:1","granularity":"daily","bucket_start":"` + now.Format(time.RFC3339Nano) + `","bucket_end":"` + now.Add(24*time.Hour).Format(time.RFC3339Nano) + `","snapshot_count":1,"documents":{"created":0,"draft":0,"submitted":0,"approved":0,"rejected":0,"cancelled":0},"workflow":{"open_tasks":0,"pending_approvals":0,"completed_tasks":0,"approval_rate":0,"rejection_rate":0},"reliability":{"outbox_pending":0,"outbox_dead_letters":0,"dispatch_success":0,"dispatch_retries":0,"dead_letter_rate":0}}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT payload_json FROM analytics_rollups WHERE granularity = $1 ORDER BY bucket_start ASC LIMIT $2")).WithArgs("daily", 10).WillReturnRows(rollupRows)
	if len(repo.ListRollups("daily", 10)) != 1 {
		t.Fatal("expected analytics rollup")
	}
	rollupQueryRows := sqlmock.NewRows([]string{"payload_json"}).AddRow([]byte(`{"id":"daily:1","granularity":"daily","bucket_start":"` + now.Format(time.RFC3339Nano) + `","bucket_end":"` + now.Add(24*time.Hour).Format(time.RFC3339Nano) + `","snapshot_count":1,"documents":{"created":0,"draft":0,"submitted":0,"approved":0,"rejected":0,"cancelled":0},"segments":{"by_document_type":{},"by_location":{}},"workflow":{"open_tasks":0,"pending_approvals":0,"completed_tasks":0,"approval_rate":0,"rejection_rate":0},"reliability":{"outbox_pending":0,"outbox_dead_letters":0,"dispatch_success":0,"dispatch_retries":0,"dead_letter_rate":0}}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT payload_json FROM analytics_rollups")).WithArgs("daily", now.Add(-time.Hour), now.Add(time.Hour), 5).WillReturnRows(rollupQueryRows)
	if len(repo.QueryRollups("daily", now.Add(-time.Hour), now.Add(time.Hour), 5)) != 1 {
		t.Fatal("expected queried analytics rollup")
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_document_facts (snapshot_id, captured_at, location_id, document_type, created_count, draft_count, submitted_count, approved_count, rejected_count, cancelled_count) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,$9,$10)")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_workflow_facts (snapshot_id, captured_at, pending_approvals, open_tasks, completed_tasks) VALUES ($1,$2,$3,$4,$5)")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO analytics_reliability_facts (snapshot_id, captured_at, outbox_pending, dead_letters, dispatch_success, dispatch_retries) VALUES ($1,$2,$3,$4,$5,$6)")).WillReturnResult(sqlmock.NewResult(1, 1))
	facts := FactBundle{
		Documents:   []DocumentFact{{SnapshotID: "s1", CapturedAt: now, LocationID: "loc_hq", DocumentType: "generic_request", Created: 1}},
		Workflow:    []WorkflowFact{{SnapshotID: "s1", CapturedAt: now, PendingApprovals: 1}},
		Reliability: []ReliabilityFact{{SnapshotID: "s1", CapturedAt: now, OutboxPending: 1}},
	}
	if err := repo.SaveFacts(facts); err != nil {
		t.Fatalf("save facts failed: %v", err)
	}
	docFactRows := sqlmock.NewRows([]string{"snapshot_id", "captured_at", "location_id", "document_type", "created_count", "draft_count", "submitted_count", "approved_count", "rejected_count", "cancelled_count"}).AddRow("s1", now, "loc_hq", "generic_request", 1, 0, 0, 0, 0, 0)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT snapshot_id, captured_at, COALESCE(location_id,''), COALESCE(document_type,''), created_count, draft_count, submitted_count, approved_count, rejected_count, cancelled_count FROM analytics_document_facts")).WillReturnRows(docFactRows)
	wfFactRows := sqlmock.NewRows([]string{"snapshot_id", "captured_at", "pending_approvals", "open_tasks", "completed_tasks"}).AddRow("s1", now, 1, 0, 0)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT snapshot_id, captured_at, pending_approvals, open_tasks, completed_tasks FROM analytics_workflow_facts")).WillReturnRows(wfFactRows)
	relFactRows := sqlmock.NewRows([]string{"snapshot_id", "captured_at", "outbox_pending", "dead_letters", "dispatch_success", "dispatch_retries"}).AddRow("s1", now, 1, 0, 0, 0)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT snapshot_id, captured_at, outbox_pending, dead_letters, dispatch_success, dispatch_retries FROM analytics_reliability_facts")).WillReturnRows(relFactRows)
	queriedFacts := repo.QueryFacts(FactQuery{})
	if len(queriedFacts.Documents) != 1 || len(queriedFacts.Workflow) != 1 || len(queriedFacts.Reliability) != 1 {
		t.Fatal("expected queried facts")
	}
	reportRows := sqlmock.NewRows([]string{"dimension_key", "label", "created", "draft", "submitted", "approved", "rejected", "cancelled"}).AddRow("generic_request", "Generic Request", 1, 0, 0, 0, 0, 0)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(f.document_type,''), COALESCE(dt.display_name, COALESCE(f.document_type,'')),")).WillReturnRows(reportRows)
	if len(repo.ReportingBreakdown(FactQuery{}, "document_type")) != 1 {
		t.Fatal("expected reporting rows")
	}
}
