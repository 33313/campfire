<script>
  // The board shows every state except the default one and the rejected one;
  // the backlog shows the default state. Rejected items reach neither. The two
  // share their filter bar, their selection and their bulk actions, because
  // they are one view split by state (SPEC.md §7.2).

  import { untrack, tick } from "svelte";
  import { flip } from "svelte/animate";
  import { crossfade, fade } from "svelte/transition";

  import { api, peek } from "../lib/api.js";
  import { latest } from "../lib/latest.js";
  import { app, viewsOn } from "../lib/state.svelte.js";
  import { onNotify, touches } from "../lib/stream.svelte.js";
  import { failure } from "../lib/ui.svelte.js";
  import { createItem } from "../lib/create.js";
    import { NAV } from "../lib/icons.js";
  import { ms, easeOut, QUICK, GLIDE } from "../lib/motion.js";
  import { kindle } from "../lib/embers.js";
  import { lift } from "../lib/ghost.js";

  import Icon from "../components/Icon.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import SearchBox from "../components/SearchBox.svelte";
  import FilterBar from "../components/FilterBar.svelte";
  import SelectionBar from "../components/SelectionBar.svelte";
  import StatePip from "../components/StatePip.svelte";
  import Card from "../components/Card.svelte";
  import CardMenu from "../components/CardMenu.svelte";
  import ItemPreview from "../components/ItemPreview.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";

  let { teamId } = $props();

  let team = $state(null);
  let states = $state([]);
  let board = $state(null);
  let error = $state(null);
  let selected = $state([]);
  let focused = $state(null); // { item, comments } for the preview aside
  let menu = $state(null);    // { item, ids, x, y } for the right-click menu
  let drag = $state(null);   // { ids, x, y, live, target } while a card is held
  let landing = $state([]);  // cards hidden under the held copy until it lands on them
  let ghost = null;          // the held copy (ghost.js)
  let held = null;           // the card it was picked up from

  // A view preference, private to this browser: it decides what is drawn,
  // never what is true.
  let key = $derived(`buckets:${teamId}:board`);
  let showBuckets = $state(untrack(() => localStorage.getItem(key) === "1"));
  // Items done for more than three days are hidden unless this is on
  // (SPEC.md §7.2); a long Done column is history, not work.
  let doneKey = $derived(`done:${teamId}:board`);
  const TZ = Intl.DateTimeFormat().resolvedOptions().timeZone;
  let allDone = $state(untrack(() => localStorage.getItem(doneKey) === "1"));

  // What the board shows: the filter bar's query and the header's search, as
  // last accepted, and-ed with the views switched on in the sidebar, which go
  // by id so the server applies what each one stores (SPEC.md §9.4). A query
  // being tried is not applied until the server accepts it, so a refused one
  // never rides along with the next request (SPEC.md §9.2).
  let applied = $state({ csql: "", text: "" });
  const ask = latest();
  let drawn = ""; // what the board on screen was asked with
  const asking = (filter) => JSON.stringify([filter, viewsOn().map((v) => [v.id, v.query]), showBuckets, allDone]);

  // Anything that changes what to ask redraws: a view switched or edited, a
  // toggle. Once, however it changed.
  $effect(() => {
    const now = asking(applied);
    untrack(() => { if (now !== drawn) load(); });
  });

  // A card that changes column travels there, whoever moved it: a drop, a bulk
  // move, or a teammate's change arriving on the stream. One that arrives from
  // nowhere, or leaves for nowhere, fades.
  const [send, receive] = crossfade({
    duration: () => ms(GLIDE), easing: easeOut,
    fallback: (node, p) => fade(node, { duration: p.instant ? 0 : ms(QUICK) }),
  });

  // A card that was open work a moment ago and is now in the final state
  // feeds the fire once it has landed (embers.js). Only one that was on the
  // board before: a filter that brings finished work into view has finished
  // nothing. Each comes with the column it is now in, because that is where to
  // look for it — its old card may still be fading out of another.
  function finished(before, after) {
    const open = new Set(before.columns.filter((c) => c.state.role !== "final")
      .flatMap((c) => (c.items || []).map((i) => i.id)));
    return after.columns.filter((c) => c.state.role === "final")
      .flatMap((c) => (c.items || []).filter((i) => open.has(i.id)).map((i) => [i.id, c.state.id]));
  }

  const fetchBoard = (filter, signal) => api.getBoard(teamId,
    { ...filter, viewIds: viewsOn().map((v) => v.id), tz: TZ }, showBuckets, allDone, 100, signal);

  // Coming back to the board draws it at once, as it was last shown, and asks
  // again behind it: a 304 when nothing changed, and when something did, the
  // cards travel to where they are now (SPEC.md §12.4). The team and its
  // states are still asked for once a visit.
  let known = false;
  {
    const was = [peek(() => api.getTeam(teamId)), peek(() => api.listWorkStates(teamId)),
      peek(() => fetchBoard(applied))];
    if (was.every(Boolean)) {
      team = was[0];
      states = was[1];
      board = was[2];
    }
  }

  /** Asks for the board with a filter, the applied one unless another is being
      tried. Resolves to what became of it: { ok }, { error } for a query the
      server refused, or { superseded } when a newer request took its place. */
  async function load(filter = applied) {
    const key = asking(filter);
    const shown = board;
    const res = await ask(async (signal) => {
      if (!known) [team, states] = await Promise.all([api.getTeam(teamId), api.listWorkStates(teamId)]);
      known = true;
      const next = await fetchBoard(filter, signal);
      // The same filter asked again keeps what left it; a new one starts afresh.
      return key === drawn && shown ? keep(shown, next) : next;
    });
    if (res.superseded) return res;
    if (res.error) {
      if (res.error.field === "query") return res; // the board stays as it was
      error = res.error;
      return { failed: true };
    }
    try {
      const before = board;
      board = res.value;
      applied = filter;
      drawn = key;
      if (before) {
        const done = finished(before, board);
        if (done.length) {
          await tick();
          // A run finished together feeds the fire one after another.
          setTimeout(() => done.slice(0, 6).forEach(([id, state], i) => kindle(
            document.querySelector(`.column-body[data-state="${state}"] .card[data-id="${id}"]`),
            i * 0.09)), ms(GLIDE));
        }
      }
      if (focused) focused = await fetchFocus(focused.item.id).catch(() => null);
      error = null;
    } catch (err) {
      error = err;
    }
    return { ok: true };
  }

  // What is on the board, by identifier: its cards and the buckets they wear.
  let onScreen = $derived(new Set((board?.columns || []).flatMap((c) => c.items || [])
    .flatMap((i) => [i.identifier, i.bucketIdentifier])));
  // Its own team's changes, and another team's to what is on it. A bucket's
  // progress counts tickets of any team, so with buckets shown every item
  // event counts.
  $effect(() => onNotify(refresh, (n) => n.teamId === teamId || touches(n, onScreen, teamId)
    || (showBuckets && n.kind.startsWith("item."))));

  // A card that stops matching the filter while you look at it does not
  // vanish: it goes where it went, as any move does — into Done, kindling —
  // and stays there, faded, until the filter, a view or a toggle changes.
  // The cards on screen that the new board left out are looked up; those
  // still on this board are put in their column, by rank, marked kept.
  // ponytail: ten looked up per refresh; past that, the rest vanish as they
  // would have. A kept card is looked up again on each refresh, so it follows
  // a later move too.
  async function keep(shown, next) {
    const there = new Set(next.columns.flatMap((c) => (c.items || []).map((i) => i.id)));
    const gone = shown.columns.flatMap((c) => c.items || []).filter((i) => !there.has(i.id)).slice(0, 10);
    if (!gone.length) return next;
    const found = await Promise.all(gone.map((i) => api.getItem(i.id).catch(() => null)));
    const columns = new Map(next.columns.map((c) => [c.state.id, c]));
    for (const item of found) {
      const col = item && item.teamId === teamId && columns.get(item.stateId);
      if (!col) continue; // deleted, rejected, or back in the backlog: gone from the board itself
      const items = (col.items ||= []);
      const at = items.findIndex((i) => i.rank > item.rank);
      items.splice(at < 0 ? items.length : at, 0, { ...item, kept: true });
    }
    return next;
  }

  async function fetchFocus(id) {
    const [item, comments, history] = await Promise.all([
      api.getItem(id), api.listComments(id), api.getLog({ item: id, limit: 50 }),
    ]);
    return { item, comments: comments || [], history: history.events || [] };
  }

  // A click previews the card beside the board rather than leaving it.
  async function preview(item) {
    if (focused?.item.id === item.id) { focused = null; return; }
    try { focused = await fetchFocus(item.id); } catch (err) { failure(err); }
  }

  // A change of yours in flight is not cancelled by a refresh from the
  // stream: the refresh waits for it and runs after. Your own next change
  // does cancel it, since the newest of yours wins.
  let trying = 0;
  let again = false;
  async function apply(filter) {
    trying++;
    try { return await load(filter); } finally {
      if (!--trying && again) { again = false; load(); }
    }
  }
  const refresh = () => (trying ? (again = true) : load());
  function setShowBuckets(on) {
    showBuckets = on;
    localStorage.setItem(key, on ? "1" : "0");
  }
  function setAllDone(on) {
    allDone = on;
    localStorage.setItem(doneKey, on ? "1" : "0");
  }
  // The previewed card is the anchor of a selection that starts from it: a
  // modifier-click on a second card takes it along. Without that, the card you
  // were already looking at is the one the bulk action misses, and it has to
  // be clicked a second time to join the selection it started.
  function toggle(id) {
    if (!selected.length && focused && focused.item.id !== id) selected.push(focused.item.id);
    const i = selected.indexOf(id);
    if (i < 0) selected.push(id); else selected.splice(i, 1);
  }

  const create = () => createItem({ team, states, reload: () => load() });

  // Right-clicking a card outside the selection makes that card the selection:
  // a menu that names one ticket and edits five others is a trap.
  function openMenu(item, x, y) {
    if (!selected.includes(item.id)) selected = [item.id];
    menu = { item, ids: [...selected], x, y };
  }

  // Dragging is the pointer's, not the browser's. Native drag and drop draws a
  // ghost of its own, flickers its drop target on every child the pointer
  // crosses, and gives nothing to hang an insertion line on.
  //
  // What the drag carries: the whole selection when the card under the pointer
  // is part of it, and that card alone otherwise. Board order, not selection
  // order, so a run of cards lands the way it was read rather than the way it
  // was clicked.
  function grab(item, e) {
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey) return;
    const ids = !selected.includes(item.id) ? [item.id]
      : (board?.columns || []).flatMap((c) => c.items || [])
        .map((i) => i.id).filter((id) => selected.includes(id));
    drag = { ids, x: e.clientX, y: e.clientY, live: false, target: null };
    held = e.currentTarget;
    addEventListener("pointermove", move);
    addEventListener("pointerup", release);
    addEventListener("pointercancel", cancel);
    addEventListener("keydown", escape);
  }

  // A press only becomes a drag once the pointer has travelled, so a click
  // stays a click.
  function move(e) {
    if (!drag.live) {
      if (Math.hypot(e.clientX - drag.x, e.clientY - drag.y) < 5) return;
      drag.live = true;
      getSelection()?.removeAllRanges();
      document.body.classList.add("grabbing");
      ghost = lift(held, drag.x, drag.y, drag.ids.length); // held where it was pressed
    }
    ghost.move(e.clientX, e.clientY);
    drag.target = targetAt(e.clientX, e.clientY);
  }

  // Where a drop would land: between the two cards around the pointer, by
  // their midpoints, with the held cards left out — a card is not its own
  // neighbour. `top` is where the line is drawn, in the gap between them.
  function targetAt(x, y) {
    const body = document.elementFromPoint(x, y)?.closest(".column")?.querySelector(".column-body");
    if (!body) return null;
    let after = 0, before = 0, edge = null;
    for (const el of body.querySelectorAll(".card[data-id]")) {
      if (drag.ids.includes(+el.dataset.id)) continue;
      const box = el.getBoundingClientRect();
      if (y < box.top + box.height / 2) { before = +el.dataset.id; edge = box.top - 4; break; }
      after = +el.dataset.id; edge = box.bottom + 4;
    }
    const frame = body.getBoundingClientRect();
    const top = (edge ?? frame.top + 10) - frame.top + body.scrollTop - 1;
    return { stateId: +body.dataset.state, after, before, top };
  }

  function release() {
    const { live, target, ids } = drag;
    stop();
    if (!live) return;
    // The click that ends a drag is not a click on the card.
    const eat = (e) => { e.preventDefault(); e.stopPropagation(); };
    addEventListener("click", eat, { capture: true, once: true });
    setTimeout(() => removeEventListener("click", eat, { capture: true }));
    drop(target, ids);
  }

  // Let go anywhere but a column, or Escape, and the card goes back home.
  function cancel() {
    if (drag?.live) drag.target = null;
    release();
  }

  function stop() {
    drag = null;
    document.body.classList.remove("grabbing");
    removeEventListener("pointermove", move);
    removeEventListener("pointerup", release);
    removeEventListener("pointercancel", cancel);
    removeEventListener("keydown", escape);
  }
  const escape = (e) => e.key === "Escape" && cancel();

  // One call per card, each ranked after the one before it and before the
  // card under the line, so the run keeps its order and lands where the line
  // was. The first one names the card above the line too: naming only the one
  // below ranked it anywhere above that card, usually at the top.
  //
  // A refusal stops the run: the ones already moved stay moved, and the toast
  // says why.
  //
  // The held copy then settles onto the card it was lifted from, wherever that
  // card now is — its new place, or its old one if the move was refused or
  // called off — and the card shows again under it.
  async function drop(target, ids) {
    const copy = ghost;
    const home = ids.includes(+held.dataset.id) ? +held.dataset.id : ids[0];
    const origin = held.closest(".column-body")?.dataset.state;
    ghost = null;
    if (target) {
      landing = ids;
      let { stateId, after, before } = target;
      try {
        for (const id of ids) {
          await api.rankItem(id, stateId, after, before);
          after = id;
        }
      } catch (err) { failure(err); }
      await load();
      await tick();
    }
    // Looked for in the column it went to, then the one it came from. The copy
    // leaving the old column is still in the page, fading, and on a move to
    // the right it comes first: the held card flew back to where it started.
    const at = (state) => state &&
      document.querySelector(`.column-body[data-state="${state}"] .card[data-id="${home}"]`);
    const spot = at(target?.stateId) || at(origin);
    await copy.land(spot?.getBoundingClientRect());
    landing = [];
  }
</script>

{#if error}
  <ErrorPanel {error} retry={() => load()} />
{:else if board}
  <ScreenHeader title="Board"
                sub="{board.team.shortCode} · {board.columns.length} of {states.length} states">
    {#snippet right()}
      <SearchBox value={applied.text} {teamId}
                 onsearch={(t) => apply({ ...applied, text: t })} />
      <button class="btn primary" onclick={create}>
        <Icon body={NAV.plus} size={13} />
        <span>Create</span>
      </button>
    {/snippet}
  </ScreenHeader>

  <!-- The selection bar takes the filter bar's place rather than a row of
       its own, so starting a selection moves nothing on the screen. -->
  <div class="toolbar">
    <div inert={selected.length > 0}>
      <FilterBar {teamId} {applied} {apply} {showBuckets} {setShowBuckets} {allDone} {setAllDone} />
    </div>
    {#if selected.length}
      <SelectionBar ids={selected} {states}
                    onapplied={async () => { selected = []; await load(); }}
                    onclear={() => (selected = [])} />
    {/if}
  </div>

  <div class="body">
    <div class="backlog-layout">
      <div class="grow" style="overflow-x:auto">
        <div class="board">
          {#each board.columns as col (col.state.id)}
            <section class="column">
              <div class="column-head">
                <StatePip role={col.state.role} />
                <h2>{col.state.name}</h2>
                <span class="column-count">{col.count}</span>
              </div>
              <div class="column-body" data-state={col.state.id}>
                {#if drag?.target?.stateId === col.state.id}
                  <i class="drop-line" style:top="{drag.target.top}px"></i>
                {/if}
                {#each col.items || [] as item (item.id)}
                  <!-- A card that has just been carried somewhere does not also
                       travel there: the copy in the hand already did. -->
                  <div class="card-slot" class:landing={landing.includes(item.id)} class:kept={item.kept}
                       title={item.kept ? "No longer matches the filter; it goes when the filter changes" : null}
                       in:receive={{ key: landing.includes(item.id) ? -item.id : item.id,
                                     instant: landing.includes(item.id) }}
                       out:send={{ key: item.id }}
                       animate:flip={{ duration: landing.includes(item.id) ? 0 : ms(GLIDE),
                                       easing: easeOut }}>
                    <Card {item} selected={selected.includes(item.id)}
                          focused={focused?.item.id === item.id} rendering={team.iconRendering}
                          dragging={drag?.live && drag.ids.includes(item.id)}
                          ontoggle={toggle} ongrab={grab} onpreview={preview} onmenu={openMenu} />
                  </div>
                {/each}
                {#if !col.items?.length}
                  <div class="explain" style="padding:6px 2px">Nothing here.</div>
                {/if}
              </div>
            </section>
          {/each}
        </div>
      </div>

      {#if focused}
        <ItemPreview item={focused.item} comments={focused.comments} history={focused.history}
                     rendering={team.iconRendering} onclose={() => (focused = null)}
                     onchange={() => load()} />
      {/if}
    </div>
  </div>

  {#if menu}
    <CardMenu ids={menu.ids} item={menu.item} {states} x={menu.x} y={menu.y}
              onclose={() => (menu = null)}
              onapplied={async () => { selected = []; await load(); }} />
  {/if}
{/if}
