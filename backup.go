package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/campfire/campfire/internal/backup"
	"github.com/campfire/campfire/internal/config"
	"github.com/campfire/campfire/internal/store"
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
	if err := backup.Ship(snap, cfg.Snapshots.ShipTo); err != nil {
		return snap, fmt.Errorf("shipping %s: %w", snap.Path, err)
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
	if err := backup.Restore(snapshot, cfg.Database); err != nil {
		return err
	}
	fmt.Printf("restored %s into %s\n", snapshot, cfg.Database)
	return nil
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
