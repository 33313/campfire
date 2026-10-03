package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/campfire/campfire/internal/backup"
	"github.com/campfire/campfire/internal/core"
	"github.com/campfire/campfire/internal/store"
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

func TestAReleaseVersionOutranksTheBuildStamp(t *testing.T) {
	defer func(v string) { version = v }(version)
	version = "v1.2.3"
	if got := buildVersion(); got != "v1.2.3" {
		t.Fatalf("got %q, want the version set at link time", got)
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
// the command line, and shipped, is restored over later writes and brings
// back exactly what it held.
func TestSnapshotRestoreAndCheckFromTheCommandLine(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "campfire.db")
	snaps, offsite := filepath.Join(dir, "snapshots"), filepath.Join(dir, "offsite")
	cfg := filepath.Join(dir, "campfire.json")
	body := fmt.Sprintf(`{"database":%q,"snapshots":{"enabled":true,"dir":%q,"keep":3,"shipTo":%q}}`,
		dbPath, snaps, offsite)
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
	if _, err := os.Stat(filepath.Join(offsite, filepath.Base(taken[0].Path))); err != nil {
		t.Fatalf("the snapshot was not shipped: %v", err)
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
