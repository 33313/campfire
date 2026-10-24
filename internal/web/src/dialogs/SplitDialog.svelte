<script>
  import Dialog from "../components/Dialog.svelte";

  /** Split — one ticket becomes several. */
  let { item, close } = $props();

  let lines = $state(["", ""]);
  let rejectSource = $state(false);

  function done() {
    const titles = lines.map((t) => t.trim()).filter(Boolean);
    if (titles.length) close({ titles, rejectSource });
  }
</script>

<Dialog title="Split {item.identifier}" wide {close}>
  {#snippet body()}
    <p class="explain">
      One ticket per line, with this one's bucket, priority, assignee and labels,
      each following up {item.identifier}.
    </p>
    <div class="field">
      <span class="field-label">New tickets</span>
      <div style="display:flex;flex-direction:column;gap:6px">
        {#each lines as _, i}
          <div style="display:flex;gap:6px">
            <input class="input" style="flex:1" placeholder="A new ticket's title"
                   bind:value={lines[i]} />
            <button class="btn small" onclick={() => lines.splice(i, 1)}>−</button>
          </div>
        {/each}
      </div>
    </div>
    <button class="btn small" onclick={() => lines.push("")}>Add a line</button>
    <label style="display:flex;gap:8px;align-items:center">
      <input class="check" type="checkbox" bind:checked={rejectSource} />
      <span>Reject {item.identifier} once the split is written</span>
    </label>
  {/snippet}
  {#snippet footer()}
    <span class="consequence">New tickets follow up the source</span>
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" onclick={done}>Split</button>
    </div>
  {/snippet}
</Dialog>
