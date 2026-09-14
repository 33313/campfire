<script>
  // The backlog is the default state's column, seen as a table.

  import { untrack } from "svelte";

  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { route, navigate } from "../lib/router.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { failure } from "../lib/ui.svelte.js";
  import { createItem } from "../lib/create.js";
  import { filterFromView } from "../lib/options.js";
  import { whenDay } from "../lib/format.js";
  import { NAV } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import SearchBox from "../components/SearchBox.svelte";
  import FilterBar from "../components/FilterBar.svelte";
  import SelectionBar from "../components/SelectionBar.svelte";
  import PriorityGlyph from "../components/PriorityGlyph.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import Avatar from "../components/Avatar.svelte";
  import LoadOlder from "../components/LoadOlder.svelte";
  import ItemPreview from "../components/ItemPreview.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";

  let { teamId } = $props();

  let team = $state(null);
  let states = $state([]);
  let column = $state(null);
  let error = $state(null);
  let selected = $state([]);
  let focused = $state(null); // { item, comments } for the detail aside
  // Set when live activity arrived while older pages were on screen: reloading
  // would throw them away, so the refresh is offered rather than taken.
  let stale = $state(false);
  let paged = $state(false);

  let key = $derived(`buckets:${teamId}:backlog`);
  let showBuckets = $state(untrack(() => localStorage.getItem(key) === "1"));

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
      column = await api.getBacklog(teamId, filter, showBuckets);
      if (focused) focused = await fetchFocus(focused.item.id).catch(() => null);
      paged = false;
      stale = false;
      error = null;
    } catch (err) {
      error = err;
    }
  }

  $effect(() => onNotify(() => (paged ? (stale = true) : load())));

  async function fetchFocus(id) {
    const [item, comments, history] = await Promise.all([
      api.getItem(id), api.listComments(id), api.getLog({ item: id, limit: 50 }),
    ]);
    return { item, comments: comments || [], history: history.events || [] };
  }

  async function focus(item) {
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

  let items = $derived(column?.items || []);
  let allSelected = $derived(items.length > 0 && items.every((i) => selected.includes(i.id)));
  let someSelected = $derived(items.some((i) => selected.includes(i.id)));
  let older = $derived(column ? column.count - items.length : 0);

  function toggleAll() {
    selected = allSelected ? [] : items.map((i) => i.id);
  }

  // The backlog's "load older" is the ordinary filtered list continued from
  // the column's cursor (SPEC.md §13.2 rule 9).
  async function loadOlder() {
    const f = { ...filter, teamIds: [teamId], stateIds: [column.state.id] };
    if (!showBuckets) f.kinds = ["ticket"];
    try {
      const page = await api.listItems(f, column.cursor);
      column.items = items.concat(page.items || []);
      column.cursor = page.cursor || "";
      paged = true;
    } catch (err) { failure(err); }
  }

  const create = () => createItem({ team, states, reload: load });
</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if column}
  <ScreenHeader title="Backlog" sub="{team.shortCode} · {column.count} items">
    {#snippet right()}
      <SearchBox value={filter.text || ""} {teamId}
                 onsearch={(t) => setFilter({ ...filter, text: t })} />
      <button class="btn primary" onclick={create}>
        <Icon body={NAV.plus} size={13} />
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
      <div class="grow">
        {#if stale}
          <LoadOlder label="New activity — refresh" onclick={load} />
        {/if}
        <table class="table">
          <thead>
            <tr>
              <th style="width:34px">
                <input class="check" type="checkbox" aria-label="Select every row"
                       checked={allSelected} indeterminate={someSelected && !allSelected}
                       onchange={toggleAll} />
              </th>
              <th style="width:26px">Pr</th>
              <th style="width:84px">Key</th>
              <th>Title</th>
              <th style="width:180px">Bucket</th>
              <th style="width:44px">As</th>
              <th style="width:90px">Planned</th>
            </tr>
          </thead>
          <tbody>
            {#each items as item (item.id)}
              <!-- A click focuses the row into the aside; the identifier opens the item. -->
              <tr class:selected={selected.includes(item.id)} class:focused={focused?.item.id === item.id}
                  onclick={() => focus(item)} ondblclick={() => navigate(`/${item.identifier}`)}>
                <td>
                  <input class="check" type="checkbox" aria-label="Select {item.identifier}"
                         checked={selected.includes(item.id)}
                         onclick={(e) => e.stopPropagation()}
                         onchange={() => toggle(item.id)} />
                </td>
                <td><PriorityGlyph level={item.priority} /></td>
                <td class="num">
                  <a class="ident" href="/{item.identifier}" onclick={(e) => e.stopPropagation()}>
                    {item.identifier}
                  </a>
                </td>
                <td class="cell-title">{item.title}</td>
                <td>
                  {#if item.bucketIdentifier}
                    <!-- The bucket follows through to the bucket, like the key
                         beside it; the row click still focuses the ticket. -->
                    <a style="display:flex;align-items:center;gap:6px;color:var(--text)"
                       href="/{item.bucketIdentifier}" onclick={(e) => e.stopPropagation()}>
                      <BucketIcon name={item.bucketIcon} size={11} rendering={team.iconRendering}
                                  color={item.bucketColor} />
                      <span style="font-size:12px">{item.bucketTitle}</span>
                    </a>
                  {:else}
                    <span class="dash">—</span>
                  {/if}
                </td>
                <td><Avatar name={item.assigneeName} /></td>
                <td class="num">
                  {#if item.plannedEnd}{whenDay(item.plannedEnd)}{:else}<span class="dash">—</span>{/if}
                </td>
              </tr>
            {:else}
              <tr><td colspan="7"><div class="empty-note">Nothing in the backlog.</div></td></tr>
            {/each}
          </tbody>
        </table>
        {#if older > 0 && column.cursor}
          <LoadOlder label="Load {older} older" onclick={loadOlder} />
        {/if}
      </div>

      {#if focused}
        <ItemPreview item={focused.item} comments={focused.comments} history={focused.history}
                     rendering={team.iconRendering} onclose={() => (focused = null)}
                     onchange={load} />
      {/if}
    </div>
  </div>
{/if}
