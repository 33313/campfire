# Campfire

A self-hosted kanban work tracker for individuals and small teams. One binary, one data file.
Backup is copying that file; recovery is putting it back.

- [Run it](#run-it)
- [Commands](#commands)
- [Configuration](#configuration)
- [Agents (MCP)](#agents-mcp)
- [Backup and recovery](#backup-and-recovery)
- [Behind a proxy](#behind-a-proxy)
- [Container image](#container-image)
- [Scale](#scale)
- [Working on it](#working-on-it)

## Run it

```
./build.sh && ./campfire
```

`build.sh` compiles the Svelte client into `internal/web/app`, then builds the binary that embeds
it. Node is needed for that step and nothing else — the released binary carries the client inside
it.

It prints an address. Open it and create the first account: that account is an instance
administrator. There is no setup wizard, no database to install, and no open registration
afterwards — an administrator creates accounts or hands out an invitation link.

## Commands

```
campfire                 # run the server (default)
campfire init            # write a configuration file with every setting in it
campfire check           # verify the database's integrity and exit
campfire snapshot        # take a snapshot now and exit
campfire restore <path>  # replace the data file with a snapshot
```

`-config <path>` points at a configuration file other than the default.

## Configuration

One JSON file, in the operating system's data directory:

| OS | Path |
|---|---|
| Linux | `~/.local/share/campfire/campfire.json` |
| macOS | `~/Library/Application Support/Campfire/campfire.json` |
| Windows | `%LocalAppData%\Campfire\campfire.json` |

```json
{
  "addr": "127.0.0.1:8080",
  "database": "/home/you/.local/share/campfire/campfire.db",
  "snapshots": {
    "enabled": true,
    "dir": "/home/you/.local/share/campfire/snapshots",
    "interval": "6h",
    "keep": 14,
    "shipTo": ""
  },
  "mcp": { "enabled": false }
}
```

Three defaults are worth knowing:

- **`addr` binds to loopback.** A binary run on a laptop must not silently serve the network it is
  attached to. Change it deliberately, and put a reverse proxy in front when you do.
- **`snapshots.enabled` is true**, including for one person who has never considered backups.
  Snapshots are online: no locking, no downtime, each verified as it is written.
- **`mcp.enabled` is false.** While it is off, `/mcp` is not served and an agent token
  authenticates nothing.

**The database must be on local disk.** Network filesystems (NFS, SMB) and cloud-synchronised
folders (Dropbox, iCloud Drive, OneDrive, Google Drive) are **unsupported**: SQLite's locking is not
honoured across them and the file corrupts, often silently and only under load. To keep the data
elsewhere, point `snapshots.shipTo` there and leave the live file local.

## Agents (MCP)

The server is the MCP server — same origin, same port, nothing to package and no second process.

1. **Enable it.** Set `mcp.enabled` to `true` and restart. The log line
   `agent interface (MCP) on /mcp` confirms it.
2. **Issue a token** on your account screen. It is shown once.
3. **Install it.** That same dialog shows the finished command:

   ```
   claude mcp add --scope user --transport http campfire \
     https://your-instance/mcp --header "Authorization: Bearer <token>"
   ```

   To keep the token out of the file, export it in your shell and pass
   `--header 'Authorization: Bearer ${CAMPFIRE_MCP}'` instead — the variable is read from the
   client's environment at call time.

A client configured with JSON wants the same three facts:

```json
{
  "mcpServers": {
    "campfire": {
      "type": "http",
      "url": "https://your-instance/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

A stdio-only client needs a bridge; Campfire ships none of its own. Use
`npx -y mcp-remote https://your-instance/mcp --header "Authorization: Bearer <token>"`.

An agent acts as the person who issued the token, with exactly that person's permissions, resolved
on every call. It can create and edit tickets and buckets, move and rank them, manage bucket
membership, labels, relations, comments and saved views, and read boards, backlogs, buckets, items
and the log. It cannot create accounts, change teams or membership, reassign a bucket's owning
team, delete anything, take or restore backups, or issue tokens — those operations have no tool at
all. Every mutation records the token it arrived on; filter the log with the Agent chip to see what
agents did.

An instance bound to loopback is reachable only from its own machine, by an agent as much as by a
browser.

## Backup and recovery

- **Snapshots** run on `snapshots.interval` into `snapshots.dir`, keeping `snapshots.keep`. Each is
  a complete, self-contained database file, verified when written and again whenever the
  administration screen lists it.
- **Off-host copies.** Set `snapshots.shipTo` to a second directory — a mount of another machine, an
  object-store gateway — and every snapshot is copied there as it is taken. The recovery point is
  the last snapshot, so shorten the interval if you need a tighter one.
- **Restoring** replaces the data file, which the running process holds open. Stop the server, run
  `campfire restore <snapshot>`, start it again. The replaced file is moved aside rather than
  deleted, so restoring the wrong snapshot is itself recoverable.
- **Integrity** is checked at startup and hourly. On corruption at startup the server refuses to
  start and names the snapshot to restore from.

There is no failover, no rolling deployment, no high availability: recovery from total host loss is
restore time, in minutes. Restarts take seconds, and sessions live in the database, so a restart
logs nobody out.

## Behind a proxy

The board updates in place over server-sent events. Three things must hold for that long-lived
response to survive an intervening proxy:

1. **Response buffering off** for `/api/stream`. The server sends `X-Accel-Buffering: no`, which
   nginx honours; other proxies need it said explicitly.
2. **Idle read timeout above the heartbeat interval.** The server beats every 20 seconds, well
   inside the common 60-second default.
3. **File-descriptor limits allowing one open connection per active client**, plus headroom. Fifty
   people need nothing unusual; the default 1024 is thin at a few hundred.

```nginx
location /api/stream {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 1h;
}

location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Serve it over TLS. The session cookie is marked `Secure` only when the server knows it is on HTTPS,
so terminate TLS at the proxy and keep the instance on loopback behind it.

## Container image

```
docker build -t campfire .
docker run -p 8080:8080 -v campfire-data:/data campfire
```

The image binds `0.0.0.0:8080` because loopback inside a container reaches nobody. Everything else
is the same application, configured the same way; `/data` holds the database and the snapshots.

## Scale

Sized for 10,000 registered users and ~3,000 active per day: about 100,000 writes a day, under two
per second on average and tens per second at peak; under 100 GB after five years with the audit log
included. One ordinary machine handles this with room to spare. Sharding, read replicas, a caching
tier and a job queue are not built, and should not be introduced without measurements contradicting
that.

## Working on it

| Path | What is there |
|---|---|
| `main.go`, `backup.go`, `mcp.go` | The command: serve, init, check, snapshot, restore. |
| `internal/store` | The schema, the migrations, and every query. One SQLite file behind a single writer and a read pool. |
| `internal/core` | The application. Every operation exists here exactly once, behind one permission check and one validation. |
| `internal/api` | The HTTP interface described by [`api/openapi.yaml`](api/openapi.yaml), plus the event stream. |
| `internal/mcp` | The agent interface: a second interface over the same handlers. |
| `internal/web` | The client — Svelte, built by Vite into `app/`, embedded and served by the server. |
| `internal/auth`, `internal/backup`, `internal/config`, `internal/stream` | Credentials, snapshots, the configuration file, and the fan-out hub. |
| `e2e` | The Playwright suite. It builds nothing: it runs the real binary against a throwaway data directory and drives it in a browser. |

`go test ./...` covers the server; `npx playwright test` drives the built binary in a browser.

Two documents are normative, and the tests cite them. [`SPEC.md`](SPEC.md) fixes behaviour — the
entities, permissions, lifecycle, API, storage and agent interface — and says nothing about
appearance. [`DESIGN.md`](DESIGN.md) fixes appearance — colour and type tokens, geometry, component
states, icon paths — and says nothing about behaviour. Where they appear to disagree, `SPEC.md`
wins on behaviour and `DESIGN.md` on appearance.

The mockups `DESIGN.md` describes are **not in this repository**: they live in `~/dev/.design/kanban/`
as one `.dc.html` file per screen plus `canvas.json`. `SPEC.md` and `DESIGN.md` are symlinked into
that directory, so editing either from either side edits the copy tracked here.
