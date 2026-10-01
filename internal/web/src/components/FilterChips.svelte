<script>
  import Icon from "./Icon.svelte";
  import { NAV } from "../lib/icons.js";
  import { app, userName, labelName } from "../lib/state.svelte.js";
  import { openModal } from "../lib/ui.svelte.js";
  import { assigneeOptions, priorityOptions, bucketOptions } from "../lib/options.js";
  import PickDialog from "../dialogs/PickDialog.svelte";

  /** One chip per dimension of SPEC.md §9.2, and no query box. The filter bar
      and the view dialog both edit a filter with these. */
  let { filter, setFilter, view = null } = $props();

  // The server names every clause a view stores, deleted referents included,
  // so a chip says "DEV-6 Campfire - v1.0.0" rather than "bucket 6".
  let named = $derived(Object.fromEntries((view?.clauses || [])
    .map((c) => [`${c.dimension}:${c.refId}`, c])));
  // Buckets are not in the shared context, so remember the names we picked.
  let bucketNames = $state({});
  const clause = (dim, id) => named[`${dim}:${id}`];
  const nameOf = (dim, id, fallback) => clause(dim, id)?.display || fallback;
  // A chip names its first two values and counts the rest. Values of one
  // dimension are any-of; labels are the exception, every one of them.
  const names = (list, joint = ", ") =>
    list.slice(0, 2).join(joint) + (list.length > 2 ? ` +${list.length - 2}` : "");
  const missing = (dim, ids) => (ids || []).some((id) => clause(dim, id)?.missing);

  let assigneeOn = $derived(!!filter.assigneeIds?.length || !!filter.unassigned);
  let assigneeLabel = $derived(assigneeOn ? names([
    ...(filter.unassigned ? ["Unassigned"] : []),
    ...(filter.assigneeIds || []).map((id) => nameOf("assignee", id, userName(id))),
  ]) : "Assignee");
  let priorityLabel = $derived(filter.priorities?.length
    ? names(filter.priorities.map((p) => `P${p}`)) : "Priority");
  let labelLabel = $derived(filter.labelIds?.length
    ? names(filter.labelIds.map((id) => nameOf("label", id, labelName(id))), " & ") : "Label");
  let bucketLabel = $derived(filter.bucketIds?.length
    ? names(filter.bucketIds.map((id) => bucketNames[id] || nameOf("bucket", id, `bucket ${id}`))) : "Bucket");

  // Reporter, state and team have no chip of their own: only a view made
  // through the API or an agent sets them. Shown, so nothing filters unseen.
  const EXTRA = [["reporter", "reporterIds", "Reporter"], ["state", "stateIds", "State"], ["team", "teamIds", "Team"]];
  let extras = $derived(EXTRA.flatMap(([dim, key, title]) => (filter[key] || []).map((id) => ({
    dim, key, id, label: `${title}: ${nameOf(dim, id, dim === "reporter" ? userName(id) : `${dim} ${id}`)}`,
  }))));

  // Every chip picks several. Escape keeps what was there; Done with nothing
  // picked clears the chip.
  const pickMany = (title, options, picked, explain = null) =>
    openModal(PickDialog, { title, options, many: true, picked: picked || [], explain });

  async function pickAssignee() {
    // -1 stands for "unassigned" here: it is a value beside the people.
    const picked = await pickMany("Assignee", assigneeOptions(-1),
      [...(filter.unassigned ? [-1] : []), ...(filter.assigneeIds || [])]);
    if (!picked) return;
    setFilter({ ...filter, unassigned: picked.includes(-1), assigneeIds: picked.filter((v) => v !== -1) });
  }

  async function pickPriority() {
    const picked = await pickMany("Priority", priorityOptions(), filter.priorities);
    if (picked) setFilter({ ...filter, priorities: picked });
  }

  async function pickLabel() {
    const picked = await pickMany("Label", app.labels.map((l) => ({ value: l.id, label: l.name })),
      filter.labelIds, "An item must carry every label picked.");
    if (picked) setFilter({ ...filter, labelIds: picked });
  }

  async function pickBucket() {
    // "No bucket" is a value for moving items, not for finding them.
    const options = (await bucketOptions()).filter((o) => o.value !== 0);
    const picked = await pickMany("Bucket", options, filter.bucketIds);
    if (!picked) return;
    for (const o of options) bucketNames[o.value] = o.label;
    setFilter({ ...filter, bucketIds: picked });
  }
</script>

<!-- A chip is two buttons: the picker and, once a value is set, the clear.
     Nesting the clear inside the picker would put interactive content inside a
     button — invalid, and unreachable by keyboard. -->
{#snippet chip(label, on, pick, clear, gone = false)}
  <span class="chip-group" title={gone ? "Names something deleted, so it matches nothing" : null}>
    <button class="chip" class:on class:missing={gone} onclick={pick}>
      <span>{label}</span>
      {#if !on}<Icon body={NAV.chevron} size={10} />{/if}
    </button>
    {#if on}
      <button class="chip on" class:missing={gone} aria-label="Clear {label}" onclick={clear}>
        <Icon body={NAV.close} size={10} />
      </button>
    {/if}
  </span>
{/snippet}

{@render chip(assigneeLabel, assigneeOn, pickAssignee,
  () => setFilter({ ...filter, assigneeIds: [], unassigned: false }), missing("assignee", filter.assigneeIds))}
{@render chip(priorityLabel, !!filter.priorities?.length, pickPriority,
  () => setFilter({ ...filter, priorities: [] }))}
{@render chip(labelLabel, !!filter.labelIds?.length, pickLabel,
  () => setFilter({ ...filter, labelIds: [] }), missing("label", filter.labelIds))}
{@render chip(bucketLabel, !!filter.bucketIds?.length, pickBucket,
  () => setFilter({ ...filter, bucketIds: [] }), missing("bucket", filter.bucketIds))}
{#each extras as x (x.dim + x.id)}
  {@render chip(x.label, true, null,
    () => setFilter({ ...filter, [x.key]: filter[x.key].filter((id) => id !== x.id) }), !!clause(x.dim, x.id)?.missing)}
{/each}
