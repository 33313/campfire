# Campfire — Architectural Specification

**Version:** 1.0 (release candidate)
**Covers:** what the application does, what it does not do, and how it is built.
**Does not cover:** visual design, layout, colour, copy. These belong to a separate design
document that derives from this one.

## Conventions

- **WILL** / **WILL NOT** describe product behaviour: what the application does for a user.
- **MUST** / **MUST NOT** describe implementation requirements: rules the code has to satisfy.
- Section references are links. Terms in **bold** on first use are defined in [§2](#2-definitions).
- Rejected alternatives are listed in [Appendix B](#appendix-b--rejected-alternatives), not in the
  body.

---

## 1. Purpose and audience

Campfire is a work-tracking application organised as a kanban board.

The audience is **individuals and small to medium teams who run the application themselves**. A
typical deployment has no operator, no database administrator, and no infrastructure team. The
expected adoption path is one person using it, then bringing it to a team of five to fifty.

This is not an enterprise product. It is self-hosted and freely available, and no revenue model
depends on hosting it.

Two principles govern every decision in this document:

1. **Small surface.** Anything expressible with an existing concept WILL NOT become a new concept.
2. **One deployment story.** A laptop and a shared server run the same application, configured the
   same way. Editions WILL NOT exist.

---

## 2. Definitions

| Term | Definition |
|---|---|
| **Instance** | One running copy of the application with its own data. All users of an instance share one trust boundary. |
| **Team** | A group that owns work. Has a short code, members, and work states. |
| **Short code** | A team's unique prefix, e.g. `SOL`. Immutable, never reused. |
| **Item** | A ticket or a bucket. Both are stored in one table and share almost all fields. |
| **Ticket** | A unit of work. May belong to one bucket. |
| **Bucket** | A milestone that groups related work, e.g. "Release: 1.0". Cannot contain other buckets. |
| **Icon** | One of a fixed set of glyphs, carried by a bucket. Tickets have none. |
| **Identifier** | An item's permanent public name, e.g. `SOL-42`. Also its permalink. |
| **Work state** | A column on a board, defined per team, e.g. "In Progress". |
| **State role** | A lifecycle meaning attached to one work state: `default`, `accepted`, `rejected`, or `final`. |
| **Terminal state** | A state whose role is `final` or `rejected`. |
| **Board** | A team's view of its items, one column per work state, excluding the default and rejected states. |
| **Backlog** | A team's view of items in its default state. |
| **Label** | A free-text tag, created by typing it. Instance-wide. |
| **Relation** | A directed link between two items, e.g. *blocks*. |
| **Event** | One entry in the audit log, recording a single change and who made it. |
| **Rank** | An item's manual position within a board column. |
| **CSQL** | The Campfire Simple Query Language: the one way a filter is written ([§9.2](#92-filtering)). |
| **Saved view** | A named, stored CSQL query, owned by one team. |
| **Local instance** | The application running against storage on the same machine. |
| **Remote instance** | An instance reached over a network. |
| **Agent** | A program acting on one user's behalf, holding exactly that user's permissions. |
| **Agent token** | A revocable credential naming the user an agent acts as. Carries no permissions of its own. |

---

## 3. Scope

### 3.1 In scope for v1

Accounts and authentication; teams and permissions; tickets and buckets; per-team work states;
labels; relations; comments; cloning and splitting a ticket; bulk moves and bulk edits; manual
ordering; filtering in CSQL; saved views; an audit log; live board updates; an agent interface
over the Model Context Protocol; backup and restore.

### 3.2 Out of scope, permanently

These WILL NOT be built. Each is listed with what the application does instead.

| The application WILL NOT | It WILL instead |
|---|---|
| Move a ticket between teams | Keep every ticket in the team that created it. Identifiers are permanent and a moved ticket would break its own permalink. |
| Nest a bucket inside another bucket | Support one level of grouping. Cross-team milestones are handled by [§6.4](#64-bucket-membership). |
| Classify a ticket by type | Let the bucket, the title and the description say what the work is ([Appendix B](#appendix-b--rejected-alternatives)). A bucket carries an icon; a ticket carries nothing of the kind. |
| Offer per-ticket or per-team privacy | Let every authenticated user read every item and comment in the instance. Permissions govern writing, not reading. |
| Support offline editing and later synchronisation | Run against a local instance or a remote instance ([§13.1](#131-delivery)). An export, if offered, is a one-way copy and never a merge. |
| Ship a native, desktop, or mobile client | Serve a web client from the server ([§13.1](#131-delivery)). The API contract ([§11](#11-api-contract)) is published so that others can build one. |

The term "offline" WILL NOT appear in the product.

### 3.3 Deferred to a later specification

Not built in v1, not designed here, and not to be re-argued during v1:

Attachments, beyond pictures in prose ([§6.1](#61-fields)) · notifications · email delivery ·
watchers and subscriptions · sprints and iterations · time tracking · custom fields · WIP limits ·
bulk editing of title, description, planned end and labels · external authentication (SSO, LDAP,
OIDC) · public or anonymous access · imports from other trackers · audit log retention and archival
policy.

The data model MUST NOT preclude these. Scaffolding for them WILL NOT be built.

---

## 4. Entities

Eight.

| Entity | Purpose |
|---|---|
| **User** | An account. |
| **Team** | An owning group. |
| **Item** | A ticket or a bucket, discriminated by `kind`. |
| **WorkState** | A board column with an optional lifecycle role. |
| **Label** | A free-text tag. |
| **Relation** | A directed link between two items. |
| **SavedView** | A named, stored CSQL query belonging to one team. |
| **Event** | One audit log entry. |

A `Board` entity WILL NOT exist. A team has one board, and everything configurable about it is its
work states. The board is a view.

A `Bucket` entity WILL NOT exist. A bucket is an `Item` with `kind = 'bucket'`
([§6.2](#62-buckets)).

A `SavedView` entity does exist, because a saved view has its own name, owner and lifecycle and
nothing to be an attribute of: a team would have to carry a list of filters, and
[§13.2](#132-implementation-rules) rule 7 keeps relational structures relational. What its query
names by id is rows belonging to the saved view, in the same way comments are rows belonging to an
item, and is no more an entity than comments are ([§9.4](#94-saved-views)).

---

## 5. Accounts, teams and permissions

### 5.1 Accounts

- Required: username (unique), password.
- Optional: email, TOTP second factor, avatar.
- A flow MUST NOT depend on email delivery. Email is optional, so a password reset is issued by an
  instance administrator as a one-time reset, never by a mailed link.
- Open registration WILL NOT be offered. An instance administrator creates accounts or issues
  invitation links.

### 5.2 Roles

Three roles exist. Others WILL NOT be added in v1.

| Role | Can |
|---|---|
| **Instance administrator** | Create users and teams, issue invitations and password resets, read the global log, reassign bucket ownership, act as a team administrator anywhere. |
| **Team administrator** | Manage one team: membership, work states, bucket icon rendering, team name, picture and description, archival of the team's items. |
| **Member** | Create and edit tickets in teams they belong to. |

Any authenticated user may comment on any item, regardless of team membership.

A user may belong to any number of teams, with a different role in each.

### 5.3 Teams

A team owns a name, a short code, members, work states ([§7](#7-work-states-and-lifecycle)), the
rendering it uses for bucket icons ([§6.2](#62-buckets)), and an optional picture and description.
The picture and description are presentation: nothing derives from them.

A team WILL NOT own a list of ticket types. A ticket is described by the bucket it sits in, its title
and its description, and a type field adds a taxonomy to administer without adding anything the three
of them do not already say ([Appendix B](#appendix-b--rejected-alternatives)).

A team's short code is set at creation and MUST NOT change. A short code MUST NOT be reused after a
team is archived; reuse would mint identifiers colliding with the archived team's.

---

## 6. Items

### 6.1 Fields

Every item, ticket or bucket, has:

| Field | Notes |
|---|---|
| Identifier | Permanent. See [§6.3](#63-identifiers). |
| Title | Required. |
| Description | Markdown. |
| Reporter | Set automatically to the creating user. Not editable. |
| Assignee | Optional, single, empty by default. Never inferred from the reporter. |
| Priority | `P1`–`P4`, default `P3`. |
| Icon | Buckets only. One of a fixed set ([§6.2](#62-buckets)). A ticket WILL NOT have one. |
| Work state | From the owning team's states. |
| Planned end | Optional date. |
| Actual start | Set automatically on the item's first move out of the default state. |
| Actual end | Set automatically on entering a terminal state. Cleared on leaving one. |
| Labels | Zero or more. See [§8.1](#81-labels). |
| Relations | Zero or more. See [§8.2](#82-relations). |
| Comments | Markdown, flat, editable, by any user. |
| Rank | Position within a board column. See [§9.3](#93-ordering). |

**Pictures.** A description or a comment may show pictures pasted or dropped into it. Each is
uploaded on its own and named in the Markdown by its address, `![name](/api/images/12)`, so the
text stays the one thing that is edited, logged and copied. Pictures are kept in the database, so a
backup is still one file, and readable by every authenticated user, like the text that shows them.

- A picture MUST be PNG, JPEG, GIF or WebP, decided from its bytes, never from what the client
  says it sent. SVG MUST NOT be accepted: it is a document that can run script.
- One setting bounds a picture's size, `images.maxMB`, 10 by default; 0 turns pictures off.
- An upload writes no event. A picture changes nothing until a description or a comment shows it,
  and that edit is the event.
- A picture named from off the instance is shown as a link, never fetched: drawing it would have
  every reader's browser call a third party.

An item WILL NOT have a planned start date. Accepting work starts its clock: actual start is set on
the first move out of the default state, which includes acceptance.

### 6.2 Buckets

A bucket differs from a ticket in one way: **a bucket WILL NOT belong to another bucket.**

A bucket's detail view lists every item assigned to it, grouped by owning team, with progress shown
as a count of items in a terminal state (`37 / 50`). This is a view, not a field.

A bucket's assignee is presented as its **owner**: the person accountable for the milestone. It is
optional and empty by default.

A bucket carries an **icon**, chosen from a fixed set when it is created and editable afterwards. It
stands in for the bucket wherever the bucket is named — on a card, in the bucket index, in the log —
so that a card's bucket line is recognised before it is read. The set is fixed and WILL NOT be
extended or uploaded to; an open set is a second thing for a team to administer. A bucket that has
not been given one shows the default. The icon is recognition and nothing more: it MUST NOT be given
meaning, and nothing reads it, filters on it, or derives behaviour from it. The set ships in two
renderings, monochrome and coloured, and a team picks the one its own surfaces use — the same kind of
per-team choice as its work states. Which rendering is in force is presentation and MUST NOT change
which icon a bucket carries.

Buckets use their owning team's work states. The client WILL hide them from board columns by
default and show them by a per-board toggle, so that an engineering board is not diluted by
milestones and a planning team's board can consist entirely of them. The toggle is a CSQL clause,
`kind = ticket`, and-ed with the reader's filter ([§9.2](#92-filtering)).

A bucket WILL accept tickets from any team. This is how cross-team milestones work, and it is why
the application WILL NOT have a separate organisation-wide "goals" layer.

### 6.3 Identifiers

An identifier is `<team short code>-<number>`, e.g. `SOL-42`. It is also the item's permalink.

- Each team has **one counter**, shared by tickets and buckets. `SOL-42` may be a bucket and
  `SOL-43` a ticket.
- An identifier MUST NOT encode whether an item is a ticket or a bucket. One format means one
  pattern for finding references in text. Telling the two apart is a presentation concern.
- An identifier is permanent. It records the team that **created** the item.
- Ownership is a separate, stored field. Code MUST read ownership from that field and MUST NOT
  parse it out of the identifier.

Ownership rules:

- A ticket's owning team MUST NOT change.
- A bucket's owning team may be reassigned by an instance administrator, because a milestone must
  survive the dissolution of the team that planned it.
- **A reassigned bucket keeps its identifier.** If `SOL-42` moves to team TERRA, TERRA owns
  `SOL-42` and sees it among its own buckets. The prefix records origin, not current ownership.

### 6.4 Bucket membership

Bucket membership is a field **on the ticket**, governed by ordinary ticket-edit permission. A new
permission concept WILL NOT be introduced.

- A member of team SOL may put any SOL ticket into any bucket in the instance, and remove it.
- A user outside SOL MUST NOT add or remove SOL tickets, whoever owns the bucket.
- **One exception:** an administrator of the bucket's owning team may *remove* a ticket from that
  bucket. Removal only. This is the only operation that writes to a ticket outside the actor's team,
  and it MUST be logged as an event on the ticket.

Without the exception, a bucket's scope is world-writable. Without the rest of the rule, a small
planning team becomes a bottleneck for work it does not do.

A bucket's lifecycle is independent of its contents in both directions. Moving a bucket to a
terminal state MUST NOT change its tickets, and a bucket's state MUST NOT be derived from its
tickets. A bucket WILL NOT auto-complete.

Buckets WILL NOT be hard-deleted through the interface; rejection with reason `cancelled` is the
disposal route. Deletion is an instance-administrator operation and MUST state how many items across
how many teams are affected.

### 6.5 Cloning, splitting and bulk edits

Three operations over items that already exist. None of them is a new concept. Each is a shorthand
for edits a member could make one at a time, governed by ordinary ticket-edit permission
([§6.4](#64-bucket-membership)), and each leaves the same trail in the log as the long way round.

**Clone.** One ticket becomes two. The new ticket takes a fresh identifier from its team's counter
and copies title, description, priority, assignee, bucket and labels. It enters the default state,
its reporter is the cloning user, and it carries a *relates to* relation
([§8.2](#82-relations)) to the ticket it came from. Provenance is a relation rather than a new field,
because a relation is already stored, already displayed from both ends, and already filterable.

**Split.** One ticket becomes several. The actor gives a title per new ticket; each new ticket is
created the way a clone is, and carries a *follows up* relation to the source instead of *relates
to*, and a *relates to* relation to each of the other tickets the same split made. The source keeps its state unless the actor also rejects it in the same action, in which case
the reason is `cancelled`. A split WILL NOT delete or absorb the source: deletion is not a state, and
the tickets that replaced it are reachable from it.

**Bulk edit.** A selection of items on a board or in a backlog takes one new value for one field, in
one action. Four fields are editable this way:

| Field | Notes |
|---|---|
| Work state | The transition of [§7.3](#73-transitions) applied to each item in turn: no transition graph, no new permission, no new lifecycle. A rejection reason, if the target is the rejected state, applies to the whole selection. |
| Assignee | Including clearing it. |
| Priority | |
| Bucket | Including removing it. Governed by [§6.4](#64-bucket-membership) per item, so a ticket outside the actor's team is refused like any other. |

One field per action. A bulk edit WILL NOT be a form over several fields at once: a selection is
where a mis-click is least visible, and one field is a change the actor can describe to themselves
before making it and reverse afterwards.

The fields left out — title, description, planned end and labels — are deferred
([§3.3](#33-deferred-to-a-later-specification)). Title and description are prose, planned end is a
judgement about one ticket, and a selection is the wrong place to write any of the three. Labels are
left out because
the operation is not one value but three — add, remove, replace — and picking one for v1 would settle
a question nobody has asked yet.

Requirements:

- Each item created by a clone or a split MUST allocate its identifier atomically
  ([§13.2](#132-implementation-rules) rule 2), one item at a time. A batch MUST NOT reserve a range.
- A clone or a split MUST NOT copy the source's comments, history, relations, actual start or
  actual end. The edge back to the source, and a split's edges between its new tickets, are the only
  relations a new item is created with.
- Every created item and every item touched by a bulk edit MUST write its own event
  ([§10](#10-audit-log)). A bulk operation WILL NOT write one summary event; a log that summarises is
  a log that cannot answer what happened to one ticket.
- An item in a bulk edit that the actor may not edit MUST be refused on its own and MUST NOT abort
  the rest. The interface reports what was refused.
- A bulk edit MUST go through the same handler and the same permission check as the single-item edit
  ([§14.1](#141-relationship-to-the-http-api) makes the same demand of the agent interface). It is a
  loop over one operation, not a second write path.

---

## 7. Work states and lifecycle

### 7.1 Definition

Each team defines an ordered list of work states. Exactly one state per team holds each of the four
roles:

| Role | Meaning |
|---|---|
| `default` | Where new work lands. |
| `accepted` | Work agreed to but not started. |
| `rejected` | Terminal. Work that will not be done. |
| `final` | Terminal. Work that was completed. |

States without a role are intermediate, and a team may have any number of them. A team MUST always
have all four roles filled.

Example: `Backlog` (default), `To Do` (accepted), `In Progress`, `In Review`, `Done` (final),
`Rejected` (rejected).

### 7.2 Board and backlog

- The **board** shows every state except the default state and the rejected state.
- The **backlog** shows items in the default state.
- Rejected items WILL NOT appear on any board. They are reachable by filter and by permalink.
- Items that have been in the final state for more than three days are hidden from the board
  unless it asks for all of them, by a per-board toggle. A Done column that only grows is
  history, not work.

Backlog is therefore a consequence of the state model, not a separate feature.

### 7.3 Transitions

- A new item enters the default state. If the creator ticks *Mark as accepted*, it enters the
  accepted state instead.
- **Any item may move to any state at any time.** The application WILL NOT enforce a transition
  graph, guard rules, or per-state permissions. The audit log ([§10](#10-audit-log)) records who
  moved what.
- Moving an item to the rejected state requires a reason: `cancelled` or `duplicate`, plus optional
  free text. A `duplicate` rejection should carry a relation to the item it duplicates.
- Entering a terminal state sets actual end. Leaving one clears it.

Rejection is a state, not a separate "resolution" field. The application has one lifecycle
mechanism.

---

## 8. Labels and relations

### 8.1 Labels

Labels are free text, instance-wide, and created by typing them. Labels WILL NOT be administered,
and WILL NOT come from a predefined set. Labels are filterable.

**Labels WILL NOT appear on board cards.** They appear on an item's detail view only. A card carries
identifier, title, priority, assignee and bucket — the bucket shown as its icon and name. Labels are
a filtering mechanism, and cards carrying them become unreadable.

### 8.2 Relations

A fixed set. Users WILL NOT be able to define new relation types. Each relation is stored once as a
directed edge and displayed from both ends.

| Forward | Inverse |
|---|---|
| blocks | blocked by |
| follows up | followed up by |
| duplicates | duplicated by |
| relates to | relates to |

Relations may cross teams and may link a ticket to a bucket. A relation is added from either end, in
either column: *A blocked by B* is the one edge *B blocks A*. An item MAY be created with relations,
made in the same write.

A relation WILL NOT carry lifecycle meaning. A *blocked by* edge is information; it MUST NOT prevent
any transition.

### 8.3 Pull requests

An instance MAY be connected to GitHub through GitHub Apps of its own, created from a manifest by
an instance administrator, each installed on the account or organisation it serves; a private app
installs only on its owner, a public one anywhere. A pull request in a repository an app can read
is listed on every item its title names — identifiers at the start or the end of the title, bare
or in brackets — with its state: open, draft, merged or closed. Forgetting an app unlinks the
accounts linked through it.

The title is the whole link. Retitling a pull request moves it; one that names nothing is not kept.
A pull request MUST NOT change an item's state: like a relation, it is information. Its link and
its state changes are written to the item's history with no actor. Deliveries from an installation
that is not linked are ignored. Unlinking an account keeps what it already listed.

---

## 9. Views, filtering and ordering

### 9.1 Views

| View | Shows |
|---|---|
| Board | One team's items, one column per work state, excluding default and rejected. |
| Backlog | One team's items in its default state. |
| Bucket | A bucket's fields, plus its contents grouped by owning team. |
| Bucket index | Buckets across the instance. See default scoping below. |
| Item detail | One item in full, with comments and history. |
| Log | Audit events, scoped globally or to one team. |

The board and the backlog both allow several items to be selected at once. That selection is what a
bulk edit acts on ([§6.5](#65-cloning-splitting-and-bulk-edits)); it is a property of the view and is
not stored.

The bucket index defaults to **participation**: buckets my team owns, plus buckets containing at
least one of my team's tickets. Ownership alone is a secondary filter. Scoping by ownership would
show a team the buckets it created and hide the release it is working toward.

### 9.2 Filtering

Every filter is a **CSQL** query: the Campfire Simple Query Language. CSQL is the one way a filter
is expressed, by the filter bar, by a saved view ([§9.4](#94-saved-views)) and by an agent
([§14](#14-agent-interface-mcp)). The client offers two editors over it, and neither is a second
filter model: **chips**, which build the common query without typing, and **raw** mode, which
edits the text. Layout belongs to the design document.

```
assignee in (marta, me) and (bucket = SOL-6 or label = roses) and not priority = 4
```

**Fields.**

| Field | Values | Operators |
|---|---|---|
| `assignee`, `reporter` | A username; `me`, whoever is reading; `none` (assignee only). | `=` `!=` `in` `not in` |
| `label` | A label's name; `none`, carrying no label. | `=` `!=` `in` `not in` |
| `bucket` | An identifier such as `SOL-6`, or a bucket's title in quotes while one bucket has it; `none`. | `=` `!=` `in` `not in` |
| `priority` | `1` to `4`, or `P1` to `P4`. | `=` `!=` `in` `not in` `<` `<=` `>` `>=` |
| `state` | A work state's name, matched in every team. | `=` `!=` `in` `not in` |
| `team` | A short code or a team's name. | `=` `!=` `in` `not in` |
| `kind` | `ticket` or `bucket`. | `=` `!=` `in` `not in` |
| `created`, `started`, `completed` | A day: `2026-09-01`, `today`, or relative to today, `-7d`, `+2w`; `none` (started, completed). | `=` `!=` `<` `<=` `>` `>=` |
| `due` | A day, as above; `none`. | `=` `!=` `<` `<=` `>` `>=` |
| `text` | A phrase, searched in title and description. | `~` (contains) `!~` |

**Composition.** Conditions are joined by `and`, `or` and `not`, with parentheses; `and` binds
tighter than `or`. `in (a, b)` is any of them. `!=` is exactly `not =`: `assignee != marta`
includes the unassigned. Several `label = …` conditions and-ed mean every one of those labels.
Keywords and field names are case-insensitive. Any value may be quoted, and `#12` names a label,
user, bucket, team or state by its id.

**Days.** A date field compares whole days in the reader's time zone, which every request
carries; an agent that sends none reads in UTC. `created >= -7d` is the last seven days and today;
`completed = today` is today. A relative day stays relative when saved.

| Decision | Rule | Why |
|---|---|---|
| One language | Chips, raw text, saved views and agents all speak CSQL. | Small surface: a chip is a way to write a query, not a second way to filter. |
| Who parses | The client parses for suggestions and syntax errors; the server parses again and is the authority. | Typing costs no requests and errors show at once; the server still refuses anything it cannot run. |
| Names | Resolved by the server when a query is applied. A name that names nothing is refused, with the span it was at. | A typo that matched nothing in silence would read as an empty board. |
| What it compiles to | Conditions on the item's own columns and its labels. | No new filter dimension: CSQL writes the dimensions down; it is not a second data model. |
| Not in CSQL | Ordering, functions, and conditions across relations. | Board order is manual ([§9.3](#93-ordering)); the rest waits for a need. |

Requirements:

- **CSQL's own words are no one's name.** `and`, `or`, `not`, `in`, `none`, `me` and `today`, in any
  case, MUST NOT be accepted as a username, a label, a team's short code or name, or a work state's
  name: `assignee = none` must never be able to mean a user called none.
- **The client and server parsers MUST agree.** One corpus of queries, with the output or the
  error each must give, is run by both test suites; a syntax error's message and span MUST be
  identical in both.
- **A query is applied whole or not at all.** A query the server refuses MUST leave the screen on
  the last query it accepted, and MUST NOT be resent with later requests: switching a view on
  while a refused draft is in the box changes the board.
- **Typing MUST cost no requests**, and each applied change — a chip closed, a raw query entered, a
  view switched — exactly one. A request superseded by a newer one MUST be cancelled, and a late
  response MUST NOT replace a newer one.
- The values offered as suggestions — users, labels, buckets, work states, teams — are cached in
  the client and marked stale by the event kinds that can change them ([§12](#12-live-updates)).
  A stale list is refetched when it is next needed, not when the event arrives. A "too far
  behind" reset ([§12.2](#122-resume)) marks every list stale, and so does a query the server
  refuses for naming something the client offered. The cache MUST only ever shape suggestions and
  labels: the server resolves every name, so a stale cache can omit a suggestion but cannot change
  a result.

A filter may be named and saved ([§9.4](#94-saved-views)).

### 9.3 Ordering

Items are ordered within a board column by dragging. Position is stored as a **lexical rank**: a
short string ordered lexically, where an item dropped between two neighbours takes a string sorting
between theirs. A move writes one row.

Requirements:

- The rank alphabet MUST be restricted to `0-9A-Za-z`.
- The rank column MUST use binary collation.
- `UNIQUE (state_id, rank)` MUST be enforced, with retry on conflict, because two clients can
  compute the same midpoint from the same read.

### 9.4 Saved views

A **saved view** is a name attached to a CSQL query ([§9.2](#92-filtering)). It introduces no new
filter dimension and no new permission concept. In the client a view is a switch: several can be on
at once, and the views that are on are and-ed with each other and with the filter bar's query. A
team switches to *Assigned to marta* or *Label: api* during a meeting in one click. A view that is
on is applied by the server from what it stores, so it means the same to everyone who switches it
on, `me` aside.

| Decision | Rule | Why |
|---|---|---|
| Ownership | One team, set at creation. | Every surface a view is applied to is one team's board or backlog, so an instance-wide list would grow with the instance and be mostly irrelevant on any given board. |
| Creation | Any member or administrator of the owning team. | Naming a filter is not an administrative act, and requiring an administrator makes the meeting wait for one. |
| Editing and deletion | Any member or administrator of the owning team. | This is ordinary edit permission ([§6.4](#64-bucket-membership)), and a creator-based rule would strand every view whose author leaves. |
| Visibility | Every authenticated user may read and apply any saved view. | Reading is instance-wide ([§3.2](#32-out-of-scope-permanently)); a private view would be the first object in the application some authenticated users cannot see. |
| Pinning | To the board, the backlog, both, or neither. | The board and the backlog are the same items split by state ([§7.2](#72-board-and-backlog)), and a triage filter is wanted on both. |
| Ordering in the UI | Alphabetical by name, case-folded. | Manual rank exists for board columns ([§9.3](#93-ordering)), and a second ordering mechanism is not worth a drag target on a list of names. |
| A deleted label, assignee, state, bucket or team | The condition is kept, names it by `#id`, and matches nothing until someone edits it. The view names the missing reference wherever it is offered. | Dropping the condition would silently widen the view, and a wider result read as the same filter is worse than an empty one. |

A view's owning team governs where it is listed, not what it may match: a view owned by SOL may
filter on TERRA's tickets, because team is one of [§9.2](#92-filtering)'s dimensions like any other.

A saved view is deleted outright rather than archived. It holds no work, and what it used to be is
in the log.

Requirements:

- A saved view MUST store its query canonical: every label, user, bucket and team by `#id`, never
  by name, so a rename changes nothing ([Appendix B](#appendix-b--rejected-alternatives)). Beside
  it, one row per referent the query names by id.
- Every mutation of a saved view MUST write an event ([§10](#10-audit-log)): creation, rename,
  query change, pinning and deletion.
- A referent row MUST NOT be removed by cascade when its referent is deleted, and a label a view
  names MUST NOT be reaped by lazy cleanup ([§13.2](#132-implementation-rules) rule 6). A view that
  quietly repairs itself is a view nobody can trust to still mean what it says.
- Names are unique within a team, in an explicit normalised column
  ([§13.2](#132-implementation-rules) rule 4).

---

## 10. Audit log

One append-only **Event** table serves every history surface. An event records: actor, timestamp,
item or entity affected, owning team, field changed, previous value, new value.

| Surface | Filter |
|---|---|
| Item history | That item. |
| Team log | That team. |
| Global log | Unfiltered. Instance administrators only. |

Every mutation writes an event: creation, field edits, state transitions, comments, label changes,
relation changes, bucket assignment, and administrative actions such as membership and team
configuration.

There WILL NOT be a second logging mechanism, and there WILL NOT be a history table per entity type.

Requirements:

- Events MUST NOT be edited or deleted.
- The log MUST be ordered by its **monotonic sequence number**, never by timestamp. Timestamps tie.
- The log MUST be paginated by keyset, never by offset.
- Anything filterable MUST be a real column. Free-form payload is opaque and MUST NOT be queried
  into.

The sequence number is also the resume cursor for live updates ([§12.2](#122-resume)). This is safe
because the storage engine has a single writer, so events commit in sequence order and none can
become visible below a cursor already issued. That guarantee comes from
[§13](#13-deployment-and-storage) and is revisited in [Appendix
C](#appendix-c--conditions-for-changing-storage-engine).

---

## 11. API contract

The client reaches the server over an HTTP API described by a **written, versioned OpenAPI
document**, maintained in the source and treated as the definition of the interface rather than as
generated documentation.

This is required even though v1 ships one client, served by the server it talks to:

- It keeps the client from reaching into server internals during development.
- It is the entire cost of leaving a separate client buildable later
  ([§3.2](#32-out-of-scope-permanently)).

Live updates ([§12](#12-live-updates)) are **not** described by the OpenAPI document, which does not
usefully describe a long-lived stream. Their contract is [§12](#12-live-updates) and is normative in
the same way.

---

## 12. Live updates

Boards update in place as other people work.

### 12.1 Transport

The application WILL use **server-sent events**, on the same origin, under the same authentication
as the rest of the API.

The application WILL NOT use WebSocket or UDP for this. See
[Appendix B](#appendix-b--rejected-alternatives).

### 12.2 Resume

On reconnect, the browser resends the last event identifier it received. This maps directly onto the
audit log's sequence number ([§10](#10-audit-log)): the client resumes from its cursor and receives
what it missed. Cursor bookkeeping WILL NOT be hand-written on either side.

Requirements:

- **The client MUST open the stream before fetching state.** Order is: subscribe, fetch, then apply
  buffered notifications from the cursor. Fetching first leaves a window in which changes are
  silently lost.
- A cursor older than retention MUST receive an explicit "too far behind" response instructing the
  client to discard its state and refetch. A silent partial replay MUST NOT occur.

### 12.3 Payloads

Pushed messages are **thin notifications**: sequence number, event kind, item identifier, owning
team. They MUST NOT carry entity contents. The client refetches through the ordinary API.

This does three things at once: it prevents the stream disclosing data the recipient may not read,
it keeps the stream stable while entity shapes change, and it makes the refetch path the only path
that renders data.

The cost this leaves, a refetch per interested client per write, is bounded by
[§12.4](#124-the-stream-is-not-a-source-of-truth), not by carrying data in the notification.

Authorization MUST be evaluated per event at fan-out, against current permissions. It MUST NOT be
cached at subscribe time: a revoked permission must take effect on the next event, and a granted one
must not be missed.

### 12.4 The stream is not a source of truth

**Every screen MUST be correct if the stream never connects.** Live updates accelerate a refetch the
application can always perform on its own. State MUST NOT exist only in the fan-out path.

Two consequences:

- Clients MUST debounce refetches and jitter reconnections. One write to a board watched by three
  hundred people is otherwise three hundred simultaneous refetches, and a restart drops every
  connection at once.
- Clients MUST ignore event kinds they do not recognise, so that adding a kind is not a breaking
  change.

A refetch is what a notification costs, so a client pays it only where it buys something:

- **A screen refetches only for notifications that can change what it shows.** The owning team a
  notification carries is the first test. A screen that shows another team's items — a bucket's
  contents, a relation, the bucket a card belongs to — also listens for those items by identifier.
- **A hidden client refetches nothing.** What it would have refetched waits until it is shown, and
  then happens once. Left hidden past a grace the instance sets (a minute by default), it closes its
  stream and resumes from its cursor when shown ([§12.2](#122-resume)), so an idle tab holds no
  connection and loses nothing.
- **A refetch that finds nothing new costs no body.** Responses carry an ETag and are compressed; the
  client asks conditionally and keeps what it had on a 304. The ETag is a fingerprint of the content
  alone, so it discloses nothing the body would not.
- **A "too far behind" response is recovered from, not reported.** The client discards what it holds
  and refetches every screen and list. It MUST NOT require a reload.
- A screen returned to MAY be drawn at once from what the client last fetched for it, provided it is
  asked for again at once. The client keeps that in memory only, and drops it when the account
  changes.

### 12.5 Operational requirements

To be documented for anyone deploying the application:

- Response buffering must be disabled on any intervening proxy.
- Idle read timeouts must exceed the heartbeat interval. The server sends heartbeats well inside the
  common sixty-second default.
- File-descriptor limits must allow one open connection per active client. A tab hidden past the
  grace holds none.

---

## 13. Deployment and storage

The application is **one codebase, one binary, one container image, and one storage engine**. The
client is a web application embedded in and served by the server. Storage is an embedded file
database. An instance is one executable and one data file.

Backup is copying a file. Recovery is putting it back. A deployment WILL NOT require a database
administrator.

### 13.1 Delivery

The browser is the client. The application WILL NOT ship a separate client program to install,
choose between, or keep in step with a server version.

There is one binary. Separate client-only and server-only builds WILL NOT be produced; see
[Appendix B](#appendix-b--rejected-alternatives).

Two deployment profiles exist. They differ in operational care, not in architecture, and every
configuration exists in both.

| | Individual | Shared |
|---|---|---|
| Form | An executable and a data directory | The same executable, or the container image |
| Storage | One file | One file |
| Backup | Built-in snapshots | Built-in snapshots, optionally shipped off-host |
| Processes | One | One |
| Availability | Restart in seconds | Restart in seconds; recovery time equals restore time |

First run of the individual profile creates the account and a personal team, then opens the client.
The application WILL NOT show a setup wizard, and WILL NOT show controls that depend on a team the
user has not been told to create.

Two defaults are security- and integrity-bearing:

- The server MUST bind to loopback only unless explicitly configured otherwise. A binary run on a
  laptop must not silently serve the network it is attached to.
- The database file MUST live in the operating system's data directory. Network filesystems and
  cloud-synchronised folders are unsupported and MUST be documented as such; the storage engine
  corrupts on both.

### 13.2 Implementation rules

Normative. Each is inexpensive here, and together they keep
[Appendix C](#appendix-c--conditions-for-changing-storage-engine) a bounded piece of work.

1. **An invariant MUST NOT be enforced by read-then-write in application code.** Use a single atomic
   statement, a database constraint, or an optimistic version column with compare-and-swap and
   retry.
2. **Identifiers are allocated atomically.** The per-team counter is incremented and returned in one
   statement inside the inserting transaction, with `UNIQUE (team_id, number)` as a backstop.
   `SELECT max(number) + 1` followed by an insert MUST NOT be used, even though a single writer
   would forgive it: a duplicate identifier is unrecoverable once it is in URLs, branch names and
   messages. Database sequence objects MUST NOT be used; per-team sequences would mean schema
   changes at runtime, and they produce visible gaps in ticket numbers.
3. **Timestamps are generated by the application**, as epoch microseconds in a 64-bit integer.
   Database defaults and `now()` MUST NOT be used.
4. **The database never folds case.** Anything matched case-insensitively — usernames, emails,
   labels, team codes — is stored in an explicit normalised column, folded by the application at
   write time, with the unique index on that column.
5. **In-process state MUST NOT guard business behaviour.** This forbids in-memory sessions,
   in-process rate limiting, in-process caches of permission or board state, and any in-process
   mutex protecting an invariant. Sessions and rate-limit counters live in the database. This is
   what makes a restart invisible rather than a mass logout.
6. **Background jobs WILL NOT be used in v1.** Cleanup happens lazily on write.
7. **Relational structures stay relational.** Labels are a join table, not an array column.
8. **Strict column typing**, so the engine rejects a value of the wrong type rather than storing it.
9. **Keyset pagination** on every unbounded list.
10. **Live-update state MUST NOT be required for correctness**
    ([§12.4](#124-the-stream-is-not-a-source-of-truth)).

### 13.3 Durability, backup and recovery

The audience has no operator, so recovery is a feature of the application rather than a runbook.

1. **Scheduled online snapshots.** The server takes consistent snapshots of its own database at a
   configured interval, to a configured directory, retaining a configured count. It runs without
   locking, without downtime, and without an external tool. **On by default**, including for a solo
   user who has never considered backups.
2. **Optional off-host log shipping.** Continuous write-ahead log shipping to a directory, giving
   point-in-time recovery over the snapshots' retention. It runs inside the server process, as a
   library rather than a sidecar ([§13](#13-deployment-and-storage)). Off by default; one setting,
   `snapshots.replica`, to enable.
3. **Restore is a first-class command**, invoked the same way in both profiles.
4. **The restore path MUST be exercised in continuous integration.** A backup that has never been
   restored is not a backup.
5. **Integrity is checked at startup and periodically.** On detecting corruption the server MUST
   refuse to start and MUST name the snapshot to restore from.

### 13.4 Availability

One process, one node. The application WILL NOT provide failover, rolling deployment, or high
availability.

- Restart takes seconds, and sessions live in the database ([§13.2](#132-implementation-rules) rule
  5), so a restart does not log anyone out.
- Recovery from total host loss is restore time: minutes, from a scheduled snapshot.
- A team of five to fifty absorbs a few seconds of downtime during an upgrade.

Every mechanism that would improve on this requires either a concurrent-writer database or a
coordination layer, and each requires an operator this audience does not have. The conditions under
which that trade changes are in
[Appendix C](#appendix-c--conditions-for-changing-storage-engine).

### 13.5 Scale

Sizing for a large case: 10,000 registered users, roughly 3,000 active per day.

| Measure | Estimate |
|---|---|
| Writes | ~100,000 per day; under 2 per second average, tens per second at peak |
| Reads | A few per second, indexed, page-sized results |
| Storage | Under 100 GB after five years, audit log included |

An embedded database on one ordinary machine handles this with roughly two orders of magnitude to
spare. Single-node is not a scale constraint for this workload.

Sharding, read replicas, a caching tier, and a job queue WILL NOT be built, and MUST NOT be
introduced without measurements contradicting the table above.

The real risks are volume-shaped rather than user-shaped and appear at 100 users as readily as at
10,000: an unbounded "all items" query against a board holding tens of thousands of tickets, and an
unpaginated audit log. Both are addressed in [§9.2](#92-filtering) and [§10](#10-audit-log).

---

## 14. Agent interface (MCP)

The application WILL expose a **Model Context Protocol** server, so that a user's agent can do what
that user can do in the client: create and edit tickets and buckets, move items between work states,
order a backlog, manage bucket membership, labels, relations and comments, and read boards,
backlogs, buckets, items and the log.

It is a section rather than part of [§11](#11-api-contract) because its transport, its
authentication and its default are its own. It is not a second product, and it is not a second set
of rules.

### 14.1 Relationship to the HTTP API

The MCP server is **a second interface over the same handlers**, not a separate surface.

- Every tool MUST dispatch to the handler its HTTP counterpart uses, behind the same validation and
  the same permission checks ([§5.2](#52-roles)). A tool MUST NOT reach storage directly.
- Every tool MUST correspond to exactly one operation in the OpenAPI document
  ([§11](#11-api-contract)), and the tool list is derived from that correspondence over the exposed
  subset ([§14.3](#143-what-is-exposed)). A tool without an operation MUST NOT exist; it would be
  behaviour with no written contract.

Reuse of the authorisation and validation path is the entire point. A second implementation of
either would be a second set of rules to keep in step, and a permission bug would live on the
interface fewer people look at.

### 14.2 Transport and authentication

The MCP server is served by the same process, on the same origin as the client and the HTTP API, at
a fixed path, over **streamable HTTP**. There is one binary and one process
([§13](#13-deployment-and-storage)): a second port, a second process, and a stdio bridge shipped
with the application WILL NOT exist ([Appendix B](#appendix-b--rejected-alternatives)). An agent
that speaks only stdio puts its own proxy in front of the HTTP transport.

Authentication is by **agent token**: a bearer credential a user issues to themselves, names, and
revokes. Cookies are not used, because an agent has no browser and cannot perform an interactive
login.

Requirements:

- A token MUST be stored hashed, with a name, a creation time and a last-used time. It is a
  credential, not an entity; sessions are stored the same way and [§4](#4-entities) lists neither.
- A token MUST carry no permissions of its own. It names a user, and every call resolves that user's
  current permissions at the time of the call, never at issue time — the same rule live updates
  follow at fan-out ([§12.3](#123-payloads)).
- Revocation MUST be individual and MUST take effect on the next call, which it does because no
  session or permission state is held in the process ([§13.2](#132-implementation-rules) rule 5). An
  instance administrator MUST be able to revoke any token.
- The transport MUST NOT hold session state in the process (rule 5). Either it is operated
  statelessly, or a transport session identifier is a database row like any other session.

### 14.3 What is exposed

An agent may do what a **member** does ([§5.2](#52-roles)). It may not do what an administrator
does, and it may not perform an operation that is irreversible.

| Exposed | Not exposed |
|---|---|
| Create and edit tickets and buckets | Create users; issue invitations and password resets |
| Move an item between work states, with rejection reason | Create or archive teams; change membership or work states |
| Bulk edit a selection in one call ([§6.5](#65-cloning-splitting-and-bulk-edits)) | Reassign a bucket's owning team |
| Clone and split a ticket ([§6.5](#65-cloning-splitting-and-bulk-edits)) | Hard deletion of items, teams or accounts |
| Rank items on a board and in a backlog | Backup, snapshot and restore |
| Add and remove bucket membership | Issue or revoke agent tokens |
| Add and remove labels and relations | |
| Write and edit comments | |
| Create, edit and delete saved views ([§9.4](#94-saved-views)) | |
| Read boards, backlogs, buckets, items and the log | |

The log is exposed under its ordinary scoping ([§10](#10-audit-log)): the global log to instance
administrators only.

The line is drawn at operations a person performs rarely, deliberately, and out of band. Each
excluded one is either irreversible or changes who can do what, and none can be undone by writing
another event: an account exists or it does not, deletion is not a state
([§6.4](#64-bucket-membership) makes rejection the disposal route), and a restore replaces the
instance. None of them is in a member's day, so excluding them costs an agent nothing and removes
every case where a misread instruction is unrecoverable. Token issuance is excluded for a further
reason: an agent that can mint a credential outlives the one it was given.

These exclusions are a property of the interface, not of the user. An instance administrator's agent
is a member, and the same person performs the excluded operations in the client. The exclusions MUST
NOT be implemented as a second permission model ([§5.2](#52-roles)); a tool that does not exist
needs no rule.

### 14.4 Attribution

Every mutation through this interface writes an event ([§10](#10-audit-log)) with the user as actor,
and additionally records **the agent token the call arrived on**, as a real column, null for
anything done in the client. [§10](#10-audit-log) requires anything filterable to be a real column,
and "what did agents do here" is a question an instance administrator WILL ask.

- A second actor concept MUST NOT be introduced. A user is accountable for what their agent does,
  and a synthetic bot user would attribute the work to nobody
  ([Appendix B](#appendix-b--rejected-alternatives)).
- History surfaces WILL name the token beside the actor — *marta (agent: laptop)* — so that an
  agent's actions are distinguishable at a glance and filterable in the log.

### 14.5 Live updates

The MCP server is **request and response only**. It WILL NOT subscribe to the event stream, and MCP
subscriptions and resource-change notifications WILL NOT be offered.

An agent that wants to know what changed reads the audit log from its last sequence number
([§10](#10-audit-log)) — the cursor the browser already resumes from ([§12.2](#122-resume)) — which
needs no new mechanism and no per-connection state in the process
([§13.2](#132-implementation-rules) rule 5). The stream exists to keep a watched board current in a
person's field of view. An agent has no field of view, and refetching is what
[§12.4](#124-the-stream-is-not-a-source-of-truth) requires of every client anyway.

### 14.6 Enabling and defaults

**Off by default.** One setting turns it on, in the same configuration file as everything else, in
both deployment profiles. There is no edition, no separate build and no second process
([§1](#1-purpose-and-audience)).

Scheduled snapshots are on by default because they prevent data loss
([§13.3](#133-durability-backup-and-recovery)); this sits on the other side of that line, because it
widens the credential surface of an instance whose owner has not asked for an agent.

While it is disabled the path MUST NOT be served and an agent token MUST NOT authenticate anything,
so that turning the setting off is a complete answer to whether the interface is reachable.

---

## Appendix A — Storage engine

The body of this document is vendor-neutral. This appendix is not, because several rules in
[§13.2](#132-implementation-rules) depend on specific versions.

| Role | Engine | Minimum version | Required for |
|---|---|---|---|
| Storage | SQLite | 3.37 | `RETURNING` (3.35) for atomic identifier allocation; `STRICT` tables (3.37) |

This fixes the minimum version the rules assume. Features outside them MUST NOT be used.

---

## Appendix B — Rejected alternatives

Each row states what the application uses and what it does not, so that settled decisions are not
reopened without new information.

| Area | The application WILL use | It WILL NOT use | Reason |
|---|---|---|---|
| Bucket identity | The owning team's counter (`SOL-42`) | A separate global code (`REL10`) | A second namespace makes every bucket creation a naming negotiation, and blocks converting a ticket into a bucket without breaking links. |
| Bucket lifecycle | The owning team's work states | A separate `open`/`done`/`cancelled` lifecycle | One lifecycle mechanism. Board clutter is solved by a display toggle ([§6.2](#62-buckets)). |
| Ticket classification | The bucket, the title and the description | A per-team `type` field | A type is a taxonomy every team has to administer, and `SOL-96 · task` says nothing the title and the bucket have not already said. It was the one card field nobody filtered on, and removing it takes a column off the card and a page out of team settings. |
| Bucket recognition | One icon per bucket, from a fixed set | An uploaded image, or a colour per bucket | A fixed set needs no storage, no upload path and no moderation, and it stays legible at the size a card gives it. Colour is spoken for elsewhere, by state and by priority. |
| Duplicating and dividing work | Clone and split, each leaving a relation | Parent and child tickets, or subtasks | Subtasks are a second containment mechanism beside buckets ([§3.2](#32-out-of-scope-permanently)). A *follows up* edge says the same thing with the relation model that already exists, and does not make a ticket's identity depend on another ticket. |
| Acting on many items | A bulk edit of one field over a selection | A form editing several fields at once | One field is a change the actor can state before making it and reverse afterwards; a multi-field form over a selection is the one place a mis-click is both easy and invisible. The fields left out are deferred, not refused ([§3.3](#33-deferred-to-a-later-specification)). |
| Ordering | Lexical string ranks | Fractional numeric ranks | A float between two neighbours exhausts double precision after roughly fifty drops on the same midpoint, and repairing it needs a background job the application otherwise does not have. |
| Storage | One embedded file database | A database server as a second backend | The audience has no operator. A second backend costs two divergent implementations and a class of bugs that only reproduce in production, for users who do not exist. |
| Deployment | One binary | Separate local and enterprise editions | Editions diverge, and migrating between them requires a tool mapping every version pair, which is never built. |
| Delivery | One binary serving a web client | Separate client-only and server-only builds | The server already serves the client, so a client-only build would contain nothing. Network routing is the job of a proxy in front, not of a separate binary. |
| Client | A web application | A native, desktop, or mobile client | A distributed client forces a permanent public API compatibility window. A served client cannot fall out of step with its server. |
| Live updates | Server-sent events | WebSocket | The application needs server-to-client push only; every case needing full duplex — presence, typing indicators, collaborative editing — is out of scope. WebSocket would also require a second authentication path, because browsers cannot set headers on the handshake, and reconnection and resume would be hand-written. |
| Live updates | Server-sent events | UDP | Browsers cannot use it, most networks pass little but TCP/443, and it guarantees neither ordering nor delivery. |
| Sync | A single instance of record | Offline editing with later merge | Identifiers are minted atomically so that `SOL-42` is never issued twice; two disconnected clients would both mint it. Merging would also require the audit log to gain entries in its own past. |
| Workflow | Free movement between states | A transition graph with guard rules | Enforcement is the most common source of friction in trackers of this kind, and the audit log already records who moved what. |
| Filtering | One language, CSQL, with chips and raw text as two editors over it | Chips alone, or raw text alone | Chips alone cannot say or, not, or "all of these labels"; raw text alone asks everyone to learn a syntax for the common case. Two editors over one query give both without a second filter model. |
| Filtering | CSQL over the existing API | A GraphQL endpoint | GraphQL is a second API beside the one [§11](#11-api-contract) defines and every MCP tool maps to, and it brings no filter model of its own: its `where` input is the same tree CSQL already parses to. |
| Suggestion values | A client cache, marked stale by events and refetched on next use | A server cache such as Redis, or a search engine | Redis and search engines are second processes beside the one binary ([§1](#1-purpose-and-audience)), and the lists are small enough to match in memory. Free text stays a scan of titles and descriptions until one is measured slow; SQLite's trigram FTS5 is the step after, as it keeps matching inside words. |
| Saved views | A canonical query naming referents by `#id`, with a row per referent | A query stored as typed, by name | A name-based query breaks on a rename and hides whether it still references anything that exists; the id form and its rows keep both answerable. |
| Saved views | One list per team, readable by everyone | Per-user private views | Reading is instance-wide, so a private view would be the first object some authenticated users cannot see, and a view nobody else can apply does not survive the meeting it was made for. |
| Agent interface | The Model Context Protocol | A bespoke agent API | A bespoke API is the HTTP API under another name plus a client library that does not exist. Agents already speak MCP. |
| Agent interface | MCP served by the one binary | A separate MCP process or binary | A second process needs a second copy of authorisation and configuration, a second thing to keep in step with the server version, and a second deployment story. It would also be a second writer against a single-writer database. |
| Agent interface | Streamable HTTP on the same origin | A stdio transport shipped with the application | stdio means a second executable, or a subprocess per agent, and it cannot reach a remote instance ([§13.1](#131-delivery)). A generic proxy in front of the HTTP transport is the caller's to run and costs the application nothing. |
| Agent identity | The user, with the token recorded on the event | A bot or service account | An account acting as itself is bounded by nobody's permissions, and the log would credit the work to a robot rather than to the person who asked for it. |

**Reversal condition — live update transport.** If presence, typing indicators, or collaborative
editing enter scope, WebSocket becomes the correct transport. Because the stream carries thin
notifications rather than entities ([§12.3](#123-payloads)), that change replaces the transport and
nothing else.

---

## Appendix C — Conditions for changing storage engine

The trigger is **not** user count, which [§13.5](#135-scale) addresses. It is an availability
requirement this design does not meet: a deployment that cannot tolerate seconds of downtime during
an upgrade, or minutes of recovery after host loss. That implies replicas, which imply a database
accepting concurrent writers.

Because [§13.2](#132-implementation-rules) is followed, the port is bounded. Three things would need
building:

| Work | Detail |
|---|---|
| Text search | The `text` condition is a `LIKE` scan, which folds ASCII case in SQLite and not in most engines; it needs the new engine's case-insensitive match. |
| Schema | A second schema file in the new engine's dialect. |
| Live-update fan-out | A second implementation behind one interface, using the engine's own notification channel. A broker or additional service WILL NOT be needed. |

One hazard must be handled deliberately, because it cannot occur under a single writer and would
therefore pass every existing test:

> **With concurrent writers, a sequence number is not a safe resume cursor**
> ([§10](#10-audit-log), [§12.2](#122-resume)). Sequence values are allocated before commit, so a
> transaction can commit after one holding a higher number has already been read. A client resuming
> from that cursor skips the earlier event permanently.
