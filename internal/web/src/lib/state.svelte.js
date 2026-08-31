// The context every screen reads: who you are, the teams you can see, and the
// per-team lists the filter chips resolve names against.

import { api } from "./api.js";

export const app = $state({
  user: null,
  teams: [],          // memberships, for the sidebar
  allTeams: [],
  team: null,         // the team in context
  views: [],
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
  const [views, labels, users, backlog] = await Promise.all([
    api.listSavedViews(app.team.id), api.listLabels(), api.listUsers(),
    api.getBacklog(app.team.id, {}, false, 1).catch(() => null),
  ]);
  app.views = views || [];
  app.labels = labels || [];
  app.users = users || [];
  app.backlogCount = backlog ? backlog.count : 0;
}

export function userName(id) {
  return app.users.find((u) => u.id === id)?.username || `user ${id}`;
}

export function labelName(id) {
  return app.labels.find((l) => l.id === id)?.name || `label ${id}`;
}
