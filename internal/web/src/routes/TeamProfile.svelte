<script>
  // A team as the rest of the instance sees it, at /t/<id>/profile: its
  // picture, what it does, who is in it, and what it has been doing.
  //
  // The same page as a person's profile, for the same reason: reading is
  // instance-wide (SPEC.md §3.2). A team administrator edits the picture and
  // the description where they are read; everything else about a team is in
  // its settings.

  import { api } from "../lib/api.js";
  import { app, reloadContext } from "../lib/state.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { toast, failure } from "../lib/ui.svelte.js";
  import { loadBuckets, fieldName, eventChange } from "../lib/events.svelte.js";
  import { when, plural, moreEntries } from "../lib/format.js";
  import { NAV } from "../lib/icons.js";
  import { pickPicture, AVATAR } from "../lib/picture.js";

  import Icon from "../components/Icon.svelte";
  import Avatar from "../components/Avatar.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import Section from "../components/Section.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";

  let { teamId } = $props();

  let team = $state(null);
  let members = $state([]);
  let activity = $state([]);
  let total = $state(0);
  let error = $state(null);

  async function load() {
    try {
      let log;
      [team, members, log] = await Promise.all([
        api.getTeam(teamId), api.listMembers(teamId), api.getLog({ team: teamId, limit: 30 }),
      ]);
      activity = log.events || [];
      total = log.total || 0;
      error = null;
    } catch (err) {
      error = err;
    }
  }

  load();
  loadBuckets(); // the bucket ids the activity carries resolve against this
  $effect(() => onNotify(load));

  let acting = $derived(app.user.isAdmin
    || app.teams.some((m) => m.teamId === teamId && m.role === "admin"));

  // Editing -----------------------------------------------------------------

  const DESCRIPTION = 1000;

  let editing = $state(false);
  let draft = $state("");

  async function write(body) {
    try {
      await api.updateTeam(teamId, body);
      await load();
      await reloadContext(teamId); // the team list is where the sidebar reads it
      return true;
    } catch (err) {
      failure(err);
      return false;
    }
  }

  async function save() {
    if (await write({ description: draft })) {
      editing = false;
      toast("Saved");
    }
  }

  async function choosePicture(e) {
    try {
      const avatar = await pickPicture(e);
      if (avatar) write({ avatar });
    } catch (err) {
      failure(err);
    }
  }
</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if team}
  <ScreenHeader title={team.name} sub={team.archivedAt ? "Archived team" : "Team"}>
    {#snippet right()}
      {#if editing}
        <button class="btn" onclick={() => (editing = false)}>Cancel</button>
        <button class="btn primary" disabled={draft.length > DESCRIPTION} onclick={save}>Save</button>
      {:else}
        {#if acting}
          <button class="btn small" onclick={() => { draft = team.description || ""; editing = true; }}>
            <Icon body={NAV.pen} size={13} /><span>Edit profile</span>
          </button>
        {/if}
        <a class="btn small" href="/t/{teamId}/settings">
          <Icon body={NAV.settings} size={13} /><span>Settings</span>
        </a>
      {/if}
    {/snippet}
  </ScreenHeader>

  <div class="body profile-body">
    <div class="profile-top measure">
      <div class="profile-head">
        {#snippet picture()}
          {#if team.avatar}
            <img class="profile-picture" src={team.avatar} alt="" />
          {:else}
            <span class="profile-picture">{team.shortCode}</span>
          {/if}
        {/snippet}
        {#if editing}
          <label class="profile-picture-pick" title="Choose a picture">
            {@render picture()}
            <span class="hint">Change</span>
            <input type="file" accept="image/*" aria-label="Picture" hidden
                   onchange={choosePicture} />
          </label>
        {:else}
          {@render picture()}
        {/if}

        <div class="profile-facts">
          <h1>{team.name}</h1>
          <div class="profile-meta">
            <span class="code-badge">{team.shortCode}</span>
            <span class="field-label">Since</span>
            <span class="ident">{when(team.createdAt)}</span>
          </div>
          {#if editing}
            <textarea class="input" rows="3" aria-label="Description"
                      style="width:100%;max-width:62ch"
                      placeholder="What this team works on, and how to reach it."
                      bind:value={draft}></textarea>
            <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
              <span class="explain" style="flex:0 0 auto">{draft.length} / {DESCRIPTION}</span>
              {#if team.avatar}
                <button class="btn quiet small" onclick={() => write({ avatar: "" })}>
                  Remove picture
                </button>
              {/if}
              <span class="explain">
                Everyone signed in can read this. A picture is scaled to a {AVATAR}px square.
              </span>
            </div>
          {:else if team.description}
            <div class="prose profile-bio">{team.description}</div>
          {:else}
            <span class="explain">No description.</span>
          {/if}
          <div style="display:flex;gap:8px;margin-top:4px">
            <a class="btn small" href="/t/{teamId}/board">
              <Icon body={NAV.board} size={13} /><span>Board</span>
            </a>
            <a class="btn small" href="/t/{teamId}/backlog">
              <Icon body={NAV.backlog} size={13} /><span>Backlog</span>
            </a>
          </div>
        </div>
      </div>

      <Section title="Members" sub={plural(members.length, "member")}>
        {#snippet children()}
          <div class="profile-teams">
            {#each members as m (m.userId)}
              <div class="relation-row">
                <Avatar name={m.username} />
                <a href="/users/{encodeURIComponent(m.username)}"
                   style="flex:1;font-size:12.5px;overflow:hidden;text-overflow:ellipsis">
                  {m.username}
                </a>
                <span class="state-badge">{m.role}</span>
              </div>
            {:else}
              <div class="explain">Nobody yet. A team administrator adds people in settings.</div>
            {/each}
          </div>
        {/snippet}
      </Section>
    </div>

    <Section class="profile-activity measure" title="Recent activity"
             sub="{activity.length} newest first">
      {#snippet children()}
        <div class="profile-activity-rows">
          {#each activity as e (e.seq)}
            <div class="relation-row">
              <span class="ident" style="width:74px;flex:0 0 74px">
                {#if e.itemIdentifier}
                  <a href="/{e.itemIdentifier}">{e.itemIdentifier}</a>
                {:else}
                  —
                {/if}
              </span>
              <span class="ident" style="width:90px;flex:0 0 90px;overflow:hidden;text-overflow:ellipsis">
                {e.actorName}
              </span>
              <span class="field-label" style="width:90px;flex:0 0 90px">{fieldName(e)}</span>
              <span style="flex:1;min-width:0;font-size:12.5px;overflow-wrap:anywhere">
                {eventChange(e)}
              </span>
              <span class="ident">{when(e.at, true)}</span>
            </div>
          {:else}
            <div class="explain">Nothing yet.</div>
          {/each}
        </div>
        <!-- A glance, not the log: the rest is one click away, already
             filtered to this team. -->
        <p class="explain" style="margin:10px 0 0">
          {#if total > activity.length}{moreEntries(total - activity.length)} ·{/if}
          <a href="/log?team={teamId}">View all</a>
        </p>
      {/snippet}
    </Section>
  </div>
{/if}
