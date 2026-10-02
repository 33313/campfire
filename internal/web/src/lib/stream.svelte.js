// The live-update stream.
//
// It is an accelerator, never a source of truth. Every screen fetches what it
// needs through the ordinary API and is correct if the stream never connects
// (SPEC.md §12.4); a notification only causes a refetch, and only for those
// who show something it can change.

import { notice, staleAll } from "./catalog.svelte.js";

// `beat` counts the changes that have arrived, so the sidebar's fire can flare
// on each one.
export const stream = $state({ status: "connecting", beat: 0 }); // connecting | live | reconnecting | asleep

let source = null;
let started = false;
let cursor = 0;
let attempt = 0;
let retry = null;  // the reconnection waiting out its backoff
let sleep = null;  // closes the stream of a tab left hidden
let grace = 60000; // how long that is; the instance says (getInstance)
let batch = [];    // what arrived since the last refetch
let pending = null;
const subscribers = new Set();

/**
 * Calls `fn` after notifications that can change what the caller shows, as
 * `wants(notification)` decides; with no `wants`, after every one. A batch of
 * them calls it once. Returns the function that stops it.
 */
export function onNotify(fn, wants = () => true) {
  const s = { fn, wants, due: false };
  subscribers.add(s);
  return () => subscribers.delete(s);
}

/**
 * Whether a notification can change a screen showing the items `ids`
 * (identifiers), drawn in team `team`'s states. Item events count when the
 * item is on screen, and a relation always: one made from elsewhere names the
 * item it starts at, not the one on screen. Anything else counts when it is
 * the team's or nobody's — a work state, the team, a user.
 */
export function touches(n, ids, team = 0) {
  if (n.kind.startsWith("item.relation_")) return true;
  if (/^(item|comment)\./.test(n.kind)) return ids.has(n.identifier);
  return !n.teamId || n.teamId === team;
}

export function configure(hiddenTabGrace) {
  if (hiddenTabGrace >= 0) grace = hiddenTabGrace;
}

// The client MUST open the stream before fetching state: subscribe, fetch,
// then apply what arrived in between (SPEC.md §12.2).
export function openStream() {
  started = true;
  clearTimeout(retry);
  source?.close();
  source = new EventSource(cursor ? `/api/stream?cursor=${cursor}` : "/api/stream");

  source.onopen = () => { attempt = 0; stream.status = "live"; };

  source.onmessage = (e) => {
    let n;
    try { n = JSON.parse(e.data); } catch { return; }
    // A cursor past retention gets an explicit instruction, never a silent
    // partial replay (SPEC.md §12.2): everything is refetched. The stream
    // goes on from now, or reconnects, on its own.
    if (n.reset) {
      staleAll();
      for (const s of subscribers) s.due = true;
      schedule();
      return;
    }
    if (n.seq) cursor = n.seq;
    // Unrecognised kinds are ignored, so adding one is not a breaking change
    // (SPEC.md §12.4). "hello" is the opening frame and needs no refetch.
    if (n.kind === "hello" || !n.kind) return;
    notice(n.kind);
    stream.beat++;
    batch.push(n);
    schedule();
  };

  source.onerror = () => {
    source.close();
    source = null;
    stream.status = "reconnecting";
    // Jittered reconnection: a restart drops every connection at once, and
    // three hundred clients returning together is a thundering herd.
    retry = setTimeout(openStream, Math.min(30000, 1000 * 2 ** attempt++) * (0.5 + Math.random()));
  };
}

// One write to a board watched by three hundred people would otherwise be
// three hundred simultaneous refetches (SPEC.md §12.4).
function schedule() {
  if (!pending) pending = setTimeout(flush, 300 + Math.random() * 400);
}

function flush() {
  pending = null;
  for (const s of subscribers) if (!s.due && batch.some(s.wants)) s.due = true;
  batch = [];
  if (!document.hidden) run();
}

function run() {
  for (const s of subscribers) if (s.due) { s.due = false; s.fn(); }
}

// A tab nobody is looking at refetches nothing: what it would have refetched
// waits until it is shown. Left hidden past the grace, it closes its stream
// too, and resumes from its cursor when shown, so the gap is replayed rather
// than lost (SPEC.md §12.2) and an idle tab holds no connection.
document.addEventListener("visibilitychange", () => {
  if (!started) return;
  clearTimeout(sleep);
  if (document.hidden) {
    sleep = setTimeout(() => {
      clearTimeout(retry);
      source?.close();
      source = null;
      stream.status = "asleep";
    }, grace);
    return;
  }
  if (!source) wake();
  run();
});

// Back online, a stream waiting out its backoff reconnects now.
addEventListener("online", () => { if (started && !source && !document.hidden) wake(); });

function wake() {
  attempt = 0;
  stream.status = "connecting";
  openStream();
}
