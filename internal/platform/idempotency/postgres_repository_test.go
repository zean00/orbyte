package idempotency

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositorySaveGetAndList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	record := Record{
		Operation:    "document.create",
		Key:          "idem-1",
		ActorID:      "user-1",
		RequestHash:  "hash-1",
		Status:       "succeeded",
		ResponseCode: 201,
		Response:     map[string]any{"document_id": "doc-1"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO idempotency_records (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.Save(record); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	rows := sqlmock.NewRows([]string{"operation_key", "idempotency_key", "actor_id", "request_hash", "status", "response_code", "response_json", "error_message", "created_at", "updated_at"}).
		AddRow("document.create", "idem-1", "user-1", "hash-1", "succeeded", 201, []byte(`{"document_id":"doc-1"}`), "", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT operation_key, idempotency_key, actor_id, request_hash, status, response_code,")).
		WillReturnRows(rows)
	got, ok := repo.Get("document.create", "idem-1")
	if !ok || got.Key != "idem-1" || got.ActorID != "user-1" || got.Response["document_id"] != "doc-1" {
		t.Fatalf("unexpected get result ok=%v record=%+v", ok, got)
	}

	listRows := sqlmock.NewRows([]string{"operation_key", "idempotency_key", "actor_id", "request_hash", "status", "response_code", "response_json", "error_message", "created_at", "updated_at"}).
		AddRow("document.create", "idem-1", "user-1", "hash-1", "succeeded", 201, []byte(`{"document_id":"doc-1"}`), "", now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT operation_key, idempotency_key, actor_id, request_hash, status, response_code,")).
		WillReturnRows(listRows)
	items := repo.List()
	if len(items) != 1 || items[0].Key != "idem-1" {
		t.Fatalf("unexpected list result: %+v", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
