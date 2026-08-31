<script>
  import { toasts } from "../lib/ui.svelte.js";
</script>

{#if toasts.length}
  <div class="toast-stack">
    {#each toasts as t (t.id)}
      <div class="toast {t.kind}">{t.message}</div>
    {/each}
  </div>
{/if}
