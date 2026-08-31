// The option lists the pick and bulk-edit dialogs are handed.

import { api } from "./api.js";
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
  const page = await api.listItems({ kinds: ["bucket"] }, "", 200);
  return [{ value: 0, label: "No bucket" }].concat((page.items || []).map((b) => ({
    value: b.id, label: `${b.identifier}  ${b.title}`, icon: { bucket: b.icon },
  })));
}

export function userOptions() {
  return app.users.map((u) => ({ value: u.id, label: u.username, icon: { avatar: u.username } }));
}

/** A saved view's clauses, as the filter every list endpoint already takes. */
export function filterFromView(view) {
  const filter = {
    stateIds: [], labelIds: [], assigneeIds: [], reporterIds: [],
    bucketIds: [], priorities: [], teamIds: [], text: "",
  };
  const into = {
    state: "stateIds", label: "labelIds", assignee: "assigneeIds",
    reporter: "reporterIds", bucket: "bucketIds", priority: "priorities", team: "teamIds",
  };
  for (const c of view.clauses || []) {
    if (c.dimension === "text") filter.text = c.text;
    // Zero is a real value of the assignee dimension: unassigned.
    else if (c.dimension === "assignee" && c.refId === 0) filter.unassigned = true;
    // A clause naming something deleted keeps its identifier, which matches
    // nothing. That is the point: an empty result, never a wider one.
    else if (into[c.dimension]) filter[into[c.dimension]].push(c.refId);
  }
  return filter;
}

/** A filter, as the clauses a saved view stores. */
export function clausesFromFilter(filter) {
  const clauses = [];
  const from = {
    labelIds: "label", assigneeIds: "assignee", reporterIds: "reporter",
    bucketIds: "bucket", priorities: "priority", stateIds: "state",
  };
  for (const [key, dimension] of Object.entries(from)) {
    for (const refId of filter[key] || []) clauses.push({ dimension, refId });
  }
  if (filter.unassigned) clauses.push({ dimension: "assignee", refId: 0 });
  if (filter.text) clauses.push({ dimension: "text", text: filter.text });
  return clauses;
}
