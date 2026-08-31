// Package config reads the one configuration file an instance has.
//
// A laptop and a shared server run the same application, configured the same
// way (SPEC.md §1). Every setting exists in both profiles; there are no
// editions and no profile-specific keys.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	// Addr is where the server listens. It binds to loopback only unless this
	// is explicitly set to something else: a binary run on a laptop must not
	// silently serve the network it is attached to (SPEC.md §13.1).
	Addr string `json:"addr"`
	// Database is the data file. It lives in the operating system's data
	// directory by default. Network filesystems and cloud-synchronised folders
	// are unsupported — SQLite corrupts on both.
	Database string `json:"database"`

	Snapshots Snapshots `json:"snapshots"`

	// MCP is the agent interface. Off by default: it widens the credential
	// surface of an instance whose owner has not asked for an agent
	// (SPEC.md §14.6). While it is disabled the path is not served and an
	// agent token authenticates nothing.
	MCP MCP `json:"mcp"`
}

type Snapshots struct {
	// Enabled is true by default, including for a solo user who has never
	// considered backups (SPEC.md §13.3).
	Enabled  bool   `json:"enabled"`
	Dir      string `json:"dir"`
	Interval string `json:"interval"`
	Keep     int    `json:"keep"`
	// ShipTo turns on continuous write-ahead log shipping to a directory,
	// giving point-in-time recovery. Off by default; one setting to enable.
	ShipTo string `json:"shipTo"`
}

type MCP struct {
	Enabled bool `json:"enabled"`
}

// Default is what an instance runs with when nothing is configured.
func Default() Config {
	dir := DataDir()
	return Config{
		Addr:     "127.0.0.1:8080",
		Database: filepath.Join(dir, "campfire.db"),
		Snapshots: Snapshots{
			Enabled:  true,
			Dir:      filepath.Join(dir, "snapshots"),
			Interval: "6h",
			Keep:     14,
		},
	}
}

// DataDir is the operating system's data directory for this application.
func DataDir() string {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LocalAppData"); v != "" {
			return filepath.Join(v, "Campfire")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "Campfire")
		}
	default:
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			return filepath.Join(v, "campfire")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "campfire")
		}
	}
	return "campfire-data"
}

// DefaultPath is where the configuration file is looked for.
func DefaultPath() string { return filepath.Join(DataDir(), "campfire.json") }

// Load reads the file at path, filling anything absent from Default. A missing
// file is not an error: an instance with no configuration is the individual
// profile, and it must start without one.
func Load(path string) (Config, error) {
	cfg := Default()
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Database == "" {
		return errors.New("database path is empty")
	}
	if c.Snapshots.Enabled {
		if c.Snapshots.Dir == "" {
			return errors.New("snapshots are enabled but no directory is set")
		}
		if _, err := c.SnapshotInterval(); err != nil {
			return err
		}
		if c.Snapshots.Keep < 1 {
			return errors.New("snapshots.keep must be at least 1")
		}
	}
	return nil
}

func (c Config) SnapshotInterval() (time.Duration, error) {
	if c.Snapshots.Interval == "" {
		return 6 * time.Hour, nil
	}
	d, err := time.ParseDuration(c.Snapshots.Interval)
	if err != nil {
		return 0, fmt.Errorf("snapshots.interval: %w", err)
	}
	if d < time.Minute {
		return 0, errors.New("snapshots.interval must be at least a minute")
	}
	return d, nil
}

// Write saves the configuration, creating the directory if needed. It is used
// by `campfire init` so that the file an operator edits already exists with
// every key in it.
func (c Config) Write(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o600)
}
