package application

import (
	"regexp"
	"testing"
	"time"

	"clinic/internal/platform/audit"
	"clinic/internal/platform/document"
	"clinic/internal/platform/eventing"
	"clinic/internal/platform/model"
	"clinic/internal/platform/shared"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestKernelActionsCreateDocumentAndModelBundleCommitsCrossKernelWrites(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	models := model.NewService()
	if err := models.Register(model.Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		Fields:      []model.FieldDefinition{{Key: "name", Type: "string", Required: true}},
	}); err != nil {
		t.Fatalf("register model failed: %v", err)
	}
	actions := NewPostgresKernelActions(db, document.NewService(), models, audit.NewService(), eventing.NewService())
	now := time.Now().UTC()
	record := document.Record{
		Header: document.Header{ID: "doc-1", Type: "generic_request", Status: "draft", Version: 1, ETag: "doc-1:1", OrganizationID: "org", CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now, TotalAmount: shared.Money{Currency: "IDR"}},
		Body:   document.Body{SchemaVersion: "v1", Payload: map[string]any{"title": "Bundle"}},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT model_key, display_name, COALESCE(owner_module_key,''), version_key, COALESCE(create_permission_key,''), COALESCE(list_permission_key,''), COALESCE(read_permission_key,''), COALESCE(update_permission_key,''), COALESCE(default_sort,''), fields_json, relations_json FROM model_definitions WHERE model_key = $1")).
		WithArgs("party").
		WillReturnRows(sqlmock.NewRows([]string{"model_key", "display_name", "owner_module_key", "version_key", "create_permission_key", "list_permission_key", "read_permission_key", "update_permission_key", "default_sort", "fields_json", "relations_json"}).
			AddRow("party", "Party", "", "v1", "", "", "", "", "", []byte(`[{"key":"name","label":"","type":"string","required":true}]`), []byte(`[]`)))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO model_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_events (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_records (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	_, modelRecord, _, err := actions.CreateDocumentAndModelBundle(record, "party", "u1", model.CompositeMutation{
		Values: map[string]any{"name": "Acme Clinic"},
	})
	if err != nil {
		t.Fatalf("create bundle failed: %v", err)
	}
	if modelRecord.ModelKey != "party" || modelRecord.Values["name"] != "Acme Clinic" {
		t.Fatalf("unexpected model record: %+v", modelRecord)
	}
}
