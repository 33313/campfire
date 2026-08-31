<script>
  import { initials } from "../lib/format.js";

  /** Unassigned is a dashed square, never a person glyph. */
  let { name = null, size = "" } = $props();
</script>

{#if name}
  <span class="avatar {size}" title={name}>{initials(name)}</span>
{:else}
  <span class="avatar empty {size}" title="Unassigned"></span>
{/if}
