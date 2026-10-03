# Campfire — Design Handoff

**For:** whoever builds the client.
**Status:** the source of truth. Reconciled with the built client on 2026-09-01. Where an artboard
or the published canvas shows something else, this file wins — the drawings came first and are not
kept in step.

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
| This file *(this repo)* | Tokens, components, and the rules the interface is expressing. | Appearance. Nothing overrules it. |
| `*.dc.html` *(design dir)* | The original drawings, one file per screen. | Reference for anything this document does not say. Superseded wherever it disagrees. |
| `THEME.md` *(design dir)* | The desktop theme the palette was taken from. | Background only. Not a dependency. |

`kanban-ui.html` in the design directory is the published canvas — the same artboards wrapped in a
viewer, and the artifact published from it is a snapshot of that bundle. Both carry the standing of
the artboards, no more: they are a picture of the intent before the client existed. Read the
`*.dc.html` files directly; the bundle exists to be looked at, not built from.

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
| `accent` | `#f0a84a` | amber. Active nav, P2, primary buttons, selection, focus, identifiers in context, a deadline three days away or less. |
| `accent-hover` | `#f7bf6e` | link hover only. |
| `accent-tint` | `rgba(240,168,74,0.18)` | selection, active nav, drop targets, the on-state of a toggle. |
| `accent-tint-weak` | `rgba(240,168,74,0.08)` | a selected table row, a drag placeholder. |
| `alert` | `#e5604d` | clay. **Errors, rejection, overdue deadlines and P1 only** — one meaning: act now. Never decoration. |
| `alert-tint` | `rgba(229,96,77,0.14)` | the count block in a destructive confirm. |
| `ok` | `#b2bd52` | olive. Terminal states, progress fill, verified snapshots, the live dot. |

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
| Right-hand detail aside (the preview) | 320px fixed |
| Item detail column | 860px maximum, centred in what is left; 20px 24px padding |
| Board column | 280px minimum, 12px gutter. Columns share spare width above that, capped at 420px, and scroll sideways below it |
| Card | 10px 11px padding, 9px internal gap, 8px between cards |
| Table row | 42px; header row 34px; 12px column gap; 14px side padding |
| Button | 30px tall, 0 12px padding (primary 0 14px). 28px in a screen header or beside a field, where it sits with 28px inputs; only a dialog footer keeps 30px |
| Chip / small control | 26px tall, 0 8–10px |
| Input | 30–34px tall, 0 11px |
| Avatar | 20px in lists and cards, 24px in the sidebar, 26px on a comment |
| Settings rail | 190px fixed |
| Icon | 15px nav, 13px inline with text, 11px on a card, 9–10px as a marker |

Icon strokes: 1.4 for outline glyphs, 1.5–1.8 for markers, 2 for priority and check marks.
`stroke-linecap="square"`, `stroke-linejoin="miter"` — round caps are off-language.

These are the numbers for a ~1440px viewport, and they are not re-specified per size. A display that
wants them larger is the desktop's job, or the browser's: OS scaling and browser zoom both report a
smaller viewport in CSS pixels, and the geometry above then lands at the size it was drawn at.

The interface never zooms itself: no `zoom` or scale on `#app`. A zoomed app is a second coordinate
space, and every drag, menu and animation that measures the screen would have to divide by the zoom
by hand; each one that forgot would be a bug. A client rect and a CSS length inside the app are the
same pixels.

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
4. **Priority is four glyphs, two signals, one alarm.** The shape carries the level on its own;
   colour marks P1 and P2 only. Do not collapse it to a coloured dot, and do not give the default
   level a colour: it is on most cards.
5. **Labels never appear on a board card.** They live on the item detail view. `SPEC.md` §8.1.

---

## 3. Priority

Four levels. Rendered at 14px on cards and rows, 16px in a legend, `stroke-width="2"`,
`viewBox="0 0 16 16"`.

| Level | Colour | Glyph | Path |
|---|---|---|---|
| P1 Highest | `alert` | double chevron up | `M4 8.5 8 4.5l4 4` + `M4 12 8 8l4 4` |
| P2 High | `accent` | chevron up | `M3.8 10.2 8 6l4.2 4.2` |
| P3 Medium *(default)* | `muted` | bar | `M3.8 8h8.4` |
| P4 Low | `faint` | chevron down | `M3.8 5.8 8 10l4.2-4.2` |

P1 wears the one red the interface already has, because red already means act now. Every glyph is a
stroke: none has a fill or a background, so the four read as one family. Blue is out of priority;
steel is a bucket colour only.

This mapping is provisional. Cool hues pop harder in a warm room, and a violet P1 was measured to
out-pull this red, but no cool pairing has been settled on yet. The measurements and the candidates
are in `priority-colour-handoff.md` and `priority-lineup.html` in the design directory.

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
`#a27459`, `bookmark` `#b087a3`, `page` `#8aa8a0`, `pen` `#d9a05b`, `wrench` `#8f8578`, `lens`
`#a3a961`, `flag` `#7d94a8`. A team picks which rendering its surfaces use (`SPEC.md` §6.2), so both
ship. Monochrome is the default and the one every artboard here draws.

### A bucket's colour

A bucket may also carry one colour, on the same terms as the icon: recognition only, nothing reads
it. The set is fixed at seven and there is no default member — a bucket without a colour is drawn
exactly as it always was.

| Key | Hex |
|---|---|
| `amber` | `#d9a05b` |
| `rust` | `#a27459` — a brown, whatever its key says: red on a card means urgent or late |
| `olive` | `#a3a961` |
| `steel` | `#7d94a8` |
| `mauve` | `#b087a3` |
| `sage` | `#8aa8a0` |
| `stone` | `#8f8578` |

Where a bucket has one, its colour beats both the team's rendering and the icon's own default: it
was chosen for this bucket. It tints the bucket chip on a card — border and text in the colour, never
a fill: a filled chip on most cards outshouts the priority glyph beside it — and the bucket's icon wherever that
is drawn.

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
| **Toggle** | 30×17px. On: `accent-tint` field, 1px accent, 11px accent knob, right. Off: `sunk`, 1px `border`, `ghost` knob, left. The knob travels between the two (§8). |
| **Label chip** | 1px `border`, `muted-soft`, 3px 8px. Not clickable, not coloured, never on a card. |
| **Label editor** | one component wherever labels are added — the item detail and the create dialog. Chips and a 104px × 24px `sunk` box on one wrapping line, the box always present and carrying no placeholder; 1px `accent` when focused. Existing labels that start with what has been typed drop under it in the drop-down panel (`surface`, 1px `border-strong`), the typed prefix in `accent`; ↑↓ move, Enter takes the highlighted one or what was typed, Backspace on an empty box takes back the last chip. A label already on the item is never offered. |
| **Filter chip** | 26px, 1px `border`, `muted`, chevron. Active chip carries the accent treatment and an × to clear. |
| **Avatar** | square. A person's picture where they have one, cropped square from the centre and `object-fit: cover`; otherwise `border` fill with `muted-soft` mono initials. Unassigned is a 1px dashed `border-strong` square of the same size — never a person glyph. |
| **State pip** | 5px circle. `ghost` for a plain state, `ok` for the final state, `alert` for rejected. |
| **Card** | `surface`, 1px `border`. Selected: 1px `accent`, `bright` title. Focused (the card in the preview): 1px `border-strong` and a 2px inset accent left edge — the accent border belongs to the selection alone, so a card is never drawn as selected when it is not. Dragging: `opacity: 0.4` where it was, while a copy of it is held under the pointer (§8). Contents, in rows: title → bucket chip → deadline (calendar glyph and the planned end, mono 11.5 `text`, only when set; `accent` when three days away or less, `alert` once past; `dim` once the ticket is in the final state, whatever the date) → footer (priority glyph, identifier, avatar pushed right). Six fields, no seventh. |
| **Bucket chip** *(on a card)* | icon + name, mono 9.5 caps, 1px `border`, `dim`, on its own row between the title and the footer, sized to its name. Wears the bucket's colour where it has one (§4). Omitted when there is no bucket. A long name wraps within the chip, and the border follows its longest line rather than the card's width. |
| **Bare field** | a property typed into where it is drawn. No fill and no border until the pointer is on it (then 1px `border`), 1px `accent` on `sunk` when focused, `field-sizing: content`. It keeps the type of the thing it stands for — a title stays a title, a description stays serif prose at the 66ch measure. Save (floppy) and cancel are two 24px square buttons under the field, appearing only once it has actually changed. |
| **Aside control** | the preview's version of the same idea for a property that is picked rather than typed: a 34px row, transparent until hover, then `sunk` + 1px `border`. May be an anchor — the bucket row follows through to the bucket — without taking the link colour. |
| **Swatch** | 20px square, 1px `border-strong`, filled with the colour itself. Selected: a `surface` inset ring inside a `bright` outline. The empty one is "no colour". |
| **Search box** | 250px, 28px tall, magnifier inside the border on the left. Searches on Enter. What matches drops under it as you type — the same panel as the card menu, a row of identifier + title each, at most six — from two characters and one request per pause in the typing. ↑↓ move, Enter on a highlighted row opens that item, Escape and a click away close it; Enter on none of them runs the search as before. |
| **State badge** | the work state named in a bordered mono-caps box, `muted`. Terminal states carry their pip and take `ok` / `alert` text. |
| **Live indicator** | the live pip (6px, `ok`) and the stream status as an eyebrow. |
| **Toast** | bottom right, `surface`, 1px `border`, max 420px; 1px `alert` when it reports a failure. Stacks upward. |
| **Board column** | header (state pip, name in mono caps, count in a bordered box, right) over a `sunk` body. While a card is held, a 2px `accent` line marks where it will land, in the gap between the two cards around the pointer; nothing moves until it is dropped. Escape puts it back. |
| **Table row** | 42px, `rule-faint` underline. Focused row: `accent-tint` + 2px accent left border. Selected row: `accent-tint-weak`. Deprioritised row: `opacity: 0.32`. |
| **Selection bar** | takes the filter bar's place while a selection is non-empty, fading in and out over it (120ms, none under reduced motion) so nothing below moves; a 1px `accent` underline marks the mode. Count in accent mono caps, then the actions of §6, then Clear on the right. |
| **Card menu** | the right-click menu on a card. 186px, fixed at the pointer and kept inside the viewport, `surface` on 1px `border-strong` — the team drop-down's panel. An eyebrow head names what it will act on (the identifier, or "n selected"), then the selection bar's five actions, then Clone, Split and Open for a single ticket, separated by a 1px `border` rule. Closes on Escape, on a click away, and before any dialog it opens. |
| **Dialog** | `surface`, 1px `border-strong` (1px `alert` when destructive). Title row, body at 16px padding, footer on `bar` with the consequence stated on the left in mono caps and the actions on the right. |
| **Stream banner** | too-far-behind: 1px `alert`, alert pip, Reload button. Reconnecting: 1px `border`, `muted` pip, no action. |
| **State role badge** | mono caps in a 1px box — `muted` for default/accepted, `ok` for final, `alert` for rejected. Intermediate has no box, just `ghost` text. |

---

## 6. Screens

| Artboard | Screen | Notes for the build |
|---|---|---|
| `SignIn.dc.html` | Sign in | Username + password, optional TOTP step (`Dialogs.dc.html`). |
| `Main.dc.html` | Board | One column per work state, less the default and rejected ones; the header says "4 of 6 states". Search box in the header, show-buckets toggle and filter chips in the filter bar. A plain click on a card previews it in the aside; the identifier in the preview opens it. |
| `Backlog.dc.html` | Backlog | Table + preview aside, same preview as the board. Carries the selection bar and the checkbox column. |
| `ItemDetail.dc.html` | Item detail | Header actions: Copy link · **Clone** · **Split** · Reject · Edit (Save · Cancel while open). Relations, comments/history tabs. The fields are a grid in the one column: the artboard's field aside is drawn beside the board's sidebar, and two asides on one screen read as neither. |
| `BucketIndex.dc.html` | Bucket index | Cards with progress bar, per-bucket icon, scope switch (participation / owned / instance). |
| `BucketDetail.dc.html` | Bucket detail | Contents grouped by owning team, per-row remove ×. `Icon` and `Colour` are fields here. The screen is the item detail screen — a bucket and a ticket are one namespace. |
| `Log.dc.html` | Log | Keyset paging: the footer says "load older from 41192", never a page number. Agent actions carry a small accent glyph beside the actor. |
| `TeamSettings.dc.html` | Team settings | Work states, Members, Identity, Archive. Identity carries the icon-rendering switch (mono / colour). **No ticket types tab.** |
| `Admin.dc.html` | Administration | Users, backup/restore, agent access. |
| `Components.dc.html` | Foundations | The token and component reference this document describes. |
| *(none)* | Account | Settings that are not about how you appear: password, second factor, and the tokens that act as you. Reached from Settings on your own profile, not from the sidebar. Never drawn. |
| *(none)* | Profile | A person at `/users/<name>`, and where the identity block at the foot of the sidebar goes. Fits the viewport: the picture, name, email, last seen, bio and teams are fixed at the top, and only the activity panel below them scrolls. Memberships sit side by side in a 230px-minimum grid, not stacked. Your own carries Edit profile and Settings; in edit mode the picture *is* the file control (a mono-caps `Change` strip across its foot) and the email and bio are inputs in place, with Save and Cancel in the screen header. Never drawn. |
| `Dialogs.dc.html` | Dialogs and states | Create item · Reject · New bucket · Split · Bulk move · Bulk edit · delete confirms · second factor · stream banners. |

The sidebar also lists the saved views of `SPEC.md` §9.4 under a `Views` section heading, between
the main navigation and team settings. A view is a switch, not a place: a click turns it on or
off, several can be on, and Board or Backlog stays the marked place. A view that is on is
accent-coloured on `accent-tint-weak`, without the place's left bar, with a mono-caps `ON` at its
right; `All off` appears in the section heading once two are on. Which are on is remembered per
team in the browser, and an old `?view=` link switches its view on. A view appears only in the
sidebar, never in the bar.

The filter bar edits one CSQL query (`SPEC.md` §9.2) two ways, and opens on the first.

The `</>` mode switch leads the bar, right after the `Filter` eyebrow, in both modes: it never moves,
so switching back is where switching there was.

- **Chips.** Assignee, State, Label, Bucket and Priority, then `+ More` (Reporter, Team, Created,
  Started, Completed, Due) and `Clear` once anything is set. A chip opens a dropdown in place, the card menu's panel, with a search box once it
  lists more than seven values. Each value row leads with a mono box that a click cycles: `[ ]`
  not in the filter, `[+]` is (accent), `[−]` isn't (alert, the name struck through). A mono hint
  at the foot says so: `Click: [+] is · [−] isn't · Esc undoes`. The dropdown applies when it
  closes; Escape leaves the filter as it was. A set chip reads `Label: api, −roses` (two values,
  then `+N`) in the accent, with its own clear. A date chip offers presets (Today, Last 7 days,
  Last 30 days; Due: Before today, Today, Next 7 days, No due date; Started and Completed: Not yet)
  and a From–To range; a preset applies at once. A field from `+ More` stays on the bar while it
  holds something; `Clear` takes the bar back to the five.
- **Raw.** `</>` (accent while on) turns the chips into the query they make, in a mono box at most
  420px wide, with a `?` beside it for a cheat sheet: three short sections (Fields, Operators,
  Examples) under mono-caps eyebrows, at most 520px wide, kept inside the window and scrolling
  within itself when the window is short. As you type, a list under the box
  offers what can come next at the caret — a field, its operators, its values, then `and` or `or`.
  A muted note follows an offer only where the offer alone does not say it: a bucket's title, a
  team's name, a priority's word, what `-7d` means. Tab takes the first offer, or the one moved to with the arrows; Enter runs
  the query unless an offer was moved to. Nothing runs while typing. A mistake is underlined wavy
  in `alert` where it is, and its sentence heads the list, unless the caret is in it, which is a
  word still being typed. A query the server refuses keeps the board as it was. Nothing in the box
  ever blocks: whenever it holds something not in use, `Discard` beside it puts the query in use
  back. `</>` goes back to the chips at once when they can show the query, applying the box on the
  way; a box that cannot be applied says "Not applied. Discard it to go back to the chips.", and
  Discard then goes back; a query in use that the chips cannot show says so and offers `Clear
  filter`.

The views that are on are and-ed with the bar. Save as view (shown once the bar holds a filter) and
the sidebar's + open a dialog with the name, the same two-way editor (chips wrapping, nothing run
until Save) and the pins; right-clicking a view opens a menu (the card menu's panel) with Edit… and
Delete…. Saving the bar's filter clears the bar and switches the new view on.

`canvas.json` in the design directory holds the layout and the annotations; the annotations are
design intent in prose and are worth reading once.

---

## 7. Bulk, clone and split — the interactions the mockups added

Behaviour is in `SPEC.md` §6.5. What the design fixes:

- **Selection** lives on the board and the backlog. A checkbox column leads each row; the header
  checkbox is select-all and goes indeterminate on a partial selection. A board card has no
  checkbox: ⌘/Ctrl-click picks it into the selection, where a plain click previews it.
- **Selection bar** offers, in this order: `Move to…` (primary), `Reject…`, then `Assignee`,
  `Priority`, `Bucket`, then `Clear`. All five are v1.
- **Bulk edit is one field at a time.** The dialog has a field switcher across the top and a value
  list below. There is no multi-field form.
- **Clone** and **Split** are buttons on the item header, not menu items buried in an overflow.
- **Split** takes a title per new ticket and offers one checkbox: reject the original afterwards.

---

## 8. Motion

Things arrive; they do not appear. Motion says where something came from and where it went, and is
over before anyone could wait for it. Nothing loops except the fire (§9), and only where there is
nothing else to look at.

| Token | Value | Used for |
|---|---|---|
| `snap` | 90ms | colour, border and background answering the pointer — every control |
| `quick` | 140ms | a drop-down, a dialog, a toast leaving, the selection bar, a tick in a checkbox, the toggle knob |
| `glide` | 220ms | a card travelling, the preview sliding in, a screen fading up, a toast arriving |
| `ease-out` | `cubic-bezier(0.33, 1, 0.68, 1)` | the one curve: fast out, soft landing. Colour changes use plain `ease-out` |

| What | How |
|---|---|
| Drop-down (team, search, labels) | fades in from 4px above: it hangs from what opened it |
| Card menu | fades in where the pointer is, and does not travel |
| Dialog | the scrim fades; the dialog rises 8px into place |
| Preview aside | slides in 12px from the right edge it lives on |
| Screen | its body fades up once it has something to show; header and rails stay put |
| Card changing column | travels there, whoever moved it — a drop, a bulk move, a teammate's change on the stream. Cards around it close up or make room. A card from nowhere or to nowhere fades |
| Card held in a drag | a copy follows the pointer from the spot it was picked up by, lifted to 103% and off the surface on the product's only shadow (`0 10px 24px` at 50% black). It turns about that spot with the speed it is carried at, at most 6°, and swings back upright when the hand stops. A run of cards carries its count in an `accent` badge |
| Card dropped | the copy travels straight from the hand onto the card's new place in 180ms, on `ease-out`, and the card shows under it; the cards around it make room. Called off — Escape, or let go outside a column — it settles back where it came from |
| Card reaching the final state | it feeds the fire, once it has landed. The card kindles: a wash of `accent-tint` rises through it over 1s, and its border runs `accent` → `ok` → rest, while the column's count bumps. About 26 square embers leave it. They rise on buoyancy, drift on a turbulent draft, flicker, shrink, and cool along the palette — `accent-hover`, `accent`, `alert`, `faint` — over 0.6–1.4s. One carrier ember, white-hot (`bright`) with a 64px streak cooling behind it, rises off the card and sweeps to the campfire mark in the sidebar in 0.62s. On arrival the mark roars: the flame and core leap and settle, with an `accent` glow, and it throws a few embers of its own. Drawn on one canvas that exists only while an ember is alive. Only a card that was open work on the board a moment ago: a filter that brings finished work into view has finished nothing. A run finished together feeds it one card after another, six at most. A card finished while a filter it no longer matches is on screen still lands in Done and feeds the fire (see the next row) |
| Card leaving the filter in view | it goes where it went, as any move does, and stays there: a card that stops matching the filter while on screen — moved, finished, relabelled, by you or anyone — glides to its new column (kindling, in Done) and, 1.4s on, settles to 50% opacity over 400ms, titled "No longer matches the filter". It stays through every refresh, following a later move, until the filter, a view or a toggle changes; a new filter shows only what matches it. Under reduced motion it is simply at 50% |
| Progress bar | both parts grow from the left in proportion, over 700ms, when drawn |
| Toast | rises into the stack; the ones above make room; it fades on the way out |
| Button | gives by 1px while pressed |
| Focus | `:focus-visible` is a 1px `accent` outline; a focused input also carries a 3px `accent-tint-weak` halo |

Exits are instant or `quick`, never `glide`: nobody waits for a dismissed thing to finish going.

**Reduced motion** turns every animation and transition off, scripted ones included — things still
change, nothing travels. The end-to-end suite runs that way, because it checks where things land.

---

## 9. The mark

Campfire is where people stop to warm up and plan their next move. The brand says so once, quietly,
and then gets out of the way: the product is about the work, not about itself.

The mark is a flame over two crossed logs, in the 16px grid, square and mitred like every glyph:

| Part | Path | Lit | Out |
|---|---|---|---|
| flame | `M8 1.2 12.6 9.2 10.8 12H5.2L3.4 9.2Z` | `accent` | `ghost` |
| core | `M8 5.6 10.4 9.6 9.4 12H6.6L5.6 9.6Z` | `accent-hover` | `dim` |
| logs | `M2.8 12.6 13.2 14.8M2.8 14.8 13.2 12.6`, stroke 1.4 | `faint` | `faint` |

Where it appears, and nowhere else:

- **Sidebar**, 19px beside the wordmark (mono 15 / 500 caps at 0.18em, `text`), the pair centred
  over the sidebar. It is lit while the live stream is up and burns down to embers while it is not,
  so the one piece of branding is also a status light nobody has to look for. It flares once for
  each change that arrives.
- **Loading**, 32px, burning — the flame leans, the core flickers on its own beat. It fades in only
  after 300ms, so a fast boot never shows it.
- **Sign in**, 36px, burning, above the wordmark. This is the one screen allowed a glow: a low
  `accent-tint-weak` ellipse from the bottom edge, as if from the fire the card is drawn up to.
- **Favicon**, lit, on `page`.

---

## 10. What the design does not cover

Ask rather than invent:

- Light mode. There is none, and the palette is not built for one.
- Mobile and narrow layouts. The rails are fixed and the board columns flex, and that is the whole
  of it. Narrower than the rails allow, the interface scrolls
  rather than reflows.
- Empty states beyond the board's empty column, the log's end-of-list, and the one-line `empty-note`
  a list falls back to.
- Error and validation copy beyond the input error state, the per-field message under a refused
  field, and the failure toast.
- Keyboard shortcuts and focus order. Escape closes a dialog and reverts a bare field; Enter submits
  a search or a label; nothing else is bound.
