# Contributing

Building, testing and the package map are in the [README](README.md#contributing).

## Specification

[`SPEC.md`](SPEC.md) fixes behaviour: the entities, permissions, lifecycle, API, storage and agent
interface. [`DESIGN.md`](DESIGN.md) fixes appearance: colour and type tokens, geometry, component
states, icon paths. The tests cite them. Where they appear to disagree, `SPEC.md` wins on behaviour
and `DESIGN.md` on appearance.

## Tests

The browser suite runs in CI. Each checkout serves it on its own port (`E2E_PORT` overrides it), so
worktrees can run it side by side.

## Releases

Every merge into main that carries a change file is a release; nobody bumps a version or pushes a
tag by hand. A pull request adds one with `npm run changie -- new` (or, without prompts,
`npm run changie -- new --kind Fixed --body "…"`). changie is pinned in `package.json` and run
with `go run`, so there is nothing to install and nothing of it in `go.mod`.

From v1, a version promises compatibility for what other things depend on:

- **The data file.** A binary opens any data file, or snapshot, written by its own version or an
  earlier one, migrating it if it's older (README "Upgrading"). A schema change therefore needs a
  migration, not a major release.
- **The configuration file**: its keys and what they mean.
- **The command line**: commands, flags, and output a script reads (`campfire snapshot` prints the
  path of the snapshot).
- **The MCP tools**: names, arguments and result shapes.
- **Addresses kept outside the instance**: permalinks (`/SOL-42`), image addresses in item text
  (`/api/images/12`), a forge's webhook address, `/mcp`, and `/api/stream`, which proxies name.

The HTTP API is not covered. It has one client, served by the same binary, and any release may change
both together ([SPEC §11](SPEC.md#11-api-contract)), the live-update stream's payloads included.

The kind sets the bump. Breaking and Removed are incompatible changes to the covered surface:

| Kind | Bump |
|---|---|
| Breaking, Removed | major |
| Added, Changed, Deprecated | minor |
| Fixed, Security | patch |

A pull request that changes nothing for the people who run Campfire (tests, CI, contributor docs)
carries the `skip-changeset` label instead. The `changeset` check requires one or the other, and its
summary previews the release.

On the merge, [`release.yaml`](.github/workflows/release.yaml) writes `CHANGELOG.md`, commits and
tags `Release vX.Y.Z` with the deploy key in the `GH_DK` secret, pushes the image to ghcr.io, and
publishes the GitHub release with the binaries and `SHA256SUMS`. If it fails after the tag is
pushed, rerun it: it finishes a release that has no published GitHub release.
