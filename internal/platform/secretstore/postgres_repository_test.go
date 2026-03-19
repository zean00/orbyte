package secretstore

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
	secret := Secret{
		Ref:       "secret://google-client",
		Name:      "Google Client Secret",
		Value:     "super-secret",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO secret_store (secret_ref, name, secret_value, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (secret_ref) DO UPDATE SET name = EXCLUDED.name, secret_value = EXCLUDED.secret_value, status = EXCLUDED.status, updated_at = EXCLUDED.updated_at")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.Save(secret); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	getRows := sqlmock.NewRows([]string{"secret_ref", "name", "secret_value", "status", "created_at", "updated_at"}).
		AddRow(secret.Ref, secret.Name, secret.Value, secret.Status, now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT secret_ref, name, secret_value, status, created_at, updated_at FROM secret_store WHERE secret_ref = $1")).
		WillReturnRows(getRows)
	got, ok := repo.Get(secret.Ref)
	if !ok || got.Ref != secret.Ref || got.Value != secret.Value {
		t.Fatalf("unexpected get result ok=%v secret=%+v", ok, got)
	}

	listRows := sqlmock.NewRows([]string{"secret_ref", "name", "secret_value", "status", "created_at", "updated_at"}).
		AddRow(secret.Ref, secret.Name, "", secret.Status, now, now)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT secret_ref, name, '' AS secret_value, status, created_at, updated_at FROM secret_store ORDER BY secret_ref ASC")).
		WillReturnRows(listRows)
	items := repo.List()
	if len(items) != 1 || items[0].Ref != secret.Ref || items[0].Value != "" {
		t.Fatalf("unexpected list result: %+v", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
