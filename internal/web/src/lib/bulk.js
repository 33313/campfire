// The bulk actions, shared by the selection bar and the card menu.
//
// One set of ids, one field at a time (SPEC.md §6.5). Both callers ask for the
// same thing in a different place on the screen, so they ask through here
// rather than each keeping a copy that drifts.

import { api } from "./api.js";
import { openModal, toast, failure } from "./ui.svelte.js";
import MoveDialog from "../dialogs/MoveDialog.svelte";
import BulkEditDialog from "../dialogs/BulkEditDialog.svelte";

/** Applies one field to every id. Resolves true when something was written. */
async function applyToAll(ids, field, value, reason, note) {
  try {
    const res = await api.bulkEditItems({ itemIds: ids, field, value, reason, note });
    // An item the actor may not edit is refused on its own; the rest go
    // through, and the interface reports what was refused (SPEC.md §6.5).
    if (res.refused?.length) toast(`${res.refused.length} refused: ${res.refused[0].reason}`, "alert");
    return true;
  } catch (err) {
    failure(err);
    return false;
  }
}

export async function moveMany(ids, states, only) {
  const res = await openModal(MoveDialog, { states: states.filter(only), count: ids.length });
  return res ? applyToAll(ids, "state", res.stateId, res.reason, res.note) : false;
}

export async function editMany(ids, field, options) {
  const res = await openModal(BulkEditDialog, { field, count: ids.length, options });
  return res ? applyToAll(ids, field.toLowerCase(), res.value) : false;
}
