Campfire is a kanban board for your team's work. This page covers everything you can do on
screen. The API is described in `api/openapi.yaml` in the source.

## The board and the backlog

Each team has its own **work states**, the columns of its board. Four have a special role:

| Role | Meaning |
|---|---|
| default | New work lands here. These items are the **backlog**. |
| accepted | Agreed to, not started. |
| final | Done. Hidden after three days unless **Show old done** is on. |
| rejected | Won't be done. Not on the board; find it with a filter or its identifier. |

The **board** shows the other states as columns. The **backlog** shows the default state as a table.

- Move any item to any state, at any time. The log records who moved what.
- Rejecting asks for a reason, *cancelled* or *duplicate*, and an optional note.
- **Actual start** is set when an item first leaves the backlog. **Actual end** is set when it
  reaches done or rejected, and cleared if it leaves again.

## Tickets and buckets

A **ticket** is a piece of work. A **bucket** is a milestone that groups tickets, such as *Release
1.0*. It has an icon and an optional colour, which its tickets show on the board.

- Every item has a permanent **identifier** such as `SOL-42`. `/SOL-42` opens it.
- A bucket takes tickets from any team. Its page groups them by team and counts done or rejected
  ones as progress.
- Buckets stay off the board unless **Show buckets** is on.
- **Buckets** in the sidebar lists the ones your team is in, the ones it owns, or all of them.

## Creating

**Create** opens a form with every field, so you can fill it all in at once.

- **(Me!)** assigns it to you.
- **(Several)** makes one item per line of the title, sharing the rest of the form. Pasting a list
  into the title does the same. If one fails, the ones before it are made and the rest stay in the
  box.
- **Mark as accepted** skips the backlog.
- Type a label and press Enter to add it. Backspace in the empty box removes the last one.

## Working with cards

- **Click** a card to open it in the preview and edit it there. Click its identifier for the full
  page.
- **Drag** a card to move it; the amber line shows where it will land. Hold it near the edge of the
  screen to scroll. Escape, or dropping outside a column, cancels.
- **Ctrl-click** (**⌘-click** on a Mac) selects several cards, and dragging one moves them all. In
  the backlog, use the checkboxes.
- With cards selected, the **selection bar** replaces the filter bar. *Move to…*, *Reject…*,
  *Assignee*, *Priority* and *Bucket* change them all at once.

### The card menu

**Right-click** a card for the same actions. A card outside the selection is selected first.

- **Move to**, **Assignee**, **Priority** and **Bucket** open a sub-menu; click a value to apply
  it. For a single card, its current value is marked.
- **Reject…** asks for a reason.
- On one ticket: **Clone** copies it as a related ticket, **Split…** makes a follow-up ticket per
  title and can reject the original, and **Open** opens it.

## Writing: Markdown and pictures

Descriptions and comments use Markdown, as on GitHub: `**bold**`, `*italic*`, `` `code` ``,
`~~struck~~`, `# headings`, lists, `- [ ] tasks`, `> quotes`, tables, code blocks and links. Line
breaks are kept, and identifiers such as `SOL-42` become links.

**Paste** or **drop** a picture to upload it at the cursor. The instance sets the size limit.
Pictures from other sites show as links.

In the preview, click the description to edit it. Escape discards the edit.

## Filtering

Filters are queries in **CSQL**, the Campfire Simple Query Language, but you rarely need to type
one. The filter bar builds it from **chips**: *Assignee*, *State*, *Label*, *Bucket*, *Priority*,
and more under *+ More*. Click a value to cycle it through `[+]` (must match), `[−]` (must not)
and off. The filter applies when the list closes; Escape cancels.

The `</>` switch shows the query to read or type. Tab takes a suggestion, Enter runs the query,
and mistakes are underlined. The `?` beside the box is a quick summary.

### Fields

| Field | Values | Operators |
|---|---|---|
| `assignee`, `reporter` | a username; `me`; `none` (assignee only) | `=` `!=` `in` `not in` |
| `label` | a label; `none` | `=` `!=` `in` `not in` |
| `bucket` | an identifier such as `SOL-6`, or a title in quotes; `none` | `=` `!=` `in` `not in` |
| `priority` | `1` to `4`, or `P1` to `P4`; 1 is highest | `=` `!=` `in` `not in` `<` `<=` `>` `>=` |
| `state` | a work state's name, in any team | `=` `!=` `in` `not in` |
| `team` | a short code or a team's name | `=` `!=` `in` `not in` |
| `kind` | `ticket` or `bucket` | `=` `!=` `in` `not in` |
| `created`, `started`, `completed`, `due` | a day: `2026-09-01`, `today`, `-7d`, `+2w`; `none` (not `created`) | `=` `!=` `<` `<=` `>` `>=` |
| `text` | a phrase, searched in titles and descriptions | `~` contains, `!~` does not |

### Combining conditions

- Use `and`, `or`, `not` and parentheses. `and` binds tighter than `or`.
- `in (a, b)` matches any of them; `not in (a, b)` matches none.
- `!=` means `not =`, so `assignee != marta` includes unassigned items.
- `label = a and label = b` needs both labels.
- Quote values with spaces: `state = "In Review"`. Fields and keywords ignore case.
- Days are whole days in your time zone. `created >= -7d` is the last seven days and today. A
  relative day stays relative in a saved view.
- `#12` refers to a label, user, bucket, team or state by its id.

### Examples

| Query | Finds |
|---|---|
| `assignee in (marta, me) and state != Done` | open work of yours and Marta's |
| `label = api or label = none` | labelled api, or not labelled at all |
| `due < today and completed = none` | late work |
| `bucket = SOL-6 and not priority = 4` | the milestone's work, less the lowest priority |
| `created >= -7d and kind = ticket` | tickets from the last week |
| `text ~ "boiler"` | anything that mentions the boiler |

The search box in the header is a shortcut for `text ~`, showing matches as you type.

## Saved views

A **view** is a saved query, listed under *Views* in the sidebar of the team that owns it. Click a
view to turn it on or off. Several can be on at once.

Every view that is on and the filter bar are joined with `and`, so each narrows the results
further. *All off* turns them all off.

- **Save as view** saves the current filter. **+** beside *Views* starts from scratch.
- **Right-click** a view to edit or delete it. Anyone in the team can, and the change is everyone's.
- **Pin** a view to the board, the backlog, or both. Turned on from elsewhere, it opens the board,
  or the backlog if pinned only there.
- A view marked **!** names something since deleted. That condition matches nothing until you edit
  the view.

## Keys and menus

| Where | Key | Does |
|---|---|---|
| Anywhere | Escape | closes the top dialog, menu or list; cancels a drag |
| A card | Ctrl-click / ⌘-click | adds it to or removes it from the selection |
| A card | right-click | opens the card menu |
| The card menu | → / ← | opens a sub-menu / goes back |
| A view | right-click, or the menu key | *Edit…* and *Delete…* |
| The search box | ↑ ↓, Enter | picks a match and opens it; Enter with none picked searches |
| The query box | Tab, ↑ ↓, Enter | takes a suggestion, moves between them, runs the query |
| A filter chip's list | ↑ ↓, Space, Enter | moves, cycles a value, applies |
| Labels | Enter, Backspace, ↑ ↓ | adds a label, removes the last, picks a suggestion |
| The preview's title | Enter, Escape | saves, discards |
| The preview's description | Enter, then Escape | edits, discards |
| A backlog row | double-click | opens the item |

## The log and live updates

**Log** lists every change, newest first: who changed what, from what to what. Changes by an agent
carry the agent mark, and the *Agent* chip shows only those. *Field* and *Item* narrow it further.
An item's own history is its *History* tab.

Boards update live as others work. The campfire at the top of the sidebar is lit while connected,
flares with each change, and burns down while disconnected. Nothing is lost: it catches up when it
reconnects.
