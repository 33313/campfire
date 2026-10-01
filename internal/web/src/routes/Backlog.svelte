<script>
  // The backlog is the default state's column, seen as a table.

  import { untrack } from "svelte";

  import { api } from "../lib/api.js";
  import { latest } from "../lib/latest.js";
  import { app, viewsOn } from "../lib/state.svelte.js";
  import { navigate } from "../lib/router.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { failure } from "../lib/ui.svelte.js";
  import { createItem } from "../lib/create.js";
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

  // What the backlog shows: the filter bar's query and the header's search, as
  // last accepted, and-ed with the views switched on in the sidebar (SPEC.md
  // §9.4). A query being tried is not applied until the server accepts it, so
  // a refused one never rides along with the next request (SPEC.md §9.2).
  let applied = $state({ csql: "", text: "" });
  const ask = latest();
  const TZ = Intl.DateTimeFormat().resolvedOptions().timeZone;
  let drawn = "";
  const asking = (filter) => JSON.stringify([filter, viewsOn().map((v) => [v.id, v.query]), showBuckets]);
  // The filter as the API takes it, for the column and for "load older".
  const asked = (filter) => ({ ...filter, viewIds: viewsOn().map((v) => v.id), tz: TZ });

  $effect(() => {
    const now = asking(applied);
    untrack(() => { if (now !== drawn) load(); });
  });

  /** Asks with a filter, the applied one unless another is being tried.
      Resolves to what became of it: { ok }, { error } for a query the server
      refused, or { superseded } when a newer request took its place. */
  async function load(filter = applied) {
    const key = asking(filter);
    const res = await ask(async (signal) => {
      if (!team) [team, states] = await Promise.all([api.getTeam(teamId), api.listWorkStates(teamId)]);
      return api.getBacklog(teamId, asked(filter), showBuckets, 100, signal);
    });
    if (res.superseded) return res;
    if (res.error) {
      if (res.error.field === "query") return res; // the backlog stays as it was
      error = res.error;
      return { failed: true };
    }
    column = res.value;
    applied = filter;
    drawn = key;
    paged = false;
    stale = false;
    error = null;
    if (focused) focused = await fetchFocus(focused.item.id).catch(() => null);
    return { ok: true };
  }

  $effect(() => onNotify(() => (paged ? (stale = true) : refresh())));

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
    const f = { ...asked(applied), teamIds: [teamId], stateIds: [column.state.id] };
    if (!showBuckets) f.kinds = ["ticket"];
    try {
      const page = await api.listItems(f, column.cursor);
      column.items = items.concat(page.items || []);
      column.cursor = page.cursor || "";
      paged = true;
    } catch (err) { failure(err); }
  }

  const create = () => createItem({ team, states, reload: () => load() });
</script>

{#if error}
  <ErrorPanel {error} retry={() => load()} />
{:else if column}
  <ScreenHeader title="Backlog" sub="{team.shortCode} · {column.count} items">
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
      <FilterBar {teamId} {applied} {apply} {showBuckets} {setShowBuckets} />
    </div>
    {#if selected.length}
      <SelectionBar ids={selected} {states}
                    onapplied={async () => { selected = []; await load(); }}
                    onclear={() => (selected = [])} />
    {/if}
  </div>

  <div class="body">
    <div class="backlog-layout">
      <div class="grow">
        {#if stale}
          <LoadOlder label="New activity — refresh" onclick={() => load()} />
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
                     onchange={() => load()} />
      {/if}
    </div>
  </div>
{/if}
