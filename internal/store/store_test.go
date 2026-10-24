package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Testing opens a fresh database in the test's temporary directory.
func Testing(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "campfire.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// A new file gets the schema; a file with tables is opened as it is.
func TestOpenCreatesTheSchemaOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campfire.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	seedTeam(t, db)
	db.Close()

	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	var n int
	if err := again.R.QueryRow(`SELECT count(*) FROM teams`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("reopened with %d teams: %v", n, err)
	}
}

// A file at every past schema version opens, migrates, and ends up with the
// schema a new file gets. Past schemas are kept in testdata (see migrations).
func TestEveryPastVersionMigrates(t *testing.T) {
	want := shape(t, Testing(t))
	for v := 0; v < Version(); v++ {
		path := filepath.Join(t.TempDir(), "campfire.db")
		old, err := sql.Open("sqlite", dsn(path, false))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := old.Exec(schemaAt(t, max(v, 1))); err != nil {
			t.Fatalf("version %d schema: %v", v, err)
		}
		if _, err := old.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v)); err != nil {
			t.Fatal(err)
		}
		old.Close()

		snapshots := 0
		db, err := OpenMigrating(path, func(*DB, int) error { snapshots++; return nil })
		if err != nil {
			t.Fatalf("version %d: %v", v, err)
		}
		got, _ := userVersion(db.W)
		if same := shape(t, db) == want; snapshots != 1 || got != Version() || !same {
			t.Fatalf("version %d: %d snapshots, now version %d, schema matches a new file: %v", v, snapshots, got, same)
		}
		db.Close()
	}
}

// schemaAt is the schema a binary wrote at version v.
func schemaAt(t *testing.T, v int) string {
	t.Helper()
	if v == Version() {
		return schema
	}
	b, err := os.ReadFile(fmt.Sprintf("testdata/v%d.sql", v))
	if err != nil {
		t.Fatalf("no schema for version %d: copy schema.sql to testdata/v%d.sql before adding a migration from it", v, v)
	}
	return string(b)
}

// shape is the tables' columns, and the indexes and triggers: what a migration
// changes. ponytail: CHECK constraints are not compared; compare
// sqlite_master.sql too if one ever drifts.
func shape(t *testing.T, db *DB) string {
	t.Helper()
	rows, err := db.W.Query(`
		SELECT m.type || ' ' || m.name || coalesce(' ' || p.name || ' ' || p.type || ' ' || p."notnull" || ' ' || coalesce(p.dflt_value, '') || ' ' || p.pk, '')
		FROM sqlite_master m LEFT JOIN pragma_table_info(m.name) p ON m.type = 'table'
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// A migration runs only after its snapshot succeeds, in one transaction, and a
// file newer than the binary is refused.
func TestMigrationNeedsItsSnapshotAndRunsAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campfire.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	defer func(m []string) { migrations = m }(migrations)
	migrations = append(migrations, `ALTER TABLE labels ADD COLUMN colour TEXT`)
	colour := func() (n int) {
		db, _ := sql.Open("sqlite", dsn(path, true))
		defer db.Close()
		db.QueryRow(`SELECT count(*) FROM pragma_table_info('labels') WHERE name = 'colour'`).Scan(&n)
		return n
	}

	if _, err := Open(path); err == nil {
		t.Fatal("migrated without a snapshot")
	}
	if _, err := OpenMigrating(path, func(*DB, int) error { return errors.New("disk full") }); err == nil || colour() != 0 {
		t.Fatalf("migrated after the snapshot failed: %v", err)
	}

	migrations = append(migrations, `ALTER TABLE nowhere ADD COLUMN x TEXT`)
	if _, err := OpenMigrating(path, func(*DB, int) error { return nil }); err == nil || colour() != 0 {
		t.Fatalf("a failed migration left half its changes: %v", err)
	}
	migrations = migrations[:len(migrations)-1]

	from := 0
	db, err = OpenMigrating(path, func(_ *DB, v int) error { from = v; return nil })
	if err != nil || from != 1 || colour() != 1 {
		t.Fatalf("migrate: %v, snapshot saw version %d, colour columns %d", err, from, colour())
	}
	db.Close()

	migrations = migrations[:len(migrations)-1]
	var tooNew *TooNewError
	if _, err := Open(path); !errors.As(err, &tooNew) || tooNew.File != 2 || tooNew.Binary != 1 {
		t.Fatalf("opened a file newer than the binary: %v", err)
	}
}

func TestWriteRollsBackOnError(t *testing.T) {
	db := Testing(t)
	want := context.Canceled
	err := db.Write(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO labels (name, name_folded, created_at) VALUES ('x','x',1)`); err != nil {
			return err
		}
		return want
	})
	if err != want {
		t.Fatalf("Write returned %v, want %v", err, want)
	}
	var n int
	if err := db.R.QueryRow(`SELECT count(*) FROM labels`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("rolled-back insert left %d rows", n)
	}
}

func TestUniqueViolationIsRecognised(t *testing.T) {
	db := Testing(t)
	ins := `INSERT INTO labels (name, name_folded, created_at) VALUES ('API','api',1)`
	if _, err := db.W.Exec(ins); err != nil {
		t.Fatal(err)
	}
	_, err := db.W.Exec(ins)
	if !IsUniqueViolation(err) {
		t.Fatalf("second insert: IsUniqueViolation(%v) = false", err)
	}
	if got := ConstraintIndex(err); got != "labels.name_folded" {
		t.Fatalf("ConstraintIndex = %q", got)
	}
}

// SPEC.md §6.2: a bucket never belongs to another bucket, and a ticket never
// carries an icon. Both are held by the schema, not by application code.
func TestItemKindConstraints(t *testing.T) {
	db := Testing(t)
	seedTeam(t, db)
	_, err := db.W.Exec(`
		INSERT INTO items (kind, origin_team_id, number, team_id, title, reporter_id,
		                   state_id, rank, icon, created_at, updated_at)
		VALUES ('ticket', 1, 1, 1, 't', 1, 1, 'V', 'bug', 1, 1)`)
	if err == nil {
		t.Fatal("a ticket was allowed an icon")
	}
	if _, err := db.W.Exec(`
		INSERT INTO items (id, kind, origin_team_id, number, team_id, title, reporter_id,
		                   state_id, rank, created_at, updated_at)
		VALUES (10, 'bucket', 1, 2, 1, 'b', 1, 1, 'W', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	_, err = db.W.Exec(`
		INSERT INTO items (kind, origin_team_id, number, team_id, title, reporter_id,
		                   state_id, rank, bucket_id, created_at, updated_at)
		VALUES ('bucket', 1, 3, 1, 'b2', 1, 1, 'X', 10, 1, 1)`)
	if err == nil {
		t.Fatal("a bucket was allowed to sit inside another bucket")
	}
}

func seedTeam(t *testing.T, db *DB) {
	t.Helper()
	stmts := []string{
		`INSERT INTO users (id, username, username_folded, password_hash, created_at) VALUES (1,'ann','ann','h',1)`,
		`INSERT INTO teams (id, name, name_folded, short_code, short_code_folded, created_at) VALUES (1,'Solstice','solstice','SOL','sol',1)`,
		`INSERT INTO work_states (id, team_id, name, name_folded, role, position, created_at) VALUES (1,1,'Backlog','backlog','default',1,1)`,
	}
	for _, s := range stmts {
		if _, err := db.W.Exec(s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}
