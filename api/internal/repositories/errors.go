// Package repositories implements persistence with hand-written SQL.
package repositories

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors returned by repositories; services translate them.
var (
	ErrNotFound   = errors.New("not found")
	ErrDuplicate  = errors.New("duplicate")
	ErrReferenced = errors.New("still referenced")
)

// translate maps driver errors to repository sentinels, keeping the cause.
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return errors.Join(ErrDuplicate, err)
		case "23503":
			return errors.Join(ErrReferenced, err)
		}
	}
	return err
}
