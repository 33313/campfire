<script>
  import Icon from "./Icon.svelte";
  import Avatar from "./Avatar.svelte";
  import Mark from "./Mark.svelte";
  import { NAV } from "../lib/icons.js";
  import { app, toggleView } from "../lib/state.svelte.js";
  import { stream } from "../lib/stream.svelte.js";
  import { route, navigate } from "../lib/router.svelte.js";
  import { failure } from "../lib/ui.svelte.js";
  import { api } from "../lib/api.js";
  import { useView, saveNewView, canEditView } from "../lib/views.js";
  import ViewMenu from "./ViewMenu.svelte";
  import { newTeam } from "../lib/teams.js";

  let team = $derived(app.team || app.teams[0]);
  let teamId = $derived(team ? team.teamId || team.id : null);
  let known = $derived(app.allTeams.find((t) => t.id === teamId));

  const NAV_ITEMS = $derived(teamId ? [
    { icon: "board", label: "Board", href: `/t/${teamId}/board` },
    { icon: "backlog", label: "Backlog", href: `/t/${teamId}/backlog`, count: app.backlogCount },
    { icon: "buckets", label: "Buckets", href: "/buckets" },
    { icon: "log", label: "Log", href: "/log" },
  ] : []);

  // A link made before views were switches names one as ?view=: it switches
  // that view on, and the address goes back to the plain screen.
  $effect(() => {
    const id = +route.search.get("view");
    if (!id || !app.views.some((v) => v.id === id)) return;
    toggleView(id, true);
    navigate(route.path, true);
  });

  // The view whose menu is open, and where. A click switches a view on or
  // off; changing or deleting it is behind a right-click (or the keyboard's
  // menu key, which fires the same event at no pointer position).
  let viewMenu = $state(null);
  function openViewMenu(e, view) {
    if (!canEditView(view)) return;
    e.preventDefault();
    const r = e.currentTarget.getBoundingClientRect();
    viewMenu = { view, x: e.clientX || r.left + 16, y: e.clientY || r.bottom };
  }

  // Switching team is a drop-down under the name it changes, not a modal: a
  // modal takes the whole screen away to answer a question about the sidebar,
  // and there is nothing to confirm.
  let open = $state(false);
  let menu = $state(null);

  // A drop-down closes when you look away from it. The pointer check is on the
  // way down and ignores its own panel, or a click on an option would close the
  // menu before the option was chosen.
  //
  // <details> would carry the open state for free, but only hides its panel
  // through a user-agent rule that any display of ours overrides, so the panel
  // is drawn when it is open and absent when it is not.
  $effect(() => {
    if (!open) return;
    const away = (e) => { if (!menu?.contains(e.target)) open = false; };
    addEventListener("pointerdown", away, true);
    return () => removeEventListener("pointerdown", away, true);
  });

  function choose(id) {
    open = false;
    navigate(`/t/${id}/board`);
  }

  async function createTeam() {
    open = false;
    await newTeam().catch(failure);
  }

</script>

<!-- The team's picture stands for it in the picker, or a blank square where
     it has none, so the names line up either way. The switch and every option
     draw the same. -->
{#snippet teamAvatar(t)}
  {#if t?.avatar}
    <img class="team-avatar" src={t.avatar} alt="" />
  {:else}
    <span class="team-avatar"></span>
  {/if}
{/snippet}

<aside class="sidebar">
  <div class="sidebar-head">
    <!-- The fire is lit while the stream is live and flares on each change that
         arrives; it burns down while the stream is away. -->
    <div class="brand" title={stream.status === "live" ? "Live" : stream.status}>
      {#key stream.beat}
        <Mark size={19} lit={stream.status === "live"} crackle={stream.beat > 0} />
      {/key}
      <span class="wordmark">Campfire</span>
    </div>
    <div class="team-menu" bind:this={menu}>
      <button class="team-switch" class:on={open} aria-expanded={open} aria-haspopup="menu"
              onclick={() => (open = !open)}
              onkeydown={(e) => e.key === "Escape" && (open = false)}>
        {@render teamAvatar(known)}
        <span class="team-name">{known ? known.name : "No team yet"}</span>
        <Icon body={NAV.chevron} size={11} color="var(--faint)" />
      </button>
      {#if open}
        <div class="team-list" role="menu">
          {#each app.allTeams as t}
            <button class="team-option" class:on={t.id === teamId} onclick={() => choose(t.id)}>
              {@render teamAvatar(t)}
              <span class="team-name">{t.name}</span>
              <span class="note">
                {app.teams.some((m) => m.teamId === t.id) ? "member" : "read only"}
              </span>
            </button>
          {:else}
            <p class="explain" style="margin:0;padding:8px 9px">
              {app.user.isAdmin
                ? "This instance has no teams. Create the first one to get a board."
                : "You are not in a team yet. An instance administrator creates them."}
            </p>
          {/each}
          <!-- An instance with no teams has nothing else to offer here, and the
               administration screen is not where a new administrator looks first. -->
          {#if app.user.isAdmin}
            <button class="team-option" onclick={createTeam}>
              <Icon body={NAV.plus} size={11} />
              <span class="team-name">New team</span>
            </button>
          {/if}
        </div>
      {/if}
    </div>
  </div>

  {#if teamId}
    <nav class="nav">
      {#each NAV_ITEMS as item}
        <a class="nav-item" class:on={route.path === item.href} href={item.href}>
          <Icon body={NAV[item.icon]} size={15} />
          <span class="label">{item.label}</span>
          {#if item.count > 0}<span class="count">{item.count}</span>{/if}
        </a>
      {/each}
    </nav>

    <div class="nav">
      <div class="nav-section">
        <span class="field-label">Views</span>
        <span style="margin-left:auto"></span>
        {#if app.viewsOn.length > 1}
          <button class="btn quiet small" style="height:18px;padding:0 4px;font-size:11px"
                  onclick={() => app.viewsOn.forEach((id) => toggleView(id, false))}>All off</button>
        {/if}
        <!-- A new view starts from no filter; its dialog picks one. -->
        <button class="btn quiet small" title="New view" aria-label="New view"
                style="height:18px;padding:0 4px" onclick={() => saveNewView(teamId, {})}>
          <Icon body={NAV.plus} size={11} />
        </button>
      </div>
      {#each app.views as v}
        {@const gone = (v.clauses || []).filter((c) => c.missing).map((c) => c.display)}
        {@const lit = app.viewsOn.includes(v.id)}
        <!-- A switch, not a place: Board stays where you are, and the views
             that are on stack onto it. -->
        <button class="nav-item view-switch" class:lit aria-pressed={lit} onclick={() => useView(v)}
                oncontextmenu={(e) => openViewMenu(e, v)}
                title={canEditView(v) ? "Right-click to edit or delete" : null}>
          <Icon body={NAV.view} size={13} />
          <span class="label">{v.name}</span>
          {#if gone.length}
            <span class="count" style="color:var(--alert)" title="Names what was deleted: {gone.join(', ')}">!</span>
          {/if}
          {#if lit}<span class="count" aria-hidden="true">on</span>{/if}
        </button>
      {/each}
    </div>
    {#if viewMenu}
      <ViewMenu view={viewMenu.view} x={viewMenu.x} y={viewMenu.y} onclose={() => (viewMenu = null)} />
    {/if}
  {/if}

  <div class="sidebar-foot">
    {#if teamId}
      <a class="nav-item" class:on={route.path === `/t/${teamId}/profile`} href="/t/{teamId}/profile">
        <Icon body={NAV.team} size={15} />
        <span class="label">Team</span>
      </a>
      <a class="nav-item" class:on={route.path === `/t/${teamId}/settings`} href="/t/{teamId}/settings">
        <Icon body={NAV.settings} size={15} />
        <span class="label">Team settings</span>
      </a>
    {/if}
    {#if app.user.isAdmin}
      <a class="nav-item" class:on={route.path === "/admin"} href="/admin">
        <Icon body={NAV.admin} size={15} />
        <span class="label">Administration</span>
      </a>
    {/if}
    <!-- Your profile, not your settings: the block shows who you are, and that
         is the page it goes to. Settings is a button on the profile. Signing
         out sits beside who you are signed in as. -->
    <div class="whoami">
      <a class="who-link" href="/users/{app.user.username}">
        <Avatar name={app.user.username} size="big" />
        <span class="who">
          <b>{app.user.username}</b>
          <small>{app.user.isAdmin ? "Instance administrator" : "Member"}</small>
        </span>
      </a>
      <button class="btn quiet small" title="Sign out" aria-label="Sign out" style="padding:0 5px"
              onclick={async () => { await api.signOut().catch(() => {}); location.href = "/"; }}>
        <Icon body={NAV.signout} size={13} />
      </button>
    </div>
  </div>
</aside>
