package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/store"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: migrate <up|status>")
	}
	postgres, err := store.OpenFromEnv()
	if err != nil {
		fatalf("open database: %v", err)
	}
	if postgres == nil || postgres.DB == nil {
		fatalf("DATABASE_URL is required")
	}
	defer func() { _ = postgres.Close() }()

	switch os.Args[1] {
	case "up":
		if err := migrateUp(postgres.DB, "migrations"); err != nil {
			fatalf("migrate up: %v", err)
		}
	case "status":
		if err := printStatus(postgres.DB, "migrations"); err != nil {
			fatalf("migration status: %v", err)
		}
	default:
		fatalf("unsupported command %q", os.Args[1])
	}
}

func migrateUp(db *sql.DB, dir string) error {
	if err := ensureMigrationTable(db); err != nil {
		return err
	}
	files, err := migrationFiles(dir)
	if err != nil {
		return err
	}
	applied, err := appliedMigrations(db)
	if err != nil {
		return err
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		name := filepath.Base(file)
		checksum := checksumFor(content)
		if applied[name] == checksum {
			fmt.Printf("skip %s\n", name)
			continue
		}
		if applied[name] != "" && applied[name] != checksum {
			return fmt.Errorf("migration checksum changed for %s", name)
		}
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO schema_migrations (filename, checksum, applied_at)
			VALUES ($1, $2, $3)
		`, name, checksum, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		fmt.Printf("applied %s\n", name)
	}
	return nil
}

func printStatus(db *sql.DB, dir string) error {
	if err := ensureMigrationTable(db); err != nil {
		return err
	}
	files, err := migrationFiles(dir)
	if err != nil {
		return err
	}
	applied, err := appliedMigrations(db)
	if err != nil {
		return err
	}
	for _, file := range files {
		name := filepath.Base(file)
		state := "pending"
		if applied[name] != "" {
			state = "applied"
		}
		fmt.Printf("%s %s\n", state, name)
	}
	return nil
}

func ensureMigrationTable(db *sql.DB) error {
	_, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL
		)
	`)
	return err
}

func appliedMigrations(db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(context.Background(), `SELECT filename, checksum FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := map[string]string{}
	for rows.Next() {
		var filename string
		var checksum string
		if err := rows.Scan(&filename, &checksum); err != nil {
			return nil, err
		}
		items[filename] = checksum
	}
	return items, nil
}

func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func checksumFor(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
