<script>
  // An item's relations, one row each: the kind as seen from the item, the
  // other end, and its state. The ticket page, its preview and the create form
  // draw them alike (DEV-94); a list that can lose one passes `onremove`.
  import Icon from "./Icon.svelte";
  import StateBadge from "./StateBadge.svelte";
  import { NAV, RELATION_LABELS } from "../lib/icons.js";

  // `links` is off where following one would leave unsaved work, as in a form.
  let { relations = [], onremove = null, compact = false, links = true } = $props();
</script>

{#each relations as r}
  <div class="relation-row" class:compact>
    <span class="relation-kind">{RELATION_LABELS[r.kind] || r.kind}</span>
    {#if links}
      <a class="ident" style="color:var(--accent)" href="/{r.identifier}">{r.identifier}</a>
    {:else}
      <span class="ident" style="color:var(--accent)">{r.identifier}</span>
    {/if}
    <span class="relation-title">{r.title}</span>
    {#if r.stateName && !compact}<StateBadge name={r.stateName} role={r.stateRole} />{/if}
    {#if onremove}
      <button class="btn quiet small" title="Remove" aria-label="Remove {r.identifier}"
              onclick={() => onremove(r)}>
        <Icon body={NAV.close} size={11} />
      </button>
    {/if}
  </div>
{/each}
