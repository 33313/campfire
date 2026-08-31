package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// Testing opens a fresh migrated database in the test's temporary directory.
func Testing(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "campfire.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenMigratesAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campfire.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var version int
	if err := db.W.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version == 0 {
		t.Fatal("migrations did not run")
	}
	db.Close()

	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	var version2 int
	if err := again.W.QueryRow(`PRAGMA user_version`).Scan(&version2); err != nil {
		t.Fatal(err)
	}
	if version2 != version {
		t.Fatalf("reopen changed version %d -> %d", version, version2)
	}
}

// SPEC.md §13.2 rule 8: the engine rejects a value of the wrong type rather
// than storing it. That is what STRICT buys, and it is worth one test.
func TestTablesAreStrict(t *testing.T) {
	db := Testing(t)
	_, err := db.W.Exec(`INSERT INTO users (id, username, username_folded, password_hash, created_at)
	                     VALUES (1, 'a', 'a', 'h', 'not-a-number')`)
	if err == nil {
		t.Fatal("a text value was accepted into an INTEGER column")
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

// The rank uniqueness constraint of SPEC.md §9.3 is what makes two clients
// computing the same midpoint a retry rather than a silent collision. It only
// works if the column really is unique per state.
func TestRankIsUniquePerState(t *testing.T) {
	db := Testing(t)
	seedTeam(t, db)
	insert := func(number int, rank string) error {
		_, err := db.W.Exec(`
			INSERT INTO items (kind, origin_team_id, number, team_id, title,
			                   reporter_id, state_id, rank, created_at, updated_at)
			VALUES ('ticket', 1, ?, 1, 't', 1, 1, ?, 1, 1)`, number, rank)
		return err
	}
	if err := insert(1, "V"); err != nil {
		t.Fatal(err)
	}
	if err := insert(2, "V"); !IsUniqueViolation(err) {
		t.Fatalf("duplicate rank in one state was accepted: %v", err)
	}
	// The same rank in a different state is fine: ranks are per column.
	if _, err := db.W.Exec(`INSERT INTO work_states (id, team_id, name, name_folded, position, created_at)
	                        VALUES (2, 1, 'Doing', 'doing', 2, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.W.Exec(`
		INSERT INTO items (kind, origin_team_id, number, team_id, title,
		                   reporter_id, state_id, rank, created_at, updated_at)
		VALUES ('ticket', 1, 3, 1, 't', 1, 2, 'V', 1, 1)`); err != nil {
		t.Fatalf("same rank in another state: %v", err)
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

// SPEC.md §9.4: a clause is kept when its referent is deleted. It must match
// nothing, not disappear — a view that quietly repairs itself is a view
// nobody can trust.
func TestSavedViewClauseSurvivesReferentDeletion(t *testing.T) {
	db := Testing(t)
	seedTeam(t, db)
	if _, err := db.W.Exec(`INSERT INTO labels (id, name, name_folded, created_at) VALUES (7,'api','api',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.W.Exec(`INSERT INTO saved_views (id, team_id, name, name_folded, created_at, updated_at)
	                        VALUES (1, 1, 'API work', 'api work', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.W.Exec(`INSERT INTO saved_view_clauses (view_id, dimension, ref_id) VALUES (1, 'label', 7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.W.Exec(`DELETE FROM labels WHERE id = 7`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.R.QueryRow(`SELECT count(*) FROM saved_view_clauses WHERE view_id = 1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("clause count after deleting its label = %d, want 1", n)
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
