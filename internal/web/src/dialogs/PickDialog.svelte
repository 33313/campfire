<script>
  import { untrack } from "svelte";
  import Dialog from "../components/Dialog.svelte";
  import OptionIcon from "./OptionIcon.svelte";

  /** Choose one of a list. Closes with the value, or null for "Any". With
      `many`, rows toggle and Done closes with every value picked, [] for any. */
  let { title, options, any = false, many = false, picked = [], explain = null, close } = $props();

  let chosen = $state(untrack(() => [...picked]));
  const flip = (v) => (chosen = chosen.includes(v) ? chosen.filter((x) => x !== v) : [...chosen, v]);
</script>

{#snippet body()}
  {#if explain}<p class="explain">{explain}</p>{/if}
  {#if any && !many}
    <button class="option-row" onclick={() => close(null)}>Any</button>
  {/if}
  {#each options as o}
    {#if many}
      <button class="option-row" class:on={chosen.includes(o.value)} aria-pressed={chosen.includes(o.value)}
              onclick={() => flip(o.value)}>
        <OptionIcon icon={o.icon} />
        <span>{o.label}</span>
      </button>
    {:else}
      <button class="option-row" onclick={() => close(o.value)}>
        <OptionIcon icon={o.icon} />
        <span>{o.label}</span>
        {#if o.note}<span class="note">{o.note}</span>{/if}
      </button>
    {/if}
  {/each}
{/snippet}

{#snippet done()}
  <button class="btn quiet" disabled={!chosen.length} onclick={() => (chosen = [])}>Clear</button>
  <div class="actions">
    <button class="btn primary" onclick={() => close(chosen)}>Done</button>
  </div>
{/snippet}

<Dialog {title} {close} {body} footer={many ? done : null} />
