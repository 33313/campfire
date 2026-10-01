// A saved view is a switch in the sidebar: on, it stacks its filter onto the
// board or backlog, and several can be on at once (SPEC.md §9.4). Making,
// changing and deleting a view happen here, in dialogs, apart from using one.

import { api } from "./api.js";
import { app, reloadViews, toggleView } from "./state.svelte.js";
import { route, navigate } from "./router.svelte.js";
import { openModal, failure, toast } from "./ui.svelte.js";
import { clausesFromFilter } from "./options.js";
import SaveViewDialog from "../dialogs/SaveViewDialog.svelte";
import ConfirmDialog from "../dialogs/ConfirmDialog.svelte";

/** Where a view opens from elsewhere: the backlog if it is pinned only there, else the board. */
export function viewPath(view) {
  return `/t/${view.teamId}/${view.pinBacklog && !view.pinBoard ? "backlog" : "board"}`;
}

/** Whether the board or backlog is on screen, where a view takes effect. */
export const onScreen = () => /^\/t\/\d+\/(board|backlog)$/.test(route.path);

/**
 * The sidebar's click: on the board or backlog it flips the view; from
 * anywhere else it switches it on and goes to where it is pinned.
 */
export function useView(view) {
  if (onScreen()) return toggleView(view.id);
  toggleView(view.id, true);
  navigate(viewPath(view));
}

/** Names a filter, starting from the one given, and switches on the view it becomes. */
export async function saveNewView(teamId, filter) {
  const res = await openModal(SaveViewDialog, { filter });
  if (!res) return false;
  try {
    const view = await api.createSavedView({ teamId, ...stored(res) });
    await reloadViews();
    useView(view); // new, so never already on
    toast("Saved.");
    return true;
  } catch (err) { failure(err); return false; }
}

/** Changes a view's name, filter or pins. A screen it is on redraws. */
export async function editView(view) {
  const res = await openModal(SaveViewDialog, { view });
  if (!res) return;
  try {
    await api.updateSavedView(view.id, { teamId: view.teamId, ...stored(res) });
    await reloadViews();
    toast("Saved.");
  } catch (err) { failure(err); }
}

/** Deletes a view, after asking. */
export async function deleteView(view) {
  const sure = await openModal(ConfirmDialog, {
    title: `Delete ${view.name}`,
    lines: ["The view goes for everyone. The items it shows are not touched."],
  });
  if (!sure) return;
  try {
    await api.deleteSavedView(view.id);
    toggleView(view.id, false);
    await reloadViews();
    toast("Deleted.");
  } catch (err) { failure(err); }
}

// The dialog's answer as the API stores it.
const stored = ({ name, pinBoard, pinBacklog, filter }) =>
  ({ name, pinBoard, pinBacklog, clauses: clausesFromFilter(filter) });

/** Whether you may change the view: any member of its team, or an administrator. */
export function canEditView(view) {
  return app.user.isAdmin || app.teams.some((m) => m.teamId === view.teamId);
}
