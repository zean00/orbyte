package document

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"orbyte/internal/platform/shared"
)

func TestPostgresRepositoryDefinitionLifecycle(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_definitions (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveDefinition(Definition{Type: "x", DisplayName: "X", SchemaVersion: "v1", OwnerModuleKey: "documents", AllowedLinkTypes: []string{"related_to"}, AllowedAttachmentTypes: []string{"document"}}); err != nil {
		t.Fatalf("save definition failed: %v", err)
	}

	rows := sqlmock.NewRows([]string{"document_type", "display_name", "schema_version", "workflow_key", "numbering_key", "owner_module_key", "allowed_link_types_json", "allowed_attachment_types_json"}).AddRow("x", "X", "v1", "wf", "", "documents", []byte(`["related_to"]`), []byte(`["document"]`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_type, display_name, schema_version, COALESCE(workflow_key, ''), COALESCE(numbering_key, ''), COALESCE(owner_module_key, ''),")).WillReturnRows(rows)
	def, ok := repo.GetDefinition("x")
	if !ok {
		t.Fatal("expected definition")
	}
	if len(def.AllowedLinkTypes) != 1 || len(def.AllowedAttachmentTypes) != 1 || def.OwnerModuleKey != "documents" {
		t.Fatal("expected allowed types")
	}

	listRows := sqlmock.NewRows([]string{"document_type", "display_name", "schema_version", "workflow_key", "numbering_key", "owner_module_key", "allowed_link_types_json", "allowed_attachment_types_json"}).AddRow("x", "X", "v1", "wf", "", "documents", []byte(`["related_to"]`), []byte(`["document"]`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_type, display_name, schema_version, COALESCE(workflow_key, ''), COALESCE(numbering_key, ''), COALESCE(owner_module_key, ''),")).WillReturnRows(listRows)
	if len(repo.ListDefinitions()) != 1 {
		t.Fatal("expected list definitions")
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_extension_definitions (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveExtensionDefinition(ExtensionDefinition{DocumentType: "x", ModuleKey: "analytics", DisplayName: "Analytics", SchemaVersion: "v1"}); err != nil {
		t.Fatalf("save extension definition failed: %v", err)
	}
	extRows := sqlmock.NewRows([]string{"document_type", "module_key", "display_name", "schema_version", "read_permission_key", "write_permission_key"}).AddRow("x", "analytics", "Analytics", "v1", "", "")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_type, module_key, display_name, schema_version, COALESCE(read_permission_key, ''), COALESCE(write_permission_key, '')")).WithArgs("x").WillReturnRows(extRows)
	if len(repo.ListExtensionDefinitions("x")) != 1 {
		t.Fatal("expected extension definitions")
	}
}

func TestPostgresRepositoryRecordLifecycle(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	record := Record{Header: Header{ID: "d1", Type: "x", Status: "draft", Version: 1, ETag: "d1:1", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}}, Body: Body{SchemaVersion: "v1", Payload: map[string]any{"a": 1}}}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveRecord(record); err != nil {
		t.Fatalf("save record failed: %v", err)
	}

	getRows := sqlmock.NewRows([]string{"document_id", "document_type", "status", "version", "etag", "organization_id", "location_id", "number", "created_by", "created_at", "updated_by", "updated_at", "submitted_by", "submitted_at", "schema_version", "payload_json", "content_hash", "total_amount_minor", "total_amount_currency", "metadata_json"}).AddRow("d1", "x", "draft", 1, "d1:1", "org", "", "", "u1", now, "u1", now, "", nil, "v1", []byte(`{"a":1}`), "", 0, "IDR", []byte(`{"flow_key":"x"}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id, document_type, status, version, etag, organization_id,")).WillReturnRows(getRows)
	record, found := repo.GetRecord("d1")
	if !found {
		t.Fatal("expected record")
	}
	if record.Header.Metadata["flow_key"] != "x" {
		t.Fatalf("expected record metadata, got %+v", record.Header.Metadata)
	}

	listRows := sqlmock.NewRows([]string{"document_id", "document_type", "status", "version", "etag", "organization_id", "location_id", "number", "created_by", "created_at", "updated_by", "updated_at", "submitted_by", "submitted_at", "schema_version", "payload_json", "content_hash", "total_amount_minor", "total_amount_currency", "metadata_json"}).AddRow("d1", "x", "draft", 1, "d1:1", "org", "", "", "u1", now, "u1", now, "", nil, "v1", []byte(`{"a":1}`), "", 0, "IDR", []byte(`{"flow_key":"x"}`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id, document_type, status, version, etag, organization_id,")).WillReturnRows(listRows)
	if len(repo.ListRecords()) != 1 {
		t.Fatal("expected list records")
	}
}

func TestPostgresRepositoryRelatedCollections(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM document_lines WHERE document_id = $1")).WithArgs("d1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_lines (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.SaveLines("d1", []Line{{ID: "l1", DocumentID: "d1", LineNo: 1, LineType: "service", Payload: map[string]any{"code": "A"}, Amount: shared.Money{Currency: "IDR"}}}); err != nil {
		t.Fatalf("save lines failed: %v", err)
	}

	lineRows := sqlmock.NewRows([]string{"document_line_id", "document_id", "line_no", "line_type", "line_schema_ref", "payload_json", "amount_minor", "amount_currency"}).AddRow("l1", "d1", 1, "service", "", []byte(`{"code":"A"}`), 0, "IDR")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_line_id, document_id, line_no, line_type, COALESCE(line_schema_ref, ''), payload_json, amount_minor, COALESCE(amount_currency, '')")).WithArgs("d1").WillReturnRows(lineRows)
	if len(repo.ListLines("d1")) != 1 {
		t.Fatal("expected list lines")
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM document_links WHERE document_id = $1")).WithArgs("d1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_links (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.SaveLinks("d1", []Link{{ID: "k1", DocumentID: "d1", LinkedDocumentID: "d2", LinkType: "related_to", CreatedAt: now}}); err != nil {
		t.Fatalf("save links failed: %v", err)
	}

	linkRows := sqlmock.NewRows([]string{"link_id", "document_id", "linked_document_id", "link_type", "metadata_json", "created_at"}).AddRow("k1", "d1", "d2", "related_to", []byte(`{"a":1}`), now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT link_id, document_id, linked_document_id, link_type, metadata_json, created_at")).WithArgs("d1").WillReturnRows(linkRows)
	if len(repo.ListLinks("d1")) != 1 {
		t.Fatal("expected list links")
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM document_attachments WHERE document_id = $1")).WithArgs("d1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_attachments (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.SaveAttachments("d1", []Attachment{{ID: "a1", DocumentID: "d1", AttachmentType: "document", FileName: "x.pdf", ContentType: "application/pdf", StorageKey: "s3://x.pdf", SizeBytes: 10, CreatedAt: now}}); err != nil {
		t.Fatalf("save attachments failed: %v", err)
	}

	attachmentRows := sqlmock.NewRows([]string{"attachment_id", "document_id", "attachment_type", "file_name", "content_type", "storage_key", "size_bytes", "created_at"}).AddRow("a1", "d1", "document", "x.pdf", "application/pdf", "s3://x.pdf", 10, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT attachment_id, document_id, attachment_type, file_name, content_type, storage_key, size_bytes, created_at")).WithArgs("d1").WillReturnRows(attachmentRows)
	if len(repo.ListAttachments("d1")) != 1 {
		t.Fatal("expected list attachments")
	}
}
