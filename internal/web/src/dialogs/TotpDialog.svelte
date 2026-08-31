<script>
  import Dialog from "../components/Dialog.svelte";

  let { setup, close } = $props();
  let code = $state("");
</script>

<Dialog title="Set up your second factor" {close}>
  {#snippet body()}
    <p class="explain">Add this secret to your authenticator app, then prove a code.</p>
    <input class="input" value={setup.secret} readonly onclick={(e) => e.currentTarget.select()} />
    <p class="explain">{setup.uri}</p>
    <label class="field">
      <span class="field-label">Code</span>
      <input class="input" inputmode="numeric" maxlength="6" placeholder="000000" bind:value={code} />
    </label>
  {/snippet}
  {#snippet footer()}
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" onclick={() => close(code)}>Enable</button>
    </div>
  {/snippet}
</Dialog>
