// How an event reads. The log and an item's history render the same rows, so
// they render them through the same two helpers rather than each keeping a
// copy that drifts.

import { api } from "./api.js";
import { app } from "./state.svelte.js";
import { plural } from "./format.js";

// Bucket references in the log are ids; this is the one lookup that turns them
// into the identifier and title a reader recognises. Loaded once, shared.
const cache = $state({ buckets: new Map() });
let loading = null;

export function loadBuckets() {
  loading ||= api.listItems({ kinds: ["bucket"] }, "", 200)
    .then((page) => (cache.buckets = new Map((page.items || []).map((b) => [String(b.id), b]))))
    .catch(() => {});
  return loading;
}

// The events that carry no before and no after: what happened is the fact,
// and a dotted kind on screen is the log talking to itself.
const DESCRIBE = {
  "comment.created": "added a comment", "comment.updated": "edited a comment",
  "comment.deleted": "deleted a comment", "item.created": "created", "item.deleted": "deleted",
};

// The log stores a field under its column's name. These are the ones that
// name is not already a word for; the rest only lose their underscores.
const FIELDS = {
  plannedEnd: "planned end", is_admin: "administrator", totp: "second factor",
  icon_rendering: "icon style", pinning: "pinned to", clauses: "filter",
  split_from: "split from", cloned_from: "cloned from",
  invite_issued: "invitation", reset_issued: "password reset",
  agent_token_issued: "agent token", agent_token_revoked: "agent token",
};

/** A stored field name as a reader says it. */
export function fieldLabel(field) {
  return FIELDS[field] || field.replaceAll("_", " ");
}

// What a row is about when it is not the item in its item column.
const ENTITIES = { team: "team", work_state: "work state", saved_view: "view", user: "user" };

/** What changed, rather than the event kind's dotted name. */
export function fieldName(e) {
  const [entity, verb] = (e.kind || "").split(".");
  if (entity === "work_state") return verb === "updated" ? "work state" : `work state ${verb}`;
  if (e.field) return fieldLabel(e.field);
  if (entity === "comment") return "comment";
  if (verb === "created" || verb === "deleted") {
    return ENTITIES[entity] ? `${ENTITIES[entity]} ${verb}` : verb;
  }
  return (e.kind || "").replaceAll(/[._]/g, " ");
}

// Identity-bearing fields are stored as the id they reference — a real column,
// per SPEC.md §10 — so both screens resolve them for display.
export function eventValue(e, raw) {
  if (e.field === "assignee" || e.field === "reporter") {
    if (raw === "0") return "unassigned";
    return app.users.find((u) => String(u.id) === raw)?.username || raw;
  }
  if (e.field === "bucket") {
    if (raw === "0") return "no bucket";
    const b = cache.buckets.get(raw);
    return b ? `${b.identifier} ${b.title}` : raw;
  }
  if (e.field === "team") return app.allTeams.find((t) => String(t.id) === raw)?.name || raw;
  return raw;
}

// A work state's change is stored as "<name> <role>", the role empty for an
// intermediate state; a reorder as "<name> <position>".
function stateValue(v) {
  const i = v.lastIndexOf(" ");
  const [name, role] = [v.slice(0, i), v.slice(i + 1)];
  return role ? `${name} (${role})` : name;
}

// The rows a before and an after would say wrongly: a flag stored as
// true/false, a value that only makes sense with its verb, an id.
const PHRASE = {
  is_admin: (e) => e.newValue === "true" ? "made an administrator" : "no longer an administrator",
  archived: (e) => e.newValue === "true" ? "archived" : "restored",
  totp: (e) => e.newValue ? "turned on" : "turned off",
  password: (e) => e.newValue === "reset" ? "reset with a link" : "changed",
  profile: () => "updated",
  label: (e) => e.newValue ? `added ${e.newValue}` : `removed ${e.oldValue}`,
  clauses: (e) => plural(+e.newValue, "clause"),
  split_from: (e) => e.oldValue,
  cloned_from: (e) => e.oldValue,
  invite_issued: () => "issued",
  reset_issued: (e) => `issued for ${e.newValue}`,
  agent_token_issued: (e) => `issued ${e.newValue}`,
  agent_token_revoked: (e) => `revoked ${e.oldValue}`,
  order: (e) => {
    const [from, to] = [e.oldValue, e.newValue].map((v) => v.slice(v.lastIndexOf(" ") + 1));
    return `${e.newValue.slice(0, e.newValue.lastIndexOf(" "))}: position ${from} → ${to}`;
  },
};

/** The change itself, as one phrase: old → new where both are known.
 *
 * A creation has no previous value, and a description change has neither — the
 * prose is not worth two copies in the log (SPEC.md §10). Neither is a dash
 * followed by the new value: an em dash standing for "there was nothing here
 * before" is noise on every row that records something beginning.
 */
export function eventChange(e) {
  if (PHRASE[e.field]) return PHRASE[e.field](e);
  if (e.kind === "work_state.updated") return `${stateValue(e.oldValue)} → ${stateValue(e.newValue)}`;
  // What was deleted is named, not "removed": the field column already says
  // it was deleted.
  if (e.kind?.endsWith(".deleted") && e.oldValue) return e.oldValue;
  const before = e.oldValue ? eventValue(e, e.oldValue) : "";
  const after = e.newValue ? eventValue(e, e.newValue) : "";
  if (before && after) return `${before} → ${after}`;
  if (after) return after;
  if (before) return `${before} removed`;
  return DESCRIBE[e.kind] || `${fieldName(e)} updated`;
}
