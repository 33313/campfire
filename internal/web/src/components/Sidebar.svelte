<script>
  import Icon from "./Icon.svelte";
  import Avatar from "./Avatar.svelte";
  import { NAV } from "../lib/icons.js";
  import { app } from "../lib/state.svelte.js";
  import { route, navigate } from "../lib/router.svelte.js";
  import { openModal, failure, toast } from "../lib/ui.svelte.js";
  import { api } from "../lib/api.js";
  import PickDialog from "../dialogs/PickDialog.svelte";
  import SaveViewDialog from "../dialogs/SaveViewDialog.svelte";
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

  function viewHref(view) {
    return `/t/${teamId}/${view.pinBacklog && !view.pinBoard ? "backlog" : "board"}?view=${view.id}`;
  }

  async function switchTeam() {
    const options = app.allTeams.map((t) => ({
      value: t.id, label: `${t.shortCode}  ${t.name}`,
      note: app.teams.some((m) => m.teamId === t.id) ? "member" : "read only",
    }));
    // An instance with no teams has nothing else to offer here, and the
    // administration screen is not where a new administrator looks first.
    if (app.user.isAdmin) options.push({ value: "new", label: "New team" });
    const picked = await openModal(PickDialog, {
      title: app.allTeams.length ? "Switch team" : "No teams yet",
      options,
      explain: app.allTeams.length ? null : app.user.isAdmin
        ? "This instance has no teams. Create the first one to get a board."
        : "You are not in a team yet. An instance administrator creates them.",
    });
    if (picked === "new") await newTeam().catch(failure);
    else if (picked) navigate(`/t/${picked}/board`);
  }

  // Saving from the sidebar saves the filter as it stands on the board — which,
  // from here, is the unfiltered one.
  async function saveView() {
    const res = await openModal(SaveViewDialog);
    if (!res) return;
    try {
      await api.createSavedView({ teamId, ...res, clauses: [] });
      toast("Saved. It is in the sidebar under Views.");
      app.views = await api.listSavedViews(teamId);
    } catch (err) { failure(err); }
  }
</script>

<aside class="sidebar">
  <div class="sidebar-head">
    <div class="wordmark">Campfire</div>
    <button class="team-switch" onclick={switchTeam}>
      <span class="code-badge">{known ? known.shortCode : "—"}</span>
      <span class="team-name">{known ? known.name : "No team yet"}</span>
      <Icon body={NAV.chevron} size={11} color="var(--faint)" />
    </button>
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
        <button class="btn quiet small" title="Save the current filter as a view"
                style="margin-left:auto;height:18px;padding:0 4px" onclick={saveView}>
          <Icon body={NAV.plus} size={11} />
        </button>
      </div>
      {#each app.views as v}
        <a class="nav-item" href={viewHref(v)}>
          <Icon body={NAV.view} size={13} />
          <span class="label">{v.name}</span>
        </a>
      {/each}
    </div>
  {/if}

  <div class="sidebar-foot">
    {#if teamId}
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
    <a class="whoami" href="/account" style="text-decoration:none">
      <Avatar name={app.user.username} size="big" />
      <span class="who">
        <b>{app.user.username}</b>
        <small>{app.user.isAdmin ? "Instance administrator" : "Member"}</small>
      </span>
    </a>
  </div>
</aside>
