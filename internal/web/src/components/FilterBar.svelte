<script>
  import Icon from "./Icon.svelte";
  import LiveIndicator from "./LiveIndicator.svelte";
  import { NAV } from "../lib/icons.js";
  import { app, userName, labelName } from "../lib/state.svelte.js";
  import { openModal, failure, toast } from "../lib/ui.svelte.js";
  import { assigneeOptions, priorityOptions, bucketOptions, clausesFromFilter } from "../lib/options.js";
  import { api } from "../lib/api.js";
  import PickDialog from "../dialogs/PickDialog.svelte";
  import SaveViewDialog from "../dialogs/SaveViewDialog.svelte";

  /** One chip per dimension of SPEC.md §9.2, and no query box. */
  // setAllDone is the board's alone: the backlog has no final state.
  let { teamId, filter, setFilter, showBuckets, setShowBuckets, allDone, setAllDone } = $props();

  let assigneeOn = $derived(!!filter.assigneeIds?.length || !!filter.unassigned);
  let assigneeLabel = $derived(filter.assigneeIds?.length
    ? userName(filter.assigneeIds[0]) : filter.unassigned ? "Unassigned" : "Assignee");
  let priorityLabel = $derived(filter.priorities?.length ? `P${filter.priorities[0]}` : "Priority");
  let labelLabel = $derived(filter.labelIds?.length ? labelName(filter.labelIds[0]) : "Label");

  // Buckets are not in the shared context, so remember the name we picked.
  let bucketNames = $state({});
  let bucketLabel = $derived(filter.bucketIds?.length
    ? bucketNames[filter.bucketIds[0]] || `bucket ${filter.bucketIds[0]}` : "Bucket");

  async function pickAssignee() {
    // -1 stands for "unassigned" here, because 0 is the "any" the dialog
    // already returns as null.
    const picked = await openModal(PickDialog, {
      title: "Assignee", any: true, options: assigneeOptions(-1),
    });
    if (picked === undefined) return;
    setFilter(picked === null ? { ...filter, unassigned: false, assigneeIds: [] }
      : picked === -1 ? { ...filter, unassigned: true, assigneeIds: [] }
      : { ...filter, unassigned: false, assigneeIds: [picked] });
  }

  async function pickPriority() {
    const picked = await openModal(PickDialog, {
      title: "Priority", any: true, options: priorityOptions(),
    });
    if (picked === undefined) return;
    setFilter({ ...filter, priorities: picked === null ? [] : [picked] });
  }

  async function pickLabel() {
    const picked = await openModal(PickDialog, {
      title: "Label", any: true,
      options: app.labels.map((l) => ({ value: l.id, label: l.name })),
    });
    if (picked === undefined) return;
    setFilter({ ...filter, labelIds: picked === null ? [] : [picked] });
  }

  async function pickBucket() {
    const options = await bucketOptions();
    const picked = await openModal(PickDialog, { title: "Bucket", any: true, options });
    if (picked === undefined) return;
    for (const o of options) bucketNames[o.value] = o.label;
    setFilter({ ...filter, bucketIds: picked === null || picked === 0 ? [] : [picked] });
  }

  async function saveAsView() {
    const res = await openModal(SaveViewDialog);
    if (!res) return;
    try {
      await api.createSavedView({ teamId, ...res, clauses: clausesFromFilter(filter) });
      toast("Saved. It is in the sidebar under Views.");
      app.views = await api.listSavedViews(teamId);
    } catch (err) { failure(err); }
  }
</script>

<!-- A chip is two buttons: the picker and, once a value is set, the clear.
     Nesting the clear inside the picker would put interactive content inside a
     button — invalid, and unreachable by keyboard. -->
{#snippet chip(label, on, pick, clear)}
  <span class="chip-group">
    <button class="chip" class:on onclick={pick}>
      <span>{label}</span>
      {#if !on}<Icon body={NAV.chevron} size={10} />{/if}
    </button>
    {#if on}
      <button class="chip on" aria-label="Clear {label}" onclick={clear}>
        <Icon body={NAV.close} size={10} />
      </button>
    {/if}
  </span>
{/snippet}

<div class="filter-bar">
  <span class="eyebrow">Filter</span>

  {@render chip(assigneeLabel, assigneeOn, pickAssignee,
    () => setFilter({ ...filter, assigneeIds: [], unassigned: false }))}
  {@render chip(priorityLabel, !!filter.priorities?.length, pickPriority,
    () => setFilter({ ...filter, priorities: [] }))}
  {@render chip(labelLabel, !!filter.labelIds?.length, pickLabel,
    () => setFilter({ ...filter, labelIds: [] }))}
  {@render chip(bucketLabel, !!filter.bucketIds?.length, pickBucket,
    () => setFilter({ ...filter, bucketIds: [] }))}

  <button class="btn quiet small" onclick={saveAsView}>
    <Icon body={NAV.view} size={13} />
    <span>Save as view</span>
  </button>

  <div class="right">
    {#if setAllDone}
      <span style="font-size:12px;color:var(--muted)" title="Done for more than three days">Show old done</span>
      <button class="toggle" class:on={allDone} aria-label="Show old done"
              onclick={() => setAllDone(!allDone)}><i></i></button>
      <span class="rule-v"></span>
    {/if}
    <span style="font-size:12px;color:var(--muted)">Show buckets</span>
    <button class="toggle" class:on={showBuckets} aria-label="Show buckets"
            onclick={() => setShowBuckets(!showBuckets)}><i></i></button>
    <span class="rule-v"></span>
    <LiveIndicator />
  </div>
</div>
