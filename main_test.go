package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
