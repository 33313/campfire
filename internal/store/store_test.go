package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/campfire/campfire/internal/csql"
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

// Migration 0005 writes each saved view's clauses as the CSQL query they
// always meant, and keeps what they named by id, which is still not removed
// when its referent is deleted (SPEC.md §9.4).
func TestClausesBecomeTheQueryTheyMeant(t *testing.T) {
	path, raw := migratedTo(t, 4)
	seedTeam(t, &DB{W: raw})
	for _, stmt := range []string{
		`INSERT INTO labels (id, name, name_folded, created_at) VALUES (7,'api','api',1)`,
		`INSERT INTO saved_views (id, team_id, name, name_folded, created_at, updated_at) VALUES (1,1,'Mine','mine',1,1)`,
		`INSERT INTO saved_views (id, team_id, name, name_folded, created_at, updated_at) VALUES (2,1,'All','all',1,1)`,
		`INSERT INTO saved_view_clauses (view_id, dimension, ref_id, text_value) VALUES
		   (1,'label',7,NULL), (1,'label',8,NULL), (1,'assignee',0,NULL), (1,'assignee',1,NULL),
		   (1,'priority',1,NULL), (1,'priority',2,NULL), (1,'bucket',60,NULL), (1,'text',NULL,'say "hi"')`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v, err := SavedViewByID(context.Background(), db.R, 1)
	if err != nil {
		t.Fatal(err)
	}
	n, err := csql.Parse(v.Query)
	if err != nil {
		t.Fatalf("migrated %q does not parse: %v", v.Query, err)
	}
	// Dimensions and-ed; within one, any-of; labels every-one-of.
	parts := strings.Split(csql.Canonical(n), " and ")
	slices.Sort(parts)
	want := []string{"assignee in (none, #1)", "bucket in (#60)", "label = #7", "label = #8",
		"priority in (1, 2)", `text ~ "say \"hi\""`}
	if !slices.Equal(parts, want) {
		t.Fatalf("migrated to %q: %q", v.Query, parts)
	}
	if empty, _ := SavedViewByID(context.Background(), db.R, 2); empty.Query != "" {
		t.Fatalf("a view with no clauses became %q", empty.Query)
	}

	// What it names by id is kept, and outlives its referent.
	if _, err := db.W.Exec(`DELETE FROM labels WHERE id = 7`); err != nil {
		t.Fatal(err)
	}
	refs, _ := ViewRefs(context.Background(), db.R, 1)
	if len(refs) != 4 {
		t.Fatalf("refs %+v, want label 7 and 8, assignee 1, bucket 60", refs)
	}
	for _, r := range refs {
		if r.Field == "label" && r.ID == 7 && r.Name != "" {
			t.Fatal("a deleted label still has a name")
		}
	}
}

// The first draft of CSQL wrote an empty field as `is empty`; migration 0006
// rewrites what it saved as none.
func TestIsEmptyBecomesNone(t *testing.T) {
	path, raw := migratedTo(t, 5)
	seedTeam(t, &DB{W: raw})
	if _, err := raw.Exec(`INSERT INTO saved_views (id, team_id, name, name_folded, query, created_at, updated_at)
	    VALUES (1,1,'Loose','loose','assignee is empty or (bucket is not empty and text ~ "is empty")',1,1)`); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v, _ := SavedViewByID(context.Background(), db.R, 1)
	// The phrase is left alone: it opens with a quote, not the space the
	// rewrite looks for.
	if want := `assignee = none or (bucket != none and text ~ "is empty")`; v.Query != want {
		t.Fatalf("rewritten to %q, want %q", v.Query, want)
	}
	if _, err := csql.Parse(v.Query); err != nil {
		t.Fatalf("%q does not parse: %v", v.Query, err)
	}
}

// migratedTo opens a database at path with the first n migrations applied by
// hand, as an instance of that version would have it.
func migratedTo(t *testing.T, n int) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campfire.db")
	raw, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := migrations.ReadDir("migrations")
	for _, e := range entries[:n] {
		body, _ := migrations.ReadFile("migrations/" + e.Name())
		if _, err := raw.Exec(string(body)); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
	}
	if _, err := raw.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, n)); err != nil {
		t.Fatal(err)
	}
	return path, raw
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
