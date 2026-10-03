// The values a filter is written with — users, labels, buckets, work states,
// teams — kept in the client for suggestions and chips (SPEC.md §9.2).
//
// The rule that keeps it honest: an event that can change a list marks it
// stale, and a stale list is refetched the next time something needs it, not
// when the event arrives. A busy board costs nothing until someone opens a
// chip. A "too far behind" reset, or a query the server refuses for a name we
// offered, marks every list stale. And the server resolves every name itself,
// so a stale list can only leave out a suggestion, never change a result.

import { api } from "./api.js";
import { app } from "./state.svelte.js";
import { PRIORITY } from "./icons.js";
import { quote } from "./csql.js";

export const lists = $state({ buckets: null, states: null });

const stale = new Set(["buckets", "states"]);
const inflight = {};
let statesOf = null; // the team the states are of: another team's are stale

// Which lists an event kind can change. A notification does not say whether
// an item is a bucket, so every item event marks the buckets.
const MARKS = {
  "item.created": ["buckets"], "item.updated": ["buckets"], "item.deleted": ["buckets"],
  "item.label_added": ["labels"], "item.label_removed": ["labels"],
  "work_state.created": ["states"], "work_state.updated": ["states"], "work_state.deleted": ["states"],
  "team.created": ["teams"], "team.updated": ["teams"], "team.archived": ["teams"],
  "user.created": ["users"], "user.updated": ["users"],
};

/** The stream's notice of an event: the lists it can change are stale. */
export function notice(kind) {
  for (const list of MARKS[kind] || []) stale.add(list);
}

/** Everything is stale: after a reset, or a name the server did not know. */
export function staleAll() {
  for (const list of ["users", "labels", "teams", "buckets", "states"]) stale.add(list);
}

const FETCH = {
  users: async () => { app.users = (await api.listUsers()) || []; },
  labels: async () => { app.labels = (await api.listLabels()) || []; },
  teams: async () => { app.allTeams = (await api.listTeams()) || []; },
  buckets: async () => { lists.buckets = (await api.listItems({ csql: "kind = bucket" }, "", 200)).items || []; },
  states: async () => {
    statesOf = app.team?.id;
    lists.states = statesOf ? (await api.listWorkStates(statesOf)) || [] : [];
  },
};
// ponytail: buckets come in one page of 200; past that, switch the bucket
// list to a prefix search on the server.

const LIST = { assignee: "users", reporter: "users", label: "labels", bucket: "buckets",
  state: "states", team: "teams" };

/** Makes sure what a field's values come from is fresh. Call from a handler or an effect. */
export function ensure(field) {
  const list = LIST[field];
  if (list === "states" && statesOf !== app.team?.id) stale.add("states");
  if (!list || !stale.has(list) || inflight[list]) return;
  stale.delete(list);
  inflight[list] = FETCH[list]().catch(() => stale.add(list)).finally(() => delete inflight[list]);
}

/**
 * The values a field offers, as the tokens a query writes them with: a
 * label to show, and a note. Reads only; `ensure` is what fetches.
 */
export function values(field) {
  switch (field) {
    case "assignee": case "reporter":
      return [{ token: "me", label: "me" },
        ...(field === "assignee" ? [{ token: "none", label: "Unassigned" }] : []),
        ...app.users.map((u) => tok(u.username))];
    case "label":
      return [{ token: "none", label: "No labels" }, ...app.labels.map((l) => tok(l.name))];
    case "bucket":
      return [{ token: "none", label: "No bucket" },
        ...(lists.buckets || []).map((b) => ({ token: b.identifier, label: b.identifier, note: b.title }))];
    case "priority":
      return [1, 2, 3, 4].map((p) => ({ token: String(p), label: PRIORITY[p].name }));
    case "state":
      return [...new Set((lists.states || []).map((s) => s.name))].map(tok);
    case "kind":
      return [{ token: "ticket", label: "Tickets" }, { token: "bucket", label: "Buckets" }];
    case "team":
      return app.allTeams.map((t) => ({ token: t.shortCode, label: t.shortCode, note: t.name }));
  }
  return [];
}

const tok = (name) => ({ token: quote(name), label: name });

/** How a chip shows one of its tokens. */
export function labelOf(field, token) {
  const v = values(field).find((x) => x.token === token);
  if (v) return field === "priority" ? `P${token}` : v.label;
  return token.startsWith('"') ? JSON.parse(token) : token;
}
