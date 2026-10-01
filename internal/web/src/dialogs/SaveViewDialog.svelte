<script>
  import { untrack } from "svelte";
  import Dialog from "../components/Dialog.svelte";
  import FilterChips from "../components/FilterChips.svelte";
  import { filterFromView } from "../lib/options.js";

  // A new view starts from the filter it was saved from; an existing one from
  // what it stores. Either way the filter is edited here, with the same chips
  // as the filter bar, each taking several values, and not on the board.
  let { view = null, filter: start = {}, close } = $props();

  // The dialog starts from what it was opened with, and edits a copy.
  const from = untrack(() => view);
  let name = $state(from?.name ?? "");
  let filter = $state(from ? filterFromView(from) : untrack(() => ({ ...start })));
  let pinBoard = $state(from ? from.pinBoard : true);
  let pinBacklog = $state(from ? from.pinBacklog : false);
</script>

<Dialog title={view ? "Edit view" : "Save as view"} {close}>
  {#snippet body()}
    <p class="explain">
      {view ? "A change here is a change for everyone who uses the view."
        : "Everyone can switch it on; anyone in this team can edit or delete it."}
      With other views on, values of one filter add up and different filters narrow.
    </p>
    <label class="field">
      <span class="field-label">Name</span>
      <input class="input" placeholder="Assigned to marta" bind:value={name} />
    </label>
    <div class="field">
      <span class="field-label">Filter</span>
      <div style="display:flex;flex-wrap:wrap;gap:6px">
        <FilterChips {filter} setFilter={(f) => (filter = f)} {view} />
      </div>
    </div>
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
              onclick={() => close({ name: name.trim(), pinBoard, pinBacklog, filter })}>Save</button>
    </div>
  {/snippet}
</Dialog>
