package store

import (
	"context"
	"database/sql"
)

type TransactionManager interface {
	WithinTx(ctx context.Context, fn func(Tx) error) error
}

type PostgresTransactionManager struct {
	db DB
}

func NewPostgresTransactionManager(db *sql.DB) *PostgresTransactionManager {
	return NewPostgresTransactionManagerWithDB(UninstrumentedDB(db))
}

func NewPostgresTransactionManagerWithDB(db DB) *PostgresTransactionManager {
	return &PostgresTransactionManager{db: db}
}

func (m *PostgresTransactionManager) WithinTx(ctx context.Context, fn func(Tx) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
