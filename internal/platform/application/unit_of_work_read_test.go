package application

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"orbyte/internal/platform/audit"
	"orbyte/internal/platform/document"
	"orbyte/internal/platform/eventing"
	"orbyte/internal/platform/model"
	"orbyte/internal/platform/shared"
	"orbyte/internal/platform/workflow"
)

func TestMemoryTransactionManagerUnitOfWorkSupportsReadCreateAndNoopOutbox(t *testing.T) {
	docs := document.NewService()
	models := model.NewService()
	flows := workflow.NewService()
	auditSvc := audit.NewService()
	eventingSvc := eventing.NewService()
	txm := NewMemoryTransactionManager(docs, models, flows, auditSvc, eventingSvc)
	now := time.Now().UTC()
	record := document.Record{
		Header: document.Header{
			ID:             "doc-1",
			Type:           "generic_request",
			Status:         "draft",
			Version:        1,
			ETag:           "doc-1:1",
			OrganizationID: "org",
			CreatedBy:      "u1",
			CreatedAt:      now,
			UpdatedBy:      "u1",
			UpdatedAt:      now,
			TotalAmount:    shared.Money{Currency: "IDR"},
		},
		Body: document.Body{DocumentID: "doc-1", SchemaVersion: "v1", Payload: map[string]any{"title": "x"}},
	}

	if err := txm.WithinTx(context.Background(), func(uow UnitOfWork) error {
		if err := uow.CreateDocument(record); err != nil {
			return err
		}
		if err := uow.SaveOutbox(eventing.OutboxRecord{ID: "o1"}); err != nil {
			return err
		}
		got, err := uow.GetDocument(record.Header.ID)
		if err != nil {
			return err
		}
		if got.Header.ID != record.Header.ID || got.Body.Payload["title"] != "x" {
			t.Fatalf("unexpected record: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithinTx failed: %v", err)
	}
}

func TestPostgresUnitOfWorkGetDocumentLoadsRelatedCollections(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	txm := NewPostgresTransactionManager(db)
	now := time.Now().UTC()
	payload := []byte(`{"title":"order"}`)
	linePayload := []byte(`{"sku":"SKU-1"}`)
	linkMetadata := []byte(`{"relation":"reservation"}`)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id, document_type, status, version, etag, organization_id,")).WithArgs("doc-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"document_id", "document_type", "status", "version", "etag", "organization_id",
			"location_id", "number", "created_by", "created_at", "updated_by", "updated_at",
			"submitted_by", "submitted_at", "schema_version", "payload_json", "content_hash",
			"total_amount_minor", "total_amount_currency", "metadata_json",
		}).AddRow("doc-1", "generic_request", "submitted", 2, "doc-1:2", "org", "loc_hq", "ORD-1", "u1", now, "u1", now, "u1", now, "v1", payload, "hash", int64(2500), "IDR", []byte(`{"workflow_version":1}`)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_line_id, line_no, line_type, COALESCE(line_schema_ref, ''),")).WithArgs("doc-1").
		WillReturnRows(sqlmock.NewRows([]string{"document_line_id", "line_no", "line_type", "line_schema_ref", "payload_json", "amount_minor", "amount_currency"}).
			AddRow("line-1", 1, "item", "line.v1", linePayload, int64(2500), "IDR"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT link_id, linked_document_id, link_type, COALESCE(metadata_json, '{}'::jsonb), created_at")).WithArgs("doc-1").
		WillReturnRows(sqlmock.NewRows([]string{"link_id", "linked_document_id", "link_type", "metadata_json", "created_at"}).
			AddRow("link-1", "stock-1", "reserves", linkMetadata, now))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT attachment_id, attachment_type, file_name, content_type, storage_key, size_bytes, created_at")).WithArgs("doc-1").
		WillReturnRows(sqlmock.NewRows([]string{"attachment_id", "attachment_type", "file_name", "content_type", "storage_key", "size_bytes", "created_at"}).
			AddRow("att-1", "invoice", "invoice.pdf", "application/pdf", "obj://invoice.pdf", int64(1024), now))
	mock.ExpectCommit()

	if err := txm.WithinTx(context.Background(), func(uow UnitOfWork) error {
		got, err := uow.GetDocument("doc-1")
		if err != nil {
			return err
		}
		if len(got.Lines) != 1 || got.Lines[0].SchemaRef != "line.v1" {
			t.Fatalf("unexpected lines: %+v", got.Lines)
		}
		if len(got.Links) != 1 || got.Links[0].Metadata["relation"] != "reservation" {
			t.Fatalf("unexpected links: %+v", got.Links)
		}
		if len(got.Attachments) != 1 || got.Attachments[0].StorageKey != "obj://invoice.pdf" {
			t.Fatalf("unexpected attachments: %+v", got.Attachments)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithinTx failed: %v", err)
	}
}

func TestPostgresUnitOfWorkGetDocumentReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	txm := NewPostgresTransactionManager(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id, document_type, status, version, etag, organization_id,")).WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{
			"document_id", "document_type", "status", "version", "etag", "organization_id",
			"location_id", "number", "created_by", "created_at", "updated_by", "updated_at",
			"submitted_by", "submitted_at", "schema_version", "payload_json", "content_hash",
			"total_amount_minor", "total_amount_currency", "metadata_json",
		}))
	mock.ExpectRollback()

	if err := txm.WithinTx(context.Background(), func(uow UnitOfWork) error {
		_, err := uow.GetDocument("missing")
		return err
	}); err == nil {
		t.Fatal("expected not found error")
	}
}
