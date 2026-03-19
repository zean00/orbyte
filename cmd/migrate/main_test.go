package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMigrationFilesAndChecksum(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0020_test.sql"), []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatalf("write migration failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0010_test.sql"), []byte("SELECT 2;"), 0o644); err != nil {
		t.Fatalf("write migration failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write helper file failed: %v", err)
	}

	files, err := migrationFiles(dir)
	if err != nil {
		t.Fatalf("migrationFiles failed: %v", err)
	}
	if len(files) != 2 || !strings.HasSuffix(files[0], "0010_test.sql") || !strings.HasSuffix(files[1], "0020_test.sql") {
		t.Fatalf("unexpected migration files: %+v", files)
	}

	if checksumFor([]byte("SELECT 1;")) != checksumFor([]byte("SELECT 1;")) {
		t.Fatal("expected deterministic checksum")
	}
}

func TestEnsureAppliedStatusAndMigrateUp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations (")).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := ensureMigrationTable(db); err != nil {
		t.Fatalf("ensureMigrationTable failed: %v", err)
	}

	appliedRows := sqlmock.NewRows([]string{"filename", "checksum"}).AddRow("0010_test.sql", "abc")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT filename, checksum FROM schema_migrations")).WillReturnRows(appliedRows)
	applied, err := appliedMigrations(db)
	if err != nil {
		t.Fatalf("appliedMigrations failed: %v", err)
	}
	if applied["0010_test.sql"] != "abc" {
		t.Fatalf("unexpected applied migrations: %+v", applied)
	}

	dir := t.TempDir()
	content := []byte("SELECT 42;")
	file := filepath.Join(dir, "0010_test.sql")
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatalf("write migration failed: %v", err)
	}
	checksum := checksumFor(content)

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT filename, checksum FROM schema_migrations")).WillReturnRows(sqlmock.NewRows([]string{"filename", "checksum"}))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT 42;")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations (")).
		WithArgs("0010_test.sql", checksum, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := migrateUp(db, dir); err != nil {
		t.Fatalf("migrateUp failed: %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations (")).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT filename, checksum FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"filename", "checksum"}).AddRow("0010_test.sql", checksum))
	output := captureStdout(t, func() {
		if err := printStatus(db, dir); err != nil {
			t.Fatalf("printStatus failed: %v", err)
		}
	})
	if !strings.Contains(output, "applied 0010_test.sql") {
		t.Fatalf("unexpected printStatus output: %q", output)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout failed: %v", err)
	}
	return buf.String()
}
