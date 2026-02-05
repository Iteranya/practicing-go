package database

import (
	"context"
	"database/sql"
	"fmt"
)

// TxManager handles the execution of functions within a database transaction.
// Currently Unused
type TxManager interface {
	// Run executes the given function within a transaction.
	// The function receives a context and a SQLClient (the transaction).
	// Repositories should be re-initialized or updated to use this client inside the function.
	Run(ctx context.Context, fn func(ctx context.Context, client SQLClient) error) error
}

type txManager struct {
	db *sql.DB
}

func NewTxManager(db *sql.DB) TxManager {
	return &txManager{db: db}
}

// Run starts a transaction. If the function returns nil, it commits.
// If the function returns an error or panics, it rolls back.
func (tm *txManager) Run(ctx context.Context, fn func(ctx context.Context, txClient SQLClient) error) (err error) {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Safety: Recover from panics to ensure rollback
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // Re-throw panic after rollback
		} else if err != nil {
			_ = tx.Rollback() // Rollback on error
		} else {
			// Commit if no error and no panic
			if commitErr := tx.Commit(); commitErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)
			}
		}
	}()

	// Execute the business logic with the transaction client
	err = fn(ctx, tx)
	return err
}
