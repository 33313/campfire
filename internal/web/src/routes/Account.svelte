<script>
  // Your own account: password, second factor, and the tokens that act as you.

  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { openModal, toast, failure } from "../lib/ui.svelte.js";
  import { when } from "../lib/format.js";

  import Section from "../components/Section.svelte";
  import ScreenHeader from "../components/ScreenHeader.svelte";
  import ErrorPanel from "../components/ErrorPanel.svelte";
  import FormDialog from "../dialogs/FormDialog.svelte";
  import ShowOnceDialog from "../dialogs/ShowOnceDialog.svelte";
  import TotpDialog from "../dialogs/TotpDialog.svelte";

  let tokens = $state([]);
  let me = $state(null);
  let error = $state(null);
  let current = $state("");
  let next = $state("");

  async function load() {
    try {
      const [ts, users] = await Promise.all([
        api.listAgentTokens(app.user.id).catch(() => []),
        api.listUsers(),
      ]);
      tokens = ts;
      me = users.find((u) => u.id === app.user.id) || app.user;
      error = null;
    } catch (err) {
      error = err;
    }
  }

  load();

  const guard = async (fn) => {
    try { await fn(); await load(); } catch (err) { failure(err); }
  };

  async function changePassword() {
    guard(async () => {
      await api.changePassword(current, next);
      current = next = "";
      toast("Changed. Every other session was ended.");
    });
  }

  async function issueToken() {
    const res = await openModal(FormDialog, {
      title: "Issue an agent token", action: "Issue",
      fields: [{ name: "name", label: "Name", placeholder: "laptop" }],
      explain: "Name it so you recognise it in the log later.",
    });
    if (!res) return;
    guard(async () => {
      const issued = await api.issueAgentToken(res.name);
      await openModal(ShowOnceDialog, {
        title: "Your new token",
        explain: "Shown once. Only its hash is stored.",
        value: issued.token,
        copyLabel: "Copy token",
        install: `claude mcp add --scope user --transport http campfire ${location.origin}/mcp` +
          ` --header "Authorization: Bearer ${issued.token}"`,
      });
    });
  }

  async function enableTotp() {
    try {
      const setup = await api.setupTotp();
      const code = await openModal(TotpDialog, { setup });
      if (!code) return;
      await api.enableTotp(setup.secret, code);
      toast("Second factor on");
      await load();
    } catch (err) { failure(err); }
  }

  async function disableTotp() {
    const res = await openModal(FormDialog, {
      title: "Turn off the second factor",
      destructive: true, action: "Turn off",
      fields: [{ name: "password", label: "Password", type: "password" }],
      explain: "Your password confirms it is you.",
    });
    if (res) guard(() => api.disableTotp(res.password));
  }
</script>

{#if error}
  <ErrorPanel {error} retry={load} />
{:else if me}
  <ScreenHeader title={app.user.username}
                sub={app.user.isAdmin ? "Instance administrator" : "Member"}>
    {#snippet right()}
      <button class="btn small" onclick={async () => { await api.signOut(); location.href = "/"; }}>
        Sign out
      </button>
    {/snippet}
  </ScreenHeader>

  <div class="body">
    <Section title="Password">
      {#snippet children()}
        <label class="field">
          <span class="field-label">Current</span>
          <input class="input" type="password" bind:value={current} />
        </label>
        <label class="field">
          <span class="field-label">New</span>
          <input class="input" type="password" bind:value={next} />
        </label>
        <button class="btn primary" style="margin-top:10px;align-self:flex-start"
                onclick={changePassword}>Change password</button>
      {/snippet}
    </Section>

    <Section title="Second factor" sub={me.hasTotp ? "on" : "off"}>
      {#snippet children()}
        {#if me.hasTotp}
          <button class="btn destructive small" onclick={disableTotp}>Turn off</button>
        {:else}
          <button class="btn small" onclick={enableTotp}>Set up an authenticator app</button>
        {/if}
      {/snippet}
    </Section>

    <Section title="Agent tokens" sub="{tokens.length} live">
      {#snippet children()}
        <p class="explain">
          An agent acts as you, with your permissions at the time of each call.
          It cannot create accounts, change teams, delete anything, or issue tokens.
        </p>
        <button class="btn small" style="margin:10px 0" onclick={issueToken}>Issue a token</button>
        <table class="table">
          <thead>
            <tr><th>Name</th><th style="width:180px">Last used</th><th style="width:100px"></th></tr>
          </thead>
          <tbody>
            {#each tokens as t (t.id)}
              <tr>
                <td>{t.name}</td>
                <td class="explain">{t.lastUsedAt ? when(t.lastUsedAt, true) : "never"}</td>
                <td>
                  <button class="btn quiet small"
                          onclick={() => guard(() => api.revokeAgentToken(t.id))}>Revoke</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/snippet}
    </Section>
  </div>
{/if}
