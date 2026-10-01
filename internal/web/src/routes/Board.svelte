<script>
  // The board shows every state except the default one and the rejected one;
  // the backlog shows the default state. Rejected items reach neither. The two
  // share their filter bar, their selection and their bulk actions, because
  // they are one view split by state (SPEC.md §7.2).

  import { untrack, tick } from "svelte";
  import { flip } from "svelte/animate";
  import { crossfade, fade } from "svelte/transition";

  import { api } from "../lib/api.js";
  import { app, viewsOn } from "../lib/state.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { failure } from "../lib/ui.svelte.js";
  import { createItem } from "../lib/create.js";
  import { filterFromView, stackFilters } from "../lib/options.js";
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
  let allDone = $state(untrack(() => localStorage.getItem(doneKey) === "1"));

  // The views switched on in the sidebar and the bar's own filter, stacked
  // into the one filter the screen asks for: the same dimensions composed the
  // same way, so a view is never a second way to be filtered (SPEC.md §9.4).
  // Any change to either redraws, a saved edit to a view that is on included.
  let filter = $state({});
  let shown = $derived(stackFilters([filter, ...viewsOn().map(filterFromView)]));
  let asked = $derived(JSON.stringify(shown));

  $effect(() => {
    asked;
    untrack(load);
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

  async function load() {
    try {
      if (!team) [team, states] = await Promise.all([api.getTeam(teamId), api.listWorkStates(teamId)]);
      const before = board;
      board = await api.getBoard(teamId, shown, showBuckets, allDone);
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
  }

  $effect(() => onNotify(load));

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

  function setFilter(next) { filter = next; }
  function setShowBuckets(on) {
    showBuckets = on;
    localStorage.setItem(key, on ? "1" : "0");
    load();
  }
  function setAllDone(on) {
    allDone = on;
    localStorage.setItem(doneKey, on ? "1" : "0");
    load();
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

  const create = () => createItem({ team, states, reload: load });

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
  <ErrorPanel {error} retry={load} />
{:else if board}
  <ScreenHeader title="Board"
                sub="{board.team.shortCode} · {board.columns.length} of {states.length} states">
    {#snippet right()}
      <SearchBox value={filter.text || ""} {teamId}
                 onsearch={(t) => setFilter({ ...filter, text: t })} />
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
      <FilterBar {teamId} {filter} {setFilter} {showBuckets} {setShowBuckets} {allDone} {setAllDone} />
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
                  <div class="card-slot" class:landing={landing.includes(item.id)}
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
                     onchange={load} />
      {/if}
    </div>
  </div>

  {#if menu}
    <CardMenu ids={menu.ids} item={menu.item} {states} x={menu.x} y={menu.y}
              onclose={() => (menu = null)}
              onapplied={async () => { selected = []; await load(); }} />
  {/if}
{/if}
