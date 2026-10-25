# Campfire

A self-hosted kanban tracker: a Go server with the Svelte client embedded, on one SQLite file.
It's pre-v1, and the only install is the maintainer's own. Tickets live in Campfire itself,
reachable through the `campfire` MCP server. For ticket work, use `/ticket <ID>`, e.g.
`/ticket DEV-120`.

## Where things are decided

- `SPEC.md` fixes behaviour and `DESIGN.md` fixes appearance. Before planning a change, read the
  sections it touches. When the code and those files disagree, point it out instead of guessing.
- README "Contributing" maps the packages.

## How to work

- Read before planning: the code the change touches, end to end, and every caller of what you
  will change.

## Writing

- UI copy, docs, README, change files and PR bodies are plain and factual: what it does, what it
  needs, what it won't do. No narrative ("you open the tab and…"), no metaphor, no selling.
- Code comments explain why, briefly. Don't copy the literary voice of older comments.

## Tests

- Go tests by default (`go test ./...`, ~5s). Test the behaviour `SPEC.md` fixes, not styling or
  what the compiler already catches.
- e2e (`e2e/campfire.spec.js`) is only for what only a browser shows: a main flow, drag and drop,
  live updates, rendering and sanitising. No tests for focus details, request counts or looks.
  Each test makes its own team with `newTeam` (`e2e/helpers.js`); never rely on another test's data.
- Run e2e locally at most once, and only when the client changed: `npx playwright test`, ~20s,
  `-g <title>` for one test. CI runs it anyway. If a failure isn't yours, say so instead of
  rerunning until it passes.
- The maintainer uses Firefox. For browser-dependent CSS or JS, also run
  `npx playwright test --browser=firefox`.
- Worktrees share the main checkout's `node_modules` through a symlink (`.claude/settings.json`).
  On a branch that changes `package.json`, replace the symlink first (`rm node_modules && npm ci`);
  otherwise `npm` rewrites every checkout's dependencies.
- Never kill `campfire` processes by name: the maintainer's own instance runs as `./campfire` from
  the main checkout (port 8080, the `campfire` MCP server). Stop a test server by its port.

## Shipping

- Name the branch and commit after the ticket: `dev-120-ci`, `DEV-120: <what changed>`.
- Commits may be signed. If a commit fails on a passphrase, ask the user to run `! ssh-add`.
  Never turn signing off.
- Every PR adds a change file (`npm run changie -- new`; a hand-written body must be quoted), or
  carries the `skip-changeset` label when nothing changes for people who run Campfire. Before v1,
  a breaking change is `Changed`: `Breaking` and `Removed` would release v1.0.0.
- A schema change appends a migration in `internal/store/store.go` (see the comment there).
  The API and MCP may still break freely before v1.
- Multi-part work goes out as stacked PRs with `gh stack`.
- Release binaries and the Docker image are the only install paths. Tags stay on main.
