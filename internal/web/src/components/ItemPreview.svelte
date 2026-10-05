<script module>
  import { api } from "../lib/api.js";

  /** What the preview shows of an item: it, its comments and its recent history. */
  export async function fetchPreview(id) {
    const [item, comments, history] = await Promise.all([
      api.getItem(id), api.listComments(id), api.getLog({ item: id, limit: 50 }),
    ]);
    return { item, comments: comments || [], history: history.events || [] };
  }
</script>

<script>
  // The focused item in full, without leaving the board or the table
  // (DESIGN.md §6). Its identifier is the way through to the full view, and
  // its fields are editable here rather than only there.

  import Icon from "./Icon.svelte";
  import Avatar from "./Avatar.svelte";
  import BucketIcon from "./BucketIcon.svelte";
  import PriorityGlyph from "./PriorityGlyph.svelte";
  import Markdown from "./Markdown.svelte";
  import { openModal, failure } from "../lib/ui.svelte.js";
  import { assigneeOptions, priorityOptions } from "../lib/options.js";
  import { fieldName, eventChange } from "../lib/events.svelte.js";
  import { ensure } from "../lib/catalog.svelte.js";
  import { pictures } from "../lib/pictures.js";
  import { when, plural } from "../lib/format.js";
  import { NAV, PRIORITY } from "../lib/icons.js";
  import PickDialog from "../dialogs/PickDialog.svelte";
  import BucketDialog from "../dialogs/BucketDialog.svelte";
  import MoveDialog from "../dialogs/MoveDialog.svelte";

  let { item: it, comments = [], history = [], rendering = "mono", onclose, onchange } = $props();

  // What was said and what happened, in the aside rather than only on the
  // detail view. The counts alone said an argument had taken place without
  // saying what it was.
  let tab = $state("comments");
  let newComment = $state("");
  ensure("bucket"); // the bucket ids history carries resolve against this

  async function addComment() {
    if (!newComment.trim()) return;
    const body = newComment;
    newComment = "";
    try {
      await api.addComment(it.id, body);
      await onchange();
    } catch (err) { failure(err); }
  }

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
  // Whether the description is open for writing rather than read rendered.
  let writing = $state(false);
  function focusIf(el, on) {
    if (on) { el.focus(); el.selectionStart = el.value.length; }
  }
  let titleChanged = $derived(draft.id === it.id && draft.title !== it.title);
  let descriptionChanged = $derived(draft.id === it.id && draft.description !== (it.description || ""));

  // A title emptied by accident is put back rather than saved as blank. A
  // pasted one loses its line breaks, as it did in a one-line input.
  function saveTitle() {
    draft.title = draft.title.replace(/\s*\n\s*/g, " ");
    draft.title.trim() ? patch({ title: draft.title }) : (draft.title = it.title);
  }

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

  async function pickBucket() {
    const picked = await openModal(BucketDialog, { value: it.bucketId || 0, rendering });
    if (picked !== undefined) patch({ bucketId: picked });
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
  <div>
    <!-- The title is the field: a heading wrapped round one is a heading with
         no text in it. A textarea, so a long title wraps instead of being cut
         off; Enter still saves rather than breaking the line. -->
    <textarea class="bare" rows="1" aria-label="Title" bind:value={draft.title}
              style="font-size:17px;font-weight:600;letter-spacing:-0.008em;color:var(--bright)"
              onkeydown={(e) => {
                if (e.key === "Enter") { e.preventDefault(); saveTitle(); }
                else if (e.key === "Escape") draft.title = it.title;
              }}></textarea>
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
    {#if it.description && !writing && !descriptionChanged}
      <!-- Read rendered. A click on it, but not on a link in it, opens the
           Markdown it was written in, where it was. -->
      <Markdown class="prose bare-read" style="font-size:14px" text={it.description}
                tabindex="0" aria-label="Description" title="Click to edit"
                onclick={(e) => !e.target.closest("a") && (writing = true)}
                onkeydown={(e) => e.key === "Enter" && (e.preventDefault(), (writing = true))} />
    {:else}
      <textarea class="bare prose" rows="1" style="font-size:14px" aria-label="Description"
                placeholder="No description. Write one here." bind:value={draft.description}
                use:focusIf={writing} use:pictures onblur={() => (writing = false)}
                onkeydown={(e) => e.key === "Escape" && (draft.description = it.description || "")}
      ></textarea>
    {/if}
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
        <Icon body={NAV.pen} size={13} />
      </button>
    </div>
  </div>
  <div class="aside-rule"></div>
  <div style="display:flex;gap:6px;flex-wrap:wrap">
    {#each it.labels || [] as name}<span class="label-chip">{name}</span>{:else}
      <span class="dash">—</span>
    {/each}
  </div>
  <span class="ident">{plural((it.relations || []).length, "relation")}</span>

  <div class="tabs aside-tabs">
    <button class="tab" class:on={tab === "comments"} onclick={() => (tab = "comments")}>
      <span>Comments</span><span class="count">{comments.length}</span>
    </button>
    <button class="tab" class:on={tab === "history"} onclick={() => (tab = "history")}>
      <span>History</span><span class="count">{history.length}</span>
    </button>
  </div>

  {#if tab === "comments"}
    <div class="aside-activity">
      {#each comments as c}
        <div class="aside-comment">
          <div class="comment-meta">
            <Avatar name={c.authorName} />
            <b><a href="/users/{c.authorName}">{c.authorName}</a></b>
            <time>{when(c.createdAt)}</time>
          </div>
          <Markdown class="comment-body" text={c.body} />
        </div>
      {:else}
        <div class="explain">No comments yet.</div>
      {/each}
    </div>
    <div>
      <textarea class="input" rows="2" placeholder="Write a comment…" aria-label="Comment"
                style="width:100%" bind:value={newComment} use:pictures></textarea>
      <button class="btn primary" style="margin-top:6px" onclick={addComment}>Comment</button>
    </div>
  {:else}
    <div class="aside-activity">
      <!-- The same two helpers the log and the detail view read events
           through, so the three cannot drift. -->
      {#each history as e}
        <div class="aside-event">
          <span class="field-label">{fieldName(e)}</span>
          <span class="aside-event-value">
            {eventChange(e)}
          </span>
          <span class="ident">{e.actorName || "—"} · {when(e.at)}</span>
        </div>
      {:else}
        <div class="explain">No history.</div>
      {/each}
    </div>
  {/if}
</aside>
