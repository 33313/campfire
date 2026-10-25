# Changelog

Every release of Campfire, newest first. Versions follow [Semantic Versioning](https://semver.org).

This file is generated from `.changes/`; edit the release's file there, then run
`npm run changie -- merge`.

## v0.18.0 - 2026-10-25

### Added

- A newer binary migrates an older data file when it opens it, after taking a snapshot it never prunes; an older binary refuses a newer file and names that snapshot to restore. README "Upgrading" explains.

## v0.17.0 - 2026-10-24

### Added

- Move the data to another directory in *Administration → Preferences*. Snapshots in the data directory move with the database, and the server restarts. `-data <dir>` sets the data directory at startup.

## v0.16.0 - 2026-10-24

### Changed

- Shorter, plainer help text and error messages across the app. Two-factor sign-in is called 2FA throughout.

## v0.15.1 - 2026-10-24

### Fixed

- A bucket shows its owning team's picture, and each team's beside its share of the contents, where the team has one

## v0.15.0 - 2026-10-24

### Changed

- Pull requests come from GitHub, GitLab, Codeberg, Forgejo or Gitea: *Administration → Pull requests* takes a forge and an access token, and keeps up by polling (GitHub and GitLab, every 5 minutes by default) or by a webhook. This replaces the GitHub Apps; connect GitHub again with a token.

## v0.14.1 - 2026-10-17

### Fixed

- In Firefox and Safari the caret is the browser's own again, blinking at the desktop's pace; the drawn one jumped ahead where a line wrapped after a hyphen

## v0.14.0 - 2026-10-17

### Added

- Administration has an Appearance section: hide groups of bucket icons and palette colours from the pickers, and let buckets take any hex colour that reads on a card, or none
- An instance administrator can add bucket icon sets of their own as SVGs under a prefix they choose; only stroked shapes are kept, drawn at the built-in icons' weight, and a deleted set's buckets keep their keys

### Changed

- Buckets choose from 42 icons in four groups (planning, signals, work, places) and 16 colours, and the team's mono or colour icon switch is gone: an icon is drawn quiet, or in its bucket's colour
- A bucket's icon and colour are one field, which opens the icons in their groups with the colours along the foot, in the create form and on the bucket alike

## v0.13.0 - 2026-10-09

### Changed

- Labels are added through a quiet + beside the chips, which opens the box already focused, instead of an empty box always sitting there
- The create form sets the assignee on the row with priority and planned end, and Mark as accepted sits in the footer beside Create

### Fixed

- The preview leaves out labels and relations when there are none instead of drawing a dash, a bucket's preview no longer offers a bucket, and an empty bucket reads as one to pick
- In Firefox the caret stays where a password field's dots put it, and stays one crisp pixel wide instead of turning bold and thin as it moves

## v0.12.0 - 2026-10-06

### Added

- The administration screen says when a newer release is out, from a daily check of GitHub that updates.check turns off; its new Updates section names the running version
- A pull request whose title starts or ends with an identifier, such as DEV-12: or [DEV-12], is listed on that item and in its preview, with its state. Administration has a GitHub section that creates the instance's own GitHub Apps, private by default and one per organisation or account, installs and links them, private repositories included, and syncs them back 90 days

## v0.11.0 - 2026-10-05

### Added

- The sign-in card and a new last section of the documentation link to Campfire's source, as the AGPL-3.0 asks; a host running a modified copy points them at its own. Docs in the sidebar is now Documentation
- Relations are set on the create form and listed in the preview, in all seven kinds, blocked by and the other inverses included, and the other item is found by searching its title or identifier; the tickets a split makes relate to each other

### Changed

- A selection of several items stays selected after a change, so the next change can follow
- The backlog's columns sit beside the title instead of across the screen, Assignee shows the username, and right-clicking a row opens the card menu
- The bucket field is a box to type in: the buckets show under it on the first click, most-picked first, and narrow as you type

### Fixed

- A card picked up while it kindles in Done is held in the hand, not dropped into the page as a blank card

## v0.10.0 - 2026-10-05

### Added

- The create dialog's Assignee label carries a (Me!) shortcut that assigns the new item to you.
- The card menu's Move to, Assignee, Priority and Bucket open sub-menus of their values, applied in one click without a dialog.
- A card held near the top or bottom of the screen scrolls the board, and near its side scrolls it sideways, so a card can be dropped where the screen did not reach.
- Descriptions and comments are rendered as Markdown, GitHub's dialect: headings, lists, task lists, tables, code, quotes and links. An identifier such as SOL-42 links to its item.
- Paste or drop a picture into a description or a comment: it is uploaded, kept in the database, and shown inline. images.maxMB in the configuration bounds its size (10 by default; 0 turns pictures off). An existing database needs the images table from schema.sql.
- A Docs page, from the sidebar, covers getting started, tickets and buckets, the board, text formatting, filtering in CSQL, saved views, shortcuts, accounts, connecting an agent over MCP, and the HTTP API. The query box's cheat sheet links to it.

### Fixed

- The text caret blinks every half second in every browser, Firefox included, instead of at the desktop's slower rate, and stays lit while you type.

## v0.9.0 - 2026-10-03

### Changed

- A release is tagged on main again, so a build from main reports the version it follows. `go install` is no longer an install path: use a release binary or the image.

### Fixed

- The board fits five columns on a 1366px laptop screen without scrolling sideways.

## v0.8.0 - 2026-10-03

### Added

- Campfire installs with `go install github.com/33313/campfire@latest`: a release tag carries the built client.

### Changed

- The setting that turns on point-in-time recovery is `snapshots.replica`, not `snapshots.shipTo`: it names the directory the write-ahead log is replicated to. A configuration file that still says `shipTo` is refused at startup; rename the key. In the API, `GET /api/snapshots` returns `replica` as one object, `{directory, from, to}`, present when it is set.

### Fixed

- A long title in the preview beside the board or backlog wraps instead of being cut off.

## v0.7.0 - 2026-10-03

### Added

- Point-in-time recovery. With `snapshots.shipTo` set, the write-ahead log is copied there continuously by Litestream, running inside the server, and the database can be restored to any moment in the last interval × keep: `campfire restore -at "2026-10-03 14:05"`, or `-at latest`. The administration screen shows the span covered and writes the command. `shipTo` no longer receives copies of the scheduled snapshots. Flags may now follow the command: `campfire -config x restore -at latest`.

### Changed

- A filter is only CSQL. The API and agent tools drop their structured filter parameters and the board and backlog drop `buckets`; CSQL gains a `kind` field (`kind = ticket`, `kind = bucket`), which the Show buckets toggle now adds to the query. The backlog pages with its own `cursor`. Work state and bucket names in a query match case-insensitively beyond ASCII.
- The database has one schema and no migrations: a new data file gets the schema, and an existing one is opened as it is. A data file from before v0.6.0 is not upgraded.

### Fixed

- An agent passing an argument a tool does not take is told so instead of having it ignored, and the agent interface reports the real version.
- The log, history and pick dialogs name a bucket made after they were first opened, instead of showing its number or leaving it out. The board, backlog and bucket index open in a browser that blocks storage, rather than failing.

### Security

- The session cookie is marked Secure behind an HTTPS proxy that sends X-Forwarded-Proto, as the README always said; it never was. Taking a snapshot is refused from another site, like every other write, and a restore only verifies snapshots in the snapshot directory. The image is built on Alpine 3.24; 3.20 is past its end of life.

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
