<script>
  import { api } from "../lib/api.js";
  import Mark from "../components/Mark.svelte";

  // Sign in, the first-run account, and redeeming a link.
  //
  // There is no open registration and no setup wizard: an instance with no
  // accounts offers to create the first one, and everything after that is an
  // administrator's invitation (SPEC.md §5.1, §13.1).
  let { mode = "signin", token = "", onsignedin } = $props();

  let username = $state("");
  let password = $state("");
  let code = $state("");
  let needsCode = $state(false);
  let error = $state("");

  const TITLES = {
    signin: "Sign in",
    bootstrap: "Create the first account",
    redeem: "Redeem your link",
  };

  async function submit(e) {
    e.preventDefault();
    error = "";
    try {
      if (mode === "bootstrap") await api.bootstrap(username, password);
      // A reset applies to the account the link was issued for, whatever was
      // typed above — so sign in as that account, not as what was typed. The
      // link is spent either way, and a mismatch would be unrecoverable.
      if (mode === "redeem") username = (await api.redeemLink(token, username, password)).username;
      const res = await api.signIn(username, password, code);
      if (res.totpRequired) {
        // No partial session is issued while the second factor is owed.
        needsCode = true;
        return;
      }
      await onsignedin();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="signin welcome">
  <div class="brand" style="flex-direction:column;gap:12px">
    <Mark size={36} burning />
    <span class="wordmark">Campfire</span>
  </div>
  <form class="signin-card" onsubmit={submit}>
    <h1>{TITLES[mode]}</h1>

    {#if mode === "bootstrap"}
      <p class="explain">
        This instance has no accounts yet. The first one is an instance administrator;
        every account after it is created here or invited by link.
      </p>
    {:else if mode === "redeem"}
      <p class="explain">
        An invitation creates your account. A reset sets a new password on the account
        it was issued for, so the username is ignored.
      </p>
    {/if}

    <label class="field">
      <span class="field-label">Username</span>
      <!-- svelte-ignore a11y_autofocus -->
      <input class="input" name="username" autocomplete="username" autofocus bind:value={username} />
    </label>

    <label class="field">
      <span class="field-label">Password</span>
      <input class="input" class:error={error && mode === "signin"} type="password"
             autocomplete={mode === "signin" ? "current-password" : "new-password"}
             bind:value={password} />
      {#if mode === "bootstrap"}<span class="explain">At least ten characters.</span>{/if}
    </label>

    {#if needsCode}
      <label class="field">
        <span class="field-label">Second factor</span>
        <input class="input" inputmode="numeric" autocomplete="one-time-code"
               placeholder="000000" maxlength="6" bind:value={code} />
        <span class="explain">Six digits from your authenticator app.</span>
      </label>
    {/if}

    {#if error}<div class="explain" style="color:var(--alert)">{error}</div>{/if}

    <button class="btn primary" type="submit">
      {mode === "signin" ? "Sign in" : mode === "bootstrap" ? "Create account" : "Continue"}
    </button>
  </form>
</div>
