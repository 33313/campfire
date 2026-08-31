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

  // The title and the description are typed into where they are drawn; the
  // floppy beside them writes, the cross puts back what was there. Both only
  // appear once the field has actually been changed, so nothing hovers over
  // text that is only being read.
  //
  // The drafts follow the previewed item. Live activity refreshes `it` under
  // them, which is why the pair is keyed by id rather than reset on every load.
  let draft = $state({ id: null, title: "", description: "" });
  $effect(() => {
    if (draft.id !== it.id) draft = { id: it.id, title: it.title, description: it.description || "" };
  });
  let titleChanged = $derived(draft.id === it.id && draft.title !== it.title);
  let descriptionChanged = $derived(draft.id === it.id && draft.description !== (it.description || ""));

  // A title emptied by accident is put back rather than saved as blank.
  const saveTitle = () =>
    draft.title.trim() ? patch({ title: draft.title }) : (draft.title = it.title);

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
  const pickBucket = () => pick("Bucket", bucketOptions(), (v) => ({ bucketId: v }));

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
  <div>
    <!-- The title is the input: a heading wrapped round one is a heading with
         no text in it. -->
    <input class="bare" aria-label="Title" bind:value={draft.title}
           style="font-size:17px;font-weight:600;letter-spacing:-0.008em;color:var(--bright)"
           onkeydown={(e) => {
             if (e.key === "Enter") saveTitle();
             else if (e.key === "Escape") draft.title = it.title;
           }} />
    {#if titleChanged}
      <div class="field-actions">
        <button class="btn" title="Save title" aria-label="Save title" onclick={saveTitle}>
          <Icon body={NAV.save} size={12} />
        </button>
        <button class="btn" title="Discard" aria-label="Discard title"
                onclick={() => (draft.title = it.title)}>
          <Icon body={NAV.close} size={11} />
        </button>
      </div>
    {/if}
  </div>
  <div>
    <textarea class="bare prose" rows="1" style="font-size:14px" aria-label="Description"
              placeholder="No description. Write one here." bind:value={draft.description}
              onkeydown={(e) => e.key === "Escape" && (draft.description = it.description || "")}
    ></textarea>
    {#if descriptionChanged}
      <div class="field-actions">
        <button class="btn" title="Save description" aria-label="Save description"
                onclick={() => patch({ description: draft.description })}>
          <Icon body={NAV.save} size={12} />
        </button>
        <button class="btn" title="Discard" aria-label="Discard description"
                onclick={() => (draft.description = it.description || "")}>
          <Icon body={NAV.close} size={11} />
        </button>
      </div>
    {/if}
  </div>
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
        <input class="bare" style="height:34px" type="date" aria-label="Planned end"
               value={it.plannedEnd || ""}
               onchange={(e) => patch({ plannedEnd: e.currentTarget.value })} />
      </div>
    </div>
  </div>
  <!-- The bucket is a way through to the bucket, as it is on the detail view;
       changing it is the deliberate second action beside it. -->
  <div class="aside-field">
    <span class="field-label">Bucket</span>
    <div style="display:flex;gap:6px">
      {#if it.bucketIdentifier}
        <a class="aside-control" href="/{it.bucketIdentifier}">
          <BucketIcon name={it.bucketIcon} size={13} {rendering} color={it.bucketColor} />
          <span>{it.bucketTitle}</span>
          <span class="note">{it.bucketIdentifier}</span>
        </a>
      {:else}
        <button class="aside-control" onclick={pickBucket}>
          <span class="dash">—</span>
          <span class="note">none</span>
        </button>
      {/if}
      <button class="btn quiet" style="height:34px" aria-label="Change bucket"
              title="Change bucket" onclick={pickBucket}>
        <Icon body={NAV.pen} size={12} />
      </button>
    </div>
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
