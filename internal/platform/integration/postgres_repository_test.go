package integration

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositorySystemAndSubmissionLifecycle(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO integration_systems (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveSystem(ExternalSystem{
		Key: "http_bridge", Name: "HTTP Bridge", Status: "active", Adapter: "http",
		Description: "Bridge", Settings: map[string]any{"url": "https://example.test"}, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save system failed: %v", err)
	}

	systemRows := sqlmock.NewRows([]string{"system_key", "name", "status", "adapter_key", "description", "settings_json", "created_at", "updated_at"}).
		AddRow("http_bridge", "HTTP Bridge", "active", "http", "Bridge", []byte(`{"url":"https://example.test"}`), now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT system_key, name, status, adapter_key, COALESCE(description,''), settings_json, created_at, updated_at FROM integration_systems WHERE system_key = $1")).
		WithArgs("http_bridge").WillReturnRows(systemRows)
	system, ok := repo.GetSystem("http_bridge")
	if !ok || system.Adapter != "http" || system.Settings["url"] != "https://example.test" {
		t.Fatalf("expected persisted system, got %+v ok=%v", system, ok)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO integration_submissions (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveSubmission(SubmissionRecord{
		ID: "sub-1", ExternalSystemKey: "http_bridge", OperationType: "push_document", Status: "queued",
		DocumentID: "doc-1", CorrelationID: "corr-1", AttemptCount: 1,
		Payload: map[string]any{"title": "hello"}, Result: map[string]any{}, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save submission failed: %v", err)
	}

	submissionRows := sqlmock.NewRows([]string{"submission_id", "external_system_key", "operation_type", "status", "document_id", "correlation_id", "external_reference", "attempt_count", "last_error", "payload_json", "result_json", "processed_at", "created_at", "updated_at"}).
		AddRow("sub-1", "http_bridge", "push_document", "queued", "doc-1", "corr-1", "", 1, "", []byte(`{"title":"hello"}`), []byte(`{}`), nil, now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT submission_id, external_system_key, operation_type, status, COALESCE(document_id,''), COALESCE(correlation_id,''), COALESCE(external_reference,''), attempt_count, COALESCE(last_error,''), payload_json, result_json, processed_at, created_at, updated_at FROM integration_submissions WHERE submission_id = $1")).
		WithArgs("sub-1").WillReturnRows(submissionRows)
	record, ok := repo.GetSubmission("sub-1")
	if !ok || record.OperationType != "push_document" || record.Payload["title"] != "hello" {
		t.Fatalf("expected persisted submission, got %+v ok=%v", record, ok)
	}
}
