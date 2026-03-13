package jobs

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositoryEnqueueAndGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT job_id, job_name").
		WithArgs("bucket:1").
		WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectExec("INSERT INTO job_records").
		WithArgs("job-1", "analytics.capture_snapshot", "bucket:1", StatusQueued, 0, "", sqlmock.AnyArg(), now).
		WillReturnResult(sqlmock.NewResult(1, 1))

	job, created, err := repo.Enqueue(Job{ID: "job-1", Name: "analytics.capture_snapshot", DedupKey: "bucket:1", Status: StatusQueued, CreatedAt: now})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if !created || job.ID != "job-1" {
		t.Fatalf("expected created job, got %+v created=%v", job, created)
	}

	rows := sqlmock.NewRows([]string{"job_id", "job_name", "dedup_key", "status", "attempt_count", "last_error", "payload_json", "result_json", "created_at", "started_at", "ended_at", "lease_expires_at"}).
		AddRow("job-1", "analytics.capture_snapshot", "bucket:1", StatusQueued, 0, "", []byte(`{}`), []byte(`{}`), now, nil, nil, nil)
	mock.ExpectQuery("SELECT job_id, job_name").
		WithArgs("job-1").
		WillReturnRows(rows)

	stored, ok := repo.Get("job-1")
	if !ok || stored.ID != "job-1" {
		t.Fatalf("expected stored job, got %+v ok=%v", stored, ok)
	}
}

func TestPostgresRepositoryRenewLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	mock.ExpectExec("UPDATE job_records\\s+SET lease_expires_at = \\$1\\s+WHERE job_id = \\$2 AND status = 'running'").
		WithArgs(now.Add(5*time.Second), "job-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.RenewLease("job-1", now, 5*time.Second); err != nil {
		t.Fatalf("renew lease failed: %v", err)
	}
}
