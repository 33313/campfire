package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/33313/campfire/internal/backup"
	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/store"
)

// The subcommand is read whether it leads or follows the flags. Reading it
// only in the leading position left "campfire -config x snapshot" starting a
// server, which is how CI's snapshot step came to bind a port instead of
// writing a file.
func TestSubcommandIsReadOnEitherSideOfTheFlags(t *testing.T) {
	for _, args := range [][]string{
		{"init", "-config", "%s"},
		{"-config", "%s", "init"},
	} {
		path := filepath.Join(t.TempDir(), "campfire.json")
		spelled := append([]string(nil), args...)
		for i, a := range spelled {
			spelled[i] = strings.ReplaceAll(a, "%s", path)
		}
		if err := run(spelled); err != nil {
			t.Fatalf("%v: %v", spelled, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%v wrote no configuration file", spelled)
		}
	}
}

// The version is asked for most when something is wrong, so a configuration
// file that does not parse must not stand in the way of it.
func TestVersionNeedsNoConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campfire.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"version", "-config", path},
		{"-config", path, "version"},
		{"-version", "-config", path},
	} {
		if err := run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestAnArgumentNothingReadsIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campfire.json")
	err := run([]string{"init", "-config", path, "wat"})
	if err == nil || !strings.Contains(err.Error(), "takes no arguments") {
		t.Fatalf("got %v, want a complaint about the extra argument", err)
	}
}

// SPEC.md §13.3: the restore path is exercised in CI. A snapshot taken from
// the command line is restored over later writes and brings back exactly what
// it held. Restoring to a moment is internal/backup's TestReplicaRestoresToAMoment.
func TestSnapshotRestoreAndCheckFromTheCommandLine(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "campfire.db")
	snaps := filepath.Join(dir, "snapshots")
	cfg := filepath.Join(dir, "campfire.json")
	body := fmt.Sprintf(`{"database":%q,"snapshots":{"enabled":true,"dir":%q,"keep":3}}`, dbPath, snaps)
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var admin core.Actor
	var teamID int64
	withService := func(fn func(*core.Service)) {
		t.Helper()
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		fn(core.New(db))
	}
	create := func(title string) {
		t.Helper()
		withService(func(svc *core.Service) {
			if _, err := svc.CreateItem(ctx, admin, core.NewItem{Kind: store.KindTicket, TeamID: teamID, Title: title}); err != nil {
				t.Fatal(err)
			}
		})
	}
	withService(func(svc *core.Service) {
		u, err := svc.Bootstrap(ctx, "root", "correct horse battery")
		if err != nil {
			t.Fatal(err)
		}
		admin = core.Actor{UserID: u.ID, Username: u.Username, IsAdmin: true}
		team, err := svc.CreateTeam(ctx, admin, "Solstice", "SOL")
		if err != nil {
			t.Fatal(err)
		}
		teamID = team.ID
	})

	create("before the snapshot")
	if err := run([]string{"-config", cfg, "snapshot"}); err != nil {
		t.Fatal(err)
	}
	taken, err := backup.List(snaps)
	if err != nil || len(taken) != 1 {
		t.Fatalf("snapshots: %v, %v", taken, err)
	}
	create("after the snapshot")

	// A bare name is looked up in the snapshot directory.
	if err := run([]string{"-config", cfg, "restore", filepath.Base(taken[0].Path)}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-config", cfg, "check"}); err != nil {
		t.Fatal(err)
	}
	withService(func(svc *core.Service) {
		page, err := svc.ListItems(ctx, admin, core.Filter{}, "", 50)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].Title != "before the snapshot" {
			t.Fatalf("restored %+v, want only what the snapshot held", page.Items)
		}
	})
}

// A moment is written with its zone or in this machine's, and -at belongs to
// restore alone.
func TestRestoreAtReadsAMoment(t *testing.T) {
	for _, s := range []string{"2026-10-03T14:05:00+02:00", "2026-10-03 14:05", "2026-10-03 14:05:09"} {
		if _, err := parseMoment(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	if _, err := parseMoment("yesterday"); err == nil {
		t.Error("yesterday was read as a moment")
	}
	cfg := filepath.Join(t.TempDir(), "campfire.json")
	if err := run([]string{"-config", cfg, "-at", "latest", "check"}); err == nil || !strings.Contains(err.Error(), "-at is for restore") {
		t.Errorf("-at on check: %v", err)
	}
	if err := run([]string{"-config", cfg, "restore", "-at", "latest"}); err == nil || !strings.Contains(err.Error(), "replica") {
		t.Errorf("restore -at with no replica: %v", err)
	}
}
