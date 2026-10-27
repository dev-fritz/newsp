package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// DBTX is the subset of *sql.DB and *sql.Tx that repositories use.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKey struct{}

// Conn returns the transaction carried by ctx, or db when there is none.
// Repositories call it so they join a transaction opened by a service.
func Conn(ctx context.Context, db *sql.DB) DBTX {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return db
}

// TxManager opens transactions whose boundary is decided by services.
type TxManager struct {
	db *sql.DB
}

// NewTxManager builds a TxManager.
func NewTxManager(db *sql.DB) *TxManager {
	return &TxManager{db: db}
}

// WithinTx runs fn inside a transaction propagated through the context.
// Nested calls reuse the outer transaction. fn's error rolls back.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(ctx)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				err = errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
			}
		}
	}()

	if err = fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
