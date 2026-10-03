// The option lists the pick and bulk-edit dialogs are handed.

import { buckets } from "./catalog.svelte.js";
import { app } from "./state.svelte.js";
import { PRIORITY } from "./icons.js";

export function assigneeOptions(unassignedValue = 0) {
  return [{ value: unassignedValue, label: "Unassigned", icon: { avatar: null } }].concat(
    app.users.map((u) => ({ value: u.id, label: u.username, icon: { avatar: u.username } })));
}

export function priorityOptions() {
  return [1, 2, 3, 4].map((p) => ({ value: p, label: PRIORITY[p].name, icon: { priority: p } }));
}

export async function bucketOptions() {
  return [{ value: 0, label: "No bucket" }].concat((await buckets()).map((b) => ({
    value: b.id, label: `${b.identifier}  ${b.title}`, icon: { bucket: b.icon },
  })));
}

export function userOptions() {
  return app.users.map((u) => ({ value: u.id, label: u.username, icon: { avatar: u.username } }));
}
