# Changelog

Every release of Campfire, newest first. Versions follow [Semantic Versioning](https://semver.org).

This file is generated from `.changes/`; edit the release's file there, then run
`go tool changie merge`.

## v0.6.0 - 2026-10-02

### Changed

- Live updates cost far less. A change refetches only on screens that show something it touches, so another team's work costs your board nothing. A hidden tab refetches nothing, closes its connection after `liveUpdates.hiddenTabGrace` (a minute by default) and catches up once when shown. Responses are compressed and fingerprinted, so a refetch that finds nothing new is a 304 with no body, and a reload costs a few kilobytes instead of almost 400. A screen you come back to is drawn at once and checked behind it. After a network drop the stream reconnects as soon as the network or the tab is back, and a tab too far behind to catch up recovers without asking for a reload.

### Fixed

- A date chip's presets follow its field when the field changes.

## v0.5.0 - 2026-10-01

### Added

- Filters are CSQL, the Campfire Simple Query Language, edited two ways. Filter chips cover the common case without typing; each value cycles between not set, is and isn't, and Created, Started, Completed and Due take day presets or a range. Raw mode takes any query, with and, or, not and parentheses, offers what can come next as you type, and points at a mistake before anything is sent; nothing typed there ever blocks, as Discard puts back the filter in use; `none` is the value of an empty field and days are read in your time zone. Agents pass the same language as `query`, with `tz`. A card that stops matching the filter while on screen goes where it went — into Done, feeding the fire — and stays there, faded, until the filter changes. Suggestions match any part of a value: rev finds In Review. The words a query uses — and, or, not, in, none, me and today — can no longer be a username, label, team or work state name.

### Changed

- Saved views are switches in the sidebar. A click turns one on or off, several can be on at once, and right-clicking a view edits its name, query and pins, or deletes it. In the HTTP API and the MCP tools a saved view takes and returns a `query` in place of `clauses`, and existing views are rewritten as the query they meant; the item list's `view` parameter is and-ed with any other filter instead of replacing it.

### Fixed

- A filter change costs one request and typing costs none; a newer request cancels an older one, whose late reply no longer overwrites the board. A link to a saved view, another team's included, switches that view on, sidebar links no longer underline on hover, and the views list follows changes made elsewhere.

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
