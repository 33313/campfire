<script>
  import { untrack } from "svelte";
  import Dialog from "../components/Dialog.svelte";
  import FilterEditor from "../components/FilterEditor.svelte";
  import { failure } from "../lib/ui.svelte.js";

  // A view's name, its filter and its pins. A new view starts from the query
  // it was saved from; an existing one from what it stores. The filter is
  // edited as in the filter bar, chips or CSQL, but nothing runs until Save;
  // a query the server cannot read is shown here, where it can be fixed, and
  // the dialog stays.
  let { view = null, query: start = "", save, close } = $props();

  const from = untrack(() => view);
  let name = $state(from?.name ?? "");
  let query = $state(from ? from.query : untrack(() => start)); // as the chips last set it
  let draft = $state("");                                         // what the editor holds now
  let pinBoard = $state(from ? from.pinBoard : true);
  let pinBacklog = $state(from ? from.pinBacklog : false);
  let error = $state(null);
  let busy = $state(false);

  async function submit() {
    busy = true;
    try {
      close(await save({ name: name.trim(), query: draft, pinBoard, pinBacklog }));
    } catch (err) {
      // Shown while the query it was about is still the one in the editor.
      if (err.field === "query") error = { message: err.message, span: err.span, query: draft };
      else failure(err);
    } finally {
      busy = false;
    }
  }
</script>

<Dialog title={view ? "Edit view" : "Save as view"} {close} wide>
  {#snippet body()}
    <p class="explain">
      {view ? "Changes apply to everyone."
        : "Anyone can turn it on; anyone in this team can edit or delete it."}
      Active views are combined with AND.
    </p>
    <label class="field">
      <span class="field-label">Name</span>
      <input class="input" placeholder="My open work" bind:value={name} />
    </label>
    <div class="field">
      <span class="field-label">Filter</span>
      <div class="filter-row">
        <FilterEditor value={query} bind:draft error={error?.query === draft ? error : null} onapply={(q) => { query = q; error = null; return { ok: true }; }} />
      </div>
    </div>
    {#if view?.missing?.length}
      <p class="explain" style="color:var(--alert)">
        Refers to something deleted, so it matches nothing: {view.missing.join(", ")}.
      </p>
    {/if}
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
    <span class="consequence">Stored as CSQL</span>
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" disabled={!name.trim() || busy} onclick={submit}>Save</button>
    </div>
  {/snippet}
</Dialog>
