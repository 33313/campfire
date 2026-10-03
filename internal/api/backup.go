package api

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/33313/campfire/internal/backup"
	"github.com/33313/campfire/internal/config"
	"github.com/33313/campfire/internal/core"
)

// Snapshots is what the snapshot routes drive: the configured directory, and
// how one is taken, which is the same for the schedule, the command line and
// this API.
type Snapshots struct {
	config.Snapshots
	Take func(context.Context) (backup.Snapshot, error)
}

func (s *Server) listSnapshots(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	if err := core.RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	snaps, err := backup.List(s.Snapshots.Dir)
	if err != nil {
		return nil, backupErr(err)
	}
	// Verification is what makes the list worth showing: the screen marks a
	// snapshot verified, and an unverified one is a warning.
	for i := range snaps {
		snaps[i].Verified = backup.Verify(snaps[i].Path) == nil
	}
	c := s.Snapshots.Snapshots
	out := map[string]any{"snapshots": snaps, "directory": c.Dir, "enabled": c.Enabled,
		"keep": c.Keep, "interval": c.Interval}
	// The replica's directory, and the span it can restore to, to any moment
	// in it, once it holds one.
	if c.Replica != "" {
		from, to, err := backup.Window(r.Context(), c.Replica)
		if err != nil {
			return nil, backupErr(err)
		}
		replica := map[string]any{"directory": c.Replica}
		if !to.IsZero() {
			replica["from"], replica["to"] = from, to
		}
		out["replica"] = replica
	}
	return out, nil
}

func (s *Server) createSnapshot(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	if err := core.RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	snap, err := s.Snapshots.Take(r.Context())
	if err != nil {
		return nil, backupErr(err)
	}
	return snap, nil
}

// restoreSnapshot answers with the command to run: from a snapshot by its
// path, or from the replica to a moment. Restoring replaces the instance, so
// it is not something the running server does to itself.
func (s *Server) restoreSnapshot(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	if err := core.RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	var in struct {
		Path string
		At   time.Time
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if !in.At.IsZero() {
		return s.restoreAt(r, in.At)
	}
	// Only a snapshot in the snapshot directory: the path is a name, never a
	// way to have the server open any file it can read.
	path := filepath.Join(s.Snapshots.Dir, filepath.Base(in.Path))
	if err := backup.Verify(path); err != nil {
		return nil, &core.Error{Code: core.CodeInvalid, Field: "path", Message: "that snapshot did not verify: " + err.Error()}
	}
	return map[string]string{
		"command": "campfire restore " + path,
		"note": "Stop the server, run this command, then start it again. " +
			"Restoring replaces the data file, which one running process holds open.",
	}, nil
}

func (s *Server) restoreAt(r *http.Request, at time.Time) (any, error) {
	dir := s.Snapshots.Replica
	if dir == "" {
		return nil, &core.Error{Code: core.CodeInvalid, Field: "at", Message: "there is no replica; set snapshots.replica"}
	}
	from, to, err := backup.Window(r.Context(), dir)
	if err != nil {
		return nil, backupErr(err)
	}
	if to.IsZero() || at.Before(from) || at.After(to) {
		return nil, &core.Error{Code: core.CodeInvalid, Field: "at", Message: "the replica does not reach that moment"}
	}
	return map[string]string{
		"command": "campfire restore -at " + at.UTC().Format(time.RFC3339),
		"note": "Stop the server, run this command, then start it again. " +
			"Restoring replaces the data file, which one running process holds open.",
	}, nil
}

// backupErr keeps the reason a backup failed: an administrator looking at a
// disk that is full needs that, not "something went wrong".
func backupErr(err error) error {
	return &core.Error{Code: "internal", Message: err.Error()}
}
