// The context every screen reads: who you are, the teams you can see, and the
// per-team lists the filter chips resolve names against.

import { api } from "./api.js";

export const app = $state({
  user: null,
  teams: [],          // memberships, for the sidebar
  allTeams: [],
  team: null,         // the team in context
  views: [],
  viewsOn: [],        // ids of the views switched on, for this team
  labels: [],
  users: [],
  backlogCount: 0,
});

export async function reloadContext(teamId) {
  const [session, teams] = await Promise.all([api.getSession(), api.listTeams()]);
  app.user = session.user;
  app.teams = session.teams || [];
  app.allTeams = teams || [];
  const chosen = teamId || app.teams[0]?.teamId || app.allTeams[0]?.id;
  app.team = app.allTeams.find((t) => t.id === chosen) || null;
  if (!app.team) return;
  const [views, labels, users] = await Promise.all([
    api.listSavedViews(app.team.id), api.listLabels(), api.listUsers(), refreshBacklogCount(),
  ]);
  app.views = views || [];
  app.viewsOn = stored(app.team.id);
  app.labels = labels || [];
  app.users = users || [];
}

// The sidebar's backlog count. It is not any one screen's: the sidebar
// refreshes it on its team's notifications; read only on boot, it went stale
// with the first ticket created or moved.
export async function refreshBacklogCount() {
  const id = app.team?.id;
  if (!id) return;
  const backlog = await api.getBacklog(id, { csql: "kind = ticket" }, "", 1).catch(() => null);
  // A team switch while this was in flight has already counted the new team.
  if (app.team?.id === id) app.backlogCount = backlog ? backlog.count : 0;
}

export async function reloadViews() {
  app.views = (await api.listSavedViews(app.team.id)) || [];
}

// Which views are on is a preference of this browser, per team, like the
// board's toggles: it decides what is drawn, never what is true.
function stored(teamId) {
  try { return JSON.parse(localStorage.getItem(`views:${teamId}`)) || []; } catch { return []; }
}

/** The views switched on, in the sidebar's order. A deleted one drops out. */
export function viewsOn() {
  return app.views.filter((v) => app.viewsOn.includes(v.id));
}

/** Switches a view on or off; with no `on`, flips it. */
export function toggleView(id, on = !app.viewsOn.includes(id)) {
  app.viewsOn = on ? [...new Set([...app.viewsOn, id])] : app.viewsOn.filter((x) => x !== id);
  try { localStorage.setItem(`views:${app.team.id}`, JSON.stringify(app.viewsOn)); } catch { /* private window */ }
}

export function userName(id) {
  return app.users.find((u) => u.id === id)?.username || `user ${id}`;
}

export function labelName(id) {
  return app.labels.find((l) => l.id === id)?.name || `label ${id}`;
}
