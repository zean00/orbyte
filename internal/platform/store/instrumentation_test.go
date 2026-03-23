package store

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"orbyte/internal/platform/observability"
)

func TestInstrumentDBRecordsQueries(t *testing.T) {
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = raw.Close() }()

	obs := observability.NewService()
	monitor := NewQueryMonitor(obs, QueryMonitorOptions{
		SlowThreshold: time.Hour,
		TopOperations: 10,
		SlowQueries:   10,
	})
	db := InstrumentDB(raw, monitor, "identity", "repository")

	mock.ExpectExec(regexp.QuoteMeta("UPDATE identity_users SET display_name = $1 WHERE user_id = $2")).
		WithArgs("Alice", "user_admin").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err := db.ExecContext(context.Background(), "UPDATE identity_users SET display_name = $1 WHERE user_id = $2", "Alice", "user_admin"); err != nil {
		t.Fatalf("exec: %v", err)
	}

	rows := sqlmock.NewRows([]string{"user_id"}).AddRow("user_admin")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT user_id FROM identity_users WHERE user_id = $1")).
		WithArgs("user_admin").
		WillReturnRows(rows)
	resultRows, err := db.QueryContext(context.Background(), "SELECT user_id FROM identity_users WHERE user_id = $1", "user_admin")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = resultRows.Close() }()

	snapshot := monitor.Snapshot()
	if len(snapshot.TopOperations) != 2 {
		t.Fatalf("expected 2 query summaries, got %d", len(snapshot.TopOperations))
	}
	for _, item := range snapshot.TopOperations {
		if item.Subsystem != "identity" || item.Component != "repository" {
			t.Fatalf("unexpected summary scope: %+v", item)
		}
		if item.Operation == "" {
			t.Fatal("expected derived operation name")
		}
		if item.Fingerprint == "" {
			t.Fatal("expected fingerprint")
		}
	}
	if obs.Snapshot().Counters["db.query.identity.repository.update.total"] == 0 {
		t.Fatal("expected exec counter to be recorded")
	}
	if obs.Snapshot().Counters["db.query.identity.repository.select.total"] == 0 {
		t.Fatal("expected query counter to be recorded")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestQueryMonitorTracksSlowQueries(t *testing.T) {
	obs := observability.NewService()
	obs.RegisterLogEventDefinition(observability.LogEventDefinition{
		Key:      "db.query.slow",
		Severity: "warning",
		Category: "database",
	})
	monitor := NewQueryMonitor(obs, QueryMonitorOptions{
		SlowThreshold: 5 * time.Millisecond,
		TopOperations: 5,
		SlowQueries:   5,
	})

	monitor.Record(queryRecord{
		Operation:     "analytics.report.list",
		Subsystem:     "analytics",
		Component:     "repository",
		StatementKind: "select",
		Fingerprint:   "abcd1234",
		Duration:      12 * time.Millisecond,
		RowsAffected:  3,
	})

	snapshot := monitor.Snapshot()
	if len(snapshot.RecentSlow) != 1 {
		t.Fatalf("expected 1 slow query, got %d", len(snapshot.RecentSlow))
	}
	if snapshot.RecentSlow[0].Operation != "analytics.report.list" {
		t.Fatalf("unexpected slow query record: %+v", snapshot.RecentSlow[0])
	}
	if len(obs.QueryLogRecords("db.query.slow", "")) != 1 {
		t.Fatal("expected slow query log event")
	}
}

func TestInstrumentDBQueryRowRecordsScanError(t *testing.T) {
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = raw.Close() }()

	obs := observability.NewService()
	monitor := NewQueryMonitor(obs, QueryMonitorOptions{
		SlowThreshold: time.Hour,
		TopOperations: 10,
		SlowQueries:   10,
	})
	db := InstrumentDB(raw, monitor, "module", "repository")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT key FROM installed_modules WHERE module_key = $1")).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"key"}))

	var key string
	err = db.QueryRowContext(context.Background(), "SELECT key FROM installed_modules WHERE module_key = $1", "missing").Scan(&key)
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}

	snapshot := monitor.Snapshot()
	if len(snapshot.TopOperations) != 1 {
		t.Fatalf("expected 1 query summary, got %d", len(snapshot.TopOperations))
	}
	if snapshot.TopOperations[0].ErrorCount != 1 {
		t.Fatalf("expected query row error to be recorded, got %+v", snapshot.TopOperations[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
