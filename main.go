// Campfire: a self-hosted kanban work tracker.
//
// One binary, one process, one data file (SPEC.md §13). The web client is
// embedded in and served by this program; backup is copying a file, and
// recovery is putting it back.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/33313/campfire/internal/api"
	"github.com/33313/campfire/internal/backup"
	"github.com/33313/campfire/internal/config"
	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/mcp"
	"github.com/33313/campfire/internal/store"
	"github.com/33313/campfire/internal/stream"
	"github.com/33313/campfire/internal/updates"
	"github.com/33313/campfire/internal/web"
)

// version is set when a release is built: -ldflags "-X main.version=v1.2.3".
var version string

// buildVersion falls back to what the Go toolchain stamps from the git
// checkout it built: a pseudo-version such as v0.1.1-0.20260924…-abc123, with
// +dirty when the tree had uncommitted changes.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// errRestart asks main to start the program again, with the same arguments.
var errRestart = errors.New("restart")

func main() {
	log.SetFlags(log.Ltime)
	err := run(os.Args[1:])
	if errors.Is(err, errRestart) {
		log.Print("restarting")
		err = restart()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "campfire:", err)
		os.Exit(1)
	}
}

func usage() string {
	return `campfire — a self-hosted kanban work tracker

usage:
  campfire [serve]              run the server (default)
  campfire init                 write a configuration file with every setting in it
  campfire snapshot             take a snapshot now and exit
  campfire restore <snapshot>   replace the data file with a snapshot
  campfire restore -at <time>   rebuild the data file as it was at a moment, from
                                snapshots.replica ("latest" for the newest)
  campfire check                verify the database's integrity and exit
  campfire version              print the version and exit

flags:
  -data <dir>      data directory: the configuration file, database and snapshots
                   (default: the operating system's data directory)
  -config <path>   configuration file (default: campfire.json in the data directory)
  -version         print the version and exit
`
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		command, args = args[0], args[1:]
	}

	fs := flag.NewFlagSet("campfire", flag.ContinueOnError)
	dataDir := fs.String("data", config.DataDir(), "data directory")
	configPath := fs.String("config", "", "configuration file")
	showVersion := fs.Bool("version", false, "print the version and exit")
	at := fs.String("at", "", "restore: the moment to restore to")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage()) }
	if err := fs.Parse(args); err != nil {
		return err
	}

	// The subcommand may equally follow the flags: "campfire -config x
	// snapshot" asks for the same thing as "campfire snapshot -config x".
	// Reading it only in the leading position left the flags-first spelling
	// running the server instead, which is the worst of the answers available
	// — it takes no snapshot and says nothing.
	rest := fs.Args()
	if command == "serve" && len(rest) > 0 {
		command = rest[0]
		// Flags may follow it too: campfire -config x restore -at latest.
		if err := fs.Parse(rest[1:]); err != nil {
			return err
		}
		rest = fs.Args()
	}

	// Read before the configuration: the version is asked for most when
	// something is wrong, and that may be the configuration.
	if command == "version" || *showVersion {
		if err := requireNoArgs("version", rest); err != nil {
			return err
		}
		fmt.Println(buildVersion())
		return nil
	}

	if *at != "" && command != "restore" {
		return fmt.Errorf("-at is for restore, not %s", command)
	}

	// Absolute, so that the paths init writes still hold when the server is
	// started from another directory.
	dir, err := filepath.Abs(*dataDir)
	if err != nil {
		return err
	}
	if *configPath == "" {
		*configPath = config.Path(dir)
	}
	cfg, err := config.Load(*configPath, dir)
	if err != nil {
		return err
	}

	switch command {
	case "init":
		if err := requireNoArgs(command, rest); err != nil {
			return err
		}
		if err := cfg.Write(*configPath); err != nil {
			return err
		}
		fmt.Println("wrote", *configPath)
		return nil
	case "restore":
		if *at != "" {
			if err := requireNoArgs("restore -at", rest); err != nil {
				return err
			}
			return restoreAt(cfg, *at)
		}
		if len(rest) != 1 {
			return errors.New("restore needs the path of a snapshot, or -at and a moment")
		}
		return restore(cfg, rest[0])
	case "snapshot":
		if err := requireNoArgs(command, rest); err != nil {
			return err
		}
		return snapshotOnce(cfg)
	case "check":
		if err := requireNoArgs(command, rest); err != nil {
			return err
		}
		return check(cfg)
	case "serve":
		if err := requireNoArgs(command, rest); err != nil {
			return err
		}
		moveTo, err := serve(cfg, *configPath)
		if err != nil || moveTo == "" {
			return err
		}
		// A failed move restarts with the old data directory.
		if err := moveData(cfg, *configPath, moveTo); err != nil {
			log.Printf("moving the data to %s failed; still using %s: %v", moveTo, filepath.Dir(cfg.Database), err)
		}
		return errRestart
	case "help", "-h", "--help":
		fmt.Print(usage())
		return nil
	}
	return fmt.Errorf("unknown command %q\n\n%s", command, usage())
}

// An argument nothing reads is a request the program has misunderstood, so it
// says so rather than carrying on.
func requireNoArgs(command string, rest []string) error {
	if len(rest) > 0 {
		return fmt.Errorf("%s takes no arguments, got %q\n\n%s", command, rest[0], usage())
	}
	return nil
}

func open(cfg config.Config) (*store.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.Database), 0o700); err != nil {
		return nil, err
	}
	// Integrity is checked at startup; on detecting corruption the server
	// refuses to start and names the snapshot to restore from
	// (SPEC.md §13.3 rule 5).
	db, err := store.OpenMigrating(cfg.Database, func(db *store.DB, from int) error {
		snap, err := backup.TakeBeforeMigration(context.Background(), db, cfg.Snapshots.Dir)
		if err != nil {
			return err
		}
		log.Printf("migrating %s from schema version %d to %d; to go back, restore %s with the older binary",
			db.Path, from, store.Version(), snap.Path)
		return nil
	})
	var tooNew *store.TooNewError
	if errors.As(err, &tooNew) {
		return nil, withDowngradeAdvice(cfg, err)
	}
	if err != nil {
		return nil, withRecoveryAdvice(cfg, err)
	}
	return db, nil
}

// withDowngradeAdvice names the snapshot taken before a newer binary migrated
// the file, since there are no down-migrations.
func withDowngradeAdvice(cfg config.Config, err error) error {
	snap := backup.BeforeMigration(cfg.Snapshots.Dir, store.Version())
	if snap == "" {
		return fmt.Errorf("%w\n\nrun the newer binary; no snapshot from before the migration was found in %s", err, cfg.Snapshots.Dir)
	}
	return fmt.Errorf("%w\n\nrun the newer binary, or go back to %s, taken before the migration, losing what changed since:  campfire restore %s",
		err, snap, snap)
}

func check(cfg config.Config) error {
	db, err := open(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.CheckIntegrity(); err != nil {
		return withRecoveryAdvice(cfg, err)
	}
	fmt.Println("ok:", cfg.Database)
	return nil
}

// serve runs the server until it is stopped. moveTo is the directory an
// administrator asked to move the data to, if any.
func serve(cfg config.Config, configPath string) (moveTo string, err error) {
	db, err := open(cfg)
	if err != nil {
		return "", err
	}
	defer db.Close()

	bus := stream.NewBus()
	svc := core.New(db)
	// Events are published after the transaction that wrote them has
	// committed, so a client that refetches on the notification finds the
	// change already there.
	svc.Notify = func(e store.Event) {
		bus.Publish(stream.Notification{
			Seq: e.Seq, Kind: e.Kind, Identifier: e.ItemIdentifier, TeamID: e.TeamID})
	}

	mux := http.NewServeMux()
	apiServer := api.New(svc, bus)
	apiServer.HiddenTabGrace, _ = cfg.HiddenTabGrace() // validated on load
	apiServer.MaxImage = cfg.Images.MaxMB << 20
	apiServer.Snapshots = api.Snapshots{Snapshots: cfg.Snapshots,
		Take: func(ctx context.Context) (backup.Snapshot, error) { return takeSnapshot(ctx, cfg, db) }}
	apiServer.Updates = updates.New(buildVersion())
	moves := make(chan string, 1)
	apiServer.Data = api.Data{Database: cfg.Database, Config: configPath, Snapshots: cfg.Snapshots.Dir,
		MovesSnapshots: movesSnapshots(cfg),
		// In the container image the volume sets the data directory, and a
		// rewritten configuration file is lost with the container.
		Fixed: os.Getenv("CAMPFIRE_CONTAINER") != "",
		Move: func(dir string) error {
			if err := checkMoveTarget(cfg, configPath, dir); err != nil {
				return err
			}
			select {
			case moves <- dir:
				return nil
			default:
				return errors.New("the data is already moving")
			}
		}}
	mux.Handle("/api/", apiServer)
	if cfg.MCP.Enabled {
		// The agent interface is served from this process, on this origin, at
		// a fixed path, over streamable HTTP (SPEC.md §14.2). An agent that
		// speaks only stdio puts its own proxy in front.
		mux.Handle("/mcp", mcp.New(svc, buildVersion()))
		log.Print("agent interface (MCP) on /mcp, authenticated by agent token")
	} else {
		// While it is disabled the path MUST NOT be served, so that turning
		// the setting off is a complete answer to whether the interface is
		// reachable (SPEC.md §14.6).
		log.Print("agent interface (MCP) is off; enable it in the configuration file")
	}
	mux.Handle("/", web.Handler())

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		// No write timeout: the event stream is a long-lived response, and a
		// deadline here would cut it at the deadline rather than at the
		// client's own reconnection (SPEC.md §12.5).
		IdleTimeout: 120 * time.Second,
	}

	stopSnapshots := startSnapshots(cfg, db)
	defer stopSnapshots()
	stopReplica, err := startReplica(cfg, db)
	if err != nil {
		return "", err
	}
	defer stopReplica()
	stopChecks := startIntegrityChecks(cfg, db)
	defer stopChecks()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go apiServer.PollForges(ctx)
	if cfg.Updates.Check {
		go apiServer.Updates.Run(ctx)
	} else {
		log.Print("update check is off")
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-ctx.Done():
		case moveTo = <-moves:
		}
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		// Close event streams still open after the grace period.
		if srv.Shutdown(shutdown) != nil {
			srv.Close()
		}
	}()

	log.Printf("campfire %s listening on http://%s (data: %s)", buildVersion(), cfg.Addr, cfg.Database)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return "", err
	}
	// ListenAndServe returns when shutdown starts. Running requests finish
	// before the database closes.
	<-stopped
	log.Print("stopped")
	return moveTo, nil
}

// movesSnapshots reports whether the snapshots are in the data directory, the
// default. If so, they move with the database.
func movesSnapshots(cfg config.Config) bool {
	return cfg.Snapshots.Dir == filepath.Join(filepath.Dir(cfg.Database), "snapshots")
}

// checkMoveTarget returns why the data can't move to dir. It runs before the
// server stops.
func checkMoveTarget(cfg config.Config, configPath, dir string) error {
	if !filepath.IsAbs(dir) {
		return errors.New("give the full path of a directory")
	}
	dir = filepath.Clean(dir)
	if dir == filepath.Dir(cfg.Database) {
		return errors.New("the data is already there")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "campfire.db")); err == nil {
		return fmt.Errorf("%s already holds a campfire.db", dir)
	}
	probe, err := os.CreateTemp(dir, ".campfire-")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	probe.Close()
	os.Remove(probe.Name())
	// The configuration file is rewritten to point at the new directory.
	if f, err := os.OpenFile(configPath, os.O_WRONLY, 0); err == nil {
		f.Close()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot rewrite %s: %w", configPath, err)
	}
	return nil
}

// moveData copies the database, and the snapshots if movesSnapshots, to dir
// and points the configuration file there. Run it with the server stopped.
// The old files are kept. A failure before the configuration file is written
// changes nothing.
func moveData(cfg config.Config, configPath, dir string) error {
	dir = filepath.Clean(dir)
	db, err := store.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()
	// A snapshot is a verified, self-contained copy, write-ahead log included.
	snap, err := backup.Take(context.Background(), db, dir)
	if err != nil {
		os.Remove(snap.Path)
		return err
	}
	moved := cfg
	moved.Database = filepath.Join(dir, "campfire.db")
	if err := os.Rename(snap.Path, moved.Database); err != nil {
		os.Remove(snap.Path)
		return err
	}
	if movesSnapshots(cfg) {
		moved.Snapshots.Dir = filepath.Join(dir, "snapshots")
		if err := backup.CopyAll(cfg.Snapshots.Dir, moved.Snapshots.Dir); err != nil {
			os.Remove(moved.Database)
			return err
		}
	}
	if err := moved.Write(configPath); err != nil {
		os.Remove(moved.Database)
		return err
	}
	log.Printf("moved the data to %s; the old files in %s can be deleted", dir, filepath.Dir(cfg.Database))
	return nil
}
