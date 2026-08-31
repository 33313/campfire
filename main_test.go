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

func TestAnArgumentNothingReadsIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campfire.json")
	err := run([]string{"init", "-config", path, "wat"})
	if err == nil || !strings.Contains(err.Error(), "takes no arguments") {
		t.Fatalf("got %v, want a complaint about the extra argument", err)
	}
}
