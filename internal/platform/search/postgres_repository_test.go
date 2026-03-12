package search

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositorySaveAndListDocuments(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO search_document_summaries (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveDocument(DocumentSummary{
		DocumentID:     "d1",
		DocumentType:   "generic_request",
		Status:         "submitted",
		Version:        2,
		ETag:           "d1:2",
		OrganizationID: "org_default",
		LocationID:     "loc_hq",
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("save document summary failed: %v", err)
	}

	rows := sqlmock.NewRows([]string{"document_id", "document_type", "status", "version", "etag", "organization_id", "location_id", "updated_at"}).
		AddRow("d1", "generic_request", "submitted", 2, "d1:2", "org_default", "loc_hq", now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id, document_type, status, version, etag, organization_id, COALESCE(location_id, ''), updated_at")).WillReturnRows(rows)
	items := repo.ListDocuments()
	if len(items) != 1 {
		t.Fatalf("expected one summary, got %d", len(items))
	}
}
