<script>
  // The board shows every state except the default one and the rejected one;
  // the backlog shows the default state. Rejected items reach neither. The two
  // share their filter bar, their selection and their bulk actions, because
  // they are one view split by state (SPEC.md §7.2).

  import { untrack, tick } from "svelte";
  import { flip } from "svelte/animate";
  import { fade } from "svelte/transition";

  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { route } from "../lib/router.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { failure } from "../lib/ui.svelte.js";
  import { createItem } from "../lib/create.js";
  import { filterFromView } from "../lib/options.js";
  import { NAV } from "../lib/icons.js";
  import { ms, sparks, crossfade, easeOut, QUICK, GLIDE } from "../lib/motion.js";

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
  let dragging = null;

  // A view preference, private to this browser: it decides what is drawn,
  // never what is true.
  let key = $derived(`buckets:${teamId}:board`);
  let showBuckets = $state(untrack(() => localStorage.getItem(key) === "1"));
  // Items done for more than three days are hidden unless this is on
  // (SPEC.md §7.2); a long Done column is history, not work.
  let doneKey = $derived(`done:${teamId}:board`);
  let allDone = $state(untrack(() => localStorage.getItem(doneKey) === "1"));

  // Applying a saved view means applying the filter it stores. It is the same
  // dimensions composed the same way, so the board takes it as an ordinary
  // filter rather than gaining a second way to be filtered (SPEC.md §9.4).
  let viewId = $derived(+route.search.get("view") || 0);
  let filter = $state({});
  let viewApplied = null;

  $effect(() => {
    if (viewApplied === viewId) return;
    viewApplied = viewId;
    const view = app.views.find((v) => v.id === viewId);
    filter = view ? filterFromView(view) : {};
    load();
  });

  // A card that changes column travels there, whoever moved it: a drop, a bulk
  // move, or a teammate's change arriving on the stream. One that arrives from
  // nowhere, or leaves for nowhere, fades.
  const [send, receive] = crossfade({
    duration: () => ms(GLIDE), easing: easeOut,
    fallback: (node) => fade(node, { duration: ms(QUICK) }),
  });

  // A card that was open work a moment ago and is now in the final state
  // throws sparks once it has landed. Only one that was on the board before:
  // a filter that brings finished work into view has finished nothing.
  function finished(before, after) {
    const open = new Set(before.columns.filter((c) => c.state.role !== "final")
      .flatMap((c) => (c.items || []).map((i) => i.id)));
    return after.columns.filter((c) => c.state.role === "final")
      .flatMap((c) => (c.items || []).map((i) => i.id)).filter((id) => open.has(id));
  }

  async function load() {
    try {
      if (!team) [team, states] = await Promise.all([api.getTeam(teamId), api.listWorkStates(teamId)]);
      const before = board;
      board = await api.getBoard(teamId, filter, showBuckets, allDone);
      if (before) {
        const done = finished(before, board);
        if (done.length) {
          await tick();
          setTimeout(() => done.forEach((id) =>
            sparks(document.querySelector(`.card[data-id="${id}"]`))), ms(GLIDE));
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

  function setFilter(next) { filter = next; load(); }
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

  // What the drag carries: the whole selection when the card under the pointer
  // is part of it, and that card alone otherwise. Dragging one of five selected
  // cards to another column and watching four stay behind is the bug this
  // answers; a card outside the selection still travels on its own.
  //
  // Board order, not selection order, so a run of cards lands the way it was
  // read rather than the way it was clicked.
  function dragged() {
    if (!dragging) return [];
    if (!selected.includes(dragging.id)) return [dragging.id];
    return (board?.columns || []).flatMap((c) => c.items || [])
      .map((i) => i.id).filter((id) => selected.includes(id));
  }

  // The card the pointer is above, so a drop lands in front of it rather than
  // always at the end of the column. A card being dragged is not its own
  // neighbour: measuring against one would rank a card before itself.
  function neighbourAt(container, y, moving) {
    for (const el of container.querySelectorAll(".card[data-id]")) {
      const id = +el.dataset.id;
      if (moving.includes(id)) continue;
      const box = el.getBoundingClientRect();
      if (y < box.top + box.height / 2) return id;
    }
    return 0; // past the last card: no neighbour, so the drop ranks at the end
  }

  async function drop(e, col) {
    e.preventDefault();
    e.currentTarget.classList.remove("over");
    const moving = dragged();
    if (!moving.length) return;
    const before = neighbourAt(e.currentTarget, e.clientY, moving);
    try {
      // One call per card, each ranked after the one before it, so the run
      // keeps its order instead of reversing into the gap. A refusal stops the
      // run: the ones already moved stay moved, and the toast says why.
      let after = 0;
      for (const id of moving) {
        await api.rankItem(id, col.state.id, after, before);
        after = id;
      }
      await load();
    } catch (err) { failure(err); }
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
              <div class="column-body" role="presentation"
                   ondragover={(e) => { e.preventDefault(); e.currentTarget.classList.add("over"); }}
                   ondragleave={(e) => e.currentTarget.classList.remove("over")}
                   ondrop={(e) => drop(e, col)}>
                {#each col.items || [] as item (item.id)}
                  <div class="card-slot" in:receive={{ key: item.id }} out:send={{ key: item.id }}
                       animate:flip={{ duration: ms(GLIDE), easing: easeOut }}>
                    <Card {item} selected={selected.includes(item.id)}
                          focused={focused?.item.id === item.id} rendering={team.iconRendering}
                          ontoggle={toggle} ondrag={(it) => (dragging = it)} onpreview={preview}
                          onmenu={openMenu} />
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
