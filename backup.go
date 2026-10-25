package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/33313/campfire/internal/backup"
	"github.com/33313/campfire/internal/config"
	"github.com/33313/campfire/internal/store"
)

// Snapshots run on a ticker inside the one process. This is not a background
// job in the sense SPEC.md §13.2 rule 6 forbids — no business behaviour
// depends on it, and nothing is cleaned up lazily on its behalf. It is the
// scheduled backup §13.3 requires to be on by default.
func startSnapshots(cfg config.Config, db *store.DB) func() {
	if !cfg.Snapshots.Enabled {
		log.Print("scheduled snapshots are off; turn them on in the configuration file")
		return func() {}
	}
	interval, err := cfg.SnapshotInterval()
	if err != nil {
		log.Printf("snapshots: %v", err)
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := takeSnapshot(ctx, cfg, db); err != nil {
					log.Printf("snapshot failed: %v", err)
				}
			}
		}
	}()
	log.Printf("snapshots every %s into %s, keeping %d", interval, cfg.Snapshots.Dir, cfg.Snapshots.Keep)
	return cancel
}

// startReplica copies the write-ahead log into snapshots.replica as it is
// written, for a restore to any moment (SPEC.md §13.3). Like snapshots, it is
// the backup §13.3 asks for, not a background job in the sense of §13.2 rule 6.
func startReplica(cfg config.Config, db *store.DB) (func(), error) {
	if cfg.Snapshots.Replica == "" {
		return func() {}, nil
	}
	every, _ := cfg.SnapshotInterval() // validated on load
	keep, _ := cfg.Retention()
	stop, err := backup.Replicate(context.Background(), db.Path, cfg.Snapshots.Replica, every, keep)
	if err != nil {
		return nil, err
	}
	log.Printf("write-ahead log copied continuously into %s, restorable to any moment in the last %s", cfg.Snapshots.Replica, keep)
	return func() {
		ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		if err := stop(ctx); err != nil {
			log.Printf("replication: %v", err)
		}
	}, nil
}

// Integrity is checked at startup and periodically (SPEC.md §13.3 rule 5).
// Corruption found while running is logged with the snapshot to restore from;
// found at startup, it stops the server rather than serving bad data.
func startIntegrityChecks(cfg config.Config, db *store.DB) func() {
	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := db.CheckIntegrity(); err != nil {
					log.Printf("INTEGRITY: %v", withRecoveryAdvice(cfg, err))
				}
			}
		}
	}()
	return cancel
}

func takeSnapshot(ctx context.Context, cfg config.Config, db *store.DB) (backup.Snapshot, error) {
	snap, err := backup.Take(ctx, db, cfg.Snapshots.Dir)
	if err != nil {
		return snap, err
	}
	if err := backup.Prune(cfg.Snapshots.Dir, cfg.Snapshots.Keep); err != nil {
		return snap, err
	}
	log.Printf("snapshot %s (%d bytes, verified)", snap.Path, snap.Size)
	return snap, nil
}

func snapshotOnce(cfg config.Config) error {
	db, err := open(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	snap, err := takeSnapshot(context.Background(), cfg, db)
	if err != nil {
		return err
	}
	fmt.Println(snap.Path)
	return nil
}

// restoreAt rebuilds the database as it was at a moment, from the replica in
// snapshots.replica. "latest" is the newest moment it holds.
func restoreAt(cfg config.Config, at string) error {
	if cfg.Snapshots.Replica == "" {
		return errors.New("restoring to a moment needs snapshots.replica set")
	}
	var moment time.Time
	if at != "latest" {
		var err error
		if moment, err = parseMoment(at); err != nil {
			return err
		}
	}
	if err := backup.RestoreAt(context.Background(), cfg.Snapshots.Replica, moment, cfg.Database); err != nil {
		return err
	}
	if err := migrateRestored(cfg); err != nil {
		return err
	}
	fmt.Printf("restored %s as it was at %s\n", cfg.Database, at)
	return nil
}

// parseMoment reads a time with its zone (2026-10-03T14:05:00+02:00) or in
// this machine's (2026-10-03 14:05).
func parseMoment(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q is not a moment; write 2026-10-03 14:05, or with a zone, 2026-10-03T14:05:00+02:00", s)
}

// restore is invoked the same way in both deployment profiles
// (SPEC.md §13.3 rule 3). It refuses while the server is running, because two
// processes writing one file is exactly what the architecture rules out.
func restore(cfg config.Config, snapshot string) error {
	if _, err := os.Stat(snapshot); err != nil {
		// A bare name is looked up in the snapshot directory, so that the
		// name printed by `campfire snapshot` can be pasted back.
		alt := filepath.Join(cfg.Snapshots.Dir, snapshot)
		if _, err2 := os.Stat(alt); err2 == nil {
			snapshot = alt
		} else {
			return err
		}
	}
	if v, err := store.FileVersion(snapshot); err == nil && v > store.Version() {
		return store.TooNew(snapshot, v)
	}
	if err := backup.Restore(snapshot, cfg.Database); err != nil {
		return err
	}
	if err := migrateRestored(cfg); err != nil {
		return err
	}
	fmt.Printf("restored %s into %s\n", snapshot, cfg.Database)
	return nil
}

// migrateRestored opens a restored file the way the server does, so that an
// older one is migrated now, its own snapshot taken first.
func migrateRestored(cfg config.Config) error {
	db, err := open(cfg)
	if err != nil {
		return err
	}
	return db.Close()
}

// withRecoveryAdvice names the snapshot to restore from, because a corrupt
// database is exactly the moment the audience has no runbook to reach for
// (SPEC.md §13.3 rule 5).
func withRecoveryAdvice(cfg config.Config, err error) error {
	snaps, listErr := backup.List(cfg.Snapshots.Dir)
	if listErr != nil || len(snaps) == 0 {
		return fmt.Errorf("%w\n\nno snapshots were found in %s", err, cfg.Snapshots.Dir)
	}
	return fmt.Errorf("%w\n\nthe most recent snapshot is %s (%s)\nrestore it with:  campfire restore %s",
		err, snaps[0].Path, snaps[0].Taken.Format(time.RFC3339), snaps[0].Path)
}
