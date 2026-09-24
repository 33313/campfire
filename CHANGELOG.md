# Changelog

Every release of Campfire, newest first. Versions follow [Semantic Versioning](https://semver.org):
a major release needs an operator to act before upgrading, and its notes say what to do.

This file is generated from `.changes/`; edit the release's file there, then run
`go tool changie merge`.

## v0.1.0 - 2026-09-24

### Added

- First release. Download the binary for Linux, macOS or Windows from the GitHub release, or run the container image `ghcr.io/33313/campfire`.
- `campfire version` prints the version, and the server logs it when it starts.
