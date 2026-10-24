<script>
  // Team settings.
  //
  // There is no ticket types tab, and there never will be: a ticket is
  // described by the bucket it sits in, its title and its description
  // (SPEC.md §5.3).

  import { api } from "../lib/api.js";
  import { app, reloadContext } from "../lib/state.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { openModal, failure } from "../lib/ui.svelte.js";
  import { userOptions } from "../lib/options.js";
  import { NAV } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import Avatar from "../components/Avatar.svelte";
  import StatePip from "../components/StatePip.svelte";
  import RoleBadge from "../components/RoleBadge.svelte";
  import Section from "../components/Section.svelte";
  import SettingsRail from "../components/SettingsRail.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";
  import PickDialog from "../dialogs/PickDialog.svelte";
  import FormDialog from "../dialogs/FormDialog.svelte";
  import ConfirmDialog from "../dialogs/ConfirmDialog.svelte";

  const SECTIONS = [
    { id: "states", label: "Work states" },
    { id: "members", label: "Members" },
    { id: "identity", label: "Identity" },
    { id: "archive", label: "Archive" },
  ];
  const ROLES = ["", "default", "accepted", "final", "rejected"];

  let { teamId } = $props();

  let team = $state(null);
  let states = $state([]);
  let members = $state([]);
  let error = $state(null);
  let tab = $state("states");
  let teamName = $state("");
  let loadedName = "";
  let dragged = null;

  async function load() {
    try {
      [team, states, members] = await Promise.all([
        api.getTeam(teamId), api.listWorkStates(teamId), api.listMembers(teamId),
      ]);
      // A live notification reloads this screen. Follow the server only while
      // the field is untouched, or a rename in progress is typed over.
      if (teamName === loadedName) teamName = team.name;
      loadedName = team.name;
      error = null;
    } catch (err) {
      error = err;
    }
  }

  load();
  // Its own team's changes, and the users its members are.
  $effect(() => onNotify(load, (n) => !n.teamId || n.teamId === teamId));

  const guard = async (fn) => {
    try { await fn(); await load(); } catch (err) { failure(err); }
  };

  let mine = $derived(members.find((m) => m.userId === app.user.id));
  let acting = $derived(app.user.isAdmin || mine?.role === "admin");
  let board = $derived(states.filter((s) => s.role !== "default" && s.role !== "rejected"));
  let defaultName = $derived(states.find((s) => s.role === "default")?.name || "—");
  let rejectedName = $derived(states.find((s) => s.role === "rejected")?.name || "—");

  function appearsIn(s) {
    if (s.role === "default") return "Backlog view";
    if (s.role === "rejected") return "Hidden; find by filter or link";
    return `Board column ${board.indexOf(s) + 1}`;
  }

  // A drop renumbers every state whose position changed. The role is sent back
  // unchanged: the handler refuses any change leaving a role unfilled.
  function dropOn(target) {
    if (!dragged || dragged.id === target.id) return;
    const order = states.filter((s) => s.id !== dragged.id);
    order.splice(order.indexOf(target), 0, dragged);
    guard(async () => {
      for (let i = 0; i < order.length; i++) {
        if (order[i].position !== i + 1) {
          await api.updateWorkState(order[i].id, { role: order[i].role, position: i + 1 });
        }
      }
    });
  }

  async function editState(existing) {
    const res = await openModal(FormDialog, {
      title: existing ? `Edit ${existing.name}` : "Add a work state",
      fields: [
        { name: "name", label: "Name", value: existing?.name || "" },
        { name: "role", label: "Role", type: "select", value: existing?.role || "",
          options: ROLES.map((r) => ({ value: r, label: r || "intermediate" })) },
      ],
      explain: "Each role belongs to exactly one state. Assigning it here takes it from the state that has it now.",
    });
    if (!res) return;
    guard(() => existing
      ? api.updateWorkState(existing.id, { name: res.name, role: res.role, position: 0 })
      : api.createWorkState(teamId, res.name, res.role));
  }

  async function addMember() {
    const picked = await openModal(PickDialog, { title: "Add a member", options: userOptions() });
    if (picked) guard(() => api.setMember(teamId, picked, "member"));
  }

  async function toggleArchive() {
    if (!team.archivedAt) {
      const ok = await openModal(ConfirmDialog, {
        title: `Archive ${team.name}`,
        lines: ["The short code is never reused afterwards."],
        consequence: "Reversible", action: "Archive",
      });
      if (!ok) return;
    }
    guard(async () => {
      await api.archiveTeam(teamId, !team.archivedAt);
      await reloadContext();
    });
  }
</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if team}
  <ScreenHeader title="Team settings" sub="{team.name} · {team.shortCode}"
                badge={acting ? "Acting as team administrator" : null} />

  <div class="body">
    <SettingsRail sections={SECTIONS} active={tab} onpick={(t) => (tab = t)}>
      {#snippet children()}
        {#if tab === "states"}
          <div style="display:flex;align-items:flex-start">
            <div style="flex:1;min-width:0">
              <Section title="Work states" sub="{states.length} states · drag to reorder">
                {#snippet children()}
                  <table class="table">
                    <thead>
                      <tr>
                        <th style="width:30px"></th>
                        <th>State</th>
                        <th style="width:130px">Role</th>
                        <th style="width:180px">Appears in</th>
                        <th style="width:90px"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each states as s (s.id)}
                        <tr draggable="true"
                            ondragstart={(e) => { dragged = s; e.dataTransfer.effectAllowed = "move"; }}
                            ondragover={(e) => e.preventDefault()}
                            ondrop={(e) => { e.preventDefault(); dropOn(s); }}>
                          <td style="width:30px;cursor:grab;color:var(--ghost)">≡</td>
                          <td>
                            <span style="display:flex;align-items:center;gap:8px">
                              <StatePip role={s.role} /><span>{s.name}</span>
                            </span>
                          </td>
                          <td><RoleBadge role={s.role} /></td>
                          <td class="explain">{appearsIn(s)}</td>
                          <td>
                            <div style="display:flex;gap:4px;justify-content:flex-end">
                              <button class="btn quiet small" title="Edit" onclick={() => editState(s)}>
                                <Icon body={NAV.pen} size={13} />
                              </button>
                              {#if !s.role}
                                <button class="btn quiet small" title="Delete"
                                        onclick={() => guard(() => api.deleteWorkState(s.id))}>
                                  <Icon body={NAV.close} size={11} />
                                </button>
                              {/if}
                            </div>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                  <button class="btn small" style="margin-top:10px" onclick={() => editState(null)}>
                    <Icon body={NAV.plus} size={11} /><span>Add state</span>
                  </button>
                {/snippet}
              </Section>
            </div>

            <aside style="width:280px;flex:0 0 280px;margin:18px 20px 0 0;border:1px solid var(--border);background:var(--surface);padding:14px;display:flex;flex-direction:column;gap:12px">
              <span class="eyebrow">Result</span>
              <div style="display:flex;flex-direction:column;gap:6px">
                <span class="field-label">Board</span>
                <div style="display:flex;gap:6px;flex-wrap:wrap">
                  {#each board as s}<span class="chip">{s.name}</span>{/each}
                </div>
              </div>
              <div style="display:flex;flex-direction:column;gap:6px">
                <span class="field-label">Backlog</span>
                <div><span class="chip">{defaultName}</span></div>
              </div>
              <div style="display:flex;flex-direction:column;gap:6px">
                <span class="field-label">Off the board</span>
                <div>
                  <span class="chip" style="border-style:dashed">
                    {rejectedName} — hidden; find by filter or link
                  </span>
                </div>
              </div>
            </aside>
          </div>

        {:else if tab === "members"}
          <Section title="Members" sub="{members.length} in this team">
            {#snippet children()}
              <table class="table">
                <thead>
                  <tr><th>Member</th><th style="width:140px">Role</th><th style="width:160px"></th></tr>
                </thead>
                <tbody>
                  {#each members as m (m.userId)}
                    <tr>
                      <td>
                        <span style="display:flex;align-items:center;gap:8px">
                          <Avatar name={m.username} /><span>{m.username}</span>
                        </span>
                      </td>
                      <td>
                        <select class="input" style="height:26px" aria-label="Role for {m.username}"
                                onchange={(e) => guard(() => api.setMember(teamId, m.userId, e.currentTarget.value))}>
                          {#each ["member", "admin"] as r}
                            <option value={r} selected={m.role === r}>{r}</option>
                          {/each}
                        </select>
                      </td>
                      <td>
                        <button class="btn quiet small"
                                onclick={() => guard(() => api.removeMember(teamId, m.userId))}>
                          Remove
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
              <button class="btn small" style="margin-top:10px" onclick={addMember}>Add a member</button>
            {/snippet}
          </Section>

        {:else if tab === "identity"}
          <Section title="Identity">
            {#snippet children()}
              <label class="field">
                <span class="field-label">Name</span>
                <input class="input" bind:value={teamName} />
              </label>
              <div class="field">
                <span class="field-label">Short code</span>
                <div style="display:flex;align-items:center;gap:8px">
                  <span class="code-badge">{team.shortCode}</span>
                  <span class="explain">
                    The identifier prefix. It cannot be changed or reused.
                  </span>
                </div>
              </div>
              <button class="btn primary" style="margin-top:12px;align-self:flex-start"
                      onclick={() => guard(async () => {
                        await api.updateTeam(teamId, { name: teamName });
                        await reloadContext(teamId);
                      })}>Save</button>
            {/snippet}
          </Section>

        {:else}
          <Section title="Archive">
            {#snippet children()}
              <p class="explain">
                Archiving hides the team from the switcher. Its items are kept.
              </p>
              {#if app.user.isAdmin}
                <button class="btn" class:destructive={!team.archivedAt} onclick={toggleArchive}>
                  {team.archivedAt ? "Restore this team" : "Archive this team"}
                </button>
              {:else}
                <p class="explain">Only an instance administrator can archive a team.</p>
              {/if}
            {/snippet}
          </Section>
        {/if}
      {/snippet}
    </SettingsRail>
  </div>
{/if}
