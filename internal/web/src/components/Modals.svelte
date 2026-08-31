<script>
  import { modals, closeModal } from "../lib/ui.svelte.js";
</script>

{#each modals as entry (entry.id)}
  {@const Body = entry.component}
  <Body {...entry.props} close={(v) => closeModal(entry, v)} />
{/each}
