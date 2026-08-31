<script>
  import Dialog from "../components/Dialog.svelte";

  /** A destructive confirm that states its consequence before it happens. */
  let { title, lines = [], counts = null, confirmWord = null,
        consequence = "Permanent", action = "Delete", close } = $props();

  let typed = $state("");
  let wrong = $state(false);

  function confirm() {
    if (confirmWord && typed.trim() !== confirmWord) { wrong = true; return; }
    close(true);
  }
</script>

<Dialog {title} destructive {close}>
  {#snippet body()}
    {#each lines as line}<p class="explain">{line}</p>{/each}
    {#if counts}
      <div class="count-block">
        {#each counts as c}<b>{c.value}</b><span>{c.label}</span>{/each}
      </div>
    {/if}
    {#if confirmWord}
      <label class="field">
        <span class="field-label">Type {confirmWord} to confirm</span>
        <input class="input" class:error={wrong} placeholder={confirmWord} bind:value={typed} />
      </label>
    {/if}
  {/snippet}
  {#snippet footer()}
    <span class="consequence">{consequence}</span>
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn destructive" onclick={confirm}>{action}</button>
    </div>
  {/snippet}
</Dialog>
