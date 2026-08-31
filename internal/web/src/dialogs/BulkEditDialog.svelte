<script>
  import Dialog from "../components/Dialog.svelte";
  import OptionIcon from "./OptionIcon.svelte";
  import { plural } from "../lib/format.js";

  /** Bulk edit — one field at a time. There is no multi-field form. */
  let { field, count, options, close } = $props();

  let value = $state(0);
</script>

<Dialog title="{field} for {plural(count, 'item')}" {close}>
  {#snippet body()}
    <p class="explain">One field at a time. A selection is where a mis-click is least visible.</p>
    <div style="display:flex;flex-direction:column;gap:2px">
      {#each options as o}
        <button class="option-row" class:on={value === o.value} onclick={() => (value = o.value)}>
          <OptionIcon icon={o.icon} />
          <span>{o.label}</span>
          {#if o.note}<span class="note">{o.note}</span>{/if}
        </button>
      {/each}
    </div>
  {/snippet}
  {#snippet footer()}
    <span class="consequence">{plural(count, "event")}</span>
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" onclick={() => close({ value })}>Apply</button>
    </div>
  {/snippet}
</Dialog>
