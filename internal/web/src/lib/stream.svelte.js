// The live-update stream.
//
// It is an accelerator, never a source of truth. Every screen fetches what it
// needs through the ordinary API and is correct if the stream never connects
// (SPEC.md §12.4); a notification only causes a refetch.

import { refreshBacklogCount } from "./state.svelte.js";

// `beat` counts the changes that have arrived, so the sidebar's fire can flare
// on each one.
export const stream = $state({ status: "connecting", beat: 0 }); // connecting | live | reconnecting | stale

let source = null;
let cursor = 0;
let attempt = 0;
let pending = null;
let listener = null;

/** The screen on screen registers its refetch here; the last one wins. */
export function onNotify(fn) {
  listener = fn;
  return () => { if (listener === fn) listener = null; };
}

// The client MUST open the stream before fetching state: subscribe, fetch,
// then apply what arrived in between (SPEC.md §12.2).
export function openStream() {
  source = new EventSource(cursor ? `/api/stream?cursor=${cursor}` : "/api/stream");

  source.onopen = () => { attempt = 0; stream.status = "live"; };

  source.onmessage = (e) => {
    let n;
    try { n = JSON.parse(e.data); } catch { return; }
    // A cursor past retention gets an explicit instruction, never a silent
    // partial replay (SPEC.md §12.2).
    if (n.reset) { stream.status = "stale"; source.close(); return; }
    if (n.seq) cursor = n.seq;
    // Unrecognised kinds are ignored, so adding one is not a breaking change
    // (SPEC.md §12.4). "hello" is the opening frame and needs no refetch.
    if (n.kind === "hello") return;
    stream.beat++;
    scheduleRefresh();
  };

  source.onerror = () => {
    source.close();
    if (stream.status === "stale") return;
    stream.status = "reconnecting";
    // Jittered reconnection: a restart drops every connection at once, and
    // three hundred clients returning together is a thundering herd.
    setTimeout(openStream, Math.min(30000, 1000 * 2 ** attempt++) * (0.5 + Math.random()));
  };
}

// One write to a board watched by three hundred people would otherwise be
// three hundred simultaneous refetches (SPEC.md §12.4).
function scheduleRefresh() {
  if (pending) return;
  pending = setTimeout(() => {
    pending = null;
    listener?.();
    refreshBacklogCount();
  }, 300 + Math.random() * 400);
}
