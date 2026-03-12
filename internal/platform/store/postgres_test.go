package store

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestOpenFromEnvWithoutDatabaseURL(t *testing.T) {
	old := os.Getenv("DATABASE_URL")
	defer os.Setenv("DATABASE_URL", old)
	_ = os.Unsetenv("DATABASE_URL")
	p, err := OpenFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != nil {
		t.Fatal("expected nil postgres when DATABASE_URL missing")
	}
}

func TestCloseNilPostgres(t *testing.T) {
	var p *Postgres
	if err := p.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestClosePostgresWithDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	mock.ExpectClose()
	p := &Postgres{DB: db}
	if err := p.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestPostgresTransactionManagerWithinTxCommits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	txm := NewPostgresTransactionManager(db)
	mock.ExpectBegin()
	mock.ExpectCommit()

	if err := txm.WithinTx(t.Context(), func(tx *sql.Tx) error { return nil }); err != nil {
		t.Fatalf("WithinTx returned error: %v", err)
	}
}

func TestPostgresTransactionManagerWithinTxRollsBackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	txm := NewPostgresTransactionManager(db)
	mock.ExpectBegin()
	mock.ExpectRollback()

	expected := errors.New("boom")
	if err := txm.WithinTx(t.Context(), func(tx *sql.Tx) error { return expected }); !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}
