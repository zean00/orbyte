package store

import (
	"errors"
	"os"
	"testing"
	"time"

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

	if err := txm.WithinTx(t.Context(), func(tx Tx) error { return nil }); err != nil {
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
	if err := txm.WithinTx(t.Context(), func(tx Tx) error { return expected }); !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}

func TestConfigureDBPoolAppliesConfiguredLimits(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()
	t.Setenv("APP_DB_MAX_OPEN_CONNS", "17")
	t.Setenv("APP_DB_MAX_IDLE_CONNS", "9")
	t.Setenv("APP_DB_CONN_MAX_LIFETIME_SECONDS", "120")
	t.Setenv("APP_DB_CONN_MAX_IDLE_TIME_SECONDS", "45")

	configureDBPool(db)

	stats := db.Stats()
	if stats.MaxOpenConnections != 17 {
		t.Fatalf("expected max open conns 17, got %d", stats.MaxOpenConnections)
	}
	if got := envDurationSeconds("APP_DB_CONN_MAX_LIFETIME_SECONDS", 0); got != 120*time.Second {
		t.Fatalf("expected conn max lifetime 120s, got %s", got)
	}
	if got := envDurationSeconds("APP_DB_CONN_MAX_IDLE_TIME_SECONDS", 0); got != 45*time.Second {
		t.Fatalf("expected conn max idle time 45s, got %s", got)
	}
}
