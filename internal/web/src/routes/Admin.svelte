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
  import ConfirmDialog from "../dialogs/ConfirmDialog.svelte";

  const SECTIONS = [
    { id: "users", label: "Users" },
    { id: "teams", label: "Teams" },
    { id: "backup", label: "Backup and restore" },
    { id: "agents", label: "Agent access" },
    { id: "updates", label: "Updates" },
    { id: "github", label: "GitHub" },
    { id: "log", label: "Global log", onclick: () => navigate("/log") },
  ];

  // A round trip through GitHub comes back to its section, saying what went
  // wrong when something did.
  const query = new URLSearchParams(location.search);
  let tab = $state(query.get("tab") || "users");
  let returned = $state(query.get("error"));
  if (query.size) history.replaceState(history.state, "", location.pathname);
  let users = $state([]);
  let teams = $state([]);
  let snapshots = $state(null);
  let tokens = $state([]);
  let updates = $state(null);
  let github = $state(null);
  let error = $state(null);
  let loaded = $state(false);

  async function load() {
    try {
      [users, teams, snapshots, tokens, updates, github] = await Promise.all([
        api.listUsers(), api.listTeams(true),
        api.listSnapshots().catch(() => null),
        api.listAgentTokens(0).catch(() => []),
        api.getUpdates().catch(() => null),
        api.getGitHub().catch(() => null),
      ]);
      loaded = true;
      // A newly linked account has never been read: read it now, once.
      for (const i of github?.installations || []) {
        if (!i.syncedAt && !autoSynced.has(i.id)) { autoSynced.add(i.id); sync(i); }
      }
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

  // GitHub (DEV-13).
  let org = $state("");
  let syncing = $state({});
  const autoSynced = new Set();
  const loopback = ["localhost", "127.0.0.1", "[::1]"].includes(location.hostname);

  // GitHub creates the app from a manifest posted by the browser, and sends
  // it back here with the credentials to collect.
  async function createApp() {
    const out = await api.createGitHubApp(org.trim()).catch(failure);
    if (!out) return;
    const form = Object.assign(document.createElement("form"), { method: "post", action: out.action });
    form.append(Object.assign(document.createElement("input"), { type: "hidden", name: "manifest", value: out.manifest }));
    document.body.append(form);
    form.submit();
  }

  async function sync(i) {
    syncing[i.id] = true;
    try { await api.syncInstallation(i.id); await load(); } catch (err) { failure(err); }
    delete syncing[i.id];
  }

  async function forgetApp() {
    const ok = await openModal(ConfirmDialog, {
      title: `Forget ${github.app.slug}`,
      lines: ["Pull requests stop arriving, and every account is unlinked. Those already listed stay on their items.",
        "The app stays on GitHub until you delete it there."],
      consequence: "Creating it again starts over", action: "Forget",
    });
    if (ok) guard(() => api.deleteGitHubApp());
  }

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

  // A moment in this browser's zone, as the datetime input writes it.
  let moment = $state("");
  const local = (t) => {
    const d = new Date(t);
    return new Date(d - d.getTimezoneOffset() * 60e3).toISOString().slice(0, 16);
  };

  async function restore(s) {
    const out = await (s ? api.restoreSnapshot(s.path) : api.restoreAt(new Date(moment).toISOString()))
      .catch(failure);
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
  {#if updates?.newer}
    <div class="banner">
      <span class="grow">Campfire {updates.latest.version} is out. This instance runs {updates.current}.</span>
      <a class="btn small" href={updates.latest.url} target="_blank" rel="noopener">Release notes</a>
    </div>
  {/if}

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
            <Section title="Point in time" sub={snapshots.replica ? "on" : "off"}>
              {#snippet children()}
                {#if !snapshots.replica}
                  <p class="explain">
                    Set <code>snapshots.replica</code> to a directory on another disk or machine, and
                    the database can be restored to any moment, not only to a snapshot.
                  </p>
                {:else if !snapshots.replica.from}
                  <p class="explain">Copying to {snapshots.replica.directory}; nothing to restore to yet.</p>
                {:else}
                  <p class="explain">
                    Copied continuously to {snapshots.replica.directory}. Any moment from
                    {new Date(snapshots.replica.from).toLocaleString()} to
                    {new Date(snapshots.replica.to).toLocaleString()}.
                  </p>
                  <div style="display:flex;gap:8px;align-items:center;margin-top:10px">
                    <input class="input" type="datetime-local" aria-label="Moment to restore to"
                           min={local(snapshots.replica.from)} max={local(snapshots.replica.to)}
                           bind:value={moment} />
                    <button class="btn small" disabled={!moment} onclick={() => restore(null)}>Restore to this moment</button>
                  </div>
                {/if}
              {/snippet}
            </Section>
          {/if}

        {:else if tab === "updates"}
          <Section title="Updates" sub={updates?.check ? "checking daily" : "off"}>
            {#snippet children()}
              <p class="explain">
                This instance runs {updates?.current || "an unknown version"}.
                {#if !updates?.check}
                  Set <code>updates.check</code> to true and restart, and this screen says when a newer
                  release is out.
                {:else if updates.latest}
                  The latest release is {updates.latest.version}{updates.newer ? "" : ", so it is up to date"},
                  checked {new Date(updates.latest.checked).toLocaleString()}.
                {:else if !updates.error}
                  Asking GitHub for the latest release.
                {/if}
              </p>
              {#if updates?.error}
                <p class="explain">The last check failed: {updates.error}.</p>
              {/if}
              {#if updates?.newer}
                <a class="btn small" style="margin-top:10px" href={updates.latest.url}
                   target="_blank" rel="noopener">Release notes for {updates.latest.version}</a>
              {/if}
            {/snippet}
          </Section>

        {:else if tab === "github"}
          <Section title="GitHub" sub={github?.app ? `${github.installations.length} linked` : "off"}>
            {#snippet children()}
              {#if returned}<p class="explain" style="color:var(--alert)">{returned}</p>{/if}
              {#if !github}
                <p class="explain">GitHub is unavailable.</p>
              {:else if !github.app}
                <p class="explain">
                  A pull request whose title starts or ends with an identifier, as in
                  <code>DEV-12: fix it</code> or <code>fix it [DEV-12]</code>, is listed on that item with
                  its state. Campfire reads them as a GitHub App of its own, installed on the accounts
                  and organisations you choose, private repositories included.
                </p>
                {#if loopback}
                  <p class="explain">
                    GitHub can't reach {location.host}, so pull requests will arrive only when you sync.
                    Create the app from the instance's public address to get them as they happen.
                  </p>
                {/if}
                <div style="display:flex;gap:8px;align-items:center;margin-top:10px">
                  <input class="input" style="width:240px" placeholder="Organisation (optional)"
                         aria-label="Organisation to own the app" bind:value={org} />
                  <button class="btn primary small" onclick={createApp}>Create the GitHub App</button>
                </div>
              {:else}
                <p class="explain">
                  <a href={github.app.htmlUrl} target="_blank" rel="noopener">{github.app.slug}</a>,
                  owned by {github.app.owner}. Deliveries go to <code>{github.webhook}</code>.
                </p>
                {#if github.error}<p class="explain">GitHub did not answer: {github.error}</p>{/if}
                <div style="display:flex;gap:8px;margin:10px 0">
                  <button class="btn primary small" onclick={() => location.assign("/api/github/install")}>
                    Link an account or organisation
                  </button>
                  <button class="btn quiet small" onclick={forgetApp}>Forget the app</button>
                </div>
                <table class="table">
                  <thead>
                    <tr>
                      <th>Account</th><th style="width:120px">Kind</th>
                      <th style="width:240px">Last sync</th><th style="width:150px"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each github.installations as i (i.id)}
                      <tr>
                        <td>{i.account}</td>
                        <td class="explain">{i.accountType === "Organization" ? "organisation" : "account"}</td>
                        <td class="explain" title={i.syncError || ""}>
                          {syncing[i.id] ? "syncing…" : i.syncError ? `failed: ${i.syncError}` : ago(i.syncedAt)}
                        </td>
                        <td>
                          <button class="btn quiet small" disabled={syncing[i.id]} onclick={() => sync(i)}>Sync</button>
                          <button class="btn quiet small" onclick={() => guard(() => api.unlinkInstallation(i.id))}>Unlink</button>
                        </td>
                      </tr>
                    {/each}
                    {#each github.available || [] as i (i.id)}
                      <tr>
                        <td>{i.account}</td>
                        <td class="explain">{i.accountType === "Organization" ? "organisation" : "account"}</td>
                        <td class="explain">installed, not linked</td>
                        <td>
                          <button class="btn quiet small" onclick={() => guard(() => api.linkInstallation(i.id))}>Link</button>
                        </td>
                      </tr>
                    {/each}
                    {#if !github.installations.length && !github.available?.length}
                      <tr>
                        <td colspan="4"><div class="empty-note">Not installed on any account yet.</div></td>
                      </tr>
                    {/if}
                  </tbody>
                </table>
              {/if}
            {/snippet}
          </Section>

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
