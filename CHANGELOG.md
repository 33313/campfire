# Changelog

Every release of Campfire, newest first. Versions follow [Semantic Versioning](https://semver.org).

This file is generated from `.changes/`; edit the release's file there, then run
`go tool changie merge`.

## v0.3.0 - 2026-09-25

### Added

- A sign-out button beside your name at the foot of the sidebar, on every screen.

## v0.2.0 - 2026-09-25

### Changed

- Agent actions in the log, and agent tokens in Administration, are marked with a robot instead of a terminal.
- The board hides items that have been done for more than three days. Turn on "Show old done" in the filter bar to see them again; the API and the get_board agent tool take allDone for the same.

## v0.1.0 - 2026-09-24

### Added

- First release. Download the binary for Linux, macOS or Windows from the GitHub release, or run the container image `ghcr.io/33313/campfire`.
- `campfire version` prints the version, and the server logs it when it starts.
