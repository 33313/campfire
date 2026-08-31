<script>
  import Dialog from "../components/Dialog.svelte";
  import StatePip from "../components/StatePip.svelte";
  import RoleBadge from "../components/RoleBadge.svelte";
  import { plural } from "../lib/format.js";

  /** Move to a state — from any state, at any time. Rejecting asks for a reason. */
  let { states, count = 1, identifiers = "", close } = $props();

  let stateId = $state(null);
  let reason = $state("cancelled");
  let note = $state("");
  let rejecting = $derived(states.find((s) => s.id === stateId)?.role === "rejected");
</script>

<Dialog title={count > 1 ? `Move ${count} items` : "Move to a state"} {close}>
  {#snippet body()}
    {#if identifiers}<div class="explain">{identifiers}</div>{/if}
    <div style="display:flex;flex-direction:column;gap:2px">
      {#each states as s}
        <button class="option-row" class:on={stateId === s.id} onclick={() => (stateId = s.id)}>
          <StatePip role={s.role} />
          <span>{s.name}</span>
          <RoleBadge role={s.role} />
        </button>
      {/each}
    </div>
    {#if rejecting}
      <label class="field">
        <span class="field-label">Reason</span>
        <div style="display:flex;gap:6px">
          {#each ["cancelled", "duplicate"] as r}
            <button class="chip" class:on={reason === r} onclick={() => (reason = r)}>{r}</button>
          {/each}
        </div>
      </label>
      <label class="field">
        <span class="field-label">Note — optional</span>
        <input class="input" placeholder="Optional note" bind:value={note} />
      </label>
    {/if}
  {/snippet}
  {#snippet footer()}
    <span class="consequence">{plural(count, "event")}</span>
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" disabled={!stateId}
              onclick={() => stateId && close({ stateId, reason, note })}>Move</button>
    </div>
  {/snippet}
</Dialog>
