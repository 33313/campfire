// Package store owns the database: the connection pools, the schema, and
// the small set of primitives every other package builds on.
//
// The application runs against one embedded SQLite file (SPEC.md §13). Two
// pools are opened over it:
//
//	W  a single connection, so there is exactly one writer at any moment
//	R  several connections for reads, which WAL serves concurrently
//
// The single writer is not a performance choice. The audit log's sequence
// number is the resume cursor for live updates (SPEC.md §10, §12.2), and that
// is only safe while events commit in sequence order — which is what one
// writer guarantees. Appendix C spells out what breaks without it.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

//go:embed schema.sql
var schema string

// DB is an open Campfire database.
type DB struct {
	W    *sql.DB // one connection: the single writer
	R    *sql.DB // read-only pool
	Path string
}

// Now returns the current time as epoch microseconds.
//
// Every timestamp the application stores comes from here. Database defaults
// and now() are not used (SPEC.md §13.2 rule 3), so that a row's time is set
// by the code that decided to write it.
func Now() int64 { return time.Now().UnixMicro() }

func dsn(path string, readonly bool) string {
	pragmas := []string{
		"busy_timeout(10000)",
		"foreign_keys(1)",
		"journal_mode(WAL)",
		"synchronous(NORMAL)",
	}
	if readonly {
		// A reader must not create the file or upgrade the journal mode.
		pragmas = []string{"busy_timeout(10000)", "foreign_keys(1)"}
	}
	q := url.Values{}
	for _, p := range pragmas {
		q.Add("_pragma", p)
	}
	if readonly {
		q.Set("mode", "ro")
	}
	return "file:" + path + "?" + q.Encode()
}

// Open opens (creating if absent) the database at path, checks its integrity,
// and gives a new, empty file the schema. It refuses a file that needs
// migrating; OpenMigrating migrates it.
func Open(path string) (*DB, error) { return OpenMigrating(path, nil) }

// OpenMigrating is Open that also brings a file older than the binary up to
// the current schema. snapshot is called first, with the file's version; if it
// fails, or is nil, nothing is migrated.
func OpenMigrating(path string, snapshot func(db *DB, from int) error) (*DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	w, err := sql.Open("sqlite", dsn(abs, false))
	if err != nil {
		return nil, err
	}
	// One writer. See the package comment.
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(0)

	if err := w.Ping(); err != nil {
		w.Close()
		return nil, fmt.Errorf("open %s: %w", abs, err)
	}

	db := &DB{W: w, Path: abs}
	if err := db.checkIntegrity(); err != nil {
		w.Close()
		return nil, err
	}
	if err := db.create(); err != nil {
		w.Close()
		return nil, err
	}
	if err := db.migrate(snapshot); err != nil {
		w.Close()
		return nil, err
	}

	r, err := sql.Open("sqlite", dsn(abs, true))
	if err != nil {
		w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(8)
	r.SetMaxIdleConns(8)
	if err := r.Ping(); err != nil {
		w.Close()
		r.Close()
		return nil, err
	}
	db.R = r
	return db, nil
}

func (d *DB) Close() error {
	var errs []error
	if d.R != nil {
		errs = append(errs, d.R.Close())
	}
	if d.W != nil {
		errs = append(errs, d.W.Close())
	}
	return errors.Join(errs...)
}

// CheckIntegrity runs SQLite's own integrity check. It is run at startup and
// periodically; on failure the server refuses to start and names the snapshot
// to restore from (SPEC.md §13.3 rule 5).
func (d *DB) CheckIntegrity() error { return d.checkIntegrity() }

func (d *DB) checkIntegrity() error {
	var result string
	if err := d.W.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("database integrity check failed: %s", result)
	}
	return nil
}

// create applies the schema to a file with no tables, and stamps it with the
// current version. A file that has tables is left to migrate.
func (d *DB) create() error {
	var tables int
	if err := d.W.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table'`).Scan(&tables); err != nil {
		return err
	}
	if tables > 0 {
		return nil
	}
	tx, err := d.W.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(schema); err != nil {
		tx.Rollback()
		return fmt.Errorf("schema: %w", err)
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, Version())); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// migrations[i] takes a file from schema version i+1 to i+2. Append only: one
// that has shipped is never edited. Before adding one, copy schema.sql to
// testdata/v<its from version>.sql, then change schema.sql to match.
var migrations []string

// Version is the schema version this binary writes. A file records its own in
// PRAGMA user_version; a file with none (0) has the v1 schema, version 1.
func Version() int { return 1 + len(migrations) }

// TooNewError is a file written by a newer binary. It is refused rather than
// opened and left to fail at query time.
type TooNewError struct {
	Path         string
	File, Binary int
}

func (e *TooNewError) Error() string {
	return fmt.Sprintf("%s has schema version %d, newer than this binary's %d", e.Path, e.File, e.Binary)
}

// FileVersion reads the schema version recorded in the file at path.
func FileVersion(path string) (int, error) {
	db, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		return 0, err
	}
	defer db.Close()
	return userVersion(db)
}

func userVersion(db *sql.DB) (int, error) {
	var v int
	err := db.QueryRow(`PRAGMA user_version`).Scan(&v)
	return v, err
}

// migrate brings the file up to Version() in one transaction. There are no
// down-migrations: going back is restoring the snapshot taken first.
func (d *DB) migrate(snapshot func(*DB, int) error) error {
	from, err := userVersion(d.W)
	if err != nil {
		return err
	}
	switch {
	case from == Version():
		return nil
	case from > Version():
		return &TooNewError{Path: d.Path, File: from, Binary: Version()}
	case snapshot == nil:
		return fmt.Errorf("%s has schema version %d and needs migrating to %d", d.Path, from, Version())
	}
	if err := snapshot(d, from); err != nil {
		return fmt.Errorf("not migrating %s: snapshot failed: %w", d.Path, err)
	}

	// SQLite's procedure for changing a table: with foreign keys off, dropping
	// a rebuilt table does not cascade into its children, and the check before
	// commit holds what they would have. The pragma does nothing inside a
	// transaction, so it is set on the connection first.
	ctx := context.Background()
	conn, err := d.W.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`)

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for v := max(from, 1); v < Version(); v++ {
		if _, err := tx.Exec(migrations[v-1]); err != nil {
			return fmt.Errorf("migrating %s from version %d: %w", d.Path, v, err)
		}
	}
	rows, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	broken := rows.Next()
	rows.Close()
	if broken {
		return fmt.Errorf("migrating %s left rows pointing at nothing", d.Path)
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, Version())); err != nil {
		return err
	}
	return tx.Commit()
}

// Write runs fn inside a write transaction, retrying when SQLite reports the
// database is busy.
//
// Retry on conflict is one of the three sanctioned ways to hold an invariant
// (SPEC.md §13.2 rule 1); the other two — a single atomic statement and a
// database constraint — are used directly by callers.
func (d *DB) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	const attempts = 5
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		err = d.writeOnce(ctx, fn)
		if !IsBusy(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 10 * time.Millisecond):
		}
	}
	return err
}

func (d *DB) writeOnce(ctx context.Context, fn func(*sql.Tx) error) (err error) {
	tx, err := d.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
		if err != nil {
			tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func sqliteCode(err error) int {
	var se *sqlite.Error
	if errors.As(err, &se) {
		return se.Code()
	}
	return 0
}

// IsBusy reports whether err is SQLite refusing because the file is locked.
func IsBusy(err error) bool {
	switch sqliteCode(err) & 0xff {
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		return true
	}
	return false
}

// IsUniqueViolation reports whether err is a broken UNIQUE constraint.
//
// Callers use it to turn a constraint into behaviour: a duplicate username is
// a validation error, while a duplicate (state_id, rank) is a lost race that
// should be retried with a freshly computed midpoint (SPEC.md §9.3).
func IsUniqueViolation(err error) bool {
	return sqliteCode(err) == sqlite3.SQLITE_CONSTRAINT_UNIQUE ||
		sqliteCode(err) == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}

// ConstraintIndex names the index a unique violation was raised against, or ""
// when err is not a unique violation. SQLite reports it in the message as
// "UNIQUE constraint failed: table.column, ...".
func ConstraintIndex(err error) string {
	if !IsUniqueViolation(err) {
		return ""
	}
	const marker = "UNIQUE constraint failed: "
	msg := err.Error()
	i := strings.Index(msg, marker)
	if i < 0 {
		return ""
	}
	names := msg[i+len(marker):]
	// The driver appends the numeric result code, e.g. "labels.name_folded (2067)".
	if j := strings.LastIndex(names, " ("); j >= 0 {
		names = names[:j]
	}
	return strings.TrimSpace(names)
}
