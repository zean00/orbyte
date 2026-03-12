package model

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositoryDefinitionAndRecordLifecycle(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO model_definitions (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveDefinition(Definition{
		Key: "party", DisplayName: "Party", Version: "v1", OwnerModuleKey: "masterdata", DefaultSort: "name",
		Fields: []FieldDefinition{{Key: "name", Type: "string"}}, Relations: []RelationDefinition{{Key: "contacts", Type: "has_many", TargetModelKey: "party_contact", ForeignKey: "party_id"}},
	}); err != nil {
		t.Fatalf("save definition failed: %v", err)
	}

	rows := sqlmock.NewRows([]string{"model_key", "display_name", "owner_module_key", "version_key", "create_permission_key", "list_permission_key", "read_permission_key", "update_permission_key", "default_sort", "fields_json", "relations_json"}).
		AddRow("party", "Party", "masterdata", "v1", "", "", "", "", "name", []byte(`[{"key":"name","label":"","type":"string"}]`), []byte(`[{"key":"contacts","type":"has_many","target_model_key":"party_contact","foreign_key":"party_id"}]`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT model_key, display_name, COALESCE(owner_module_key,''), version_key, COALESCE(create_permission_key,''), COALESCE(list_permission_key,''), COALESCE(read_permission_key,''), COALESCE(update_permission_key,''), COALESCE(default_sort,''), fields_json, relations_json FROM model_definitions WHERE model_key = $1")).
		WithArgs("party").WillReturnRows(rows)
	def, ok := repo.GetDefinition("party")
	if !ok || len(def.Relations) != 1 || def.DefaultSort != "name" {
		t.Fatalf("expected definition with relations, got %+v ok=%v", def, ok)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO model_records (")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveRecord(Record{ModelKey: "party", ID: "p1", Version: 1, Values: map[string]any{"name": "Alice"}, CreatedBy: "u1", CreatedAt: now, UpdatedBy: "u1", UpdatedAt: now}); err != nil {
		t.Fatalf("save record failed: %v", err)
	}

	recordRows := sqlmock.NewRows([]string{"model_key", "record_id", "version", "values_json", "created_by", "created_at", "updated_by", "updated_at"}).
		AddRow("party", "p1", 1, []byte(`{"name":"Alice"}`), "u1", now, "u1", now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT model_key, record_id, version, values_json, created_by, created_at, updated_by, updated_at FROM model_records WHERE model_key = $1 AND record_id = $2")).
		WithArgs("party", "p1").WillReturnRows(recordRows)
	record, ok := repo.GetRecord("party", "p1")
	if !ok || record.Values["name"] != "Alice" {
		t.Fatalf("expected record, got %+v ok=%v", record, ok)
	}
}

func TestPostgresRepositoryQueryRecords(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewPostgresRepository(db)
	now := time.Now().UTC()
	def := Definition{
		Key:         "party",
		DisplayName: "Party",
		Version:     "v1",
		DefaultSort: "name",
		Fields:      []FieldDefinition{{Key: "name", Type: "string"}, {Key: "status", Type: "string"}},
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM model_records WHERE model_key = $1 AND COALESCE(values_json->>'status','') ILIKE $2")).
		WithArgs("party", "%active%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT model_key, record_id, version, values_json, created_by, created_at, updated_by, updated_at FROM model_records WHERE model_key = $1 AND COALESCE(values_json->>'status','') ILIKE $2 ORDER BY LOWER(COALESCE(values_json->>'name','')) ASC LIMIT $3 OFFSET $4")).
		WithArgs("party", "%active%", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"model_key", "record_id", "version", "values_json", "created_by", "created_at", "updated_by", "updated_at"}).
			AddRow("party", "p1", 1, []byte(`{"name":"Alice","status":"active"}`), "u1", now, "u1", now))
	items, total, err := repo.QueryRecords(def, Query{Filters: map[string]string{"status": "active"}, SortKey: "name", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("query records failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Values["name"] != "Alice" {
		t.Fatalf("expected queried records, got total=%d items=%+v", total, items)
	}
}
