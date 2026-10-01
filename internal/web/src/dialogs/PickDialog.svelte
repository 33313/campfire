<script>
  import Dialog from "../components/Dialog.svelte";
  import OptionIcon from "./OptionIcon.svelte";

  /** Choose one of a list. Closes with the value, or null for "Any". */
  let { title, options, any = false, explain = null, close } = $props();
</script>

<Dialog {title} {close}>
  {#snippet body()}
    {#if explain}<p class="explain">{explain}</p>{/if}
    {#if any}
      <button class="option-row" onclick={() => close(null)}>Any</button>
    {/if}
    {#each options as o}
      <button class="option-row" onclick={() => close(o.value)}>
        <OptionIcon icon={o.icon} />
        <span>{o.label}</span>
        {#if o.note}<span class="note">{o.note}</span>{/if}
      </button>
    {/each}
  {/snippet}
</Dialog>
