<script>
  // The bucket index. A bucket's detail is the item screen: one identifier
  // namespace, one detail view (SPEC.md §6.2, §6.3).
  //
  // The index defaults to participation: buckets my team owns, plus buckets
  // containing at least one of my team's tickets. Scoping by ownership alone
  // would show a team the buckets it created and hide the release it is
  // working toward (SPEC.md §9.1).

  import { pref, setPref } from "../lib/prefs.js";
  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { openModal, failure } from "../lib/ui.svelte.js";
  import { createItem } from "../lib/create.js";
  import { assigneeOptions } from "../lib/options.js";
  import { whenDay } from "../lib/format.js";
  import { NAV } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import Avatar from "../components/Avatar.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import StateBadge from "../components/StateBadge.svelte";
  import SearchBox from "../components/SearchBox.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";
  import LoadOlder from "../components/LoadOlder.svelte";
  import PickDialog from "../dialogs/PickDialog.svelte";

  let team = $derived(app.team);
  let page = $state(null);
  let error = $state(null);
  let scope = $state(pref("bucketScope", "participation"));

  // State and owner narrow the scope's result. They are the ordinary filter
  // dimensions of SPEC.md §9.2, applied to the list the scope produced.
  let stateFilter = $state(null); // { id, name }
  let ownerId = $state(0);        // -1 is unowned
  let text = $state("");
  // Set when live activity arrived while further pages were on screen:
  // reloading would throw them away, so the refresh is offered rather taken.
  let stale = $state(false);
  let paged = $state(false);

  let SCOPES = $derived([
    { id: "participation", label: "Participation" },
    { id: "owned", label: team ? `Owned by ${team.name}` : "Owned" },
    { id: "instance", label: "Whole instance" },
  ]);

  async function load() {
    try {
      page = await api.listBuckets(team ? team.id : 0, scope);
      paged = false;
      stale = false;
      error = null;
    } catch (err) {
      error = err;
    }
  }

  // The index is a keyset page like every other unbounded list (SPEC.md §13.2
  // rule 9), so it continues from the cursor rather than stopping at the first
  // page and calling that the whole instance.
  async function loadMore() {
    try {
      const next = await api.listBuckets(team ? team.id : 0, scope, page.cursor);
      page = { buckets: (page.buckets || []).concat(next.buckets || []), cursor: next.cursor || "" };
      paged = true;
    } catch (err) { failure(err); }
  }

  load();
  $effect(() => onNotify(() => (paged ? (stale = true) : load())));

  function setScope(id) {
    scope = id;
    setPref("bucketScope", id);
    load();
  }

  let buckets = $derived((page?.buckets || []).filter((b) =>
    (!stateFilter || b.stateId === stateFilter.id) &&
    (!ownerId || (ownerId === -1 ? !b.assigneeId : b.assigneeId === ownerId)) &&
    (!text || b.title.toLowerCase().includes(text.toLowerCase()) ||
      b.identifier.toLowerCase().includes(text.toLowerCase()))));

  // "so far" while a page is still unread: the count is what has been loaded,
  // never a total the list has not seen.
  let more = $derived(!!page?.cursor);
  let description = $derived(scope === "participation"
    ? `${buckets.length}${more ? " so far" : ""} that ${team ? team.name : "this team"} takes part in`
    : scope === "owned"
      ? `${buckets.length}${more ? " so far" : ""} owned by ${team ? team.name : "this team"}`
      : `${buckets.length}${more ? " so far" : ""} in the instance`);

  let ownerLabel = $derived(ownerId === -1 ? "Unowned"
    : ownerId ? app.users.find((u) => u.id === ownerId)?.username || `user ${ownerId}`
    : "Owner");

  async function pickState() {
    if (stateFilter) { stateFilter = null; return; }
    // Buckets use their owning team's work states, so the states offered are
    // the ones the listed buckets actually sit in.
    const seen = new Map();
    for (const b of page.buckets || []) {
      if (!seen.has(b.stateId)) {
        seen.set(b.stateId, { id: b.stateId, name: b.stateName, role: b.stateRole });
      }
    }
    const picked = await openModal(PickDialog, {
      title: "State",
      options: [...seen.values()].map((s) => ({ value: s, label: s.name, icon: { pip: s.role } })),
    });
    if (picked) stateFilter = picked;
  }

  async function pickOwner() {
    if (ownerId) { ownerId = 0; return; }
    const picked = await openModal(PickDialog, { title: "Owner", options: assigneeOptions(-1) });
    if (picked) ownerId = picked;
  }

  async function create() {
    const states = await api.listWorkStates(team.id);
    await createItem({ team, states, kind: "bucket", reload: load });
  }
</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if page}
  <ScreenHeader title="Buckets" sub={description}>
    {#snippet right()}
      <SearchBox value={text} onsearch={(t) => (text = t)} />
      {#if team}
        <button class="btn primary" onclick={create}>
          <Icon body={NAV.plus} size={13} /><span>Create</span>
        </button>
      {/if}
    {/snippet}
  </ScreenHeader>

  <div class="filter-bar">
    <span class="eyebrow">Scope</span>
    {#each SCOPES as s}
      <button class="chip" class:on={scope === s.id} onclick={() => setScope(s.id)}>{s.label}</button>
    {/each}
    <div class="right">
      <button class="chip" class:on={!!stateFilter} onclick={pickState}>
        <span>{stateFilter ? stateFilter.name : "State"}</span>
        <Icon body={stateFilter ? NAV.close : NAV.chevron} size={10} />
      </button>
      <button class="chip" class:on={!!ownerId} onclick={pickOwner}>
        <span>{ownerLabel}</span>
        <Icon body={ownerId ? NAV.close : NAV.chevron} size={10} />
      </button>
    </div>
  </div>

  <div class="body">
    {#if stale}
      <LoadOlder label="New activity — refresh" onclick={load} />
    {/if}
    {#if buckets.length}
      <div class="bucket-grid">
        <!-- A bucket card: icon and identifier, the owning team's code, the
             title, progress as a count of items in a terminal state, then
             state, date and owner. Progress is a view, not a field. -->
        {#each buckets as b (b.id)}
          {@const percent = b.progress.total
            ? Math.round((b.progress.done / b.progress.total) * 100) : 0}
          {@const started = b.progress.total
            ? Math.round((b.progress.accepted / b.progress.total) * 100) : 0}
          <a class="bucket-card" href="/{b.identifier}">
            <div style="display:flex;align-items:center;gap:8px">
              <BucketIcon name={b.icon} size={13} color={b.color} />
              <span class="ident">{b.identifier}</span>
              <span class="column-count" style="margin-left:auto">{b.teamShortCode}</span>
            </div>
            <div class="title">{b.title}</div>
            <div style="display:flex;align-items:baseline;gap:6px">
              <span class="big">{b.progress.done}</span>
              <span class="of">/ {b.progress.total}</span>
              <span class="ident" style="margin-left:auto">{percent}%</span>
            </div>
            <div class="progress">
              <i style="width:{percent}%"></i>
              <i class="accepted" style="width:{started}%"></i>
            </div>
            <div style="display:flex;align-items:center;gap:8px">
              <StateBadge name={b.stateName} role={b.stateRole} />
              <span class="ident">{b.plannedEnd ? whenDay(b.plannedEnd) : "no date"}</span>
              <span style="margin-left:auto"><Avatar name={b.assigneeName} /></span>
            </div>
          </a>
        {/each}
      </div>
    {:else}
      <div class="empty-note">No buckets match.</div>
    {/if}
    {#if more}
      <LoadOlder label="Load more" onclick={loadMore} />
    {/if}
  </div>
{/if}
