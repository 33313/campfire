package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC.md §13.1: the server MUST bind to loopback only unless explicitly
// configured otherwise, and snapshots are on by default.
func TestDefaultsAreSafeAndBackedUp(t *testing.T) {
	cfg := Default(DataDir())
	if !strings.HasPrefix(cfg.Addr, "127.0.0.1:") {
		t.Fatalf("the default bind address is %q", cfg.Addr)
	}
	if !cfg.Snapshots.Enabled {
		t.Fatal("snapshots are off by default")
	}
	// The agent interface is off by default: it widens the credential surface
	// of an instance whose owner has not asked for an agent (SPEC.md §14.6).
	if cfg.MCP.Enabled {
		t.Fatal("the agent interface is on by default")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the defaults do not validate: %v", err)
	}
}

// An instance with no configuration is the individual profile, and it must
// start without one.
func TestAMissingFileIsNotAnError(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nothing-here.json"), DataDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != Default(DataDir()).Addr {
		t.Fatal("a missing file did not fall back to the defaults")
	}
}

func TestWriteThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campfire.json")
	want := Default(DataDir())
	want.Addr = "0.0.0.0:9000"
	want.MCP.Enabled = true
	want.Snapshots.Interval = "30m"
	want.LiveUpdates.HiddenTabGrace = "5m"
	if err := want.Write(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, DataDir())
	if err != nil {
		t.Fatal(err)
	}
	if got.Addr != want.Addr || !got.MCP.Enabled || got.Snapshots.Interval != "30m" {
		t.Fatalf("round trip gave %+v", got)
	}
	d, err := got.SnapshotInterval()
	if err != nil || d.Minutes() != 30 {
		t.Fatalf("interval %v %v", d, err)
	}
	if g, err := got.HiddenTabGrace(); err != nil || g.Minutes() != 5 {
		t.Fatalf("hidden tab grace %v %v", g, err)
	}
	got.LiveUpdates.HiddenTabGrace = "-1s"
	if got.Validate() == nil {
		t.Fatal("a negative hidden tab grace was accepted")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("the configuration file is mode %v; it is readable by others", info.Mode().Perm())
	}
}

// A typo in the file should stop the server rather than silently running on a
// default the operator did not choose.
func TestUnknownKeysAreRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campfire.json")
	os.WriteFile(path, []byte(`{"adress":"0.0.0.0:9000"}`), 0o600)
	if _, err := Load(path, DataDir()); err == nil {
		t.Fatal("a misspelled key was accepted")
	}
}

func TestBadSnapshotSettingsAreRejected(t *testing.T) {
	cfg := Default(DataDir())
	cfg.Snapshots.Interval = "3s"
	if err := cfg.Validate(); err == nil {
		t.Fatal("a three-second snapshot interval was accepted")
	}
	cfg = Default(DataDir())
	cfg.Snapshots.Keep = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("keeping zero snapshots was accepted")
	}
	cfg = Default(DataDir())
	cfg.Database = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("an empty database path was accepted")
	}
}
