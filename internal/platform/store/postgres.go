package store

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Postgres struct {
	DB *sql.DB
}

func OpenFromEnv() (*Postgres, error) {
	dsn := os.Getenv("DATABASE_URL")
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
	db.SetMaxOpenConns(envInt("APP_DB_MAX_OPEN_CONNS", 25))
	db.SetMaxIdleConns(envInt("APP_DB_MAX_IDLE_CONNS", 25))
	db.SetConnMaxLifetime(envDurationSeconds("APP_DB_CONN_MAX_LIFETIME_SECONDS", time.Hour))
	db.SetConnMaxIdleTime(envDurationSeconds("APP_DB_CONN_MAX_IDLE_TIME_SECONDS", 15*time.Minute))
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func envDurationSeconds(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return fallback
	}
	return time.Duration(parsed) * time.Second
}
