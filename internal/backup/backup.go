// Package backup takes and restores snapshots of the database.
//
// The audience has no operator, so recovery is a feature of the application
// rather than a runbook (SPEC.md §13.3). Snapshots are on by default,
// including for a solo user who has never considered backups, and restore is
// a first-class command invoked the same way in both deployment profiles.
package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/campfire/campfire/internal/store"
)

// namePrefix and nameLayout make a snapshot's file name sort chronologically,
// so retention is a sort and a truncate.
const (
	namePrefix = "campfire-"
	nameLayout = "20060102-150405"
	nameSuffix = ".db"
)

type Snapshot struct {
	Path  string    `json:"path"`
	Taken time.Time `json:"taken"`
	Size  int64     `json:"size"`
	// Verified is true when the file opened and passed an integrity check. A
	// backup that has never been read is not a backup.
	Verified bool `json:"verified"`
}

// Take writes a consistent snapshot of db into dir and verifies it.
//
// VACUUM INTO runs against a live database without locking writers out and
// without an external tool, which is what "online, without downtime" asks for.
// The result is a complete, self-contained database file: restoring is putting
// it back.
func Take(ctx context.Context, db *store.DB, dir string) (Snapshot, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Snapshot{}, err
	}
	// Names are timestamps to the second so that they sort chronologically and
	// retention is a sort and a truncate. Snapshots taken inside one second —
	// a manual one landing on a scheduled one — take a suffix.
	stamp := namePrefix + time.Now().UTC().Format(nameLayout)
	path := filepath.Join(dir, stamp+nameSuffix)
	for n := 1; n < 100; n++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%02d%s", stamp, n, nameSuffix))
	}

	// VACUUM INTO refuses to overwrite, which is the behaviour we want.
	if _, err := db.W.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{Path: path, Taken: info.ModTime(), Size: info.Size()}
	if err := Verify(path); err != nil {
		return snap, fmt.Errorf("snapshot %s did not verify: %w", path, err)
	}
	snap.Verified = true
	return snap, nil
}

// Verify opens a snapshot read-only and runs SQLite's integrity check.
func Verify(path string) error {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return errors.New(result)
	}
	return nil
}

// List returns the snapshots in dir, newest first.
func List(dir string) ([]Snapshot, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), namePrefix) || !strings.HasSuffix(e.Name(), nameSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Snapshot{
			Path:  filepath.Join(dir, e.Name()),
			Taken: info.ModTime(),
			Size:  info.Size(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path > out[j].Path })
	return out, nil
}

// Prune keeps the newest `keep` snapshots and removes the rest.
func Prune(dir string, keep int) error {
	snaps, err := List(dir)
	if err != nil || keep < 1 {
		return err
	}
	var errs []error
	for _, s := range snaps[min(keep, len(snaps)):] {
		if err := os.Remove(s.Path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Ship copies a snapshot to a second directory, which may be a mount of
// somewhere else entirely.
//
// ponytail: this ships whole snapshots rather than a continuous write-ahead
// log, so the recovery point is the last snapshot rather than the last commit
// — SPEC.md §13.3 item 2 asks for the latter. Continuous shipping against
// SQLite means tracking WAL frames and generations against a byte-level base
// copy; that is a project of its own, and shortening the snapshot interval
// buys most of the same protection today. Upgrade path: a byte-copy base plus
// WAL segment shipping, replacing this function and Restore's base selection.
func Ship(snapshot Snapshot, dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return copyFile(snapshot.Path, filepath.Join(dir, filepath.Base(snapshot.Path)))
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	// Write beside the target and rename, so a reader never sees half a file.
	tmp := to + ".partial"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(tmp)
		return err
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		os.Remove(tmp)
		return err
	}
	if err := dst.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, to)
}

// Restore replaces the database at target with a snapshot.
//
// The server must not be running: one process, one data file. The existing
// file is moved aside rather than deleted, so a restore of the wrong snapshot
// is itself recoverable.
func Restore(snapshotPath, target string) error {
	if err := Verify(snapshotPath); err != nil {
		return fmt.Errorf("%s is not a usable snapshot: %w", snapshotPath, err)
	}
	if _, err := os.Stat(target); err == nil {
		aside := target + ".replaced-" + time.Now().UTC().Format(nameLayout)
		if err := os.Rename(target, aside); err != nil {
			return err
		}
		// The write-ahead log and shared-memory files belong to the file that
		// was just moved aside. Left in place they would be applied to the
		// restored database, which is a different file with different pages.
		for _, suffix := range []string{"-wal", "-shm"} {
			os.Remove(target + suffix)
		}
	}
	if err := copyFile(snapshotPath, target); err != nil {
		return err
	}
	return Verify(target)
}
