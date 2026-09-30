<script>
  // Priority is four glyphs *and* four colours (DESIGN.md §3). The chevron
  // carries the level; the ring reinforces it: solid, tinted, outline, dashed.
  import { PRIORITY } from "../lib/icons.js";

  let { level, size = 14 } = $props();
  let p = $derived(PRIORITY[level] || PRIORITY[3]);
  let n = $derived(PRIORITY[level] ? +level : 3);
</script>

<svg width={size} height={size} viewBox="0 0 16 16" fill="none"
     stroke-linecap="square" stroke-linejoin="miter"
     style:flex="0 0 {size}px" style:color={p.colour}
     aria-label={p.name} role="img">
  <title>{p.name}</title>
  {#if n === 1}
    <circle cx="8" cy="8" r="7.2" fill="currentColor" />
  {:else if n === 2}
    <circle cx="8" cy="8" r="6.6" stroke="currentColor" stroke-width="1.2"
            fill="currentColor" fill-opacity="0.18" />
  {:else if n === 3}
    <circle cx="8" cy="8" r="6.6" stroke="currentColor" stroke-width="1.2" />
  {:else}
    <circle cx="8" cy="8" r="6.6" stroke="var(--dim)" stroke-width="1.2" stroke-dasharray="2 1.8" />
  {/if}
  <g stroke={n === 1 ? "var(--on-accent)" : "currentColor"} stroke-width="1.6">
    {@html p.body}
  </g>
</svg>
