<script>
  import Dialog from "../components/Dialog.svelte";

  let { close } = $props();

  let name = $state("");
  let pinBoard = $state(true);
  let pinBacklog = $state(false);
</script>

<Dialog title="Save as view" {close}>
  {#snippet body()}
    <p class="explain">
      A name attached to the filter you already have. Everyone can apply it;
      anyone in this team can edit or delete it.
    </p>
    <label class="field">
      <span class="field-label">Name</span>
      <input class="input" placeholder="Assigned to marta" bind:value={name} />
    </label>
    <label style="display:flex;gap:8px;align-items:center">
      <input class="check" type="checkbox" bind:checked={pinBoard} />
      <span>Pin to the board</span>
    </label>
    <label style="display:flex;gap:8px;align-items:center">
      <input class="check" type="checkbox" bind:checked={pinBacklog} />
      <span>Pin to the backlog</span>
    </label>
  {/snippet}
  {#snippet footer()}
    <span class="consequence">Stored as clauses, not a query string</span>
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" disabled={!name.trim()}
              onclick={() => close({ name: name.trim(), pinBoard, pinBacklog })}>Save</button>
    </div>
  {/snippet}
</Dialog>
