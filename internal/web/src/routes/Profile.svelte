<script>
  // A user as the rest of the instance sees them, at /users/<name>: the
  // picture at a size worth looking at, the teams they are in, when they were
  // last here, and what they have been doing.
  //
  // Reading is instance-wide (SPEC.md §3.2), so this is the same account
  // record every screen already draws a name and a picture from — gathered on
  // one page rather than scattered across the ones that mention them.

  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { loadBuckets, fieldName, eventValue } from "../lib/events.svelte.js";
  import { when, ago, initials, plural } from "../lib/format.js";
  import { NAV } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import Section from "../components/Section.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";

  let { username } = $props();

  let profile = $state(null);
  let error = $state(null);

  async function load() {
    try {
      profile = await api.getProfile(username);
      error = null;
    } catch (err) {
      error = err;
    }
  }

  load();
  loadBuckets(); // the bucket ids the activity carries resolve against this

  let user = $derived(profile?.user);
  let isMe = $derived(user && user.id === app.user.id);
</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if user}
  <ScreenHeader title={user.username}
                sub={user.isAdmin ? "Instance administrator" : "Member"}>
    {#snippet right()}
      {#if isMe}
        <a class="btn small" href="/account">
          <Icon body={NAV.settings} size={13} /><span>Your account</span>
        </a>
      {/if}
    {/snippet}
  </ScreenHeader>

  <!-- Who they are stays on screen; only the activity scrolls. A profile that
       scrolls as a whole buries the picture, the bio and the teams under a
       list of events, which is the wrong way round. -->
  <div class="body profile-body">
    <div class="profile-top measure">
      <div class="profile-head">
        {#if user.avatar}
          <img class="profile-picture" src={user.avatar} alt="" />
        {:else}
          <span class="profile-picture">{initials(user.username)}</span>
        {/if}
        <div class="profile-facts">
          <h1>{user.username}</h1>
          <!-- Email is here because every authenticated user can already read it
               from the user list: permissions govern writing, not reading
               (SPEC.md §3.2). -->
          {#if user.email}
            <a class="profile-email" href="mailto:{user.email}">{user.email}</a>
          {:else}
            <span class="explain">No email address.</span>
          {/if}
          <div class="profile-meta">
            <span class="field-label">Last seen</span>
            <span class="ident">{user.lastSeenAt ? ago(user.lastSeenAt) : "never"}</span>
          </div>
          {#if user.bio}
            <div class="prose profile-bio">{user.bio}</div>
          {:else}
            <span class="explain">No bio.</span>
          {/if}
        </div>
      </div>

      <Section title="Teams" sub={plural((user.teams || []).length, "membership")}>
        {#snippet children()}
          <!-- Side by side rather than stacked: the memberships are a fact
               about a person, not a list to read down, and the page has to
               finish above the fold. -->
          <div class="profile-teams">
            {#each user.teams || [] as t}
              <div class="relation-row">
                <span class="code-badge">{t.shortCode}</span>
                <span style="flex:1;font-size:12.5px;overflow:hidden;text-overflow:ellipsis">
                  {app.allTeams.find((x) => x.shortCode === t.shortCode)?.name || t.shortCode}
                </span>
                <span class="state-badge">{t.role}</span>
              </div>
            {:else}
              <div class="explain">In no team yet. An instance administrator adds people to one.</div>
            {/each}
          </div>
        {/snippet}
      </Section>
    </div>

    <Section class="profile-activity measure" title="Recent activity"
             sub="{profile.activity.length} newest first">
      {#snippet children()}
        <!-- The same two helpers the log, the detail view and the preview read
             events through, so the four cannot drift. -->
        <div class="profile-activity-rows">
          {#each profile.activity as e}
            <div class="relation-row">
              <span class="ident" style="width:74px;flex:0 0 74px">
                {#if e.itemIdentifier}
                  <a href="/{e.itemIdentifier}">{e.itemIdentifier}</a>
                {:else}
                  —
                {/if}
              </span>
              <span class="field-label" style="width:90px;flex:0 0 90px">{fieldName(e)}</span>
              <span style="flex:1;min-width:0;font-size:12.5px;overflow-wrap:anywhere">
                {e.oldValue ? `${eventValue(e, e.oldValue)} → ` : ""}{e.newValue ? eventValue(e, e.newValue) : ""}
              </span>
              <span class="ident">{when(e.at, true)}</span>
            </div>
          {:else}
            <div class="explain">Nothing yet.</div>
          {/each}
        </div>
        <p class="explain" style="margin:10px 0 0">
          A glance, not the log. <a href="/log">The log</a> is where you read further.
        </p>
      {/snippet}
    </Section>
  </div>
{/if}
