package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/33313/campfire/internal/backup"
	"github.com/33313/campfire/internal/config"
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

// -data sets the directory for the configuration file, and the default
// directory for the database and snapshots.
func TestDataDirectoryHoldsEverything(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"init", "-data", dir}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(dir, "campfire.json"), "elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database != filepath.Join(dir, "campfire.db") || cfg.Snapshots.Dir != filepath.Join(dir, "snapshots") {
		t.Fatalf("init in %s wrote database %s, snapshots %s", dir, cfg.Database, cfg.Snapshots.Dir)
	}
}

// DEV-116: moving the data copies the database and snapshots, points the
// configuration file at the copy, and keeps the old files.
func TestMovingTheDataDirectory(t *testing.T) {
	old, to := t.TempDir(), filepath.Join(t.TempDir(), "elsewhere")
	configPath := filepath.Join(old, "campfire.json")
	cfg := config.Default(old)
	db, err := store.Open(cfg.Database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.New(db).Bootstrap(context.Background(), "root", "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	snap, err := backup.Take(context.Background(), db, cfg.Snapshots.Dir)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	for dir, why := range map[string]string{"relative": "full path", old: "already there"} {
		if err := checkMoveTarget(cfg, configPath, dir); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("moving to %s: got %v, want %q", dir, err, why)
		}
	}
	if err := checkMoveTarget(cfg, configPath, to); err != nil {
		t.Fatal(err)
	}
	if err := moveData(cfg, configPath, to); err != nil {
		t.Fatal(err)
	}

	moved, err := config.Load(configPath, old)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Database != filepath.Join(to, "campfire.db") || moved.Snapshots.Dir != filepath.Join(to, "snapshots") {
		t.Fatalf("the configuration points at %s and %s", moved.Database, moved.Snapshots.Dir)
	}
	db, err = store.Open(moved.Database)
	if err != nil {
		t.Fatal(err)
	}
	needs, err := core.New(db).NeedsBootstrap(context.Background())
	db.Close()
	if err != nil || needs {
		t.Fatalf("the moved database lost its account: %v", err)
	}
	copied, err := backup.List(moved.Snapshots.Dir)
	if err != nil || len(copied) != 1 || !copied[0].Taken.Equal(snap.Taken) {
		t.Fatalf("snapshots after the move: %v, %v; want one taken %v", copied, err, snap.Taken)
	}
	if _, err := os.Stat(cfg.Database); err != nil {
		t.Fatal("the old database is gone")
	}
	if err := checkMoveTarget(moved, configPath, filepath.Dir(cfg.Database)); err == nil {
		t.Fatal("moving back over the old campfire.db was allowed")
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

// Opening an older file snapshots it before migrating. That snapshot restores
// with the older schema, a newer binary's file is refused with its name, and
// restoring it with this binary migrates it again, through the same path.
func TestSnapshotBeforeMigrationIsTheWayBack(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "campfire.db")
	snaps := filepath.Join(dir, "snapshots")
	cfg := filepath.Join(dir, "campfire.json")
	body := fmt.Sprintf(`{"database":%q,"snapshots":{"enabled":true,"dir":%q,"keep":3}}`, dbPath, snaps)
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	setVersion := func(v int) {
		t.Helper()
		db, err := store.OpenMigrating(dbPath, func(*store.DB, int) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.W.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v)); err != nil {
			t.Fatal(err)
		}
	}
	taken := func() []string {
		t.Helper()
		paths, _ := filepath.Glob(filepath.Join(snaps, "before-migration-*.db"))
		return paths
	}

	setVersion(0) // a file from before versions were recorded
	if err := run([]string{"-config", cfg, "check"}); err != nil {
		t.Fatal(err)
	}
	if v, _ := store.FileVersion(dbPath); v != store.Version() || len(taken()) != 1 {
		t.Fatalf("after opening: version %d, snapshots %v", v, taken())
	}
	snap := taken()[0]
	older := filepath.Join(dir, "older.db")
	if err := backup.Restore(snap, older); err != nil {
		t.Fatal(err)
	}
	if v, _ := store.FileVersion(older); v != 0 {
		t.Fatalf("the snapshot restored at version %d, want the older 0", v)
	}

	setVersion(store.Version() + 1)
	if err := run([]string{"-config", cfg, "check"}); err == nil || !strings.Contains(err.Error(), "campfire restore "+snap) {
		t.Fatalf("a newer file was not refused with the way back: %v", err)
	}

	if err := run([]string{"-config", cfg, "restore", snap}); err != nil {
		t.Fatal(err)
	}
	if v, _ := store.FileVersion(dbPath); v != store.Version() || len(taken()) != 2 {
		t.Fatalf("after restoring: version %d, snapshots %v", v, taken())
	}
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
