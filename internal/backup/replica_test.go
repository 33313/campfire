package backup

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/33313/campfire/internal/store"
)

// SPEC.md §13.3: the replica restores the database as it was at any moment it
// holds — here, between two batches of writes, with Campfire's own connections
// checkpointing on top of the replication's, as they do in production.
func TestReplicaRestoresToAMoment(t *testing.T) {
	if testing.Short() {
		t.Skip("waits on replication")
	}
	ctx := context.Background()
	dir := t.TempDir()
	path, replica := filepath.Join(dir, "campfire.db"), filepath.Join(dir, "replica")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.W.Exec(`CREATE TABLE t (n INTEGER, pad BLOB)`); err != nil {
		t.Fatal(err)
	}
	stop, err := Replicate(ctx, path, replica, time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Each batch is a few thousand pages, past SQLite's automatic checkpoint,
	// and checkpoints again on top.
	batch := func(from int) {
		for n := from; n < from+3000; n++ {
			if _, err := db.W.Exec(`INSERT INTO t VALUES (?, randomblob(4000))`, n); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.W.Exec(`PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
			t.Fatal(err)
		}
	}
	replicated := func() time.Time {
		t.Helper()
		wrote := time.Now()
		for deadline := wrote.Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if _, to, err := Window(ctx, replica); err == nil && to.After(wrote) {
				return to
			}
		}
		t.Fatal("the replica did not catch up")
		return time.Time{}
	}

	batch(0)
	replicated()
	time.Sleep(time.Second)
	between := time.Now()
	time.Sleep(time.Second)
	batch(3000)
	replicated()
	if err := stop(ctx); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		at   time.Time
		want int
	}{{between, 3000}, {time.Time{}, 6000}} {
		target := filepath.Join(t.TempDir(), "restored.db")
		if err := RestoreAt(ctx, replica, tc.at, target); err != nil {
			t.Fatal(err)
		}
		got, err := sql.Open("sqlite", "file:"+target+"?mode=ro")
		if err != nil {
			t.Fatal(err)
		}
		var rows, last int
		if err := got.QueryRow(`SELECT count(*), max(n) FROM t`).Scan(&rows, &last); err != nil {
			t.Fatal(err)
		}
		got.Close()
		if rows != tc.want || last != tc.want-1 {
			t.Errorf("restored to %v: %d rows up to %d, want %d", tc.at, rows, last, tc.want)
		}
	}

	if err := RestoreAt(ctx, replica, between.Add(-time.Hour), filepath.Join(t.TempDir(), "x.db")); err == nil {
		t.Error("a moment before the replica began was restored to")
	}
}
