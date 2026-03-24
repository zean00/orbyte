package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"orbyte/internal/platform/runtimeconfig"
)

type Postgres struct {
	DB *sql.DB
}

func OpenFromEnv() (*Postgres, error) {
	dsn := runtimeconfig.Current().DatabaseURL()
	if dsn == "" {
		return nil, nil
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres connection: %w", err)
	}
	configureDBPool(db)
	return &Postgres{DB: db}, nil
}

func (p *Postgres) Close() error {
	if p == nil || p.DB == nil {
		return nil
	}
	return p.DB.Close()
}

func configureDBPool(db *sql.DB) {
	if db == nil {
		return
	}
	runtime := runtimeconfig.Current()
	db.SetMaxOpenConns(runtime.DBMaxOpenConns())
	db.SetMaxIdleConns(runtime.DBMaxIdleConns())
	db.SetConnMaxLifetime(runtime.DBConnMaxLifetime())
	db.SetConnMaxIdleTime(runtime.DBConnMaxIdleTime())
}

func envInt(key string, fallback int) int {
	return runtimeconfig.IntFromEnv(key, fallback)
}

func envDurationSeconds(key string, fallback time.Duration) time.Duration {
	return runtimeconfig.DurationSecondsFromEnv(key, fallback)
}
