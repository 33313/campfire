package store

import (
	"context"
	"database/sql"
	"path/filepath"
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
