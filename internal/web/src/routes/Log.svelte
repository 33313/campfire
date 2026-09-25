<script>
  // The audit log.
  //
  // Ordered by sequence number, never by timestamp, and paginated by keyset —
  // so the footer offers "load older from 41192", never a page number
  // (SPEC.md §10).

  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { route } from "../lib/router.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { openModal, toast, failure } from "../lib/ui.svelte.js";
  import { userOptions } from "../lib/options.js";
  import { loadBuckets, fieldName, fieldLabel, eventChange } from "../lib/events.svelte.js";
  import { clock } from "../lib/format.js";
  import { NAV } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import Avatar from "../components/Avatar.svelte";
  import LiveIndicator from "../components/LiveIndicator.svelte";
  import LoadOlder from "../components/LoadOlder.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";
  import PickDialog from "../dialogs/PickDialog.svelte";
  import FormDialog from "../dialogs/FormDialog.svelte";

  // The fields the log writes; anything filterable is a real column.
  const FIELDS = ["state", "assignee", "priority", "bucket", "label",
    "title", "description", "plannedEnd", "member", "name"];

  // A profile's "View all" arrives here as ?team= or ?actor=: that team's
  // log, or that person's actions in every team.
  const urlTeam = +route.search.get("team") || 0;
  const urlActor = +route.search.get("actor") || 0;

  let team = $derived(app.allTeams.find((t) => t.id === urlTeam) || app.team);
  // team | all. All is the whole instance to an administrator; to anyone else
  // it is one person's actions across the teams, and needs an actor.
  let scope = $state(urlActor && !urlTeam ? "all" : "team");
  let agentOnly = $state(false);
  let actorId = $state(urlActor);
  let fieldFilter = $state("");
  let itemFilter = $state(null);   // { id, identifier }
  let events = $state([]);
  let cursor = $state(0);
  let error = $state(null);
  // Set when live activity arrived while older pages were on screen: reloading
  // would throw them away, so the refresh is offered rather than taken.
  let stale = $state(false);
  let paged = $state(false);

  const LIMIT = 60;

  function params() {
    const p = { limit: LIMIT };
    if (scope === "team" && team) p.team = team.id;
    if (agentOnly) p.agent = 1;
    if (actorId) p.actor = actorId;
    if (fieldFilter) p.field = fieldFilter;
    if (itemFilter) p.item = itemFilter.id;
    return p;
  }

  async function load(before = 0) {
    try {
      const page = await api.getLog({ ...params(), before });
      events = before ? events.concat(page.events || []) : page.events || [];
      cursor = page.cursor || 0;
      paged = !!before;
      stale = false;
      error = null;
    } catch (err) {
      if (before) failure(err); else error = err;
    }
  }

  load();
  loadBuckets();
  $effect(() => onNotify(() => (paged ? (stale = true) : load())));

  async function pickActor() {
    if (actorId) {
      actorId = 0;
      if (!app.user.isAdmin) scope = "team";
      return load();
    }
    const picked = await openModal(PickDialog, { title: "Actor", options: userOptions() });
    if (picked) { actorId = picked; load(); }
  }

  async function pickField() {
    if (fieldFilter) { fieldFilter = ""; return load(); }
    const picked = await openModal(PickDialog, {
      title: "Field", options: FIELDS.map((f) => ({ value: f, label: fieldLabel(f) })),
    });
    if (picked) { fieldFilter = picked; load(); }
  }

  async function pickItem() {
    if (itemFilter) { itemFilter = null; return load(); }
    const res = await openModal(FormDialog, {
      title: "Item", action: "Filter",
      fields: [{ name: "identifier", label: "Item", placeholder: "SOL-42" }],
    });
    if (!res?.identifier.trim()) return;
    try {
      const it = await api.lookupItem(res.identifier.trim());
      itemFilter = { id: it.id, identifier: it.identifier };
      load();
    } catch (err) { failure(err); }
  }

  function exportRows() {
    // A one-way copy of what is on screen; the log itself is never edited or
    // deleted.
    navigator.clipboard?.writeText(events.map((e) => [e.seq, e.at, e.actorName,
      e.itemIdentifier, e.field, e.oldValue, e.newValue, e.teamShortCode].join("\t")).join("\n"));
    toast(`${events.length} rows copied`);
  }

</script>

{#if error}
  <ErrorPanel {error} retry={() => load()} />
{:else}
  <ScreenHeader title="Log" sub="ordered by sequence, newest first">
    {#snippet right()}
      <button class="btn small" onclick={exportRows}>
        <Icon body={NAV.export} size={13} /><span>Export</span>
      </button>
    {/snippet}
  </ScreenHeader>

  <div class="filter-bar">
    <span class="eyebrow">Scope</span>
    {#if team}
      <button class="chip" class:on={scope === "team"}
              onclick={() => { scope = "team"; load(); }}>{team.name}</button>
    {/if}
    {#if app.user.isAdmin || actorId}
      <button class="chip" class:on={scope === "all"}
              onclick={() => { scope = "all"; load(); }}>
        {app.user.isAdmin ? "Whole instance" : "All teams"}
      </button>
    {/if}
    <span class="rule-v"></span>
    <button class="chip" class:on={!!actorId} onclick={pickActor}>
      <span>{actorId ? app.users.find((u) => u.id === actorId)?.username || `user ${actorId}` : "Actor"}</span>
      <Icon body={actorId ? NAV.close : NAV.chevron} size={10} />
    </button>
    <button class="chip" class:on={!!fieldFilter} onclick={pickField}>
      <span>{fieldFilter ? fieldLabel(fieldFilter) : "Field"}</span>
      <Icon body={fieldFilter ? NAV.close : NAV.chevron} size={10} />
    </button>
    <button class="chip" class:on={!!itemFilter} onclick={pickItem}>
      <span>{itemFilter ? itemFilter.identifier : "Item"}</span>
      <Icon body={itemFilter ? NAV.close : NAV.chevron} size={10} />
    </button>
    <button class="chip" class:on={agentOnly} onclick={() => { agentOnly = !agentOnly; load(); }}>
      <Icon body={NAV.agent} size={11} /><span>Agent</span>
    </button>
    <div class="right"><LiveIndicator /></div>
  </div>

  <div class="body">
    {#if stale}
      <LoadOlder label="New activity — refresh" onclick={() => load()} />
    {/if}
    <table class="table">
      <thead>
        <tr>
          <th style="width:70px">Seq</th>
          <th style="width:100px">When</th>
          <th style="width:160px">Actor</th>
          <th style="width:90px">Item</th>
          <th style="width:110px">Field</th>
          <th>Change</th>
          <th style="width:70px">Team</th>
        </tr>
      </thead>
      <tbody>
        {#each events as e (e.seq)}
          <tr>
            <td class="num">{e.seq}</td>
            <td class="num">{clock(e.at)}</td>
            <td>
              <span style="display:flex;align-items:center;gap:6px">
                <Avatar name={e.actorName} />
                <span style="font-size:12.5px">{e.actorName || "—"}</span>
                <!-- An agent's actions are distinguishable at a glance and
                     filterable in the log (SPEC.md §14.4). -->
                {#if e.agentName}
                  <span title="agent: {e.agentName}" style="color:var(--accent);display:flex">
                    <Icon body={NAV.agent} size={11} />
                  </span>
                {/if}
              </span>
            </td>
            <td>
              {#if e.itemIdentifier}
                <a class="ident" href="/{e.itemIdentifier}">{e.itemIdentifier}</a>
              {:else}
                <span class="dash">—</span>
              {/if}
            </td>
            <td class="field-label">{fieldName(e)}</td>
            <td style="font-size:12.5px">{eventChange(e)}</td>
            <td class="num">{e.teamShortCode || ""}</td>
          </tr>
        {/each}
      </tbody>
    </table>
    {#if cursor}
      <LoadOlder label="Load older — from sequence {cursor}" onclick={() => load(cursor)} />
    {:else}
      <div style="padding:14px 20px"><span class="explain">That is the whole log.</span></div>
    {/if}
  </div>
{/if}
