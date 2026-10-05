## Overview

Campfire tracks a team's work on a kanban board. This page covers the basics first, then everyday
use, then connecting your own tools.

| Term | Meaning |
|---|---|
| Team | A group of people with its own board, backlog, work states and saved views. You can be in several. |
| Ticket | One piece of work: a title, a description, an assignee, a priority and a state. |
| Bucket | A milestone that groups tickets from any team, such as *Release 1.0*. |
| Item | A ticket or a bucket. Both have an identifier and move through work states. |
| Work state | A stage of work, such as *To Do* or *In Review*. Each team sets its own. |
| Board | The team's work in progress, one column per work state. |
| Backlog | New work, not yet accepted. |
| Identifier | An item's permanent ID, such as `SOL-42`: the team's short code and a number. |

## Getting started

1. **Pick or create a team.** Click the team name at the top of the sidebar to switch teams. An
   instance administrator can create one with **New team** in the same menu; otherwise, an
   administrator adds you to teams.
2. **Create a ticket.** Click **Create** in the top right corner of the page, type a title and click
   **Create**. Tick **Mark as accepted** to put it straight on the board. Otherwise it lands in the
   backlog, and you accept it later: open it and click **State** to pick a column.
3. **Move it along.** Drag the card from column to column on the board until it reaches *Done*.

## Tickets

### Creating

**Create**, in the top right corner of the page, opens the form. Only the title is required.

| Field | Notes |
|---|---|
| Title | Up to 300 characters. |
| Description | Markdown and pictures. See [Text formatting](#text-formatting). |
| Priority | `P1` (highest) to `P4`. Default `P3`. |
| Planned end | Optional due date. |
| Assignee | One person, or nobody. **(Me!)** assigns it to you. |
| Bucket | (Optional) The [Bucket](#buckets) it belongs to. Click the box for the list; type to narrow it by identifier or title. The ones you pick most come first. |
| Labels | Free text. Enter adds a label; Backspace in the empty box removes the last one. |
| Relations | (Optional) Pick a kind, type the other item's identifier, and press Enter. See *Comments, history and relations* below. |
| Mark as accepted | Puts the ticket on the board instead of in the backlog. |

### Editing

- **Click a card** to open it in the preview beside the board, and edit any field there.
- **Click the identifier** in the preview to open the full page. Each item also has its own
  link: Campfire's address, a slash and the identifier, such as
  `https://campfire.example.com/SOL-42`. Open or share it to go straight to the item.
- **Actual start** is set when an item first leaves the backlog. **Actual end** is set when it is
  done or rejected, and cleared if it moves back.

### Comments, history and relations

- **Comments** use Markdown. Anyone signed in can comment on any item.
- **History** lists every change to the item: who, when, and the old and new value.
- **Relations** link two items: *blocks* or *blocked by*, *follows up* or *followed up by*,
  *duplicates* or *duplicated by*, and *relates to*. Each shows on both items, the right way round,
  and in the preview. They are for reference and don't block any move.

### Clone and split

- **Clone** copies a ticket's title, description, priority, assignee, bucket and labels into a new
  ticket in the backlog, related to the original.
- **Split** creates one ticket per title you enter, each marked as a follow-up of the original. It
  can also reject the original.

## Buckets

A bucket is a milestone. Create one with **Create** → **Bucket**. It has an icon and an optional
colour, and its tickets show both on their cards.

- **Add a ticket** by setting the ticket's *Bucket* field. A bucket takes tickets from any team.
- **The bucket's page** lists its tickets by team, with progress as finished over total (`37 / 50`).
  Done and rejected tickets count as finished.
- **Buckets** in the sidebar lists the buckets your team takes part in, the ones it owns, or every
  bucket.
- A bucket has its own state. It doesn't complete when its tickets do.
- Buckets are hidden on the board unless **Show buckets** is on.
- Buckets can't be deleted. Reject one as *cancelled* instead.

## Board and backlog

Each team defines its work states in **Team settings**. Four of them have a role:

| Role | Example | Meaning |
|---|---|---|
| default | Backlog | New items land here. These make up the backlog. |
| accepted | To Do | Agreed, not started. |
| final | Done | Finished. |
| rejected | Rejected | Won't be done. Needs a reason: *cancelled* or *duplicate*. |

The **board** shows every state except default and rejected. The **backlog** lists the default state
as a table. Rejected items appear in neither; find them with a filter or by identifier.

### Moving cards

- **Drag** a card to another column, or within one to reorder it. A line shows where it will land.
  Hold it near the edge of the screen to scroll. Escape cancels.
- Any item can move to any state at any time. The log records every move.
- Items done for more than three days are hidden unless **Show old done** is on.

### Selecting cards

- **Ctrl-click** (**⌘-click** on a Mac) selects cards; dragging one moves them all. In the backlog,
  use the checkboxes.
- The **selection bar** replaces the filter bar and changes every selected item at once: *Move to…*,
  *Reject…*, *Assignee*, *Priority* and *Bucket*. The selection stays once a change goes through,
  less anything that left the screen, so the next change can follow. *Clear* ends it.

### Card menu

**Right-click** a card, or a row in the backlog, for the same actions. One outside the selection is
selected first.

| Action | Does |
|---|---|
| Move to, Assignee, Priority, Bucket | Opens a list of values; a click applies one. For one card, its current value is marked. |
| Reject… | Asks for a reason, then rejects. |
| Clone, Split… | See *Clone and split* under [Tickets](#tickets). One ticket only. |
| Open | Opens the full page. One ticket only. |

## Text formatting

Descriptions and comments are written in Markdown ([CommonMark](https://commonmark.org) with
GitHub's extensions) and can include pictures. Click a description in the preview to edit it;
Escape discards the edit.

### Inline

| Type | Result |
|---|---|
| `**bold**` | **bold** |
| `*italic*` | *italic* |
| `~~strikethrough~~` | ~~strikethrough~~ |
| `` `code` `` | `code` |
| `[text](https://example.com)` | [text](https://example.com) |
| `https://example.com` | https://example.com |
| An identifier, such as `SOL-42` | A link to that item |

### Blocks

| Type | Result |
|---|---|
| `# Title`, `## Title`, `### Title` | Headings, three levels |
| `- item` or `1. item` | Bulleted or numbered list |
| `- [ ] to do` and `- [x] done` | Task list |
| `> text` | Quote |
| ```` ``` ```` on the lines before and after | Code block |
| `\| a \| b \|` rows, with `\|---\|---\|` under the first | Table |

- A line break you type is kept.
- HTML is shown as plain text.
- Identifiers inside code are not linked.

### Pictures

- **Paste** or **drop** an image into a description or a comment. It uploads and goes in at the
  cursor as `![name](/api/images/12)`.
- PNG, JPEG, GIF and WebP are accepted. The size limit is set by the instance, 10 MB by default.
- Click a picture to open it full size.
- Pictures from other sites are shown as links and never loaded.

## Filtering

The filter bar narrows the board and the backlog. Each **chip** filters one field: *Assignee*,
*State*, *Label*, *Bucket*, *Priority*, and more under *+ More*. Click a value to cycle it through
`[+]` (must match), `[−]` (must not match) and off. The filter applies when the list closes; Escape
cancels.

Chips write a query in **CSQL**, Campfire's query language. **`</>`** shows the query, where you can
also type one: Tab takes a suggestion, Enter runs it, and errors are underlined. **?** beside the box
shows a summary.

The search box in the header is a shortcut for `text ~`, with matches shown as you type.

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

### Syntax

- Combine conditions with `and`, `or`, `not` and parentheses. `and` binds tighter than `or`.
- `in (a, b)` matches any of the values; `not in (a, b)` matches none of them.
- `!=` means `not =`, so `assignee != marta` includes unassigned items.
- `label = a and label = b` needs both labels.
- Quote values with spaces: `state = "In Review"`. Field names and keywords ignore case.
- Days are whole days in your time zone. `created >= -7d` is the last seven days and today. A
  relative day stays relative in a saved view.
- `#12` refers to a label, user, bucket, team or state by its ID.

### Examples

| Query | Finds |
|---|---|
| `assignee in (marta, me) and state != Done` | Open work of yours and Marta's |
| `label = api or label = none` | Labelled *api*, or not labelled |
| `due < today and completed = none` | Overdue work |
| `bucket = SOL-6 and not priority = 4` | The milestone's work, except P4 |
| `created >= -7d and kind = ticket` | Tickets from the last week |
| `text ~ "boiler"` | Anything that mentions the boiler |

## Saved views

A **view** is a saved filter. Views are listed under *Views* in the sidebar of the team that owns
them. Click a view to turn it on or off.

- Every view that is on, and the filter bar, are combined with `and`. *All off* turns them all off.
- **Save as view** saves the current filter. **+** beside *Views* starts an empty one.
- **Pin** a view to the board, the backlog, or both. Turning it on elsewhere opens the board, or
  the backlog if pinned only there.
- **Right-click** a view to edit or delete it. Anyone in the team can, and the change applies to
  everyone.
- A view marked **!** refers to something since deleted. That condition matches nothing until the
  view is edited.

## Keyboard and mouse

| Where | Input | Does |
|---|---|---|
| Anywhere | Escape | Closes the top dialog, menu or list; cancels a drag |
| A card | Click | Opens it in the preview |
| A card | Ctrl-click / ⌘-click | Adds it to or removes it from the selection |
| A card | Right-click | Opens the card menu |
| The card menu | → / ← | Opens a submenu / goes back |
| A backlog row | Double-click | Opens the item |
| A view | Right-click, or the menu key | *Edit…* and *Delete…* |
| The search box | ↑ ↓, Enter | Picks a match and opens it; Enter with none picked searches |
| The query box | Tab, ↑ ↓, Enter | Takes a suggestion, moves between them, runs the query |
| A filter chip's list | ↑ ↓, Space, Enter | Moves, cycles a value, applies |
| Labels | Enter, Backspace, ↑ ↓ | Adds a label, removes the last, picks a suggestion |
| The preview's title | Enter, Escape | Saves, discards |
| The preview's description | Click, then Escape | Edits, discards |

## Activity log

**Log** in the sidebar lists every change in your team, newest first: who changed which field, from
what to what. The *Agent*, *Field* and *Item* chips narrow it. An item's own changes are on its
*History* tab.

The board updates live as others work. The flame at the top of the sidebar shows the connection: lit
while connected, dimming while disconnected. After a disconnect the board catches up on its own.

## Teams and accounts

### Roles

| Role | Can |
|---|---|
| Member | Create and edit items in their teams; comment on any item. |
| Team administrator | Also manage the team: members, work states, name, picture and description. |
| Instance administrator | Also create accounts and teams, send invitations and password resets, manage backups, and read the log of every team. |

Your role can differ from team to team.

### Your account

Click your name at the foot of the sidebar to open your profile: picture, email and bio. Its
**Settings** has your password, a second factor (an authenticator app code at sign-in) and agent
tokens.

### Team settings

Team administrators find **Team settings** in the sidebar: *Work states*, *Members*, *Identity* (name,
picture, description and bucket icon style) and *Archive*.

## Agents (MCP)

Campfire has a built-in [MCP](https://modelcontextprotocol.io) server, so an AI agent such as Claude
Code can read and update your tickets. The agent acts as you, with your permissions.

### Connect an agent

1. **Enable it.** An instance administrator sets `"mcp": { "enabled": true }` in `campfire.json`
   and restarts Campfire. The server is then at `/mcp` on the same address as the app.
2. **Issue a token.** Open your profile, then **Settings** → **Agent tokens** → **Issue a token**.
   Name it after where it will be used, such as *laptop*. The token is shown once.
3. **Add it to the agent.** The token dialog shows the command for Claude Code:

```sh
claude mcp add --scope user --transport http campfire \
  https://campfire.example.com/mcp --header "Authorization: Bearer <token>"
```

Other clients take the same address and header as JSON:

```json
{
  "mcpServers": {
    "campfire": {
      "type": "http",
      "url": "https://campfire.example.com/mcp",
      "headers": { "Authorization": "Bearer <token>" }
    }
  }
}
```

A client that only supports stdio can connect through a bridge:

```sh
npx -y mcp-remote https://campfire.example.com/mcp --header "Authorization: Bearer <token>"
```

### What an agent can do

| Area | Tools |
|---|---|
| Items | `create_item`, `update_item`, `move_item`, `rank_item`, `clone_item`, `split_item`, `bulk_edit_items` |
| Reading | `get_board`, `get_backlog`, `list_items`, `get_item`, `lookup_item`, `list_buckets`, `get_bucket_contents`, `get_item_impact` |
| Labels and relations | `add_label`, `remove_label`, `list_labels`, `add_relation`, `remove_relation` |
| Comments | `add_comment`, `update_comment`, `list_comments` |
| Saved views | `list_saved_views`, `create_saved_view`, `update_saved_view`, `delete_saved_view` |
| Teams and people | `list_teams`, `get_team`, `list_work_states`, `list_team_members`, `list_users` |
| Changes | `get_log` |

An agent can't create accounts, change teams or membership, delete items, manage backups or issue
tokens.

### Tracking agent changes

Every change an agent makes is logged under your name and the token's, as *marta (agent: laptop)*.
The *Agent* chip in **Log** shows only agent changes. Revoke a token in **Settings** at any time; an
instance administrator can revoke any token.

## HTTP API

The app itself runs on a JSON API under `/api`, and your own scripts and services can use it too.
The full definition is [`api/openapi.yaml`](https://github.com/33313/campfire/blob/main/api/openapi.yaml)
(OpenAPI 3.1).

### Authentication

Sign in with `POST /api/auth/signin` and send the `campfire_session` cookie it sets with every later
request. Add `"code"` to the body if the account has a second factor. Agent tokens work only for
MCP, not for this API.

```sh
curl -c session.txt https://campfire.example.com/api/auth/signin \
  -H 'Content-Type: application/json' \
  -d '{"username": "marta", "password": "…"}'
```

### Examples

Create a ticket in team 1:

```sh
curl -b session.txt https://campfire.example.com/api/items \
  -H 'Content-Type: application/json' \
  -d '{"kind": "ticket", "teamId": 1, "title": "Renew the certificate", "priority": 2}'
```

List unfinished P1 items with a CSQL query:

```sh
curl -b session.txt -G https://campfire.example.com/api/items \
  --data-urlencode 'csql=priority = 1 and completed = none'
```

### Common endpoints

| Request | Does |
|---|---|
| `GET /api/teams` | Lists teams |
| `GET /api/teams/{id}/board` | A team's board |
| `GET /api/teams/{id}/backlog` | A team's backlog |
| `GET /api/items?csql=…` | Items matching a query |
| `GET /api/lookup/{identifier}` | An item by identifier, such as `SOL-42` |
| `POST /api/items` | Creates a ticket or a bucket |
| `PATCH /api/items/{id}` | Edits an item's fields |
| `POST /api/items/{id}/move` | Moves an item to a work state: `{"stateId": 4}` |
| `POST /api/items/{id}/comments` | Adds a comment |
| `GET /api/log` | The activity log, newest first |
| `GET /api/stream` | Live changes, as server-sent events |

A refused request returns a 4xx or 5xx status and an error body. `code` is for programs, such as
`invalid`, `forbidden`, `not_found` or `conflict`; `message` is for people.

```json
{ "error": { "code": "forbidden", "message": "…" } }
```

## Source code

Campfire is free software under the GNU Affero General Public License, version 3. Its source is at
[github.com/33313/campfire](https://github.com/33313/campfire).
