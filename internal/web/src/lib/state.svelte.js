// The context every screen reads: who you are, the teams you can see, and the
// per-team lists the filter chips resolve names against.

import { api } from "./api.js";
import { pref, setPref } from "./prefs.js";

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
  imageMaxSize: 0,    // the largest picture the instance takes, in bytes
  // What the bucket pickers offer (DEV-79); hidden keys stay valid.
  appearance: { colourMode: "palette", hiddenGroups: [], hiddenColours: [] },
  iconSets: [],       // the instance's own bucket icons
  customIcons: {},    // ...by key: { viewBox, body }
});

export function setIconSets(sets) {
  app.iconSets = sets;
  app.customIcons = Object.fromEntries(sets.flatMap((s) => s.icons.map((i) => [i.key, i])));
}

export async function reloadContext(teamId) {
  const [session, teams] = await Promise.all([api.getSession(), api.listTeams()]);
  app.user = session.user;
  app.teams = session.teams || [];
  app.allTeams = teams || [];
  // The team asked for, else the one this browser last had in context.
  const known = (id) => app.allTeams.find((t) => t.id === id);
  app.team = known(teamId) || known(pref("team")) || known(app.teams[0]?.teamId)
    || app.allTeams[0] || null;
  if (!app.team) return;
  setPref("team", app.team.id);
  const [views, labels, users] = await Promise.all([
    api.listSavedViews(app.team.id), api.listLabels(), api.listUsers(), refreshBacklogCount(),
  ]);
  app.views = views || [];
  app.viewsOn = stored(app.team.id);
  app.labels = labels || [];
  app.users = users || [];
}

/**
 * Puts a team in context for a screen that belongs to it, so the sidebar's
 * Board and Backlog lead to the team the screen is about. Only a known team:
 * asking for one the instance does not have would ask again forever.
 */
export function followTeam(id) {
  if (id && app.user && id !== app.team?.id && app.allTeams.some((t) => t.id === id)) {
    reloadContext(id);
  }
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
  return pref(`views:${teamId}`, []);
}

/** The views switched on, in the sidebar's order. A deleted one drops out. */
export function viewsOn() {
  return app.views.filter((v) => app.viewsOn.includes(v.id));
}

/** Switches a view on or off; with no `on`, flips it. */
export function toggleView(id, on = !app.viewsOn.includes(id)) {
  app.viewsOn = on ? [...new Set([...app.viewsOn, id])] : app.viewsOn.filter((x) => x !== id);
  setPref(`views:${app.team.id}`, app.viewsOn);
}

