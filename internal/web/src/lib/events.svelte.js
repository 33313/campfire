// How an event reads. The log and an item's history render the same rows, so
// they render them through the same two helpers rather than each keeping a
// copy that drifts.

import { api } from "./api.js";
import { app } from "./state.svelte.js";

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

/** What changed, rather than the event kind's dotted name. */
export function fieldName(e) {
  if (e.field) return e.field.replaceAll("_", " ");
  const [entity, verb] = (e.kind || "").split(".");
  if (entity === "comment") return "comment";
  if (verb === "created" || verb === "deleted") return verb;
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
  return raw;
}
