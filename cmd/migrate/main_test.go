package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMigrationFilesAndChecksum(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"002_second.sql": "SELECT 2;",
		"001_first.sql":  "SELECT 1;",
		"README.md":      "ignored",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("seed migration file %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("seed subdir: %v", err)
	}
	list, err := migrationFiles(dir)
	if err != nil {
		t.Fatalf("migrationFiles failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 sql files, got %+v", list)
	}
	if filepath.Base(list[0]) != "001_first.sql" || filepath.Base(list[1]) != "002_second.sql" {
		t.Fatalf("expected sorted sql files, got %+v", list)
	}
	if a, b := checksumFor([]byte("same")), checksumFor([]byte("same")); a != b {
		t.Fatalf("expected stable checksum, got %q vs %q", a, b)
	}
	if checksumFor([]byte("same")) == checksumFor([]byte("different")) {
		t.Fatal("expected different checksums for different content")
	}
}

func TestEnsureMigrationTableAndAppliedMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := ensureMigrationTable(db); err != nil {
		t.Fatalf("ensureMigrationTable failed: %v", err)
	}

	rows := sqlmock.NewRows([]string{"filename", "checksum"}).
		AddRow("001_first.sql", "abc").
		AddRow("002_second.sql", "def")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT filename, checksum FROM schema_migrations")).
		WillReturnRows(rows)
	applied, err := appliedMigrations(db)
	if err != nil {
		t.Fatalf("appliedMigrations failed: %v", err)
	}
	if len(applied) != 2 || applied["001_first.sql"] != "abc" || applied["002_second.sql"] != "def" {
		t.Fatalf("unexpected applied migrations map: %+v", applied)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPrintStatus(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"001_first.sql", "002_second.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o644); err != nil {
			t.Fatalf("seed migration file %s: %v", name, err)
		}
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	rows := sqlmock.NewRows([]string{"filename", "checksum"}).AddRow("001_first.sql", "abc")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT filename, checksum FROM schema_migrations")).
		WillReturnRows(rows)

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	defer reader.Close()
	os.Stdout = writer
	defer func() { os.Stdout = originalStdout }()

	if err := printStatus(db, dir); err != nil {
		t.Fatalf("printStatus failed: %v", err)
	}
	_ = writer.Close()
	buf := make([]byte, 4096)
	n, _ := reader.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "applied 001_first.sql") || !strings.Contains(output, "pending 002_second.sql") {
		t.Fatalf("unexpected printStatus output: %s", output)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
