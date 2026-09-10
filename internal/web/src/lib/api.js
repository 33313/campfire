// The HTTP API, as described by api/openapi.yaml.
//
// One function per operation, named after its operationId, so that a route
// added to the document has an obvious counterpart here.

class ApiError extends Error {
  constructor(status, body) {
    const err = (body && body.error) || {};
    super(err.message || "something went wrong");
    this.status = status;
    this.code = err.code || "internal";
    this.field = err.field;
  }
}
export { ApiError };

async function request(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: "same-origin",
  });
  if (res.status === 204) return null;
  let parsed = null;
  const text = await res.text();
  if (text) {
    try { parsed = JSON.parse(text); } catch { parsed = null; }
  }
  if (!res.ok) throw new ApiError(res.status, parsed);
  return parsed;
}

const get = (p) => request("GET", p);
const post = (p, b) => request("POST", p, b === undefined ? {} : b);
const patch = (p, b) => request("PATCH", p, b);
const put = (p, b) => request("PUT", p, b);
const del = (p) => request("DELETE", p);

/** Turns a filter object into the repeatable query parameters of §9.2. */
export function filterQuery(filter = {}) {
  const q = new URLSearchParams();
  const many = { state: "stateIds", label: "labelIds", assignee: "assigneeIds",
    reporter: "reporterIds", bucket: "bucketIds", priority: "priorities",
    team: "teamIds", kind: "kinds" };
  for (const [param, key] of Object.entries(many)) {
    for (const value of filter[key] || []) q.append(param, value);
  }
  if (filter.unassigned) q.set("unassigned", "1");
  if (filter.text) q.set("q", filter.text);
  if (filter.viewId) q.set("view", filter.viewId);
  return q;
}

export const api = {
  // Instance and authentication
  getInstance: () => get("/api/instance"),
  bootstrap: (username, password) => post("/api/instance/bootstrap", { username, password }),
  signIn: (username, password, code) => post("/api/auth/signin", { username, password, code }),
  signOut: () => post("/api/auth/signout"),
  getSession: () => get("/api/auth/session"),
  redeemLink: (token, username, password) => post("/api/auth/redeem", { token, username, password }),
  changePassword: (current, next) => post("/api/auth/password", { current, next }),
  setupTotp: () => post("/api/auth/totp/setup"),
  enableTotp: (secret, code) => post("/api/auth/totp/enable", { secret, code }),
  disableTotp: (password) => post("/api/auth/totp/disable", { password }),

  // Users and credentials
  listUsers: (after = 0, limit = 200) => get(`/api/users?after=${after}&limit=${limit}`),
  createUser: (body) => post("/api/users", body),
  updateUser: (id, isAdmin) => patch(`/api/users/${id}`, { isAdmin }),
  createPasswordReset: (id) => post(`/api/users/${id}/reset`),
  updateProfile: (email, avatar, bio) => patch("/api/profile", { email, avatar, bio }),
  getProfile: (username) => get(`/api/users/${encodeURIComponent(username)}/profile`),
  createInvitation: () => post("/api/invitations"),
  // userId 0 asks for every token in the instance, which needs an
  // administrator; omitting it asks for your own.
  listAgentTokens: (userId) => get(`/api/agent-tokens?userId=${userId}`),
  issueAgentToken: (name) => post("/api/agent-tokens", { name }),
  revokeAgentToken: (id) => del(`/api/agent-tokens/${id}`),

  // Teams
  listTeams: (includeArchived) => get(`/api/teams${includeArchived ? "?includeArchived=1" : ""}`),
  createTeam: (name, shortCode) => post("/api/teams", { name, shortCode }),
  getTeam: (id) => get(`/api/teams/${id}`),
  updateTeam: (id, body) => patch(`/api/teams/${id}`, body),
  archiveTeam: (id, archived) => post(`/api/teams/${id}/archive`, { archived }),
  listMembers: (id) => get(`/api/teams/${id}/members`),
  setMember: (teamId, userId, role) => put(`/api/teams/${teamId}/members/${userId}`, { role }),
  removeMember: (teamId, userId) => del(`/api/teams/${teamId}/members/${userId}`),
  listWorkStates: (id) => get(`/api/teams/${id}/states`),
  createWorkState: (id, name, role) => post(`/api/teams/${id}/states`, { name, role }),
  updateWorkState: (id, body) => patch(`/api/states/${id}`, body),
  deleteWorkState: (id) => del(`/api/states/${id}`),

  // Views over items
  getBoard: (teamId, filter, buckets, limit = 100) =>
    get(`/api/teams/${teamId}/board?${withFlags(filter, buckets, limit)}`),
  getBacklog: (teamId, filter, buckets, limit = 100) =>
    get(`/api/teams/${teamId}/backlog?${withFlags(filter, buckets, limit)}`),
  listSavedViews: (teamId) => get(`/api/teams/${teamId}/views`),
  createSavedView: (body) => post("/api/views", body),
  updateSavedView: (id, body) => patch(`/api/views/${id}`, body),
  deleteSavedView: (id) => del(`/api/views/${id}`),
  listBuckets: (teamId, scope, cursor = "") =>
    get(`/api/buckets?team=${teamId}&scope=${scope}&limit=60${cursor ? `&cursor=${cursor}` : ""}`),

  // Items
  listItems: (filter, cursor = "", limit = 100) => {
    const q = filterQuery(filter);
    q.set("limit", limit);
    if (cursor) q.set("cursor", cursor);
    return get(`/api/items?${q}`);
  },
  createItem: (body) => post("/api/items", body),
  bulkEditItems: (body) => post("/api/items/bulk", body),
  getItem: (id) => get(`/api/items/${id}`),
  updateItem: (id, body) => patch(`/api/items/${id}`, body),
  deleteItem: (id) => del(`/api/items/${id}`),
  getItemImpact: (id) => get(`/api/items/${id}/impact`),
  moveItem: (id, stateId, reason, note) => post(`/api/items/${id}/move`, { stateId, reason, note }),
  rankItem: (id, stateId, afterId, beforeId) =>
    post(`/api/items/${id}/rank`, { stateId, afterId, beforeId }),
  cloneItem: (id) => post(`/api/items/${id}/clone`),
  splitItem: (id, titles, rejectSource) => post(`/api/items/${id}/split`, { titles, rejectSource }),
  reassignBucket: (id, teamId) => post(`/api/items/${id}/reassign`, { teamId }),
  getBucketContents: (id) => get(`/api/items/${id}/contents`),
  addLabel: (id, name) => post(`/api/items/${id}/labels`, { name }),
  removeLabel: (id, name) => del(`/api/items/${id}/labels/${encodeURIComponent(name)}`),
  addRelation: (id, toId, kind) => post(`/api/items/${id}/relations`, { toId, kind }),
  removeRelation: (id) => del(`/api/relations/${id}`),
  listComments: (id, after = 0) => get(`/api/items/${id}/comments?after=${after}&limit=200`),
  addComment: (id, body) => post(`/api/items/${id}/comments`, { body }),
  updateComment: (id, body) => patch(`/api/comments/${id}`, { body }),
  deleteComment: (id) => del(`/api/comments/${id}`),
  lookupItem: (identifier) => get(`/api/lookup/${encodeURIComponent(identifier)}`),
  listLabels: () => get("/api/labels"),

  // The log, and the snapshots screen
  getLog: (params) => get(`/api/log?${new URLSearchParams(params)}`),
  listSnapshots: () => get("/api/snapshots"),
  createSnapshot: () => post("/api/snapshots"),
  restoreSnapshot: (path) => post("/api/restore", { path }),
};

function withFlags(filter, buckets, limit) {
  const q = filterQuery(filter);
  if (buckets) q.set("buckets", "1");
  q.set("limit", limit);
  return q;
}
