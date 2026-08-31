<script>
  // The board shows every state except the default one and the rejected one;
  // the backlog shows the default state. Rejected items reach neither. The two
  // share their filter bar, their selection and their bulk actions, because
  // they are one view split by state (SPEC.md §7.2).

  import { untrack } from "svelte";

  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { route } from "../lib/router.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { failure } from "../lib/ui.svelte.js";
  import { createItem } from "../lib/create.js";
  import { filterFromView } from "../lib/options.js";
  import { NAV } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import SearchBox from "../components/SearchBox.svelte";
  import FilterBar from "../components/FilterBar.svelte";
  import SelectionBar from "../components/SelectionBar.svelte";
  import StatePip from "../components/StatePip.svelte";
  import Card from "../components/Card.svelte";
  import ItemPreview from "../components/ItemPreview.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";

  let { teamId } = $props();

  let team = $state(null);
  let states = $state([]);
  let board = $state(null);
  let error = $state(null);
  let selected = $state([]);
  let focused = $state(null); // { item, comments } for the preview aside
  let dragging = null;

  // A view preference, private to this browser: it decides what is drawn,
  // never what is true.
  let key = $derived(`buckets:${teamId}:board`);
  let showBuckets = $state(untrack(() => localStorage.getItem(key) === "1"));

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

  async function load() {
    try {
      if (!team) [team, states] = await Promise.all([api.getTeam(teamId), api.listWorkStates(teamId)]);
      board = await api.getBoard(teamId, filter, showBuckets);
      if (focused) focused = await fetchFocus(focused.item.id).catch(() => null);
      error = null;
    } catch (err) {
      error = err;
    }
  }

  $effect(() => onNotify(load));

  async function fetchFocus(id) {
    const [item, comments] = await Promise.all([api.getItem(id), api.listComments(id)]);
    return { item, comments: comments || [] };
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
  function toggle(id) {
    const i = selected.indexOf(id);
    if (i < 0) selected.push(id); else selected.splice(i, 1);
  }

  const create = () => createItem({ team, states, reload: load });

  // The card the pointer is above, so a drop lands in front of it rather than
  // always at the end of the column. The dragged card is not its own
  // neighbour: measuring against it would rank the card before itself.
  function neighbourAt(container, y) {
    for (const el of container.querySelectorAll(".card[data-id]")) {
      const id = +el.dataset.id;
      if (id === dragging?.id) continue;
      const box = el.getBoundingClientRect();
      if (y < box.top + box.height / 2) return id;
    }
    return 0; // past the last card: no neighbour, so the drop ranks at the end
  }

  async function drop(e, col) {
    e.preventDefault();
    e.currentTarget.classList.remove("over");
    if (!dragging) return;
    try {
      await api.rankItem(dragging.id, col.state.id, 0, neighbourAt(e.currentTarget, e.clientY));
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
      <SearchBox value={filter.text || ""} onsearch={(t) => setFilter({ ...filter, text: t })} />
      <button class="btn primary" onclick={create}>
        <Icon body={NAV.plus} size={12} />
        <span>Create</span>
      </button>
    {/snippet}
  </ScreenHeader>

  <FilterBar {teamId} {filter} {setFilter} {showBuckets} {setShowBuckets} />

  {#if selected.length}
    <SelectionBar ids={selected} {states}
                  onapplied={async () => { selected = []; await load(); }}
                  onclear={() => (selected = [])} />
  {/if}

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
                  <Card {item} selected={selected.includes(item.id)}
                        focused={focused?.item.id === item.id} rendering={team.iconRendering}
                        ontoggle={toggle} ondrag={(it) => (dragging = it)} onpreview={preview} />
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
        <ItemPreview item={focused.item} comments={focused.comments}
                     rendering={team.iconRendering} onclose={() => (focused = null)}
                     onchange={load} />
      {/if}
    </div>
  </div>
{/if}
