# Campfire — Design Handoff

**For:** whoever builds the client.
**Status:** matches the canvas as published on 2026-08-30.

## Where the design files are

This document and `SPEC.md` live in this repository. The drawings they describe do not: they are in

```
~/dev/.design/kanban/
```

which is a scratch directory, not a package. Everything below that names a `.dc.html` file means a
file in there.

## What to read, in what order

| File | What it settles | Authority |
|---|---|---|
| `SPEC.md` *(this repo)* | What the application does and does not do; data model; API and storage rules. | Behaviour. If this document and `SPEC.md` disagree, `SPEC.md` wins. |
| This file *(this repo)* | Tokens, components, and the rules the mockups are expressing. | Appearance. |
| `*.dc.html` *(design dir)* | Pixel reference, one file per screen. | The tiebreaker for anything this document does not say. |
| `THEME.md` *(design dir)* | The desktop theme the palette was taken from. | Background only. Not a dependency. |

`kanban-ui.html` in the design directory is the published canvas — the same artboards wrapped in a
viewer. Read the `*.dc.html` files directly; the bundle exists to be looked at, not built from.

### How to read an artboard

Each `*.dc.html` is one screen at a fixed 1440×N, written as flat HTML with **inline styles and no
classes**. That is a drawing convention, not a proposal: there is no component system in there to
port. Take the numbers, not the markup. The `<x-dc>` and `<helmet>` wrappers and the trailing
`<script data-dc-script>` block belong to the canvas editor — ignore them.

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
| `accent` | `#d9a05b` | amber. Active nav, P1, primary buttons, selection, focus, identifiers in context. |
| `accent-hover` | `#e6b479` | link hover only. |
| `accent-tint` | `rgba(217,160,91,0.12)` | selection, active nav, drop targets, the on-state of a toggle. |
| `accent-tint-weak` | `rgba(217,160,91,0.06)` | a selected table row, a drag placeholder. |
| `alert` | `#c4675c` | clay. **Errors and rejection only.** Never decoration. |
| `alert-tint` | `rgba(196,103,92,0.10)` | the count block in a destructive confirm. |
| `ok` | `#a3a961` | olive. Terminal states, progress fill, verified snapshots, the live dot. |
| `steel` | `#7d94a8` | P2 only. |

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

**Every corner is square. `border-radius: 0` everywhere** — one exception, below.

| Metric | Value |
|---|---|
| Sidebar | 224px fixed |
| Screen header | 56px |
| Filter / scope bar | 46px |
| Right-hand detail aside | 320px fixed |
| Board column | 280px fixed, 12px gutter |
| Card | 10px 11px padding, 9px internal gap, 8px between cards |
| Table row | 42px; header row 34px; 12px column gap; 14px side padding |
| Button | 30px tall, 0 12px padding (primary 0 14px) |
| Chip / small control | 26px tall, 0 8–10px |
| Input | 30–34px tall, 0 11px |
| Avatar | 20px in lists and cards, 24px in the sidebar, 26px on a comment |
| Icon | 15px nav, 13px inline with text, 11px on a card, 9–10px as a marker |

Icon strokes: 1.4 for outline glyphs, 1.5–1.8 for markers, 2 for priority and check marks.
`stroke-linecap="square"`, `stroke-linejoin="miter"` — round caps are off-language.

---

## 2. Five rules that are easy to get wrong

1. **Status pips are circular.** Every dot that reports state — work-state dots, the live dot, stream
   banners, verified snapshots — is `border-radius: 50%`, and stays circular even though every other
   corner in the product is square. This is deliberate: a pip reads as a status light, not a tile.
2. **Radio buttons and checkboxes stay square.** They are controls, not status. Do not "fix" them to
   match the pips.
3. **A ticket has no type.** There is no `type` field, no per-team type list, no type filter, no type
   column, and no type on a card. See `SPEC.md` §3.2. If you find yourself adding one back, the
   answer is in Appendix B.
4. **Priority is four glyphs *and* four colours.** The shape carries the level on its own; colour
   only reinforces it. Do not collapse it to a coloured dot.
5. **Labels never appear on a board card.** They live on the item detail view. `SPEC.md` §8.1.

---

## 3. Priority

Four levels. Rendered at 14px on cards and rows, 16px in a legend, `stroke-width="2"`,
`viewBox="0 0 16 16"`.

| Level | Colour | Glyph | Path |
|---|---|---|---|
| P1 Highest | `#d9a05b` accent | double chevron up | `M4 8.5 8 4.5l4 4` + `M4 12 8 8l4 4` |
| P2 High | `#7d94a8` steel | chevron up | `M3.8 10.2 8 6l4.2 4.2` |
| P3 Medium *(default)* | `#8f8578` muted | bar | `M3.8 8h8.4` |
| P4 Low | `#5f574d` faint | chevron down | `M3.8 5.8 8 10l4.2-4.2` |

---

## 4. Bucket icons

A bucket carries one icon; a ticket carries none (`SPEC.md` §6.2). The set is **fixed at eight** —
not extensible, not uploadable. It stands in for the bucket wherever the bucket is named: on a card,
in the bucket index, in the item detail aside, in the log.

Rendered at 9–11px inline, 16px in the picker, `stroke-width` 1.3–1.8, `viewBox="0 0 16 16"`.

| Key | Path |
|---|---|
| `check` | `M2.9 2.9h10.2v10.2H2.9Z` + `M5.7 8.2 7.4 9.9l3-3.4` |
| `bug` | `<rect x="4.9" y="5" width="6.2" height="8" rx="3.1"/>` + `M4.9 8H2.5M13.5 8h-2.4M5.7 5.3 4.5 3.5M10.3 5.3l1.2-1.8` |
| `bookmark` | `M4.1 2.7h7.8v10.9L8 10.9l-3.9 2.7Z` |
| `page` | `M4.2 2.5h4.9L12 5.4v8.1H4.2Z` + `M8.9 2.5v3h3.1` |
| `pen` | `M2.9 13.1 4.5 8.5l5.8-5.8 2.9 2.9-5.8 5.8Z` + `M9.3 3.7l2.9 2.9` |
| `wrench` | `M10.5 2.7a3.3 3.3 0 0 0-3.9 4.3l-4 4a1.35 1.35 0 0 0 1.9 1.9l4-4a3.3 3.3 0 0 0 4.3-3.9l-1.9 1.9-1.6-.4-.4-1.6Z` |
| `lens` | `<circle cx="7" cy="7" r="4.2"/>` + `M10.1 10.1 13.6 13.6` |
| `flag` | `M4.1 2.5v11.1` + `M4.1 3.3h7.5L10 5.9l1.6 2.6H4.1` |

A monochrome set (`muted-soft`) is the default. A colour set exists — `check` `#7d94a8`, `bug`
`#c4675c`, `bookmark` `#b087a3`, `page` `#8aa8a0`, `pen` `#d9a05b`, `wrench` `#8f8578`, `lens`
`#a3a961`, `flag` `#7d94a8`. A team picks which rendering its surfaces use (`SPEC.md` §6.2), so both
ship. Monochrome is the default and the one every artboard here draws.

The **pail** glyph (`M3.2 5.2h9.6l-1 8a1.1 1.1 0 0 1-1.1 1H5.3a1.1 1.1 0 0 1-1.1-1Z` +
`M5.5 5.2a2.5 2.5 0 0 1 5 0`) is *not* a bucket icon. It is the sidebar's Buckets section icon and
nothing else.

---

## 5. Components

Each entry lists what exists. States not listed do not exist yet — ask before inventing one.

| Component | States and notes |
|---|---|
| **Button** | primary (accent fill, `on-accent` text, 600) · secondary (1px `border`, `muted`) · destructive (clay fill) · destructive disabled (1px clay, `dim` text) · quiet (no border, `muted`) |
| **Input** | rest (`sunk` + `border`, `faint` placeholder) · focused (1px `accent`, `bright` text, 1px accent caret) · error (1px `alert`, alert text) |
| **Checkbox** | 13–15px square. Checked: accent fill, `on-accent` tick. Unchecked: 1px `border-strong`. Indeterminate: accent fill, `on-accent` bar — used by select-all when the selection is partial. |
| **Radio** | 14px square, 1px accent, 6px square accent dot. Square by design. |
| **Toggle** | 30×17px. On: `accent-tint` field, 1px accent, 11px accent knob, right. Off: `sunk`, 1px `border`, `ghost` knob, left. |
| **Label chip** | 1px `border`, `muted-soft`, 3px 8px. Not clickable, not coloured, never on a card. |
| **Filter chip** | 26px, 1px `border`, `muted`, chevron. Active chip carries the accent treatment and an × to clear. |
| **Avatar** | square. `border` fill, `muted-soft` mono initials. Unassigned is a 1px dashed `border-strong` square of the same size — never a person glyph. |
| **State pip** | 5px circle. `ghost` for a plain state, `ok` for the final state, `alert` for rejected. |
| **Card** | `surface`, 1px `border`. Selected: 1px `accent`, `bright` title. Contents, in order: bucket line (icon + name, mono caps, omitted when there is no bucket) → title → footer (priority glyph, identifier, avatar pushed right). Five fields, no sixth. |
| **Board column** | header (state pip, name in mono caps, count in a bordered box, right) over a `sunk` body. Drop target is a 74px dashed `border` box on `accent-tint-weak`. |
| **Table row** | 42px, `rule-faint` underline. Focused row: `accent-tint` + 2px accent left border. Selected row: `accent-tint-weak`. Deprioritised row: `opacity: 0.32`. |
| **Selection bar** | appears above a table or board when a selection is non-empty. Count in accent mono caps, then the actions of §6, then Clear on the right. |
| **Dialog** | `surface`, 1px `border-strong` (1px `alert` when destructive). Title row, body at 16px padding, footer on `bar` with the consequence stated on the left in mono caps and the actions on the right. |
| **Stream banner** | too-far-behind: 1px `alert`, alert pip, Reload button. Reconnecting: 1px `border`, `muted` pip, no action. |
| **State role badge** | mono caps in a 1px box — `muted` for default/accepted, `ok` for final, `alert` for rejected. Intermediate has no box, just `ghost` text. |

---

## 6. Screens

| Artboard | Screen | Notes for the build |
|---|---|---|
| `SignIn.dc.html` | Sign in | Username + password, optional TOTP step (`Dialogs.dc.html`). |
| `Main.dc.html` | Board | Four columns shown of six states. Show-buckets toggle in the filter bar. |
| `Backlog.dc.html` | Backlog | Table + detail aside. Carries the selection bar and the checkbox column. |
| `ItemDetail.dc.html` | Item detail | Header actions: Copy link · **Clone** · **Split** · Reject · Edit. Relations, comments/history tabs, field aside. |
| `BucketIndex.dc.html` | Bucket index | Cards with progress bar, per-bucket icon, scope switch (participation / owned / instance). |
| `BucketDetail.dc.html` | Bucket detail | Contents grouped by owning team, per-row remove ×. `Icon` is a field here. |
| `Log.dc.html` | Log | Keyset paging: the footer says "load older from 41192", never a page number. Agent actions carry a small accent glyph beside the actor. |
| `TeamSettings.dc.html` | Team settings | Work states, Members, Identity, Archive. **No ticket types tab.** |
| `Admin.dc.html` | Administration | Users, backup/restore, agent access. |
| `Components.dc.html` | Foundations | The token and component reference this document describes. |
| `Dialogs.dc.html` | Dialogs and states | Create item · Reject · New bucket · Split · Bulk move · Bulk edit · delete confirms · second factor · stream banners. |

`canvas.json` in the design directory holds the layout and the annotations; the annotations are
design intent in prose and are worth reading once.

---

## 7. Bulk, clone and split — the interactions the mockups added

Behaviour is in `SPEC.md` §6.5. What the design fixes:

- **Selection** lives on the board and the backlog. A checkbox column leads each row; the header
  checkbox is select-all and goes indeterminate on a partial selection.
- **Selection bar** offers, in this order: `Move to…` (primary), `Reject…`, then `Assignee`,
  `Priority`, `Bucket`, then `Clear`. All five are v1.
- **Bulk edit is one field at a time.** The dialog has a field switcher across the top and a value
  list below. There is no multi-field form.
- **Clone** and **Split** are buttons on the item header, not menu items buried in an overflow.
- **Split** takes a title per new ticket and offers one checkbox: reject the original afterwards.

---

## 8. What the design does not cover

Ask rather than invent:

- Light mode. There is none, and the palette is not built for one.
- Responsive and mobile layouts. Every artboard is 1440 wide with fixed rails.
- Motion. No transition, easing or duration is specified anywhere.
- Empty states beyond the board's empty column and the log's end-of-list.
- Error and validation copy beyond the one input error state.
- Keyboard shortcuts and focus order.
- The control that picks the icon rendering. The setting is specified and both sets are drawn (§4),
  but no Team settings artboard shows the switch.
