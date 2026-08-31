<script>
  // The focused item in full, without leaving the board or the table
  // (DESIGN.md §6). Its identifier is the way through to the full view, and
  // its fields are editable here rather than only there.

  import Icon from "./Icon.svelte";
  import Avatar from "./Avatar.svelte";
  import BucketIcon from "./BucketIcon.svelte";
  import PriorityGlyph from "./PriorityGlyph.svelte";
  import { api } from "../lib/api.js";
  import { openModal, failure } from "../lib/ui.svelte.js";
  import { assigneeOptions, priorityOptions, bucketOptions } from "../lib/options.js";
  import { whenDay, plural } from "../lib/format.js";
  import { NAV, PRIORITY } from "../lib/icons.js";
  import PickDialog from "../dialogs/PickDialog.svelte";
  import MoveDialog from "../dialogs/MoveDialog.svelte";

  let { item: it, comments = [], rendering = "mono", onclose, onchange } = $props();

  async function patch(body) {
    try {
      await api.updateItem(it.id, { ...body, version: it.version });
      await onchange();
    } catch (err) { failure(err); }
  }

  async function pick(title, options, then) {
    const picked = await openModal(PickDialog, { title, options: await options });
    if (picked !== undefined) patch(then(picked));
  }

  // A state change is a move, with its reason and note, exactly as it is on
  // the detail view (SPEC.md §7.1).
  async function move() {
    try {
      const states = await api.listWorkStates(it.teamId);
      const res = await openModal(MoveDialog, { states, identifiers: it.identifier });
      if (!res) return;
      await api.moveItem(it.id, res.stateId, res.reason, res.note);
      await onchange();
    } catch (err) { failure(err); }
  }
</script>

<aside class="detail-aside">
  <div style="display:flex;align-items:center;gap:8px">
    <PriorityGlyph level={it.priority} />
    <a class="ident" style="color:var(--accent)" href="/{it.identifier}">{it.identifier}</a>
    <button class="btn quiet small" style="margin-left:auto" aria-label="Close" onclick={onclose}>
      <Icon body={NAV.close} size={11} />
    </button>
  </div>
  <h2 style="margin:0">{it.title}</h2>
  {#if it.description}
    <div class="prose" style="font-size:14px">{it.description}</div>
  {:else}
    <div class="explain">No description.</div>
  {/if}
  <div class="aside-rule"></div>
  <div class="aside-field">
    <span class="field-label">State</span>
    <button class="aside-control" onclick={move}>
      <span>{it.stateName}</span>
      <span class="note">{it.stateRole || "intermediate"}</span>
      <Icon body={NAV.chevron} size={11} color="var(--faint)" />
    </button>
  </div>
  <div class="aside-pair">
    <div class="aside-field">
      <span class="field-label">Assignee</span>
      <button class="aside-control"
              onclick={() => pick("Assignee", assigneeOptions(), (v) => ({ assigneeId: v }))}>
        <Avatar name={it.assigneeName} />
        <span>{it.assigneeName || "Unassigned"}</span>
      </button>
    </div>
    <div class="aside-field">
      <span class="field-label">Priority</span>
      <button class="aside-control"
              onclick={() => pick("Priority", priorityOptions(), (v) => ({ priority: v }))}>
        <PriorityGlyph level={it.priority} />
        <span>{PRIORITY[it.priority].name}</span>
      </button>
    </div>
  </div>
  <div class="aside-pair">
    <div class="aside-field">
      <span class="field-label">Reporter</span>
      <div class="aside-value"><Avatar name={it.reporterName} /><span>{it.reporterName}</span></div>
    </div>
    <div class="aside-field">
      <span class="field-label">Planned end</span>
      <div class="aside-value">
        <input class="input" style="height:34px;width:100%" type="date" aria-label="Planned end"
               value={it.plannedEnd || ""}
               onchange={(e) => patch({ plannedEnd: e.currentTarget.value })} />
      </div>
    </div>
  </div>
  <div class="aside-field">
    <span class="field-label">Bucket</span>
    <button class="aside-control" onclick={() => pick("Bucket", bucketOptions(), (v) => ({ bucketId: v }))}>
      {#if it.bucketIdentifier}
        <BucketIcon name={it.bucketIcon} size={13} {rendering} color={it.bucketColor} />
        <span>{it.bucketTitle}</span>
        <span class="note">{it.bucketIdentifier}</span>
      {:else}
        <span class="dash">—</span>
        <span class="note">none</span>
      {/if}
    </button>
  </div>
  <div class="aside-rule"></div>
  <div style="display:flex;gap:6px;flex-wrap:wrap">
    {#each it.labels || [] as name}<span class="label-chip">{name}</span>{:else}
      <span class="dash">—</span>
    {/each}
  </div>
  <div style="display:flex;gap:14px">
    <span class="ident">{plural(comments.length, "comment")}</span>
    <span class="ident">{plural((it.relations || []).length, "relation")}</span>
  </div>
</aside>
