<script>
  // Instance administration.

  import { api } from "../lib/api.js";
  import { navigate } from "../lib/router.svelte.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { openModal, failure } from "../lib/ui.svelte.js";
  import { newTeam } from "../lib/teams.js";
  import { when, ago } from "../lib/format.js";
  import { NAV } from "../lib/icons.js";

  import Icon from "../components/Icon.svelte";
  import Avatar from "../components/Avatar.svelte";
  import Section from "../components/Section.svelte";
  import SettingsRail from "../components/SettingsRail.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";
  import FormDialog from "../dialogs/FormDialog.svelte";
  import ShowOnceDialog from "../dialogs/ShowOnceDialog.svelte";

  const SECTIONS = [
    { id: "users", label: "Users" },
    { id: "teams", label: "Teams" },
    { id: "backup", label: "Backup and restore" },
    { id: "agents", label: "Agent access" },
    { id: "log", label: "Global log", onclick: () => navigate("/log") },
  ];

  let tab = $state("users");
  let users = $state([]);
  let teams = $state([]);
  let snapshots = $state(null);
  let tokens = $state([]);
  let error = $state(null);
  let loaded = $state(false);

  async function load() {
    try {
      [users, teams, snapshots, tokens] = await Promise.all([
        api.listUsers(), api.listTeams(true),
        api.listSnapshots().catch(() => null),
        api.listAgentTokens(0).catch(() => []),
      ]);
      loaded = true;
      error = null;
    } catch (err) {
      error = err;
    }
  }

  load();
  $effect(() => onNotify(load));

  const guard = async (fn) => {
    try { await fn(); await load(); } catch (err) { failure(err); }
  };

  const usernameOf = (id) => users.find((u) => u.id === id)?.username || `user ${id}`;

  async function newAccount() {
    const res = await openModal(FormDialog, {
      title: "New account", action: "Create",
      fields: [
        { name: "username", label: "Username" },
        { name: "password", label: "Password", type: "password" },
        { name: "email", label: "Email — optional", type: "email" },
        { name: "isAdmin", label: "Instance administrator", type: "checkbox", value: false },
      ],
    });
    if (res) guard(() => api.createUser(res));
  }

  // A one-time link is shown once; only its hash is kept.
  function showLink(title, invitation) {
    return openModal(ShowOnceDialog, {
      title,
      explain: "Shown once. Hand it over out of band; it expires in seven days.",
      value: `${location.origin}/invite/${invitation.token}`,
    });
  }

  async function restore(s) {
    const out = await api.restoreSnapshot(s.path).catch(failure);
    if (!out) return;
    await openModal(ShowOnceDialog, {
      title: "Restore from a shell", explain: out.note, value: out.command,
    });
  }
</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if loaded}
  <ScreenHeader title="Administration" sub={location.host} badge="Instance administrator" />

  <div class="body">
    <SettingsRail sections={SECTIONS} active={tab} onpick={(t) => (tab = t)}>
      {#snippet children()}
        {#if tab === "users"}
          <Section title="Users" sub="{users.length} accounts">
            {#snippet children()}
              <p class="explain">
                No open registration. Create an account here, or send an invitation link.
                Resets are one-time links, because no flow depends on email delivery.
              </p>
              <div style="display:flex;gap:8px;margin:10px 0">
                <button class="btn primary small" onclick={newAccount}>New account</button>
                <button class="btn small"
                        onclick={() => guard(async () => showLink("Invitation", await api.createInvitation()))}>
                  Invitation link
                </button>
              </div>
              <table class="table">
                <thead>
                  <tr>
                    <th>Username</th>
                    <th style="width:180px">Email</th>
                    <th style="width:140px">Instance role</th>
                    <th style="width:200px">Teams</th>
                    <th style="width:70px">2FA</th>
                    <th style="width:100px">Last seen</th>
                    <th style="width:130px"></th>
                  </tr>
                </thead>
                <tbody>
                  {#each users as u (u.id)}
                    <tr>
                      <td>
                        <span style="display:flex;align-items:center;gap:8px">
                          <Avatar name={u.username} /><span>{u.username}</span>
                        </span>
                      </td>
                      <td class="explain">{u.email || "no email"}</td>
                      <td>
                        <select class="input" style="height:26px" aria-label="Role for {u.username}"
                                onchange={(e) => guard(() => api.updateUser(u.id, e.currentTarget.value === "admin"))}>
                          <option value="member" selected={!u.isAdmin}>Member</option>
                          <option value="admin" selected={u.isAdmin}>Administrator</option>
                        </select>
                      </td>
                      <!-- A membership is "SOL admin"; the code is what the rest
                           of the interface names a team by. -->
                      <td class="explain">
                        {(u.teams || []).length
                          ? u.teams.map((t) => `${t.shortCode} ${t.role}`).join(", ")
                          : "no teams"}
                      </td>
                      <td>
                        {#if u.hasTotp}
                          <span style="display:flex;align-items:center;gap:6px">
                            <span class="pip final"></span><span class="explain">TOTP</span>
                          </span>
                        {:else}
                          <span class="field-label">off</span>
                        {/if}
                      </td>
                      <td class="num" class:dash={!u.lastSeenAt}>{ago(u.lastSeenAt)}</td>
                      <td>
                        <button class="btn quiet small"
                                onclick={() => guard(async () =>
                                  showLink(`Reset for ${u.username}`, await api.createPasswordReset(u.id)))}>
                          Reset password
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/snippet}
          </Section>

        {:else if tab === "teams"}
          <Section title="Teams" sub="{teams.length} teams">
            {#snippet children()}
              <button class="btn primary small" style="margin-bottom:10px"
                      onclick={() => guard(newTeam)}>New team</button>
              <table class="table">
                <thead>
                  <tr>
                    <th style="width:70px">Code</th><th>Name</th>
                    <th style="width:120px">Status</th><th style="width:120px"></th>
                  </tr>
                </thead>
                <tbody>
                  {#each teams as t (t.id)}
                    <tr>
                      <td><span class="code-badge">{t.shortCode}</span></td>
                      <td>{t.name}</td>
                      <td class="explain">{t.archivedAt ? "archived" : "active"}</td>
                      <td><a class="btn quiet small" href="/t/{t.id}/settings">Settings</a></td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/snippet}
          </Section>

        {:else if tab === "backup"}
          {#if !snapshots}
            <Section title="Backup and restore">
              {#snippet children()}<p class="explain">Snapshots are unavailable.</p>{/snippet}
            </Section>
          {:else}
            <Section title="Snapshots" sub={snapshots.enabled ? "on" : "off"}>
              {#snippet children()}
                <p class="explain">
                  Every {snapshots.interval} · keep {snapshots.keep} · {snapshots.directory}
                  {snapshots.shipTo ? ` · shipped to ${snapshots.shipTo}` : ""}
                </p>
                <button class="btn small" style="margin:10px 0"
                        onclick={() => guard(() => api.createSnapshot())}>Take one now</button>
                <table class="table">
                  <thead>
                    <tr>
                      <th>Taken</th><th style="width:110px">Size</th>
                      <th style="width:110px">Integrity</th><th style="width:110px"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each snapshots.snapshots || [] as s (s.path)}
                      <tr>
                        <td class="num">{new Date(s.taken).toLocaleString()}</td>
                        <td class="num">{(s.size / 1e6).toFixed(1)} MB</td>
                        <td>
                          <span style="display:flex;align-items:center;gap:6px">
                            <span class="pip {s.verified ? 'final' : 'rejected'}"></span>
                            <span class="explain">{s.verified ? "verified" : "unreadable"}</span>
                          </span>
                        </td>
                        <td>
                          <button class="btn quiet small" onclick={() => restore(s)}>Restore</button>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/snippet}
            </Section>
          {/if}

        {:else}
          <Section title="Agent access" sub="{tokens.length} live tokens">
            {#snippet children()}
              <p class="explain">
                A token names a user and carries no permissions of its own; every call
                resolves that user's current permissions. Revoking takes effect on the next call.
              </p>
              <table class="table">
                <thead>
                  <tr>
                    <th>Token</th><th style="width:180px">Acts as</th>
                    <th style="width:160px">Last used</th><th style="width:100px"></th>
                  </tr>
                </thead>
                <tbody>
                  {#each tokens as t (t.id)}
                    <tr>
                      <td>
                        <span style="display:flex;align-items:center;gap:8px">
                          <Icon body={NAV.agent} size={13} color="var(--accent)" />
                          <span>{t.name}</span>
                        </span>
                      </td>
                      <td>
                        <span style="display:flex;align-items:center;gap:8px">
                          <Avatar name={usernameOf(t.userId)} /><span>{usernameOf(t.userId)}</span>
                        </span>
                      </td>
                      <td class="explain">{t.lastUsedAt ? when(t.lastUsedAt, true) : "never"}</td>
                      <td>
                        <button class="btn quiet small"
                                onclick={() => guard(() => api.revokeAgentToken(t.id))}>Revoke</button>
                      </td>
                    </tr>
                  {:else}
                    <tr>
                      <td colspan="4"><div class="empty-note">No agent tokens have been issued.</div></td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/snippet}
          </Section>
        {/if}
      {/snippet}
    </SettingsRail>
  </div>
{/if}
