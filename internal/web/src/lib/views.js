// A saved view is a named CSQL query, and a switch in the sidebar: on, it
// narrows the board or backlog, and the views that are on are and-ed with
// each other and with the bar's query (SPEC.md §9.4). Making, changing and
// deleting a view happen here, in dialogs, apart from using one.

import { api } from "./api.js";
import { app, reloadViews, toggleView } from "./state.svelte.js";
import { route, navigate } from "./router.svelte.js";
import { openModal, failure, toast } from "./ui.svelte.js";
import SaveViewDialog from "../dialogs/SaveViewDialog.svelte";
import ConfirmDialog from "../dialogs/ConfirmDialog.svelte";

/** Where a view opens from elsewhere: the backlog if it is pinned only there, else the board. */
function viewPath(view) {
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

/** Names a query, starting from the one given, and switches on the view it becomes. */
export async function saveNewView(teamId, query) {
  const view = await openModal(SaveViewDialog, {
    query, save: (v) => api.createSavedView({ teamId, ...v }),
  });
  if (!view) return false;
  await reloadViews();
  useView(view); // new, so never already on
  toast("Saved.");
  return true;
}

/** Changes a view's name, query or pins. A screen it is on redraws. */
export async function editView(view) {
  const saved = await openModal(SaveViewDialog, {
    view, save: (v) => api.updateSavedView(view.id, { teamId: view.teamId, ...v }),
  });
  if (!saved) return;
  await reloadViews();
  toast("Saved.");
}

/** Deletes a view, after asking. */
export async function deleteView(view) {
  const sure = await openModal(ConfirmDialog, {
    title: `Delete ${view.name}`,
    lines: ["It is deleted for everyone. Its items are not affected."],
  });
  if (!sure) return;
  try {
    await api.deleteSavedView(view.id);
    toggleView(view.id, false);
    await reloadViews();
    toast("Deleted.");
  } catch (err) { failure(err); }
}

/** Whether you may change the view: any member of its team, or an administrator. */
export function canEditView(view) {
  return app.user.isAdmin || app.teams.some((m) => m.teamId === view.teamId);
}
