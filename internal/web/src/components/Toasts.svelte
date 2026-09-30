<script>
  import { flip } from "svelte/animate";
  import { fly, fade } from "svelte/transition";
  import { toasts } from "../lib/ui.svelte.js";
  import { ms, easeOut, QUICK, GLIDE } from "../lib/motion.js";
</script>

<!-- A toast rises into the stack, and the ones above it make room rather than
     jump; it fades on its way out. The stack is always there, empty or not, so
     the last one out still gets to fade. -->
<div class="toast-stack">
  {#each toasts as t (t.id)}
    <div class="toast {t.kind}" animate:flip={{ duration: ms(GLIDE), easing: easeOut }}
         in:fly={{ y: 10, duration: ms(GLIDE), easing: easeOut }}
         out:fade={{ duration: ms(QUICK) }}>{t.message}</div>
  {/each}
</div>
