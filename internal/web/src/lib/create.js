// Creating an item is the same three steps wherever it is offered: open the
// dialog, report a failure, refresh the list in place.
//
// The new item is never opened. Creating is usually one of a run of them, and
// leaving for the detail view throws away the list, the filter and the scroll
// position the person was working in.

import { openModal, toast, failure } from "./ui.svelte.js";
import CreateItemDialog from "../dialogs/CreateItemDialog.svelte";

export async function createItem({ team, states, kind = "ticket", reload }) {
  const res = await openModal(CreateItemDialog, { team, states, kind });
  if (!res) return null;
  if (res.error) {
    failure(res.error);
    return null;
  }
  toast(`${res.identifier} created`);
  await reload();
  return res;
}
