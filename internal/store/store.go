// Package store owns the database: the connection pools, the migrations, and
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
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

//go:embed migrations/*.sql
var migrations embed.FS

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
// and applies any pending migrations.
func Open(path string) (*DB, error) {
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
	if err := db.migrate(); err != nil {
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

// migrate applies every embedded migration whose ordinal is above the file's
// PRAGMA user_version, each in its own transaction.
func (d *DB) migrate() error {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var version int
	if err := d.W.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}

	for i, name := range names {
		ordinal := i + 1
		if ordinal <= version {
			continue
		}
		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := d.W.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		// PRAGMA takes no bound parameters, and ordinal is a loop index.
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, ordinal)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
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
