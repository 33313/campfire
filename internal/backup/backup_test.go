package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/campfire/campfire/internal/store"
)

// SPEC.md §13.3 rule 4: the restore path MUST be exercised in continuous
// integration. A backup that has never been restored is not a backup — so this
// test takes a snapshot, changes the database, restores, and checks the change
// is gone and the earlier data is back.
func TestSnapshotAndRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "campfire.db")
	snapDir := filepath.Join(dir, "snapshots")
	ctx := context.Background()

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	seed(t, db, "before the snapshot")

	snap, err := Take(ctx, db, snapDir)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Verified {
		t.Fatal("the snapshot was not verified when it was taken")
	}
	if snap.Size == 0 {
		t.Fatal("the snapshot is empty")
	}

	seed(t, db, "after the snapshot")
	if got := labels(t, db); len(got) != 2 {
		t.Fatalf("the live database holds %v", got)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Restore(snap.Path, dbPath); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	got := labels(t, restored)
	if len(got) != 1 || got[0] != "before the snapshot" {
		t.Fatalf("after restoring, the database holds %v", got)
	}
}

// The write-ahead log of the replaced database must not survive the restore:
// it belongs to a different file with different pages.
func TestRestoreRemovesTheOldWriteAheadLog(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "campfire.db")
	ctx := context.Background()

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	seed(t, db, "a")
	snap, err := Take(ctx, db, filepath.Join(dir, "snapshots"))
	if err != nil {
		t.Fatal(err)
	}
	seed(t, db, "b")
	db.Close()

	// Leave a stale write-ahead log behind, as an unclean shutdown would.
	if err := os.WriteFile(dbPath+"-wal", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(snap.Path, dbPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath + "-wal"); !os.IsNotExist(err) {
		t.Fatal("the stale write-ahead log survived the restore")
	}
}

func TestPruneKeepsTheNewest(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "campfire.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	snapDir := filepath.Join(dir, "snapshots")

	for i := 0; i < 3; i++ {
		if _, err := Take(context.Background(), db, snapDir); err != nil {
			t.Fatal(err)
		}
	}
	snaps, _ := List(snapDir)
	if len(snaps) != 3 {
		t.Fatalf("%d snapshots were taken", len(snaps))
	}
	if err := Prune(snapDir, 1); err != nil {
		t.Fatal(err)
	}
	after, _ := List(snapDir)
	if len(after) != 1 {
		t.Fatalf("after pruning to 1 there are %d snapshots", len(after))
	}
	if after[0].Path != snaps[0].Path {
		t.Fatal("pruning kept the wrong snapshot")
	}
}

func TestShipCopiesTheSnapshotElsewhere(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "campfire.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	snap, err := Take(context.Background(), db, filepath.Join(dir, "snapshots"))
	if err != nil {
		t.Fatal(err)
	}
	offHost := filepath.Join(dir, "elsewhere")
	if err := Ship(snap, offHost); err != nil {
		t.Fatal(err)
	}
	shipped := filepath.Join(offHost, filepath.Base(snap.Path))
	if err := Verify(shipped); err != nil {
		t.Fatalf("the shipped copy does not verify: %v", err)
	}
	// An empty destination is "shipping is off", not an error.
	if err := Ship(snap, ""); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsRubbish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-database.db")
	if err := os.WriteFile(path, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(path); err == nil {
		t.Fatal("a file of rubbish verified as a snapshot")
	}
	if err := Restore(path, filepath.Join(t.TempDir(), "target.db")); err == nil {
		t.Fatal("a file of rubbish was restored")
	}
}

func seed(t *testing.T, db *store.DB, label string) {
	t.Helper()
	if _, err := db.W.Exec(
		`INSERT INTO labels (name, name_folded, created_at) VALUES (?, ?, 1)`,
		label, store.Fold(label)); err != nil {
		t.Fatal(err)
	}
}

func labels(t *testing.T, db *store.DB) []string {
	t.Helper()
	rows, err := db.R.Query(`SELECT name FROM labels ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}
