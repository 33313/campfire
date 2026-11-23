# Campfire — Design rules

The rules the interface follows: tokens, type, geometry, motion, the mark, and the few decisions
that are easy to get wrong. `SPEC.md` fixes behaviour and wins where the two disagree.

This is not a description of the client. How a screen or a component looks today is in the code
(`internal/web/src`, with the tokens in `style.css`). Change this file only when a change breaks a
rule here or adds one; a new screen or component that follows the rules needs no entry.

The original drawings are in `~/dev/.design/kanban/` (`*.dc.html`, one screen each). They came
before the client and are not kept in step: background only.

---

## 1. Tokens

Nothing in the interface uses a colour that is not on this list.

### Surfaces and lines

| Token | Hex | Used for |
|---|---|---|
| `page` | `#131110` | the deepest layer, behind everything |
| `sunk` | `#100e0d` | inputs, board columns, table bodies — anything recessed |
| `surface` | `#1a1715` | sidebar, headers, panels, cards, dialogs |
| `bar` | `#16130f` | filter bar, table header row, dialog footers |
| `border` | `#2c2621` | the default 1px line |
| `border-strong` | `#3a332c` | focused inputs, dialog edges, empty-avatar dashes |
| `rule-faint` | `#211d19` | between rows in a list — one step quieter than `border` |

### Text

| Token | Hex | Used for |
|---|---|---|
| `text` | `#d6cfc4` | body, card titles, table cells |
| `bright` | `#ece5d8` | headings, the selected row, emphasis |
| `muted` | `#8f8578` | secondary text, labels, inactive nav |
| `muted-soft` | `#a89c8d` | avatar initials, mono values inside panels |
| `dim` | `#6b6157` | explanatory text under a heading |
| `faint` | `#5f574d` | column labels, timestamps, placeholders, P4 |
| `ghost` | `#4a423a` | em-dashes for empty values, disabled marks |

### Accent and signal

| Token | Hex | Rule |
|---|---|---|
| `accent` | `#f0a84a` | amber. Active nav, P2, primary buttons, selection, focus, identifiers in context, a deadline three days away or less. |
| `accent-hover` | `#f7bf6e` | link hover only. |
| `accent-tint` | `rgba(240,168,74,0.18)` | selection, active nav, drop targets, the on-state of a toggle. |
| `accent-tint-weak` | `rgba(240,168,74,0.08)` | a selected table row, a drag placeholder. |
| `alert` | `#e5604d` | clay. **Errors, rejection, overdue deadlines and P1 only** — one meaning: act now. Never decoration. |
| `alert-tint` | `rgba(229,96,77,0.14)` | the count block in a destructive confirm. |
| `ok` | `#b2bd52` | olive. Terminal states, progress fill, verified snapshots, the live dot. |
| `pr-merged` | `#a58cc4` | a merged pull request only. |

`on-accent` is `#131110` — text on an amber or clay fill.

### Type

Three families, each with one job.

| Family | Stack | Used for |
|---|---|---|
| Sans | `"IBM Plex Sans", ui-sans-serif, system-ui, sans-serif` | all interface text |
| Mono | `"IBM Plex Mono", ui-monospace, Menlo, monospace` | identifiers, times, counts, column and field labels |
| Serif | `"Source Serif 4", Georgia, serif` | descriptions and comments — prose, and only prose |

| Role | Size / weight / tracking |
|---|---|
| Item and page title | Sans 23 / 600 / −0.014em |
| Section heading | Sans 17 / 600 / −0.008em |
| Screen title | Sans 15 / 600 |
| Body, card title, table cell | Sans 13 / 400, line-height 1.42 in a card |
| Control text | Sans 12.5 / 400 |
| Explanatory text | Sans 11.5 / 400, colour `dim` |
| Prose | Serif 16 / 1.62, max 66ch |
| Comment prose | Serif 15.5 / 1.58, max 62ch |
| Identifier, time, count | Mono 11 / 0.03em |
| Column and field label | Mono 9.5 / 0.13–0.14em / uppercase |
| Section eyebrow | Mono 10 / 0.16em / uppercase |

### Geometry

**Every corner is square. `border-radius: 0` everywhere** — one exception, §2.

| Metric | Value |
|---|---|
| Sidebar | 224px fixed |
| Screen header | 56px |
| Filter / scope bar | 46px |
| Preview aside | 320px fixed |
| Reading column (item detail) | 860px maximum, centred |
| Board column | 280px drawn, 12px gutter; shares spare width up to 420px, gives way to 200px, then scrolls sideways |
| Card | 10px 11px padding, 9px internal gap, 8px between cards |
| Table row | 42px; header row 34px; 12px column gap; 14px side padding |
| Button | 30px tall; 28px in a screen header or beside a field |
| Chip / small control | 26px tall |
| Input | 30–34px tall, 0 11px |
| Avatar | 20px in lists and cards, 24px in the sidebar, 26px on a comment |
| Settings rail | 190px fixed |
| Icon | 15px nav, 13px inline with text, 11px on a card, 9–10px as a marker |

Icon strokes: 1.4 for outline glyphs, 1.5–1.8 for markers, 2 for priority and check marks.
`stroke-linecap="square"`, `stroke-linejoin="miter"` — round caps are off-language.

The numbers are for a ~1440px viewport and are not re-specified per size; OS scaling and browser
zoom handle larger displays. The interface never zooms itself (no `zoom` or scale on `#app`): a
client rect and a CSS length inside the app are the same pixels.

---

## 2. Five rules that are easy to get wrong

1. **Status pips are circular.** Every dot that reports state — work-state dots, the live dot, stream
   banners, verified snapshots — is `border-radius: 50%`. A pip reads as a status light, not a tile.
2. **Radio buttons and checkboxes stay square.** They are controls, not status.
3. **A ticket has no type.** No `type` field, list, filter, column or card mark. See `SPEC.md` §3.2
   and Appendix B.
4. **Priority is four glyphs, two signals, one alarm.** The shape carries the level; colour marks P1
   and P2 only (§3).
5. **Labels never appear on a board card.** They live on the item detail view. `SPEC.md` §8.1.

---

## 3. Priority

Four levels, `viewBox="0 0 16 16"`, `stroke-width="2"`, 14px on cards and rows.

| Level | Colour | Glyph | Path |
|---|---|---|---|
| P1 Highest | `alert` | double chevron up | `M4 8.5 8 4.5l4 4` + `M4 12 8 8l4 4` |
| P2 High | `accent` | chevron up | `M3.8 10.2 8 6l4.2 4.2` |
| P3 Medium *(default)* | `muted` | bar | `M3.8 8h8.4` |
| P4 Low | `faint` | chevron down | `M3.8 5.8 8 10l4.2-4.2` |

Every glyph is a stroke, with no fill or background. The default level has no colour: it is on most
cards. Blue is out of priority; steel is a bucket colour only.

---

## 4. Bucket icons and colours

A bucket carries one icon and optionally one colour; a ticket carries neither (`SPEC.md` §6.2). They
are recognition only: nothing reads them.

The built-in icons are 42 glyphs in four groups, offered in this order. Each key is stored and named
by the API, so a key may be added but never renamed or removed. Path data is in
`internal/web/src/lib/icons.js` (`BUCKET_ICON_GROUPS`).

| Group | Keys |
|---|---|
| Planning | `flag` `bookmark` `milestone` `target` `rocket` `hourglass` `chart` `inbox` `archive` `trophy` |
| Signals | `lens` `bolt` `star` `sparkle` `bell` `heart` `chat` `bulb` `coin` `key` `lock` |
| Work | `check` `bug` `wrench` `pen` `page` `code` `terminal` `database` `beaker` `box` `layers` |
| Places | `flame` `tent` `mountain` `leaf` `sun` `moon` `globe` `map` `anchor` `home` |

- Strokes only, square caps, mitred joins, no fills, `viewBox="0 0 16 16"`. An instance's own sets
  keep their viewBox with the stroke scaled to it (1.5 at 16 units, 2.25 at 24).
- A glyph that already means a place in the navigation — the pail, calendar, clock, shield, people,
  the search magnifier, the edit pen — is not a bucket icon. The **pail** is the sidebar's Buckets
  icon and nothing else.
- A colour tints the bucket chip's border and text and the bucket's icon, never a fill: a filled
  chip on most cards outshouts the priority glyph beside it.

The sixteen colours are one step of lightness and saturation for the warm page, in hue order. Red is
left out (late, P1) and so is the accent orange (selected). A custom `#rrggbb` must stand at least
3:1 off `surface`, the floor every palette colour clears.

| Key | Hex | Key | Hex |
|---|---|---|---|
| `amber` | `#d9a05b` | `dusk` | `#8a8fc9` |
| `straw` | `#c8b45e` | `plum` | `#9d82c2` |
| `olive` | `#a3a961` | `mauve` | `#b087a3` |
| `moss` | `#7fa86a` | `rose` | `#c48098` |
| `sage` | `#8aa8a0` | `rust` | `#a27459` |
| `teal` | `#5fa7a3` | `sand` | `#b7a587` |
| `sky` | `#6fa3c7` | `stone` | `#8f8578` |
| `steel` | `#7d94a8` | `slate` | `#737d87` |

---

## 5. Components

- **The accent border belongs to the selection.** A focused card or row that is not selected gets
  `border-strong` and an inset accent edge, never the accent border.
- **A card has six fields and no seventh:** title, bucket chip, deadline, priority, identifier,
  assignee. The deadline is `accent` within three days, `alert` once past, `dim` once the ticket is
  in the final state.
- **Avatars are square.** A picture is cropped square; without one, initials on `border`.
  Unassigned is a dashed `border-strong` square, never a person glyph.
- **One button per action, in the place it acts.** Header actions are buttons, not an overflow menu.
  Buttons in a screen header or beside a field are 28px; a dialog footer keeps 30px.
- **Destructive is clay, and says what it costs.** A destructive dialog has a 1px `alert` edge and
  states the consequence in its footer.
- **One shared panel.** Drop-downs, the card menu, sub-menus and suggestion lists are the same
  `surface` panel on 1px `border-strong`.
- Any HTML in a description or comment is shown as text; a picture from off the instance is a link,
  never fetched.

---

## 6. Screens

- **Reading screens keep a measure.** The item detail and the docs are read, not scanned: a capped
  column, prose at its measure.
- **One sidebar, one screen header.** A screen adds a filter or scope bar under its header, and at
  most one aside (the preview, or a settings rail). Two asides on one screen read as neither.
- **The preview is the item.** A plain click on a card or row previews it in the aside, editable
  there; its identifier opens the full view.

---

## 7. Bulk, clone and split

Behaviour is in `SPEC.md` §6.5.

- Selection lives on the board and the backlog. ⌘/Ctrl-click picks a card; a plain click previews.
- The selection bar and the card menu offer the same actions: `Move to…`, `Reject…`, `Assignee`,
  `Priority`, `Bucket`.
- **Bulk edit is one field at a time.** No multi-field form.
- **Clone** and **Split** are buttons on the item header.

---

## 8. Motion

Things arrive; they do not appear. Motion says where something came from and where it went, and is
over before anyone could wait for it. Nothing loops but the caret, and the fire (§9).

| Token | Value | Used for |
|---|---|---|
| `snap` | 90ms | colour, border and background answering the pointer |
| `quick` | 140ms | a drop-down, a dialog, a toast leaving, the selection bar, a toggle |
| `glide` | 220ms | a card travelling, the preview sliding in, a screen fading up, a toast arriving |
| `ease-out` | `cubic-bezier(0.33, 1, 0.68, 1)` | the one curve: fast out, soft landing |

- Exits are instant or `quick`, never `glide`: nobody waits for a dismissed thing.
- A card that changes column travels there, whoever moved it.
- The product's only shadow is under a card held in a drag (`0 10px 24px` at 50% black).
- A card reaching the final state feeds the fire: embers rise off it to the mark in the sidebar. It
  is the one flourish, and only for open work finished on the board.
- Focus is a 1px `accent` outline (`:focus-visible`); a focused input adds a 3px `accent-tint-weak`
  halo.
- **Reduced motion** turns every animation and transition off, scripted ones included. The e2e suite
  runs that way.

---

## 9. The mark

A flame over two crossed logs, in the 16px grid, square and mitred like every glyph:

| Part | Path | Lit | Out |
|---|---|---|---|
| flame | `M8 1.2 12.6 9.2 10.8 12H5.2L3.4 9.2Z` | `accent` | `ghost` |
| core | `M8 5.6 10.4 9.6 9.4 12H6.6L5.6 9.6Z` | `accent-hover` | `dim` |
| logs | `M2.8 12.6 13.2 14.8M2.8 14.8 13.2 12.6`, stroke 1.4 | `faint` | `faint` |

The brand says so once, quietly, and gets out of the way. It appears in the sidebar (lit while the
live stream is up, burnt down while it is not), while loading, on sign in (the one screen allowed a
glow), and as the favicon. Nowhere else.

---

## 10. Out of scope

- Light mode. The palette is not built for one.
- Mobile and narrow layouts. Narrower than the rails allow, the interface scrolls rather than
  reflows.
- A new keyboard binding is listed on the Docs page under *Keyboard and mouse*.
