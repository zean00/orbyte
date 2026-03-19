package reference

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepositoryTypesAndRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)
	now := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO reference_type_definitions (reference_type_key, display_name, owner_module_key, value_type, allowed_scopes_json)")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveType(TypeDefinition{Key: "currency", DisplayName: "Currency", OwnerModuleKey: "finance", ValueType: "json", AllowedScopes: []string{"deployment", "location"}}); err != nil {
		t.Fatalf("save type failed: %v", err)
	}

	typeRows := sqlmock.NewRows([]string{"display_name", "owner_module_key", "value_type", "allowed_scopes_json"}).
		AddRow("Currency", "finance", "json", []byte(`["deployment","location"]`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT display_name, COALESCE(owner_module_key,''), COALESCE(value_type,''), allowed_scopes_json FROM reference_type_definitions WHERE reference_type_key = $1")).
		WillReturnRows(typeRows)
	typ, ok := repo.GetType("currency")
	if !ok || typ.Key != "currency" || len(typ.AllowedScopes) != 2 {
		t.Fatalf("unexpected type lookup ok=%v type=%+v", ok, typ)
	}

	listTypeRows := sqlmock.NewRows([]string{"reference_type_key", "display_name", "owner_module_key", "value_type", "allowed_scopes_json"}).
		AddRow("currency", "Currency", "finance", "json", []byte(`["deployment","location"]`))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT reference_type_key, display_name, COALESCE(owner_module_key,''), COALESCE(value_type,''), allowed_scopes_json FROM reference_type_definitions")).
		WillReturnRows(listTypeRows)
	if items := repo.ListTypes(); len(items) != 1 || items[0].Key != "currency" {
		t.Fatalf("unexpected types list: %+v", items)
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO reference_records (reference_type_key, reference_key, display_name, scope_type, scope_id, status, value_json, external_codes_json, effective_from, effective_to, updated_at, updated_by)")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.SaveRecord(Record{
		TypeKey:       "currency",
		Key:           "IDR",
		DisplayName:   "Rupiah",
		Scope:         "location",
		ScopeID:       "loc_hq",
		Status:        "active",
		Value:         map[string]any{"symbol": "Rp"},
		ExternalCodes: map[string]string{"iso": "IDR"},
		UpdatedAt:     now,
		UpdatedBy:     "user_admin",
	}); err != nil {
		t.Fatalf("save record failed: %v", err)
	}

	recordRows := sqlmock.NewRows([]string{"reference_key", "display_name", "scope_type", "scope_id", "status", "value_json", "external_codes_json", "effective_from", "effective_to", "updated_at", "updated_by"}).
		AddRow("IDR", "Rupiah", "location", "loc_hq", "active", []byte(`{"symbol":"Rp"}`), []byte(`{"iso":"IDR"}`), time.Time{}, time.Time{}, now, "user_admin")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT reference_key, display_name, scope_type, COALESCE(scope_id,''), status, value_json, external_codes_json, effective_from, effective_to, updated_at, COALESCE(updated_by,'') FROM reference_records WHERE reference_type_key = $1")).
		WillReturnRows(recordRows)
	records := repo.ListRecords("currency")
	if len(records) != 1 || records[0].Key != "IDR" || records[0].Value["symbol"] != "Rp" {
		t.Fatalf("unexpected records: %+v", records)
	}

	if !zeroTime(time.Time{}).IsZero() {
		t.Fatal("expected zeroTime to preserve zero values")
	}
	if zeroTime(now).IsZero() {
		t.Fatal("expected zeroTime to preserve non-zero values")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
