<script>
  // Your own account: password, second factor, and the tokens that act as you.

  import { api } from "../lib/api.js";
  import { app, reloadContext } from "../lib/state.svelte.js";
  import { openModal, toast, failure } from "../lib/ui.svelte.js";
  import { when } from "../lib/format.js";

  import Avatar from "../components/Avatar.svelte";
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

  // A picture is scaled here and stored inline: an instance with no object
  // store and no third party to fetch from is the whole point.
  const AVATAR = 96;

  async function saveAvatar(dataUrl) {
    await api.updateProfile(me.email || "", dataUrl, me.bio || "");
    await reloadContext(app.team?.id); // the user list is where avatars are read from
  }

  // Email and bio are one write: the endpoint takes the profile as a whole, so
  // saving either without the other would clear it.
  const BIO = 1000;
  let email = $state("");
  let bio = $state("");
  let profileLoaded = null;
  $effect(() => {
    if (me && profileLoaded !== me.id) {
      profileLoaded = me.id;
      email = me.email || "";
      bio = me.bio || "";
    }
  });
  const saveProfile = () =>
    guard(async () => {
      await api.updateProfile(email, me.avatar || "", bio);
      toast("Saved");
    });

  function pickPicture(e) {
    const file = e.currentTarget.files?.[0];
    e.currentTarget.value = "";
    if (!file) return;
    const img = new Image();
    img.onload = () => {
      // Square crop from the centre, then scale: a picture is drawn at 20px
      // beside a name, and letterboxing it there reads as a mistake.
      const side = Math.min(img.width, img.height);
      const canvas = Object.assign(document.createElement("canvas"),
        { width: AVATAR, height: AVATAR });
      canvas.getContext("2d").drawImage(img, (img.width - side) / 2, (img.height - side) / 2,
        side, side, 0, 0, AVATAR, AVATAR);
      URL.revokeObjectURL(img.src);
      guard(() => saveAvatar(canvas.toDataURL("image/webp", 0.85)));
    };
    img.onerror = () => failure("that file is not an image");
    img.src = URL.createObjectURL(file);
  }

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
      <a class="btn small" href="/users/{app.user.username}">Your profile</a>
      <button class="btn small" onclick={async () => { await api.signOut(); location.href = "/"; }}>
        Sign out
      </button>
    {/snippet}
  </ScreenHeader>

  <div class="body measure">
    <Section title="Picture" sub={me.avatar ? "set" : "initials"}>
      {#snippet children()}
        <div style="display:flex;align-items:center;gap:14px">
          <Avatar name={app.user.username} size="big" />
          <input class="input" type="file" accept="image/*" aria-label="Picture"
                 style="height:auto;padding:6px 8px" onchange={pickPicture} />
          {#if me.avatar}
            <button class="btn small" onclick={() => guard(() => saveAvatar(""))}>Remove</button>
          {/if}
        </div>
        <p class="explain" style="margin:10px 0 0">
          Scaled to a {AVATAR}px square and stored with your account. Nothing is fetched from
          anywhere else to draw it.
        </p>
      {/snippet}
    </Section>

    <Section title="About you" sub="on your profile">
      {#snippet children()}
        <p class="explain">
          Everyone signed in can read this, as they can read every item and
          comment in the instance. It is on <a href="/users/{app.user.username}">your profile</a>.
        </p>
        <label class="field" style="margin-top:10px">
          <span class="field-label">Email</span>
          <input class="input" type="email" bind:value={email} />
        </label>
        <label class="field" style="max-width:none">
          <span class="field-label">Bio</span>
          <textarea class="input" rows="4" style="width:100%;max-width:62ch"
                    placeholder="How to reach you, what you work on — anything you like."
                    bind:value={bio}></textarea>
        </label>
        <p class="explain">{bio.length} / {BIO}</p>
        <button class="btn primary" style="margin-top:10px;align-self:flex-start"
                disabled={bio.length > BIO} onclick={saveProfile}>Save</button>
      {/snippet}
    </Section>

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
