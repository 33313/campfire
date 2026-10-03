// Creating an item is the same three steps wherever it is offered: open the
// dialog, report a failure, refresh the list in place.
//
// The new item is never opened. Creating is usually one of a run of them, and
// leaving for the detail view throws away the list, the filter and the scroll
// position the person was working in.

// A refusal never reaches here: the dialog keeps itself up and marks the field
// it was refused on, rather than closing and losing the form (DEV-5).

import { openModal, toast } from "./ui.svelte.js";
import CreateItemDialog from "../dialogs/CreateItemDialog.svelte";

export async function createItem({ team, states, kind = "ticket", reload }) {
  const made = await openModal(CreateItemDialog, { team, states, kind });
  if (!made) return null;
  toast(made.length > 1 ? `${made.length} created, ${made[0].identifier} to ${made.at(-1).identifier}`
    : `${made[0].identifier} created`);
  await reload();
  return made;
}
