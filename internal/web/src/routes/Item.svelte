<script>
  // Item detail: the full record, its relations, its comments and its history.
  //
  // Header actions are Copy link, Clone, Split, Reject and Edit — buttons, not
  // menu items buried in an overflow (DESIGN.md §6).

  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { navigate } from "../lib/router.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { openModal, toast, failure } from "../lib/ui.svelte.js";
  import { assigneeOptions, bucketOptions } from "../lib/options.js";
  import { when, whenDay, plural } from "../lib/format.js";
  import { loadBuckets, fieldName, eventValue } from "../lib/events.svelte.js";
  import { NAV, PRIORITY, RELATION_LABELS } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import Avatar from "../components/Avatar.svelte";
  import PriorityGlyph from "../components/PriorityGlyph.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import StatePip from "../components/StatePip.svelte";
  import StateBadge from "../components/StateBadge.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";
  import MoveDialog from "../dialogs/MoveDialog.svelte";
  import SplitDialog from "../dialogs/SplitDialog.svelte";
  import PickDialog from "../dialogs/PickDialog.svelte";
  import FormDialog from "../dialogs/FormDialog.svelte";
  import ConfirmDialog from "../dialogs/ConfirmDialog.svelte";
  import IconPickDialog from "../dialogs/IconPickDialog.svelte";

  let { identifier = null, bucketId = null } = $props();

  let item = $state(null);
  let comments = $state([]);
  let history = $state({ events: [] });
  let contents = $state(null);
  let error = $state(null);
  let tab = $state("comments");
  let editing = $state(false);
  let draftTitle = $state("");
  let draftDescription = $state("");
  let newComment = $state("");
  let labelDraft = $state("");
  let addingLabel = $state(false);

  async function load() {
    try {
      const fresh = item ? await api.getItem(item.id)
        : bucketId ? await api.getItem(bucketId)
        : await api.lookupItem(identifier);
      // The old /bucket/<id> address: the detail lives at the item's identifier.
      if (!item && bucketId) { navigate(`/${fresh.identifier}`, true); return; }
      item = fresh;
      [comments, history, contents] = await Promise.all([
        api.listComments(item.id).then((c) => c || []),
        api.getLog({ item: item.id, limit: 100 }),
        item.kind === "bucket" ? api.getBucketContents(item.id) : null,
      ]);
      error = null;
    } catch (err) {
      error = err;
    }
  }

  load();
  loadBuckets();
  $effect(() => onNotify(load));

  // A field patch leaves edit mode alone: the aside's controls patch while the
  // title and description drafts are still open, and collapsing the editor
  // would discard them.
  async function patch(body) {
    try {
      item = await api.updateItem(item.id, { ...body, version: item.version });
      await load();
    } catch (err) { failure(err); }
  }

  const guard = async (fn) => {
    try { await fn(); await load(); } catch (err) { failure(err); }
  };

  function startEditing() {
    draftTitle = item.title;
    draftDescription = item.description || "";
    editing = true;
  }

  // Done saves whatever the drafts still hold, so leaving edit mode is never
  // the thing that loses an edit.
  async function stopEditing() {
    const body = {};
    if (draftTitle !== item.title) body.title = draftTitle;
    if (draftDescription !== (item.description || "")) body.description = draftDescription;
    if (Object.keys(body).length) await patch(body);
    editing = false;
  }

  let team = $derived(app.allTeams.find((t) => t.id === item?.teamId));
  let isBucket = $derived(item?.kind === "bucket");
  let progress = $derived(item?.progress || { done: 0, total: 0 });
  let percent = $derived(progress.total ? Math.round((progress.done / progress.total) * 100) : 0);

  async function clone() {
    try { navigate(`/${(await api.cloneItem(item.id)).identifier}`); } catch (err) { failure(err); }
  }

  async function split() {
    const res = await openModal(SplitDialog, { item });
    if (!res) return;
    try {
      await api.splitItem(item.id, res.titles, res.rejectSource);
      await load();
      toast(`${plural(res.titles.length, "new ticket")}`);
    } catch (err) { failure(err); }
  }

  async function moveTo(only) {
    const states = await api.listWorkStates(item.teamId);
    const res = await openModal(MoveDialog, {
      states: only ? states.filter(only) : states, count: 1, identifiers: item.identifier,
    });
    if (!res) return;
    guard(() => api.moveItem(item.id, res.stateId, res.reason, res.note));
  }

  // Deletion is an instance-administrator operation and states how many items
  // across how many teams are affected (SPEC.md §6.4).
  async function removeBucket() {
    const impact = await api.getItemImpact(item.id);
    const ok = await openModal(ConfirmDialog, {
      title: `Delete ${item.identifier} — ${item.title}`,
      lines: ["Permanent. The identifier is never reissued.",
        `${plural(impact.items, "ticket")} across ${plural(impact.teams, "team")} lose their ` +
          "bucket. The tickets themselves are untouched."],
      counts: [{ value: impact.items, label: "items" }, { value: impact.teams, label: "teams" }],
      confirmWord: item.identifier,
      consequence: "Deletion is not a state",
    });
    if (!ok) return;
    try {
      await api.deleteItem(item.id);
      toast("Deleted");
      navigate("/buckets");
    } catch (err) { failure(err); }
  }

  // An administrator of the bucket's owning team may remove a ticket from it —
  // removal only, and it is recorded on the ticket (SPEC.md §6.4).
  async function removeContent(it) {
    const ok = await openModal(ConfirmDialog, {
      title: "Remove from bucket",
      lines: [`Remove ${it.identifier} from ${item.title}?`,
        "It stays with its own team, and the removal is recorded on the ticket."],
      consequence: "One event on the ticket",
      action: "Remove",
    });
    if (ok) guard(() => api.updateItem(it.id, { bucketId: 0 }));
  }

  async function addRelation() {
    const res = await openModal(FormDialog, {
      title: "Add a relation",
      action: "Add",
      fields: [
        { name: "kind", label: "Kind", type: "select", value: "blocks",
          options: ["blocks", "follows_up", "duplicates", "relates_to"]
            .map((k) => ({ value: k, label: RELATION_LABELS[k] })) },
        { name: "to", label: "To", placeholder: "SOL-42" },
      ],
      explain: "Relations may cross teams and may link a ticket to a bucket.",
    });
    if (!res?.to.trim()) return;
    guard(async () => {
      const other = await api.lookupItem(res.to.trim());
      await api.addRelation(item.id, other.id, res.kind);
    });
  }

  async function pickAssignee() {
    const picked = await openModal(PickDialog, {
      title: isBucket ? "Owner" : "Assignee", options: assigneeOptions(),
    });
    if (picked !== undefined) patch({ assigneeId: picked });
  }

  async function pickBucket() {
    const picked = await openModal(PickDialog, { title: "Bucket", options: await bucketOptions() });
    if (picked !== undefined) patch({ bucketId: picked });
  }



  async function pickIcon() {
    const picked = await openModal(IconPickDialog);
    if (picked) patch({ icon: picked });
  }

  async function editComment(c) {
    const res = await openModal(FormDialog, {
      title: "Edit your comment",
      fields: [{ name: "body", label: "Comment", type: "textarea", value: c.body }],
    });
    if (res) guard(() => api.updateComment(c.id, res.body));
  }

  async function addComment() {
    if (!newComment.trim()) return;
    const body = newComment;
    newComment = "";
    guard(() => api.addComment(item.id, body));
  }

  async function addLabel() {
    if (!labelDraft.trim()) return;
    const name = labelDraft.trim();
    labelDraft = "";
    addingLabel = false;
    guard(() => api.addLabel(item.id, name));
  }

</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if item}
  <header class="screen-header">
    <nav class="crumbs">
      {#if team}
        <a href="/t/{team.id}/board">{team.name}</a>
        <span>/</span>
      {/if}
      <a href={isBucket ? "/buckets" : team ? `/t/${team.id}/board` : "/"}>
        {isBucket ? "Buckets" : "Board"}
      </a>
      <span>/</span>
      <span class="here">{item.identifier}</span>
    </nav>
    <div class="right">
      <button class="btn small" onclick={() => {
        navigator.clipboard?.writeText(`${location.origin}/${item.identifier}`);
        toast("Link copied");
      }}>
        <Icon body={NAV.copy} size={12} /><span>Copy link</span>
      </button>
      {#if !isBucket}
        <button class="btn small" onclick={clone}>
          <Icon body={NAV.copy} size={12} /><span>Clone</span>
        </button>
        <button class="btn small" onclick={split}>
          <Icon body={NAV.split} size={12} /><span>Split</span>
        </button>
      {/if}
      <button class="btn small" onclick={() => moveTo((s) => s.role === "rejected")}>Reject</button>
      <button class="btn small" class:primary={editing}
              onclick={() => (editing ? stopEditing() : startEditing())}>
        {editing ? "Done" : "Edit"}
      </button>
      {#if isBucket && app.user.isAdmin}
        <button class="btn small destructive" onclick={removeBucket}>Delete</button>
      {/if}
    </div>
  </header>

  <div class="body">
    <div class="detail">
      <div class="detail-main">
        {#if editing}
          <div style="display:flex;gap:8px;margin-bottom:8px">
            <input class="input" style="flex:1;font-size:17px;height:38px" bind:value={draftTitle} />
            <button class="btn primary save" onclick={() => patch({ title: draftTitle })}>Save</button>
          </div>
        {:else}
          <h1 style="display:flex;align-items:center;gap:10px">
            {#if isBucket}<BucketIcon name={item.icon} size={18} />{/if}
            <span>{item.title}</span>
            {#if isBucket}<span class="state-badge">bucket</span>{/if}
          </h1>
        {/if}

        <div style="display:flex;align-items:center;gap:8px;margin:10px 0 18px">
          <Avatar name={item.reporterName} />
          <span style="font-size:12.5px;color:var(--muted)">
            {item.reporterName} reported this on {when(item.createdAt, true)}
          </span>
        </div>

        {#if editing}
          <textarea class="input" rows="10" style="width:100%;font-family:var(--serif);font-size:15px"
                    bind:value={draftDescription}></textarea>
          <button class="btn primary save" style="margin-top:8px"
                  onclick={() => patch({ description: draftDescription })}>Save description</button>
        {:else if item.description}
          <div class="prose">{item.description}</div>
        {:else}
          <div class="explain">No description.</div>
        {/if}

        {#if isBucket}
          <!-- A bucket's progress is a view over its contents, not a field
               (SPEC.md §6.2), shown as a count of items in a terminal state. -->
          <div class="progress-block">
            <div style="display:flex;align-items:baseline;gap:6px;margin-bottom:8px">
              <span class="big">{progress.done}</span>
              <span class="of">/ {progress.total}</span>
              <span class="ident" style="margin-left:auto">{progress.total - progress.done} open</span>
            </div>
            <div class="progress"><i style="width:{percent}%"></i></div>
            <div style="margin-top:8px">
              <span class="ident">{plural((contents || []).length, "team")}</span>
            </div>
          </div>

          <section style="margin:0 0 28px">
            <div class="section-head"><span class="eyebrow">Contents by owning team</span></div>
            {#each contents || [] as g}
              <div style="margin-bottom:4px">
                <div style="display:flex;align-items:center;gap:8px;padding:8px 0">
                  <span class="code-badge">{g.team.shortCode}</span>
                  <span style="color:var(--bright);font-size:12.5px">{g.team.name}</span>
                  <span class="ident" style="margin-left:auto">
                    {g.items.filter((i) => i.stateRole === "final" || i.stateRole === "rejected").length}
                    / {g.items.length} done
                  </span>
                </div>
                {#each g.items as it}
                  <div class="relation-row">
                    <PriorityGlyph level={it.priority} />
                    <a class="ident" style="width:84px;flex:0 0 84px" href="/{it.identifier}">
                      {it.identifier}
                    </a>
                    <span style="flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12.5px">
                      {it.title}
                    </span>
                    <StateBadge name={it.stateName} role={it.stateRole} />
                    <Avatar name={it.assigneeName} />
                    <button class="btn quiet small" title="Remove from this bucket"
                            onclick={() => removeContent(it)}>
                      <Icon body={NAV.close} size={11} />
                    </button>
                  </div>
                {/each}
              </div>
            {:else}
              <div class="explain">
                Nothing is in this bucket yet. Membership is a field on the ticket.
              </div>
            {/each}
          </section>
        {/if}

        <section style="margin-top:28px">
          <div class="section-head">
            <span class="eyebrow">Relations</span>
            <span class="ident">{(item.relations || []).length}</span>
            <button class="btn quiet small" style="margin-left:auto" onclick={addRelation}>
              <Icon body={NAV.plus} size={11} /><span>Add</span>
            </button>
          </div>
          {#each item.relations || [] as r}
            <div class="relation-row">
              <span class="relation-kind">{RELATION_LABELS[r.kind] || r.kind}</span>
              <a class="ident" style="color:var(--accent)" href="/{r.identifier}">{r.identifier}</a>
              <span style="flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">
                {r.title}
              </span>
              <StateBadge name={r.stateName} role={r.stateRole} />
              <button class="btn quiet small" title="Remove"
                      onclick={() => guard(() => api.removeRelation(r.id))}>
                <Icon body={NAV.close} size={11} />
              </button>
            </div>
          {:else}
            <div class="explain">
              No relations. A relation is information: a blocked-by edge never prevents a move.
            </div>
          {/each}
        </section>

        <section>
          <div class="tabs">
            <button class="tab" class:on={tab === "comments"} onclick={() => (tab = "comments")}>
              <span>Comments</span><span class="count">{comments.length}</span>
            </button>
            <button class="tab" class:on={tab === "history"} onclick={() => (tab = "history")}>
              <span>History</span><span class="count">{history.events.length}</span>
            </button>
          </div>

          {#if tab === "comments"}
            {#each comments as c}
              <div class="comment">
                <Avatar name={c.authorName} size="comment" />
                <div style="min-width:0;flex:1">
                  <div class="comment-meta">
                    <b>{c.authorName}</b>
                    <time>{when(c.createdAt, true)}</time>
                    {#if c.updatedAt > c.createdAt}<span class="explain">edited</span>{/if}
                    {#if c.authorId === app.user.id}
                      <button class="btn quiet small" style="margin-left:auto"
                              onclick={() => editComment(c)}>Edit</button>
                    {/if}
                  </div>
                  <div class="comment-body">{c.body}</div>
                </div>
              </div>
            {:else}
              <div class="explain">No comments yet.</div>
            {/each}
            <div class="comment">
              <Avatar name={app.user.username} size="comment" />
              <div style="flex:1">
                <textarea class="input" rows="3" placeholder="Write a comment…"
                          bind:value={newComment}></textarea>
                <button class="btn primary" style="margin-top:8px" onclick={addComment}>Comment</button>
              </div>
            </div>
          {:else}
            {#each history.events as e}
              <div class="relation-row">
                <span class="ident" style="width:70px;flex:0 0 70px">{e.seq}</span>
                <span style="display:flex;align-items:center;gap:6px;width:150px">
                  <Avatar name={e.actorName} />
                  <span style="font-size:12.5px">{e.actorName || "—"}</span>
                  <!-- History surfaces name the token beside the actor, so an
                       agent's actions are distinguishable (SPEC.md §14.4). -->
                  {#if e.agentName}
                    <span class="explain" style="color:var(--accent)">agent: {e.agentName}</span>
                  {/if}
                </span>
                <span class="field-label" style="width:90px">{fieldName(e)}</span>
                <span style="flex:1;font-size:12.5px">
                  {e.oldValue ? `${eventValue(e, e.oldValue)} → ` : ""}{e.newValue ? eventValue(e, e.newValue) : ""}
                </span>
                <span class="ident">{when(e.at, true)}</span>
              </div>
            {:else}
              <div class="explain">No history.</div>
            {/each}
          {/if}
        </section>
      </div>

      <aside class="detail-aside">
        <div class="aside-field">
          <span class="field-label">Work state</span>
          <button class="aside-control" onclick={() => moveTo(null)}>
            <StatePip role={item.stateRole} />
            <span>{item.stateName}</span>
            <span class="note">{item.stateRole || "intermediate"}</span>
            <Icon body={NAV.chevron} size={11} color="var(--faint)" />
          </button>
        </div>

        {#if item.rejectReason}
          <div class="aside-field">
            <span class="field-label">Rejected as</span>
            <div class="aside-value">
              <span>{item.rejectReason}</span>
              {#if item.rejectNote}<span class="explain">{item.rejectNote}</span>{/if}
            </div>
          </div>
        {/if}

        <div class="aside-pair">
          {#snippet assignee()}
            <!-- A bucket's assignee is presented as its owner: the person
                 accountable for the milestone (SPEC.md §6.2). -->
            <div class="aside-field">
              <span class="field-label">{isBucket ? "Owner" : "Assignee"}</span>
              <button class="aside-control" onclick={pickAssignee}>
                <Avatar name={item.assigneeName} />
                <span>{item.assigneeName || "Unassigned"}</span>
                <Icon body={NAV.chevron} size={11} color="var(--faint)" />
              </button>
            </div>
          {/snippet}
          {#snippet priority()}
            <div class="aside-field">
              <span class="field-label">Priority</span>
              <div class="aside-value">
                <PriorityGlyph level={item.priority} />
                <span>{PRIORITY[item.priority].name}</span>
                {#if editing}
                  <select class="input" style="margin-left:auto;height:26px" aria-label="Priority"
                          onchange={(e) => patch({ priority: +e.currentTarget.value })}>
                    {#each [1, 2, 3, 4] as p}
                      <option value={p} selected={p === item.priority}>P{p}</option>
                    {/each}
                  </select>
                {/if}
              </div>
            </div>
          {/snippet}
          {#if isBucket}{@render assignee()}{@render priority()}
          {:else}{@render priority()}{@render assignee()}{/if}
        </div>

        {#snippet reporter()}
          <div class="aside-field">
            <span class="field-label">Reporter</span>
            <div class="aside-value"><Avatar name={item.reporterName} /><span>{item.reporterName}</span></div>
          </div>
        {/snippet}

        {#if isBucket}
          <div class="aside-pair">
            <div class="aside-field">
              <span class="field-label">Icon</span>
              <div class="aside-value">
                <BucketIcon name={item.icon} size={16} />
                <span>{item.icon}</span>
                {#if editing}
                  <button class="btn quiet small" style="margin-left:auto" onclick={pickIcon}>Change</button>
                {/if}
              </div>
            </div>
            {@render reporter()}
          </div>
        {:else}
          {@render reporter()}
        {/if}

        <div class="aside-rule"></div>

        {#if isBucket}
          <div class="aside-field">
            <span class="field-label">Owning team</span>
            <div class="aside-value">
              <span class="code-badge">{item.teamShortCode}</span>
              <span>{team ? team.name : item.teamShortCode}</span>
            </div>
            <p class="explain">
              Reassignable by an instance administrator. The identifier stays
              {item.identifier} wherever it goes.
            </p>
          </div>
        {:else}
          <div class="aside-field">
            <span class="field-label">Bucket</span>
            <button class="aside-control" onclick={pickBucket}>
              {#if item.bucketIdentifier}
                <BucketIcon name={item.bucketIcon} size={13} />
                <span>{item.bucketTitle}</span>
                <span class="note">{item.bucketIdentifier}</span>
              {:else}
                <span class="dash">—</span>
                <span class="note">none</span>
              {/if}
            </button>
          </div>
        {/if}

        <div class="aside-rule"></div>

        <div class="aside-pair">
          <div class="aside-field">
            <span class="field-label">Planned end</span>
            <div class="aside-value">
              {#if editing}
                <input class="input" type="date" value={item.plannedEnd || ""}
                       onchange={(e) => patch({ plannedEnd: e.currentTarget.value })} />
              {:else if item.plannedEnd}
                <span class="mono">{whenDay(item.plannedEnd)}</span>
              {:else}
                <span class="dash">—</span>
              {/if}
            </div>
          </div>
          <div class="aside-field">
            <span class="field-label">Actual start</span>
            <div class="aside-value">
              {#if item.actualStart}<span class="mono">{when(item.actualStart)}</span>
              {:else}<span class="dash">—</span>{/if}
            </div>
          </div>
        </div>
        <div class="aside-field">
          <span class="field-label">Actual end</span>
          <div class="aside-value">
            {#if item.actualEnd}<span class="mono">{when(item.actualEnd)}</span>
            {:else}<span class="dash">—</span>{/if}
          </div>
        </div>

        <div class="aside-rule"></div>

        <!-- Labels live on the detail view only. They are a filtering mechanism,
             and cards carrying them become unreadable (SPEC.md §8.1). -->
        <div class="aside-field">
          <span class="field-label">Labels</span>
          <div style="display:flex;gap:6px;flex-wrap:wrap;align-items:center">
            {#each item.labels || [] as name}
              <span class="label-chip">
                <span>{name}</span>
                <button class="btn quiet" style="padding:0;height:auto;border:0"
                        aria-label="Remove {name}"
                        onclick={() => guard(() => api.removeLabel(item.id, name))}>
                  <Icon body={NAV.close} size={9} />
                </button>
              </span>
            {/each}
            {#if !addingLabel}
              <button class="label-chip" style="cursor:pointer;background:none;border-style:dashed"
                      onclick={() => (addingLabel = true)}>
                <Icon body={NAV.plus} size={9} /><span>Label</span>
              </button>
            {/if}
          </div>
          {#if addingLabel}
            <!-- svelte-ignore a11y_autofocus -->
            <input class="input" style="height:26px" placeholder="Type to create" autofocus
                   bind:value={labelDraft} onkeydown={(e) => e.key === "Enter" && addLabel()} />
          {/if}
        </div>
      </aside>
    </div>
  </div>
{/if}
