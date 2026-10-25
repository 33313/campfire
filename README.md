<p align="center"><img src="docs/banner.svg" alt="Campfire" width="100%"></p>

A kanban work tracker for individuals and small teams who host it themselves. One binary and one
SQLite data file, so a laptop and a shared server run it the same way.

![Team board](docs/screenshots/board.png)

<details>
<summary><b>More screenshots</b></summary>

**Creating a ticket**
![The create dialog](docs/screenshots/create-ticket.png)

**Ticket page**
![Ticket page](docs/screenshots/item.png)

**Saving a filter as a view**
![The save-as-view dialog with two filters set](docs/screenshots/create-view.png)

**Backlog**
![Backlog](docs/screenshots/backlog.png)

**Buckets**
![Buckets and their progress](docs/screenshots/buckets.png)

**User profile**
![User profile](docs/screenshots/profile.png)
</details>

### What it offers
- Task boards, backlog, filters
- Milestones (buckets)
- Live collaboration
- MCP & API for AI agents and third-party integrations
- Customizable work states
- Audit log
- Synced PR state from various forges (GitHub, GitLab, Codeberg, Forgejo, Gitea)
- Automatic backups & point-in-time restore
- Self-hosted, copyleft, free forever

### What it won't do
- Per-item access control
- Workflow rules
- Subtasks
- Native clients (desktop, mobile)

How to use it is documented in the app, at `/docs` on your instance, and in
[`internal/web/src/docs.md`](internal/web/src/docs.md). This file covers running it.

## Install

Download the binary for your platform from the [latest release](https://github.com/33313/campfire/releases/latest):

```
curl -fLo campfire https://github.com/33313/campfire/releases/latest/download/campfire-linux-amd64
chmod +x campfire
./campfire
```

| Platform | File |
|---|---|
| Linux | `campfire-linux-amd64`, `campfire-linux-arm64` |
| macOS | `campfire-darwin-arm64` (Apple silicon), `campfire-darwin-amd64` (Intel) |
| Windows | `campfire-windows-amd64.exe`, `campfire-windows-arm64.exe` |

`SHA256SUMS` in the release checks the download: `sha256sum --ignore-missing -c SHA256SUMS`.

Linux is the primary platform. The binaries are not signed: on macOS, download with `curl` as above
or run `xattr -d com.apple.quarantine campfire` once; on Windows, SmartScreen needs *More info*, then
*Run anyway*.

### Docker

The image is for `linux/amd64` and `linux/arm64`:

```
docker run -p 8080:8080 -v campfire-data:/data ghcr.io/33313/campfire
```

It listens on `0.0.0.0:8080` and keeps the database and snapshots in `/data/campfire`. To change
other settings, mount your own configuration file over the built-in one. It must set `addr`, since
the default is loopback, which nothing outside the container can reach:

```
docker run -p 8080:8080 -v campfire-data:/data \
  -v ./campfire.json:/home/campfire/campfire.json:ro ghcr.io/33313/campfire
```

### As a service

A minimal systemd unit for Linux, with the data in `/var/lib/campfire`:

```ini
# /etc/systemd/system/campfire.service
[Unit]
Description=Campfire
After=network.target

[Service]
User=campfire
ExecStart=/usr/local/bin/campfire -data /var/lib/campfire
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```
sudo install campfire /usr/local/bin/
sudo useradd --system --home-dir /var/lib/campfire --create-home campfire
sudo systemctl enable --now campfire
```

Run other commands as the same user, with the same `-data`:
`sudo -u campfire campfire -data /var/lib/campfire snapshot`.

## First run

Campfire prints the address it listens on. Open it and create the first account, which becomes an
instance administrator. There is no setup wizard and no open registration: afterwards, an
administrator creates accounts, or hands out invitation and password-reset links, in
*Administration → Users*. Campfire sends no email.

Until that first account exists, anyone who can reach the server can create it. Create it before
the server is reachable from other machines.

## Commands

```
campfire                      # run the server (default)
campfire init                 # write a configuration file with every setting in it
campfire check                # verify the database's integrity and exit
campfire snapshot             # take a snapshot now, print its path and exit
campfire restore <snapshot>   # replace the data file with a snapshot
campfire restore -at <time>   # rebuild the data file as it was at a moment, from the replica
campfire version              # print the version and exit
```

`-data <dir>` sets the data directory, which holds the configuration file, the database and the
snapshots. `-config <path>` points at a configuration file elsewhere.

## Configuration

Campfire runs without a configuration file, on the defaults below. `campfire init` writes one with
every setting in it, to `campfire.json` in the data directory:

| OS | Default data directory |
|---|---|
| Linux | `$XDG_DATA_HOME/campfire`, or `~/.local/share/campfire` |
| macOS | `~/Library/Application Support/Campfire` |
| Windows | `%LocalAppData%\Campfire` |

```json
{
  "addr": "127.0.0.1:8080",
  "database": "/home/you/.local/share/campfire/campfire.db",
  "snapshots": {
    "enabled": true,
    "dir": "/home/you/.local/share/campfire/snapshots",
    "interval": "6h",
    "keep": 14,
    "replica": ""
  },
  "mcp": { "enabled": false },
  "liveUpdates": { "hiddenTabGrace": "1m" },
  "images": { "maxMB": 10 },
  "updates": { "check": true }
}
```

The file is read at startup; restart after editing it.

- `addr` binds to loopback, so only the same machine can reach it. To serve a network, change it and
  put a reverse proxy in front (see [Behind a proxy](#behind-a-proxy)).
- `snapshots` are on by default. See [Backup and restore](#backup-and-restore).
- `mcp.enabled` turns on the agent interface. See [Agents (MCP)](#agents-mcp).
- `liveUpdates.hiddenTabGrace` is how long a hidden browser tab keeps its live-update connection.
  The tab catches up when it is shown again.
- `images.maxMB` limits a picture pasted into a description or comment. Pictures are stored in the
  database; `0` turns them off.
- `updates.check` asks GitHub once a day whether a newer release exists, and the administration
  screen says so. `false` turns it off.

**The database must be on local disk.** SQLite corrupts on network filesystems (NFS, SMB) and in
cloud-synchronised folders (Dropbox, iCloud Drive, OneDrive, Google Drive). To keep a copy
elsewhere, use `snapshots.replica`.

To move the data to another directory, enter it in *Administration → Preferences*. The database,
and the snapshots if they are in the data directory, are copied there, the configuration file is
updated, and the server restarts. The old files are kept until you delete them. In the container
image, mount a different volume instead.

## Backup and restore

- **Snapshots** are taken every `snapshots.interval` into `snapshots.dir`, and the newest
  `snapshots.keep` are kept. Each is a complete database file, verified when written. Taking one
  doesn't stop the server.
- **Replica.** Set `snapshots.replica` to a directory, ideally a mount of another machine, and the
  write-ahead log is copied there within a second of each write, by
  [Litestream](https://litestream.io) running inside the server. You can then restore to any moment
  in the last `interval × keep` (3.5 days by default), even if the machine is lost.
- **Restoring** replaces the data file. Stop the server, run `campfire restore <snapshot>`, or
  `campfire restore -at "2026-10-03 14:05"` for a moment from the replica (`-at latest` for the
  newest), then start it. *Administration → Backup and restore* lists the snapshots and writes the
  command for you. The replaced file is renamed, not deleted.
- **Integrity** is checked at startup and every hour. If the database is corrupt at startup, the
  server doesn't start and names the snapshot to restore.

## Upgrading

Replace the binary, or pull the image, and restart. [`CHANGELOG.md`](CHANGELOG.md) lists what each
release changes. Image tags are the version (`0.18.1`), its minor (`0.18`) and `latest`; from v1,
also its major (`1`).

A newer binary migrates an older data file when it opens it. It first takes a snapshot,
`before-migration-<time>.db` in the snapshot directory, which is never pruned. There is no
downgrade: an older binary refuses a migrated file and names that snapshot, which
`campfire restore` with the older binary brings back, losing what changed since.

Versions follow [Semantic Versioning](https://semver.org). From v1, a release that isn't a major
one keeps the data file, configuration file, command line, MCP tools and permalinks compatible; see
[CONTRIBUTING.md](CONTRIBUTING.md#releases).

## Agents (MCP)

The server includes an MCP server at `/mcp`, on the same address as the app. It is off by default;
while off, `/mcp` is not served and agent tokens authenticate nothing.

1. Set `"mcp": { "enabled": true }` and restart. The log says
   `agent interface (MCP) on /mcp, authenticated by agent token`.
2. Each person issues a token under their profile's *Settings → Agent tokens*. The dialog shows the
   command that adds it to Claude Code.

An agent acts as the person who issued the token, with their permissions. Tokens can be revoked
there at any time, and an administrator can revoke any token. Connecting other clients, and what
an agent can and can't do: the *Agents (MCP)* section of the [docs](internal/web/src/docs.md).

## Pull requests

A pull request whose title starts or ends with an item's identifier (`DEV-12: fix it`,
`[DEV-12] fix it`, `fix it DEV-12`) is listed on that item, with its state. Campfire reads them from
GitHub (and GitHub Enterprise), GitLab, Forgejo and Gitea, private repositories included.

1. **Make a read-only token** on the forge. It sees what its user sees, so a bot account limited to
   a few repositories limits Campfire to those.
   - GitHub: a fine-grained token that can read pull requests, or a classic one with `repo`.
   - GitLab: a personal, group or project token with `read_api`.
   - Forgejo or Gitea: `read:repository` and `read:user`.
2. **Add it** in *Administration → Integrations*: pick the forge (GitHub, GitLab, Codeberg, or a
   self-hosted one by address) and give the token. Campfire reads the last 90 days of pull requests;
   *Sync* does it again.
3. **Choose how it stays current.** GitHub and GitLab can be polled: Campfire checks every 5 minutes
   by default, and nothing is set up on the forge. Otherwise, and always for Forgejo and Gitea, the
   forge sends a webhook: add the address and secret the screen shows, with Pull request events
   (Merge request events on GitLab). A webhook needs the forge to reach Campfire, and GitHub can't
   send one for a personal account.

Tokens and webhook secrets are stored in the database, so snapshots contain them.

## Behind a proxy

The app updates live over server-sent events on `/api/stream`. Behind a proxy:

1. **Turn response buffering off** for `/api/stream`. The server sends `X-Accel-Buffering: no`,
   which nginx honours; other proxies need it configured.
2. **Set the idle read timeout above 20 seconds**, the heartbeat interval.
3. **Allow one open connection per active browser tab**, plus headroom. The default limit of 1024
   file descriptors gets tight at a few hundred.

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

Campfire doesn't serve TLS itself. Terminate it at the proxy and set `X-Forwarded-Proto`, so the
session cookie is marked `Secure`. The server compresses its own responses.

## Contributing

```
./build.sh && ./campfire
```

`build.sh` compiles the Svelte client into `internal/web/app`, then builds the binary that embeds
it; Node is needed for that step only. `./build.sh release <version>` builds every released platform
into `dist/`.

- `go test ./...` tests the server.
- `npm test` runs the client's CSQL parser against the same corpus as the server's.
- `npx playwright test` drives the built binary in a browser.

`npm ci` points git at `.githooks`: a commit runs gofmt, vet and the unit tests, except for a change
to words alone. `--no-verify` skips them once.

| Path | What is there |
|---|---|
| `main.go`, `backup.go` | The command: serve, init, check, snapshot, restore, version. |
| `internal/store` | The schema and every query. One SQLite file behind a single writer and a read pool. |
| `internal/csql` | CSQL, the filter language: parser, printer and compiler to SQL. `testdata/corpus.json` is what it and the client's parser must both say. |
| `internal/core` | The application. Every operation exists here once, behind one permission check and one validation. |
| `internal/api` | The HTTP interface described by [`api/openapi.yaml`](api/openapi.yaml), plus the event stream. |
| `internal/mcp` | The agent interface: a second interface over the same operations. |
| `internal/forge` | Pull requests read from GitHub, GitLab, Forgejo and Gitea. |
| `internal/web` | The client: Svelte, built by Vite into `app/`, embedded and served by the server. |
| `internal/auth`, `internal/backup`, `internal/config`, `internal/stream`, `internal/updates` | Credentials, snapshots and the replica, the configuration file, the live-update hub, and the release check. |
| `e2e` | The Playwright suite. It runs the real binary against a throwaway data directory. |
| `.changes` | Release notes: unreleased change files, and one file per release. |

[`SPEC.md`](SPEC.md) fixes behaviour and [`DESIGN.md`](DESIGN.md) fixes appearance; the tests cite
them. [CONTRIBUTING.md](CONTRIBUTING.md) covers change files and releases.

## License

Copyright (C) 2026 33313

Campfire is free software: you can redistribute it and/or modify it under the terms of the GNU
Affero General Public License, version 3 or (at your option) any later version. It comes with no
warranty. If you run a modified Campfire for others over a network, offer them its source. See
[`LICENSE`](LICENSE).

The sign-in page and the last section of the documentation link to the source. A modified Campfire
run for others complies by pointing [`internal/web/src/routes/SignIn.svelte`](internal/web/src/routes/SignIn.svelte)
and the *Source code* section of [`internal/web/src/docs.md`](internal/web/src/docs.md) at its own.

The bundled IBM Plex and Source Serif 4 fonts are under the SIL Open Font License 1.1; see
[`internal/web/static/fonts/OFL.txt`](internal/web/static/fonts/OFL.txt).
