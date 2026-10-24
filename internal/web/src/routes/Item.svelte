<script>
  // Item detail: the full record, its relations, its comments and its history.
  //
  // Header actions are Copy link, Clone, Split, Reject and Edit — buttons, not
  // menu items buried in an overflow (DESIGN.md §6). Edit becomes Save and
  // Cancel while it is open.
  //
  // The fields sit in the one column with everything else. A second sidebar
  // beside the board's own is two asides on one screen and nobody can say what
  // either is for.

  import { api, peek } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { navigate } from "../lib/router.svelte.js";
  import { onNotify, touches } from "../lib/stream.svelte.js";
  import { openModal, toast, failure } from "../lib/ui.svelte.js";
  import { assigneeOptions, priorityOptions } from "../lib/options.js";
  import { when, whenDay, plural } from "../lib/format.js";
  import { fieldName, eventChange, actorName } from "../lib/events.svelte.js";
  import { ensure } from "../lib/catalog.svelte.js";
  import { pictures } from "../lib/pictures.js";
  import { NAV, PRIORITY } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import Avatar from "../components/Avatar.svelte";
  import PriorityGlyph from "../components/PriorityGlyph.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import StatePip from "../components/StatePip.svelte";
  import StateBadge from "../components/StateBadge.svelte";
  import LabelEditor from "../components/LabelEditor.svelte";
  import Markdown from "../components/Markdown.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";
  import MoveDialog from "../dialogs/MoveDialog.svelte";
  import SplitDialog from "../dialogs/SplitDialog.svelte";
  import PickDialog from "../dialogs/PickDialog.svelte";
  import BucketDialog from "../dialogs/BucketDialog.svelte";
  import RelationList from "../components/RelationList.svelte";
  import PullRequestList from "../components/PullRequestList.svelte";
  import RelationDialog from "../dialogs/RelationDialog.svelte";
  import FormDialog from "../dialogs/FormDialog.svelte";
  import ConfirmDialog from "../dialogs/ConfirmDialog.svelte";
  import IconPicker from "../components/IconPicker.svelte";

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

  const fetchItem = () => (bucketId ? api.getItem(bucketId) : api.lookupItem(identifier));
  const fetchRest = (it) => [api.listComments(it.id), api.getLog({ item: it.id, limit: 100 }),
    it.kind === "bucket" ? api.getBucketContents(it.id) : null];

  async function load() {
    try {
      const fresh = await fetchItem();
      // The old /bucket/<id> address: the detail lives at the item's identifier.
      if (bucketId) { navigate(`/${fresh.identifier}`, true); return; }
      item = fresh;
      [comments, history, contents] = await Promise.all(fetchRest(item));
      comments ||= [];
      error = null;
    } catch (err) {
      error = err;
    }
  }

  // Coming back to an item draws it at once, as it was last shown, and asks
  // again behind it: a 304 when nothing changed (SPEC.md §12.4).
  {
    const was = peek(fetchItem);
    const rest = was && peek(() => fetchRest(was));
    if (rest && !rest.includes(undefined)) {
      item = was;
      comments = rest[0] || [];
      history = rest[1];
      contents = rest[2];
    }
  }

  load();
  ensure("bucket");
  // What the page shows: the item, its bucket and the items it relates to,
  // and a bucket's contents, which come from any team, so for a bucket every
  // item event counts.
  let onPage = $derived(new Set([identifier, item?.identifier, item?.bucketIdentifier,
    ...(item?.relations || []).map((r) => r.identifier)]));
  $effect(() => onNotify(load, (n) => touches(n, onPage, item?.teamId)
    || (item?.kind === "bucket" && n.kind.startsWith("item."))));

  // A field patch leaves edit mode alone: the pickers patch while the title and
  // description drafts are still open, and collapsing the editor would discard
  // them.
  // One at a time, each with the version the last one returned: the icon
  // picker sends an icon and a colour as fast as they are clicked, and the
  // second would otherwise carry a stale version and be refused.
  let patching = Promise.resolve();
  function patch(body) {
    return (patching = patching.then(async () => {
      try {
        item = await api.updateItem(item.id, { ...body, version: item.version });
        await load();
      } catch (err) { failure(err); }
    }));
  }

  const guard = async (fn) => {
    try { await fn(); await load(); } catch (err) { failure(err); }
  };

  // Labels are instance-wide, and the editor offers the ones that exist. A
  // ticket that makes a new one — or retires the last use of an old one —
  // changes that list, so it is re-read here rather than going stale until the
  // next reload.
  const withLabels = (fn) => guard(async () => {
    await fn();
    app.labels = (await api.listLabels()) || [];
  });

  function startEditing() {
    draftTitle = item.title;
    draftDescription = item.description || "";
    editing = true;
  }

  // Save writes whatever the drafts hold; Cancel throws them away. An
  // unchanged field writes nothing, and a title emptied by accident is left
  // alone rather than saved as blank.
  async function saveEdits() {
    const body = {};
    if (draftTitle.trim() && draftTitle !== item.title) body.title = draftTitle;
    if (draftDescription !== (item.description || "")) body.description = draftDescription;
    if (Object.keys(body).length) await patch(body);
    editing = false;
  }

  let team = $derived(app.allTeams.find((t) => t.id === item?.teamId));
  let isBucket = $derived(item?.kind === "bucket");
  let progress = $derived(item?.progress || { done: 0, total: 0 });
  let percent = $derived(progress.total ? Math.round((progress.done / progress.total) * 100) : 0);
  let started = $derived(progress.total
    ? Math.round(((progress.accepted || 0) / progress.total) * 100) : 0);

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
      lines: ["The identifier is never reissued.",
        `${plural(impact.items, "ticket")} across ${plural(impact.teams, "team")} lose their ` +
          "bucket."],
      counts: [{ value: impact.items, label: "items" }, { value: impact.teams, label: "teams" }],
      confirmWord: item.identifier,
      consequence: "Cannot be undone",
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
      lines: [`Remove ${it.identifier} from ${item.title}?`],
      consequence: "Recorded on the ticket",
      action: "Remove",
    });
    if (ok) guard(() => api.updateItem(it.id, { bucketId: 0 }));
  }

  async function addRelation() {
    const res = await openModal(RelationDialog, { item });
    if (res) guard(() => api.addRelation(item.id, res.other.id, res.kind));
  }

  async function pickAssignee() {
    const picked = await openModal(PickDialog, {
      title: isBucket ? "Owner" : "Assignee", options: assigneeOptions(),
    });
    if (picked !== undefined) patch({ assigneeId: picked });
  }

  async function pickBucket() {
    const picked = await openModal(BucketDialog, { value: item.bucketId || 0 });
    if (picked !== undefined) patch({ bucketId: picked });
  }



  async function pickPriority() {
    const picked = await openModal(PickDialog, { title: "Priority", options: priorityOptions() });
    if (picked !== undefined) patch({ priority: picked });
  }


  async function editComment(c) {
    const res = await openModal(FormDialog, {
      title: "Edit your comment",
      fields: [{ name: "body", label: "Comment", type: "textarea", value: c.body, pictures: true }],
    });
    if (res) guard(() => api.updateComment(c.id, res.body));
  }

  async function addComment() {
    if (!newComment.trim()) return;
    const body = newComment;
    newComment = "";
    guard(() => api.addComment(item.id, body));
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
        <Icon body={NAV.link} size={13} /><span>Copy link</span>
      </button>
      {#if !isBucket}
        <button class="btn small" onclick={clone}>
          <Icon body={NAV.clone} size={13} /><span>Clone</span>
        </button>
        <button class="btn small" onclick={split}>
          <Icon body={NAV.split} size={13} /><span>Split</span>
        </button>
      {/if}
      <button class="btn small" onclick={() => moveTo((s) => s.role === "rejected")}>Reject</button>
      {#if editing}
        <button class="btn small" onclick={() => (editing = false)}>
          <Icon body={NAV.close} size={11} /><span>Cancel</span>
        </button>
        <button class="btn small primary" onclick={saveEdits}>
          <Icon body={NAV.save} size={13} /><span>Save</span>
        </button>
      {:else}
        <button class="btn small" onclick={startEditing}>
          <Icon body={NAV.pen} size={13} /><span>Edit</span>
        </button>
      {/if}
      {#if isBucket && app.user.isAdmin}
        <button class="btn small destructive" onclick={removeBucket}>Delete</button>
      {/if}
    </div>
  </header>

  <div class="body">
    <div class="detail">
      <div class="detail-main">
        <h1 style="display:flex;align-items:center;gap:10px">
          {#if isBucket}<BucketIcon name={item.icon} size={18} color={item.color} />{/if}
          {#if editing}
            <input class="input" style="flex:1;font-size:17px;height:38px" aria-label="Title"
                   bind:value={draftTitle} />
          {:else}
            <span>{item.title}</span>
          {/if}
          {#if isBucket}<span class="state-badge">bucket</span>{/if}
        </h1>

        <div style="display:flex;align-items:center;gap:8px;margin:10px 0 18px">
          <Avatar name={item.reporterName} />
          <span style="font-size:12.5px;color:var(--muted)">
            <a href="/users/{item.reporterName}">{item.reporterName}</a>
            reported this on {when(item.createdAt, true)}
          </span>
        </div>

        {#if editing}
          <textarea class="input" rows="10" aria-label="Description"
                    style="width:100%;font-family:var(--serif);font-size:15px"
                    bind:value={draftDescription} use:pictures></textarea>
        {:else if item.description}
          <Markdown class="prose" text={item.description} />
        {:else}
          <div class="explain">No description.</div>
        {/if}

        <!-- The fields, in the column with everything else rather than in a
             second sidebar. A move and the pickers are always available; the
             rest open with Edit. -->
        <div class="fields">
          <div class="aside-field">
            <span class="field-label">Work state</span>
            <button class="aside-control" onclick={() => moveTo(null)}>
              <StatePip role={item.stateRole} />
              <span>{item.stateName}</span>
              <span class="note">{item.stateRole || "intermediate"}</span>
              <Icon body={NAV.chevron} size={11} color="var(--faint)" />
            </button>
          </div>

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
              <button class="aside-control" onclick={pickPriority}>
                <PriorityGlyph level={item.priority} />
                <span>{PRIORITY[item.priority].name}</span>
                <Icon body={NAV.chevron} size={11} color="var(--faint)" />
              </button>
            </div>
          {/snippet}
          {#if isBucket}{@render assignee()}{@render priority()}
          {:else}{@render priority()}{@render assignee()}{/if}

          {#if item.rejectReason}
            <div class="aside-field" style="grid-column:1/-1">
              <span class="field-label">Rejected as</span>
              <div class="aside-value">
                <span>{item.rejectReason}</span>
                {#if item.rejectNote}<span class="explain">{item.rejectNote}</span>{/if}
              </div>
            </div>
          {/if}

          <div class="aside-field">
            <span class="field-label">Reporter</span>
            <div class="aside-value"><Avatar name={item.reporterName} /><span>{item.reporterName}</span></div>
          </div>

          {#if isBucket}
            <!-- What its tickets wear on a board. Recognition only: nothing
                 reads it (SPEC.md §6.2). -->
            <div class="aside-field">
              <span class="field-label">Icon &amp; colour</span>
              <IconPicker icon={item.icon} color={item.color} onpick={patch} />
            </div>
            <div class="aside-field" style="grid-column:1/-1">
              <span class="field-label">Owning team</span>
              <div class="aside-value">
                <span class="code-badge">{item.teamShortCode}</span>
                <span>{team ? team.name : item.teamShortCode}</span>
              </div>
            </div>
          {:else}
            <!-- The bucket is a way through to the bucket. Which bucket a ticket
                 is in is changed by asking for it, not by a click that was aimed
                 at reading the milestone. -->
            <div class="aside-field">
              <span class="field-label">Bucket</span>
              <div style="display:flex;gap:6px">
                {#if item.bucketIdentifier}
                  <a class="aside-control" href="/{item.bucketIdentifier}">
                    <BucketIcon name={item.bucketIcon} size={13} color={item.bucketColor} />
                    <span>{item.bucketTitle}</span>
                    <span class="note">{item.bucketIdentifier}</span>
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
          {/if}

          <div class="aside-field">
            <span class="field-label">Planned end</span>
            <div class="aside-value">
              {#if editing}
                <input class="input" style="width:100%" type="date" aria-label="Planned end"
                       value={item.plannedEnd || ""}
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
          <div class="aside-field">
            <span class="field-label">Actual end</span>
            <div class="aside-value">
              {#if item.actualEnd}<span class="mono">{when(item.actualEnd)}</span>
              {:else}<span class="dash">—</span>{/if}
            </div>
          </div>

          <!-- Labels live on the detail view only. They are a filtering
               mechanism, and cards carrying them become unreadable (SPEC.md §8.1). -->
          <div class="aside-field" style="grid-column:1/-1">
            <span class="field-label">Labels</span>
            <LabelEditor labels={item.labels || []} bind:draft={labelDraft}
                         onadd={(name) => withLabels(() => api.addLabel(item.id, name))}
                         onremove={(name) => withLabels(() => api.removeLabel(item.id, name))} />
          </div>
        </div>

        {#if isBucket}
          <!-- A bucket's progress is a view over its contents, not a field
               (SPEC.md §6.2), shown as a count of items in a terminal state. -->
          <div class="progress-block">
            <div style="display:flex;align-items:baseline;gap:6px;margin-bottom:8px">
              <span class="big">{progress.done}</span>
              <span class="of">/ {progress.total}</span>
              <!-- Open says how much is left; accepted says how much of that
                   anyone has actually picked up. -->
              <span class="ident" style="margin-left:auto">
                {progress.total - progress.done} open, {progress.accepted || 0} accepted
              </span>
            </div>
            <div class="progress">
              <i style="width:{percent}%"></i>
              <i class="accepted" style="width:{started}%"></i>
            </div>
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
              <div class="explain">No tickets.</div>
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
          <RelationList relations={item.relations} onremove={(r) => guard(() => api.removeRelation(r.id))} />
          {#if !item.relations?.length}
            <div class="explain">No relations.</div>
          {/if}
        </section>

        {#if item.pullRequests?.length}
          <section style="margin-top:28px">
            <div class="section-head">
              <span class="eyebrow">Pull requests</span>
              <span class="ident">{item.pullRequests.length}</span>
            </div>
            <PullRequestList pullRequests={item.pullRequests} />
          </section>
        {/if}

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
                    <b><a href="/users/{c.authorName}">{c.authorName}</a></b>
                    <time>{when(c.createdAt, true)}</time>
                    {#if c.updatedAt > c.createdAt}<span class="explain">edited</span>{/if}
                    {#if c.authorId === app.user.id}
                      <button class="btn quiet small" style="margin-left:auto"
                              onclick={() => editComment(c)}>Edit</button>
                    {/if}
                  </div>
                  <Markdown class="comment-body" text={c.body} />
                </div>
              </div>
            {:else}
              <div class="explain">No comments yet.</div>
            {/each}
            <div class="comment">
              <Avatar name={app.user.username} size="comment" />
              <div style="flex:1">
                <textarea class="input" rows="3" placeholder="Write a comment…"
                          bind:value={newComment} use:pictures></textarea>
                <button class="btn primary" style="margin-top:8px" onclick={addComment}>Comment</button>
              </div>
            </div>
          {:else}
            {#each history.events as e}
              <div class="relation-row">
                <span class="ident" style="width:70px;flex:0 0 70px">{e.seq}</span>
                <span style="display:flex;align-items:center;gap:6px;width:150px">
                  <Avatar name={e.actorName} />
                  <span style="font-size:12.5px">{actorName(e)}</span>
                  <!-- History surfaces name the token beside the actor, so an
                       agent's actions are distinguishable (SPEC.md §14.4). -->
                  {#if e.agentName}
                    <span class="explain" style="color:var(--accent)">agent: {e.agentName}</span>
                  {/if}
                </span>
                <span class="field-label" style="width:90px">{fieldName(e)}</span>
                <span style="flex:1;font-size:12.5px">
                  {eventChange(e)}
                </span>
                <span class="ident">{when(e.at, true)}</span>
              </div>
            {:else}
              <div class="explain">No history.</div>
            {/each}
          {/if}
        </section>
      </div>
    </div>
  </div>
{/if}
