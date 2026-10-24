<script>
  // Instance administration.

  import { api } from "../lib/api.js";
  import { onNotify } from "../lib/stream.svelte.js";
  import { openModal, failure, toast } from "../lib/ui.svelte.js";
  import { newTeam } from "../lib/teams.js";
  import { when, ago } from "../lib/format.js";
  import { NAV, BUCKET_ICON_GROUPS, BUCKET_ICONS, BUCKET_COLOURS } from "../lib/icons.js";
  import { app, setIconSets } from "../lib/state.svelte.js";

  import Icon from "../components/Icon.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
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
    { id: "appearance", label: "Appearance" },
    { id: "backup", label: "Backup and restore" },
    { id: "updates", label: "Updates" },
    { id: "integrations", label: "Integrations" },
    { id: "preferences", label: "Preferences" },
  ];

  const query = new URLSearchParams(location.search);
  let tab = $state(query.get("tab") || "users");
  if (query.size) history.replaceState(history.state, "", location.pathname);
  let users = $state([]);
  let teams = $state([]);
  let snapshots = $state(null);
  let tokens = $state([]);
  let updates = $state(null);
  let forges = $state(null);
  let data = $state(null);
  let error = $state(null);
  let loaded = $state(false);

  async function load() {
    try {
      [users, teams, snapshots, tokens, updates, forges, data] = await Promise.all([
        api.listUsers(), api.listTeams(true),
        api.listSnapshots().catch(() => null),
        api.listAgentTokens(0).catch(() => []),
        api.getUpdates().catch(() => null),
        api.listForges().catch(() => null),
        api.getData().catch(() => null),
      ]);
      loaded = true;
      // A new forge has never been read: read it now, once.
      for (const f of forges || []) {
        if (!f.syncedAt && !autoSynced.has(f.id)) { autoSynced.add(f.id); sync(f); }
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

  // What the bucket pickers offer (DEV-79). Hiding leaves a picker only: a
  // bucket that wears a hidden icon or colour keeps it.
  const GROUP_NAMES = { planning: "Planning", signals: "Signals", work: "Work", places: "Places" };
  const MODES = [
    ["palette", "Palette", "The 16 palette colours, minus any hidden below."],
    ["hex", "Palette and hex", "Also any #rrggbb with enough contrast."],
    ["off", "Off", "No colours offered. Buckets keep theirs."],
  ];
  const offered = $derived(Object.entries(BUCKET_ICON_GROUPS)
    .filter(([g]) => !app.appearance.hiddenGroups.includes(g))
    .reduce((n, [, set]) => n + Object.keys(set).length, 0));
  const flip = (list, key) => (list.includes(key) ? list.filter((k) => k !== key) : [...list, key]);
  async function setLooks(body) {
    try { app.appearance = await api.setAppearance(body); } catch (err) { failure(err); }
  }

  // The instance's own icon sets. A prefix is the administrator's to choose:
  // it is the first half of every key in the set, and never changes.
  let setName = $state("");
  let setPrefix = $state("");
  let setFiles = $state([]);
  let fileInput = $state(null);
  let skipped = $state([]);
  const reloadSets = async () => setIconSets((await api.getInstance()).iconSets);
  async function addSet(e) {
    e.preventDefault();
    try {
      const files = await Promise.all(setFiles.map(async (f) => ({ name: f.name, svg: await f.text() })));
      const res = await api.createIconSet({ name: setName, prefix: setPrefix, files });
      skipped = res.skipped;
      setName = setPrefix = "";
      setFiles = [];
      fileInput.value = "";
      await reloadSets();
    } catch (err) { failure(err); }
  }
  async function hideSet(set) {
    try { await api.updateIconSet(set.id, { hidden: !set.hidden }); await reloadSets(); } catch (err) { failure(err); }
  }
  async function deleteSet(set) {
    const ok = await openModal(ConfirmDialog, {
      title: `Delete ${set.name}`,
      lines: [`Buckets using its ${set.icons.length} icons show the default icon until a set with the prefix ${set.prefix} is added again.`],
      consequence: "Removed from every picker", action: "Delete",
    });
    if (ok) try { await api.deleteIconSet(set.id); await reloadSets(); } catch (err) { failure(err); }
  }

  // Forges (DEV-114).
  const FORGES = {
    github: { name: "GitHub", token: "Token scope: pull requests read (classic: repo).",
      hook: "Add the webhook to an organisation or repository (Settings → Webhooks), with Pull requests events. Personal GitHub accounts have no webhooks; use polling." },
    forgejo: { name: "Forgejo", token: "Token scopes: read:repository, read:user.",
      hook: "Add the webhook under Settings → Webhooks on your profile, an organisation or a repository, with Pull request events." },
    gitlab: { name: "GitLab", token: "Token scope: read_api.",
      hook: "Add the webhook to a project or group (Settings → Webhooks), with Merge request events." },
  };
  // What the form offers: the hosted forges, whose address the server knows,
  // and the self-hosted ones, at the one you give.
  const CHOICES = [
    { id: "github", kind: "github", name: "GitHub", hosted: true },
    { id: "gitlab", kind: "gitlab", name: "GitLab", hosted: true },
    { id: "codeberg", kind: "forgejo", name: "Codeberg", hosted: true },
    { id: "forgejo", kind: "forgejo", name: "Forgejo or Gitea" },
    { id: "gitlab-own", kind: "gitlab", name: "GitLab, self-hosted" },
    { id: "github-own", kind: "github", name: "GitHub Enterprise" },
  ];
  // GitHub and GitLab are polled or send webhooks. Forgejo takes webhooks
  // everywhere, profiles included, so it only sends them.
  const pollable = (kind) => kind !== "forgejo";
  let choice = $state(CHOICES[0]);
  let forgeURL = $state("");
  let forgeToken = $state("");
  let mode = $state("poll");
  let pollMinutes = $state(5);
  let syncing = $state({});
  const autoSynced = new Set();
  const hookOf = (f) => `${location.origin}/api/forges/${f.id}/webhook`;
  const copy = (text) => { navigator.clipboard?.writeText(text); toast("Copied"); };

  async function addForge(e) {
    e.preventDefault();
    try {
      const how = pollable(choice.kind) ? mode : "webhook";
      const f = await api.addForge({ kind: choice.kind, mode: how, url: choice.hosted ? "" : forgeURL,
        token: forgeToken, pollMinutes: how === "poll" ? pollMinutes : 0 });
      forgeToken = "";
      await load();
      if (f.webhookSecret) showSecret(f, f.webhookSecret);
    } catch (err) { failure(err); }
  }

  // The webhook secret is shown once; rotating makes a new one.
  function showSecret(f, secret) {
    return openModal(ShowOnceDialog, {
      title: "Webhook secret",
      explain: `Shown once. Use it with ${hookOf(f)}.`,
      value: secret,
    });
  }

  async function rotate(f) {
    const ok = await openModal(ConfirmDialog, {
      title: `New secret for ${f.url}`,
      lines: ["The current secret stops working. Put the new one in the forge's webhook."],
      consequence: "Deliveries fail until you do", action: "Rotate",
    });
    if (!ok) return;
    try { showSecret(f, (await api.rotateForgeSecret(f.id)).webhookSecret); } catch (err) { failure(err); }
  }

  async function sync(f) {
    syncing[f.id] = true;
    try { await api.syncForge(f.id); await load(); } catch (err) { failure(err); }
    delete syncing[f.id];
  }

  async function newToken(f) {
    const res = await openModal(FormDialog, {
      title: `New token for ${f.url}`, action: "Replace",
      fields: [{ name: "token", label: "Access token", type: "password", explain: FORGES[f.kind].token }],
    });
    if (res) guard(() => api.setForgeToken(f.id, res.token));
  }

  async function removeForge(f) {
    const ok = await openModal(ConfirmDialog, {
      title: `Remove ${f.url}`,
      lines: ["New pull requests stop arriving; listed ones stay.",
        "Delete its webhook and revoke the token on the forge."],
      consequence: "Adding it again makes a new webhook", action: "Remove",
    });
    if (ok) guard(() => api.deleteForge(f.id));
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
      explain: "Shown once. Expires in seven days.",
      value: `${location.origin}/invite/${invitation.token}`,
    });
  }

  // Moving the data restarts the server (DEV-116). Poll until it answers,
  // then show the data directory.
  let moveTo = $state("");
  let moving = $state(false);
  async function moveData(e) {
    e.preventDefault();
    const to = moveTo.trim();
    const ok = await openModal(ConfirmDialog, {
      title: `Move the data to ${to}`,
      lines: [`Moves the database${data.movesSnapshots ? " and its snapshots" : ""} and restarts the server.`,
        `The old files in ${data.directory} are kept. Delete them once everything works.`],
      consequence: "Everyone is disconnected for a moment", action: "Move",
    });
    if (!ok) return;
    try { await api.moveData(to); } catch (err) { failure(err); return; }
    moving = true;
    const was = data.directory;
    for (let i = 0; i < 120; i++) {
      await new Promise((r) => setTimeout(r, 1000));
      const now = await api.getData().catch(() => null);
      if (!now) continue;
      moving = false;
      await load();
      if (now.directory === was) failure("The data was not moved. See the server log.");
      else { moveTo = ""; toast(`Data moved to ${now.directory}`); }
      return;
    }
    moving = false;
    failure("The server has not come back. Check its log.");
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
                            <span class="pip final"></span><span class="explain">on</span>
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

        {:else if tab === "appearance"}
          <Section title="Bucket icons" sub="{offered} of {Object.keys(BUCKET_ICONS).length} offered">
            {#snippet children()}
              <p class="explain">Hidden groups leave the pickers. Buckets keep their icons.</p>
              <table class="table" style="margin-top:8px">
                <tbody>
                  {#each Object.entries(BUCKET_ICON_GROUPS) as [group, set] (group)}
                    {@const on = !app.appearance.hiddenGroups.includes(group)}
                    <tr>
                      <td style="width:46px">
                        <button class="toggle" class:on aria-label="Offer {GROUP_NAMES[group]}" aria-pressed={on}
                                onclick={() => setLooks({ hiddenGroups: flip(app.appearance.hiddenGroups, group) })}><i></i></button>
                      </td>
                      <td style="width:110px">{GROUP_NAMES[group]}</td>
                      <td>
                        <span style="display:flex;gap:8px;flex-wrap:wrap;opacity:{on ? 1 : 0.35}">
                          {#each Object.keys(set) as name (name)}<BucketIcon {name} size={14} />{/each}
                        </span>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/snippet}
          </Section>
          <Section title="Icon sets" sub="{app.iconSets.length} custom sets">
            {#snippet children()}
              <p class="explain">
                SVG icons under a prefix, keyed <code>prefix.name</code> by file name.
                Only stroked shapes are kept; other files are skipped.
              </p>
              {#if app.iconSets.length}
                <table class="table" style="margin-top:8px">
                  <tbody>
                    {#each app.iconSets as set (set.id)}
                      <tr>
                        <td style="width:46px">
                          <button class="toggle" class:on={!set.hidden} aria-label="Offer {set.name}"
                                  aria-pressed={!set.hidden} onclick={() => hideSet(set)}><i></i></button>
                        </td>
                        <td style="width:110px">{set.name}</td>
                        <td style="width:110px"><span class="code-badge">{set.prefix}.*</span></td>
                        <td>
                          <span style="display:flex;gap:8px;flex-wrap:wrap;opacity:{set.hidden ? 0.35 : 1}">
                            {#each set.icons as i (i.key)}<span title={i.key}><BucketIcon name={i.key} size={14} /></span>{/each}
                          </span>
                        </td>
                        <td style="width:80px"><button class="btn quiet small" onclick={() => deleteSet(set)}>Delete</button></td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
              <form class="icon-set-form" onsubmit={addSet}>
                <label class="field"><span class="field-label">Name</span>
                  <input class="input" bind:value={setName} placeholder="Ops" /></label>
                <label class="field"><span class="field-label">Prefix</span>
                  <input class="input" bind:value={setPrefix} placeholder="ops" spellcheck="false" /></label>
                <label class="field"><span class="field-label">SVG files, at most 100</span>
                  <input class="input" type="file" multiple accept=".svg,image/svg+xml" bind:this={fileInput}
                         onchange={(e) => (setFiles = [...e.currentTarget.files])} /></label>
                <button class="btn primary small" disabled={!setName.trim() || !setPrefix.trim() || !setFiles.length}>Add set</button>
              </form>
              {#if skipped.length}
                <p class="explain" style="margin-top:8px">Skipped {skipped.length}:</p>
                <ul class="explain icon-set-skipped">
                  {#each skipped as s (s.file)}<li><code>{s.file}</code>: {s.reason}</li>{/each}
                </ul>
              {/if}
            {/snippet}
          </Section>
          <Section title="Bucket colours" sub={MODES.find(([m]) => m === app.appearance.colourMode)?.[1]}>
            {#snippet children()}
              <div style="display:flex;gap:6px;flex-wrap:wrap">
                {#each MODES as [mode, label] (mode)}
                  <button class="chip" class:on={app.appearance.colourMode === mode}
                          onclick={() => setLooks({ colourMode: mode })}>{label}</button>
                {/each}
              </div>
              <p class="explain" style="margin-top:6px">{MODES.find(([m]) => m === app.appearance.colourMode)?.[2]}</p>
              {#if app.appearance.colourMode !== "off"}
                <div style="display:flex;gap:6px;flex-wrap:wrap;margin-top:14px">
                  {#each Object.entries(BUCKET_COLOURS) as [name, value] (name)}
                    {@const on = !app.appearance.hiddenColours.includes(name)}
                    <button class="swatch" aria-pressed={on} title="{on ? '' : 'Hidden: '}{name}"
                            aria-label="Offer {name}" style="background:{value};border-color:{value};opacity:{on ? 1 : 0.25}"
                            onclick={() => setLooks({ hiddenColours: flip(app.appearance.hiddenColours, name) })}></button>
                  {/each}
                </div>
                <p class="explain" style="margin-top:6px">Click to hide or show a colour.</p>
              {/if}
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
                    Set <code>snapshots.replica</code> to a directory on another disk to restore
                    to any point in time.
                  </p>
                {:else if !snapshots.replica.from}
                  <p class="explain">Copying to {snapshots.replica.directory}; nothing to restore to yet.</p>
                {:else}
                  <p class="explain">
                    Replicating to {snapshots.replica.directory}. Restorable from
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
                Running {updates?.current || "an unknown version"}.
                {#if !updates?.check}
                  Set <code>updates.check</code> to true and restart to check for new releases.
                {:else if updates.latest}
                  Latest: {updates.latest.version}{updates.newer ? "" : " (up to date)"},
                  checked {new Date(updates.latest.checked).toLocaleString()}.
                {:else if !updates.error}
                  Checking GitHub…
                {/if}
              </p>
              {#if updates?.error}
                <p class="explain">Check failed: {updates.error}.</p>
              {/if}
              {#if updates?.newer}
                <a class="btn small" style="margin-top:10px" href={updates.latest.url}
                   target="_blank" rel="noopener">Release notes for {updates.latest.version}</a>
              {/if}
            {/snippet}
          </Section>

        {:else if tab === "integrations"}
          <Section title="Pull requests" sub={forges?.length ? `${forges.length} ${forges.length === 1 ? "forge" : "forges"}` : "off"}>
            {#snippet children()}
              {#if !forges}
                <p class="explain">Forges are unavailable.</p>
              {:else}
                <p class="explain">
                  A pull request links to a ticket when its title starts or ends with the identifier, as in
                  <code>DEV-12: fix it</code>.
                </p>

                {#each forges as f (f.id)}
                  <div style="display:flex;align-items:center;gap:8px;margin:18px 0 6px">
                    <a href={f.url} target="_blank" rel="noopener">{f.url}</a>
                    <span class="explain" title={f.syncError || ""}>
                      {FORGES[f.kind].name} · as {f.account} · {f.mode === "poll" ? `polled every ${f.pollMinutes} min` : "webhook"} ·
                      {syncing[f.id] ? "syncing…" : f.syncError ? `sync failed: ${f.syncError}` : f.syncedAt ? `synced ${ago(f.syncedAt)}` : "not synced yet"}
                    </span>
                    <span style="margin-left:auto"></span>
                    <button class="btn quiet small" disabled={syncing[f.id]} onclick={() => sync(f)}>Sync</button>
                    <button class="btn quiet small" onclick={() => newToken(f)}>New token</button>
                    <button class="btn quiet small" onclick={() => removeForge(f)}>Remove</button>
                  </div>
                  {#if f.mode === "webhook"}
                    <table class="table">
                      <tbody>
                        <tr>
                          <td class="explain" style="width:120px">Webhook</td>
                          <td><code>{hookOf(f)}</code></td>
                          <td style="width:80px"><button class="btn quiet small" onclick={() => copy(hookOf(f))}>Copy</button></td>
                        </tr>
                        <tr>
                          <td class="explain">Secret</td>
                          <td class="explain">shown once</td>
                          <td><button class="btn quiet small" onclick={() => rotate(f)}>Rotate</button></td>
                        </tr>
                      </tbody>
                    </table>
                    <p class="explain" style="margin-top:6px">{FORGES[f.kind].hook}</p>
                  {/if}
                {/each}

                <form style="margin-top:22px" onsubmit={addForge}>
                  <span class="field-label">{forges.length ? "Add another forge" : "Add a forge"}</span>
                  <div style="display:flex;gap:12px;align-items:center;margin-top:8px;flex-wrap:wrap">
                    <select class="input" style="width:200px" aria-label="Kind of forge"
                            value={choice.id} onchange={(e) => (choice = CHOICES.find((c) => c.id === e.currentTarget.value))}>
                      <optgroup label="Hosted">
                        {#each CHOICES.filter((c) => c.hosted) as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
                      </optgroup>
                      <optgroup label="Self-hosted">
                        {#each CHOICES.filter((c) => !c.hosted) as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
                      </optgroup>
                    </select>
                    {#if !choice.hosted}
                      <input class="input" style="width:240px" aria-label="Address" placeholder="https://git.example.com"
                             bind:value={forgeURL} />
                    {/if}
                    <input class="input" style="width:240px" type="password" aria-label="Access token"
                           placeholder="Access token" autocomplete="off" bind:value={forgeToken} />
                    <button class="btn primary small" disabled={!(choice.hosted || forgeURL) || !forgeToken}>Add</button>
                  </div>
                  {#if pollable(choice.kind)}
                    <div class="explain" style="display:flex;gap:6px;align-items:center;margin-top:8px">
                      Updates by
                      <select class="input" style="width:120px" aria-label="How it updates" bind:value={mode}>
                        <option value="poll">Polling</option>
                        <option value="webhook">Webhook</option>
                      </select>
                      {#if mode === "poll"}
                        every
                        <input class="input" style="width:64px" type="number" min="1" max="1440"
                               aria-label="Minutes between checks" bind:value={pollMinutes} />
                        minutes
                      {:else if choice.kind === "github"}
                        — for organisations and repositories; personal GitHub accounts have no webhooks.
                      {/if}
                    </div>
                  {/if}
                  <p class="explain" style="margin-top:8px">{FORGES[choice.kind].token}</p>
                </form>
              {/if}
            {/snippet}
          </Section>

          <Section title="Agent access" sub="{tokens.length} active tokens">
            {#snippet children()}
              <p class="explain">
                A token has its user's current permissions. Revoking takes effect immediately.
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

        {:else if tab === "preferences"}
          <Section title="Data directory" sub={data?.directory}>
            {#snippet children()}
              {#if !data}
                <p class="explain">The data directory is unavailable.</p>
              {:else}
                <p class="explain">
                  The database{data.movesSnapshots ? " and its snapshots" : ""} live here.
                  {#if !data.movesSnapshots}Snapshots stay in {data.snapshots}.{/if}
                  The configuration file is {data.config}.
                </p>
                {#if data.fixed}
                  <p class="explain" style="margin-top:8px">
                    In the container image the data is in the volume mounted at <code>/data</code>.
                    To move it, mount a different volume.
                  </p>
                {:else}
                  <form style="display:flex;gap:8px;align-items:center;margin-top:10px" onsubmit={moveData}>
                    <input class="input" style="flex:1" aria-label="New data directory" placeholder="/path/to/directory"
                           disabled={moving} bind:value={moveTo} />
                    <button class="btn primary small" disabled={moving || !moveTo.trim()}>
                      {moving ? "Moving…" : "Move"}
                    </button>
                  </form>
                  <p class="explain" style="margin-top:8px">
                    A full path on a local disk. Network shares and synced folders
                    (Dropbox, iCloud Drive, OneDrive) corrupt the database.
                  </p>
                {/if}
              {/if}
            {/snippet}
          </Section>
        {/if}
      {/snippet}
    </SettingsRail>
  </div>
{/if}
