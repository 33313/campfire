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
    this.span = err.span; // where in a query it went wrong
  }
}
export { ApiError };

// What each GET last returned, by address: its ETag, so that asking again
// costs a 304 and no body when nothing changed, and its text, so that a screen
// you come back to is drawn at once from what it showed, then asked again
// (SPEC.md §12.4). In memory only: it goes with the tab, and with the account.
const memo = new Map();
const MEMO = 64;
let peeking = false;

async function request(method, path, body, signal) {
  const kept = method === "GET" ? memo.get(path) : null;
  const headers = body === undefined ? {} : { "Content-Type": "application/json" };
  if (kept) headers["If-None-Match"] = kept.tag;
  const res = await fetch(path, {
    method,
    signal, // a superseded request is cancelled, not left to land late
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: "same-origin",
  });
  if (res.status === 304 && kept) {
    remember(path, kept);
    return JSON.parse(kept.text); // a fresh copy: screens change what they are given
  }
  if (res.status === 204) return null;
  let parsed = null;
  const text = await res.text();
  if (text) {
    try { parsed = JSON.parse(text); } catch { parsed = null; }
  }
  if (!res.ok) {
    memo.delete(path); // gone, or no longer yours to see: not drawn from memory either
    throw new ApiError(res.status, parsed);
  }
  const tag = res.headers.get("ETag");
  if (tag && method === "GET") remember(path, { tag, text });
  return parsed;
}

function remember(path, entry) {
  memo.delete(path); // the newest goes to the back; the oldest is let go
  memo.set(path, entry);
  if (memo.size > MEMO) memo.delete(memo.keys().next().value);
}

/** What `call` would fetch, from memory and at once, or undefined if it was
    never fetched. `call` is an api function, e.g. () => api.getTeam(1). */
export function peek(call) {
  peeking = true;
  try { return call(); } finally { peeking = false; }
}

/** Lets go of everything remembered: another account must not see it. */
export function forget() { memo.clear(); }

const get = (p, signal) => {
  if (!peeking) return request("GET", p, undefined, signal);
  const kept = memo.get(p);
  return kept ? JSON.parse(kept.text) : undefined;
};
const post = (p, b) => request("POST", p, b === undefined ? {} : b);
const patch = (p, b) => request("PATCH", p, b);
const put = (p, b) => request("PUT", p, b);
const del = (p) => request("DELETE", p);

/** A filter's query parameters (SPEC.md §9.2): a CSQL query in csql, the
    views switched on, and the reader's time zone, plus the paging asked for. */
function filterQuery(filter = {}, { cursor, limit } = {}) {
  const q = new URLSearchParams();
  if (filter.csql?.trim()) q.set("csql", filter.csql);
  for (const id of filter.viewIds || []) q.append("view", id);
  if (filter.tz) q.set("tz", filter.tz); // days in a query are the reader's
  if (cursor) q.set("cursor", cursor);
  if (limit) q.set("limit", limit);
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
  getBoard: (teamId, filter, allDone, limit = 100, signal) => {
    const q = filterQuery(filter, { limit });
    if (allDone) q.set("allDone", "1");
    return get(`/api/teams/${teamId}/board?${q}`, signal);
  },
  getBacklog: (teamId, filter, cursor = "", limit = 100, signal) =>
    get(`/api/teams/${teamId}/backlog?${filterQuery(filter, { cursor, limit })}`, signal),
  listSavedViews: (teamId) => get(`/api/teams/${teamId}/views`),
  createSavedView: (body) => post("/api/views", body),
  updateSavedView: (id, body) => patch(`/api/views/${id}`, body),
  deleteSavedView: (id) => del(`/api/views/${id}`),
  listBuckets: (teamId, scope, cursor = "") =>
    get(`/api/buckets?team=${teamId}&scope=${scope}&limit=60${cursor ? `&cursor=${cursor}` : ""}`),

  // Items
  listItems: (filter, cursor = "", limit = 100) => get(`/api/items?${filterQuery(filter, { cursor, limit })}`),
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
