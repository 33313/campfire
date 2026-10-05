<script>
  import Dialog from "../components/Dialog.svelte";
  import ItemSearch from "../components/ItemSearch.svelte";
  import RelationList from "../components/RelationList.svelte";
  import { RELATION_LABELS } from "../lib/icons.js";

  /** Add a relation from `item`: a kind, and the other item found by search.
      Closes with { kind, other }. */
  let { item, close } = $props();

  let kind = $state("blocks");
  let other = $state(null);
</script>

<Dialog title="Add a relation" {close}>
  {#snippet body()}
    <label class="field">
      <span class="field-label">Kind</span>
      <select class="input" bind:value={kind}>
        {#each Object.entries(RELATION_LABELS) as [value, label]}<option {value}>{label}</option>{/each}
      </select>
    </label>
    <div class="field">
      <span class="field-label">To</span>
      {#if other}
        <div>
          <RelationList relations={[{ ...other, kind }]} compact links={false} onremove={() => (other = null)} />
        </div>
      {:else}
        <ItemSearch label="To" placeholder="A title or an identifier, such as {item.identifier}"
                    exclude={[item.id]} onpick={(it) => (other = it)} />
      {/if}
    </div>
    <p class="explain">Relations may cross teams and may link a ticket to a bucket.</p>
  {/snippet}
  {#snippet footer()}
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" disabled={!other} onclick={() => close({ kind, other })}>Add</button>
    </div>
  {/snippet}
</Dialog>
