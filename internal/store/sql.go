package store

import (
	"context"
	"database/sql"
)

// Q is what every query in this package takes: a *sql.DB or a *sql.Tx.
//
// It is the whole reason the service layer can compose several writes into
// one transaction without the store knowing anything about transactions.
type Q interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// null returns nil for the zero value so that an unset optional field is
// stored as NULL rather than as an empty string a unique index would collide
// on.
func null[T comparable](v T) any {
	var zero T
	if v == zero {
		return nil
	}
	return v
}

// Zero-valued Go fields stand in for SQL NULL throughout. Identifiers are
// never 0 and timestamps are never 0, so nothing is lost by the substitution
// and every struct stays free of pointers.
type nullID = sql.NullInt64

func id(n sql.NullInt64) int64 { return n.Int64 }

func text(n sql.NullString) string { return n.String }

func num(n sql.NullInt64) int64 { return n.Int64 }
