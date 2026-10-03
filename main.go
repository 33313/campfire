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

func main() {
	log.SetFlags(log.Ltime)
	if err := run(os.Args[1:]); err != nil {
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
                                the replica in snapshots.shipTo ("latest" for the newest)
  campfire check                verify the database's integrity and exit
  campfire version              print the version and exit

flags:
  -config <path>   configuration file (default: the data directory)
  -version         print the version and exit
`
}

func run(args []string) error {
	command := "serve"
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		command, args = args[0], args[1:]
	}

	fs := flag.NewFlagSet("campfire", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "configuration file")
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

	cfg, err := config.Load(*configPath)
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
		return serve(cfg)
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
	db, err := store.Open(cfg.Database)
	if err != nil {
		return nil, withRecoveryAdvice(cfg, err)
	}
	return db, nil
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

func serve(cfg config.Config) error {
	db, err := open(cfg)
	if err != nil {
		return err
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
	apiServer.Snapshots = api.Snapshots{Snapshots: cfg.Snapshots,
		Take: func(ctx context.Context) (backup.Snapshot, error) { return takeSnapshot(ctx, cfg, db) }}
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
		return err
	}
	defer stopReplica()
	stopChecks := startIntegrityChecks(cfg, db)
	defer stopChecks()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		srv.Shutdown(shutdown)
	}()

	log.Printf("campfire %s listening on http://%s (data: %s)", buildVersion(), cfg.Addr, cfg.Database)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Print("stopped")
	return nil
}
