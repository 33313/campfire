<script>
  // A user as the rest of the instance sees them, at /users/<name>: the
  // picture at a size worth looking at, the teams they are in, when they were
  // last here, and what they have been doing.
  //
  // Reading is instance-wide (SPEC.md §3.2), so this is the same account
  // record every screen already draws a name and a picture from — gathered on
  // one page rather than scattered across the ones that mention them.
  //
  // It is also where you edit your own: this is the page the sidebar goes to,
  // and the picture, the email and the bio are edited where they are read.
  // /account keeps what is not about how you appear — password, second factor,
  // and the tokens that act as you.

  import { api } from "../lib/api.js";
  import { app, reloadContext } from "../lib/state.svelte.js";
  import { toast, failure } from "../lib/ui.svelte.js";
  import { loadBuckets, fieldName, eventChange } from "../lib/events.svelte.js";
  import { when, ago, initials, plural, moreEntries } from "../lib/format.js";
  import { NAV } from "../lib/icons.js";
  import { pickPicture, AVATAR } from "../lib/picture.js";

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

  // Editing your own -----------------------------------------------------

  const BIO = 1000;

  let editing = $state(false);
  let draftEmail = $state("");
  let draftBio = $state("");

  function startEditing() {
    draftEmail = user.email || "";
    draftBio = user.bio || "";
    editing = true;
  }

  // The endpoint takes the profile as a whole, so every write sends all three:
  // saving one without the others would clear them. While the editor is open
  // the drafts are the truth, so a picture chosen mid-edit does not undo what
  // has been typed beside it.
  async function write(avatar) {
    const email = editing ? draftEmail : user.email || "";
    const bio = editing ? draftBio : user.bio || "";
    try {
      await api.updateProfile(email, avatar, bio);
      await load();
      await reloadContext(app.team?.id); // the user list is where avatars are read from
      return true;
    } catch (err) {
      failure(err);
      return false;
    }
  }

  async function save() {
    if (await write(user.avatar || "")) {
      editing = false;
      toast("Saved");
    }
  }

  async function choosePicture(e) {
    try {
      const picture = await pickPicture(e);
      if (picture) write(picture);
    } catch (err) {
      failure(err);
    }
  }
</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if user}
  <ScreenHeader title={user.username}
                sub={user.isAdmin ? "Instance administrator" : "Member"}>
    {#snippet right()}
      {#if isMe && editing}
        <button class="btn" onclick={() => (editing = false)}>Cancel</button>
        <button class="btn primary" disabled={draftBio.length > BIO} onclick={save}>Save</button>
      {:else if isMe}
        <button class="btn small" onclick={startEditing}>
          <Icon body={NAV.pen} size={13} /><span>Edit profile</span>
        </button>
        <a class="btn small" href="/account">
          <Icon body={NAV.settings} size={13} /><span>Settings</span>
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
        <!-- In edit mode the picture is the control: clicking it chooses a new
             one, which is where anyone would click anyway. -->
        {#snippet picture()}
          {#if user.avatar}
            <img class="profile-picture" src={user.avatar} alt="" />
          {:else}
            <span class="profile-picture">{initials(user.username)}</span>
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
          <h1>{user.username}</h1>
          <!-- Email is here because every authenticated user can already read it
               from the user list: permissions govern writing, not reading
               (SPEC.md §3.2). -->
          {#if editing}
            <input class="input" type="email" aria-label="Email" placeholder="you@example.com"
                   style="max-width:320px" bind:value={draftEmail} />
          {:else if user.email}
            <a class="profile-email" href="mailto:{user.email}">{user.email}</a>
          {:else}
            <span class="explain">No email address.</span>
          {/if}
          <div class="profile-meta">
            <span class="field-label">Last seen</span>
            <span class="ident">{user.lastSeenAt ? ago(user.lastSeenAt) : "never"}</span>
          </div>
          {#if editing}
            <textarea class="input" rows="3" aria-label="Bio" style="width:100%;max-width:62ch"
                      placeholder="How to reach you, what you work on — anything you like."
                      bind:value={draftBio}></textarea>
            <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
              <span class="explain" style="flex:0 0 auto">{draftBio.length} / {BIO}</span>
              {#if user.avatar}
                <button class="btn quiet small" onclick={() => write("")}>Remove picture</button>
              {/if}
              <span class="explain">
                Everyone signed in can read this. A picture is scaled to a {AVATAR}px square and
                stored with your account.
              </span>
            </div>
          {:else if user.bio}
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
              {@const team = app.allTeams.find((x) => x.shortCode === t.shortCode)}
              <div class="relation-row">
                <span class="code-badge">{t.shortCode}</span>
                {#if team}
                  <a href="/t/{team.id}/profile"
                     style="flex:1;font-size:12.5px;overflow:hidden;text-overflow:ellipsis">
                    {team.name}
                  </a>
                {:else}
                  <span style="flex:1;font-size:12.5px">{t.shortCode}</span>
                {/if}
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
                {eventChange(e)}
              </span>
              <span class="ident">{when(e.at, true)}</span>
            </div>
          {:else}
            <div class="explain">Nothing yet.</div>
          {/each}
        </div>
        <!-- A glance, not the log: the rest is one click away, already
             filtered to this person. -->
        <p class="explain" style="margin:10px 0 0">
          {#if profile.activityTotal > profile.activity.length}
            {moreEntries(profile.activityTotal - profile.activity.length)} ·
          {/if}
          <a href="/log?actor={user.id}">View all</a>
        </p>
      {/snippet}
    </Section>
  </div>
{/if}
