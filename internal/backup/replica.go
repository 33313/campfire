package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/benbjohnson/litestream/file"
)

// A replica is the database's write-ahead log, copied continuously to a
// second directory as it is written, so the database can be restored to any
// moment the replica still holds (SPEC.md §13.3). Litestream does the work,
// inside this process: no second program runs beside the server.
//
// It takes over checkpointing while it runs, holding a read transaction so
// that no part of the log is overwritten before it has been copied.

// Replicate starts copying the database at path into dir, keeping enough to
// restore to any moment in the last retention. A full copy is written every
// snapshotEvery, so a restore replays at most that much log. stop flushes what
// is left and must run before the database is closed.
func Replicate(ctx context.Context, path, dir string, snapshotEvery, retention time.Duration) (stop func(context.Context) error, err error) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	db := litestream.NewDB(path)
	client := file.NewReplicaClient(dir)
	replica := litestream.NewReplicaWithClient(db, client)
	db.Replica, client.Replica = replica, replica

	store := litestream.NewStore([]*litestream.DB{db}, litestream.DefaultCompactionLevels)
	store.SnapshotInterval = snapshotEvery
	store.SnapshotRetention = retention
	store.Logger = logger
	db.SetLogger(logger)
	if err := store.Open(ctx); err != nil {
		return nil, fmt.Errorf("replication to %s: %w", dir, err)
	}
	return store.Close, nil
}

// Window is the span the replica in dir can restore to, earliest first. Both
// are zero when it holds nothing yet.
func Window(ctx context.Context, dir string) (from, to time.Time, err error) {
	return litestream.NewReplicaWithClient(nil, file.NewReplicaClient(dir)).TimeBounds(ctx)
}

// RestoreAt rebuilds the database as it was at the moment at from the replica
// in dir, and puts it in place of target as Restore does. A zero at is the
// latest moment the replica holds.
func RestoreAt(ctx context.Context, dir string, at time.Time, target string) error {
	from, to, err := Window(ctx, dir)
	if err != nil {
		return err
	}
	if to.IsZero() {
		return fmt.Errorf("the replica in %s holds nothing to restore", dir)
	}
	if !at.IsZero() && (at.Before(from) || at.After(to)) {
		return fmt.Errorf("the replica in %s goes back to %s and forward to %s; %s is outside that",
			dir, from.Local().Format(time.DateTime), to.Local().Format(time.DateTime), at.Local().Format(time.DateTime))
	}

	// Rebuilt beside the target, so putting it in place is a copy on one disk,
	// in a directory of its own, so whatever SQLite leaves beside it goes too.
	work, err := os.MkdirTemp(filepath.Dir(target), ".restoring-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	tmp := filepath.Join(work, "campfire.db")
	opt := litestream.NewRestoreOptions()
	opt.OutputPath, opt.Timestamp = tmp, at
	replica := litestream.NewReplicaWithClient(nil, file.NewReplicaClient(dir))
	if err := replica.Restore(ctx, opt); err != nil {
		return fmt.Errorf("restoring from %s: %w", dir, err)
	}
	if _, err := os.Stat(tmp); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the replica in %s restored nothing", dir)
	}
	return Restore(tmp, target)
}
