# Changelog

Every release of Campfire, newest first. Versions follow [Semantic Versioning](https://semver.org).

This file is generated from `.changes/`; edit the release's file there, then run
`go tool changie merge`.

## v0.4.0 - 2026-09-30

### Added

- A campfire mark: lit while live updates are connected, it flares when a change arrives, and it is the loading screen. Screens, dialogs, menus, toasts and cards move instead of appearing, and a card that reaches the final state throws sparks. Reduced motion turns all of it off.
- A dragged card is held under the pointer, sways as it is carried, and settles into place when it is dropped.
- A card that reaches the final state feeds the fire. It kindles, throws embers that rise and cool, and sends one to the campfire in the sidebar, which roars up when it arrives.

### Changed

- Brighter accent, alert and ok colours.
- P1 is drawn in red, and blue no longer marks a priority. Bucket chips drop their fill, and the rust bucket colour is now brown.
- The interface no longer zooms itself on large displays. To make it bigger, use your desktop's display scaling or the browser's zoom. This fixes cards and menus landing in the wrong place when dragged, dropped or opened on those displays.

### Fixed

- A bucket name that wraps onto two lines keeps its border around the name instead of stretching to the width of the card.
- A card dragged between two others lands between them, not at the top of the column, and an amber line shows where it will land while it is held. Escape cancels a drag.

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
