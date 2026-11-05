// The option lists the pick and bulk-edit dialogs are handed.

import { buckets } from "./catalog.svelte.js";
import { app } from "./state.svelte.js";
import { PRIORITY } from "./icons.js";
import { pref, setPref } from "./prefs.js";

export function assigneeOptions(unassignedValue = 0) {
  return [{ value: unassignedValue, label: "Unassigned", icon: { avatar: null } }].concat(
    app.users.map((u) => ({ value: u.id, label: u.username, icon: { avatar: u.username } })));
}

export function priorityOptions() {
  return [1, 2, 3, 4].map((p) => ({ value: p, label: PRIORITY[p].name, icon: { priority: p } }));
}

// The buckets picked most often come first (DEV-92). Counted in this browser,
// so the order costs no query.
// ponytail: per browser; count tickets per bucket on the server if the order
// should follow the person from one machine to another.
const PICKS = "bucketPicks";
export const byUse = (list) => {
  const n = pref(PICKS, {});
  return [...list].sort((a, b) => (n[b.id] || 0) - (n[a.id] || 0));
};
export function usedBucket(id) {
  const n = pref(PICKS, {});
  n[id] = (n[id] || 0) + 1;
  setPref(PICKS, n);
}

// A bucket worth offering: not rejected, and not finished more than three days
// ago, as the board leaves out old done tickets (SPEC.md §7.2). Typing its
// identifier into a filter and its link still reach it.
const DONE_FOR = 3 * 24 * 60 * 60 * 1e6; // microseconds, as the server's DoneFor
export const offered = (b) => b.kind !== "bucket" || (b.stateRole !== "rejected"
  && !(b.stateRole === "final" && b.actualEnd < Date.now() * 1000 - DONE_FOR));

/** The buckets to offer; `keep` is the one a card is in, offered finished or not. */
export async function bucketOptions(keep = 0) {
  const list = (await buckets()).filter((b) => b.id === keep || offered(b));
  return [{ value: 0, label: "No bucket" }].concat(byUse(list).map((b) => ({
    value: b.id, label: `${b.identifier}  ${b.title}`, icon: { bucket: b.icon },
  })));
}

export function userOptions() {
  return app.users.map((u) => ({ value: u.id, label: u.username, icon: { avatar: u.username } }));
}
