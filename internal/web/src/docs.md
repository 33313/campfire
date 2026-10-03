Campfire is a kanban board for a team's work. This page is everything the screens do, on one page.
The API and the agent interface are described elsewhere: `api/openapi.yaml` in the source.

## The board and the backlog

Each team has its own **work states**, the columns of its board. Four of them carry a role:

| Role | Meaning |
|---|---|
| default | Where new work lands. Its items are the **backlog**. |
| accepted | Agreed to, not started. |
| final | Done. Items here more than three days are hidden unless **Show old done** is on. |
| rejected | Will not be done. On no board; reachable by a filter and by its identifier. |

The **board** shows every state but the default and the rejected one; the backlog is the default
state as a table. Any item may move to any state at any time: there are no rules about which
state comes after which, and the log records who moved what. Rejecting asks for a reason,
*cancelled* or *duplicate*, and a note if you like.

An item's **actual start** is set the first time it leaves the backlog, and its **actual end** when
it reaches done or rejected (it is cleared if it leaves again).

The board scrolls as one: every column moves together, and the board scrolls sideways when its
columns do not fit the screen.

## Tickets and buckets

A **ticket** is a piece of work. A **bucket** is a milestone that groups tickets, such as *Release
1.0*; it carries an icon and, if you like, a colour, which its tickets wear on the board. A bucket
takes tickets from any team, and its page shows them by team, with progress counted as how many
are done or rejected.

Both have an **identifier** such as `SOL-42`, from the team that made them. It is permanent and it
is the address: `/SOL-42` opens it. Buckets stay off the board unless **Show buckets** is on.

**Buckets** in the sidebar lists them three ways: those your team takes part in (owns, or has a
ticket in), those it owns, and the whole instance.

## Creating

**Create** opens the form. Every field an item has is on it, so nothing needs a second visit.

- **(Me!)** beside *Assignee* assigns it to you.
- **(Several)** beside *Title* takes a title per line: one item for each, with the rest of the form
  applied to all of them. Pasting a list into the title does the same. If one is refused, the ones
  before it are made and the rest stay in the box.
- **Mark as accepted** puts it straight into the accepted state instead of the backlog.
- Labels are made by typing them: Enter adds one, Backspace in the empty box takes the last back.

## Working with cards

- **Click** a card to read it in the preview beside the board, and edit it there. Its identifier
  opens the full page.
- **Drag** a card to move it. The amber line shows where it will land. Held near the top or bottom
  of the screen, the board scrolls; near its side, it scrolls sideways. Escape, or letting
  go outside a column, puts it back.
- **Ctrl-click** (**⌘-click** on a Mac) adds cards to a selection, and dragging any of them carries
  them all. In the backlog, the checkboxes select, and the header box selects everything shown.
- With a selection, the **selection bar** takes the filter bar's place: *Move to…*, *Reject…*,
  *Assignee*, *Priority* and *Bucket* change every selected item at once, one field at a time.

### The card menu

**Right-click** a card for the selection bar's actions at the pointer. Right-clicking a card outside
the selection selects it first, so the menu always acts on what it names.

- **Move to**, **Assignee**, **Priority** and **Bucket** open a sub-menu of their values; a click
  applies one at once. For a single card its current value is marked. The sub-menu opens on hover,
  on a click, or with →; ← goes back.
- **Reject…** asks for a reason.
- For one ticket: **Clone** copies it into a new ticket that *relates to* it; **Split…** makes a new
  ticket per title, each *following up* this one, and can reject the original; **Open** opens it.

## Writing: Markdown and pictures

Descriptions and comments are Markdown, as on GitHub: `**bold**`, `*italic*`, `` `code` ``,
`~~struck~~`, `# headings`, lists, `- [ ] tasks`, `> quotes`, tables, fenced code blocks and links.
A line break you type is kept. An identifier such as `SOL-42` becomes a link to its item.

**Paste** or **drop** a picture into a description or a comment and it is uploaded and added where
the caret is, as `![name](/api/images/12)`. The instance sets the largest picture it takes. A
picture linked from another site is shown as a link, not loaded.

In the preview, click the description to edit it; Escape puts back what was there.

## Filtering

Every filter is a query in **CSQL**, the Campfire Simple Query Language. The filter bar writes one
for you with **chips**: *Assignee*, *State*, *Label*, *Bucket*, *Priority*, and more under
*+ More*. In a chip's list, a click cycles a value through `[+]` (must match), `[−]` (must not) and
off; the filter applies when the list closes, and Escape leaves it as it was.

The `</>` switch shows the query itself, to read or type. Suggestions follow the caret: Tab takes the first
(or the one chosen with ↑↓), and Enter runs the query. A mistake is underlined where it is. The `?`
beside the box is a one-screen summary of what follows.

### Fields

| Field | Values | Operators |
|---|---|---|
| `assignee`, `reporter` | a username; `me`; `none` (assignee only) | `=` `!=` `in` `not in` |
| `label` | a label; `none` | `=` `!=` `in` `not in` |
| `bucket` | an identifier such as `SOL-6`, or a bucket's title in quotes; `none` | `=` `!=` `in` `not in` |
| `priority` | `1` to `4`, or `P1` to `P4`; 1 is highest | `=` `!=` `in` `not in` `<` `<=` `>` `>=` |
| `state` | a work state's name, in every team | `=` `!=` `in` `not in` |
| `team` | a short code or a team's name | `=` `!=` `in` `not in` |
| `kind` | `ticket` or `bucket` | `=` `!=` `in` `not in` |
| `created`, `started`, `completed`, `due` | a day: `2026-09-01`, `today`, `-7d`, `+2w`; `none` (not `created`) | `=` `!=` `<` `<=` `>` `>=` |
| `text` | a phrase, searched in titles and descriptions | `~` contains, `!~` does not |

### Putting conditions together

- Join conditions with `and`, `or` and `not`, and group them with parentheses. `and` binds
  tighter than `or`.
- `in (a, b)` is any of them; `not in (a, b)` is none of them.
- `!=` is exactly `not =`: `assignee != marta` includes the unassigned.
- Several `label = …` joined by `and` mean every one of those labels.
- Quote a value with spaces in it: `state = "In Review"`. Field names and keywords are not case
  sensitive.
- Days are whole days in your time zone. `created >= -7d` is the last seven days and today; a
  relative day stays relative when it is saved.
- `#12` names a label, user, bucket, team or state by its id.

### Examples

| Query | Finds |
|---|---|
| `assignee in (marta, me) and state != Done` | open work of yours and Marta's |
| `label = api or label = none` | labelled api, or not labelled at all |
| `due < today and completed = none` | late |
| `bucket = SOL-6 and not priority = 4` | the milestone's work, less the lowest priority |
| `created >= -7d and kind = ticket` | tickets from the last week |
| `text ~ "boiler"` | anything that mentions the boiler |

The search box in the header is the same as `text ~`, and drops matches under it as you type.

## Saved views

A **view** is a named query, listed under *Views* in the sidebar for the team that owns it. A view
is a switch, not a place: a click turns it on or off, and several can be on at once.

**How they combine.** Every view that is on, and the filter bar's query, are joined with `and`:
each one narrows the board or backlog further. *All off* turns them all off.

- **Save as view** in the filter bar names the filter in it, or + beside *Views* starts from none.
- **Right-click** a view for *Edit…* and *Delete…*. Anyone in its team may change it, and the
  change is everyone's.
- A view is **pinned** to the board, the backlog, or both. Switched on from another screen, it
  opens the board, or the backlog if it is pinned only there.
- A view that names something since deleted is marked **!**; that condition matches nothing until
  the view is edited.

## Keys and menus

| Where | Key | Does |
|---|---|---|
| Anywhere | Escape | closes the dialog, menu or list on top, and calls off a drag |
| A card | Ctrl-click / ⌘-click | adds it to the selection, or takes it out |
| A card | right-click | the card menu |
| The card menu | → / ← | opens a sub-menu / goes back |
| A view | right-click, or the menu key | *Edit…* and *Delete…* |
| The search box | ↑ ↓, Enter | chooses a match, opens it; Enter on none searches |
| The query box | Tab, ↑ ↓, Enter | takes a suggestion, moves between them, runs the query |
| A filter chip's list | ↑ ↓, Space, Enter | moves, cycles a value, applies |
| Labels | Enter, Backspace, ↑ ↓ | adds a label, takes the last back, chooses a suggestion |
| The preview's title | Enter, Escape | saves it, puts it back |
| The preview's description | Enter, then Escape | opens it to edit; puts back what was there |
| A backlog row | double-click | opens the item |

## The log and live updates

**Log** lists every change, newest first: who, what, and from what to what. A change an agent made
carries the agent mark beside the person, and the *Agent* chip shows only those; *Field* and *Item*
narrow it further. An item's own history is its *History* tab.

Boards update as other people work. The campfire at the top of the sidebar is lit while live
updates are flowing, flares for each change that arrives, and burns down while they are away.
Nothing on screen is wrong while it is out: it catches up when it is back.
