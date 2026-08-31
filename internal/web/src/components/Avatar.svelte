<script>
  import { initials } from "../lib/format.js";
  import { app } from "../lib/state.svelte.js";

  /** Unassigned is a dashed square, never a person glyph. */
  //
  // Items carry the assignee's name rather than their picture, so the picture
  // is looked up in the user list the client already holds. Nobody without one
  // loses their initials.
  let { name = null, size = "" } = $props();
  let src = $derived(name ? app.users.find((u) => u.username === name)?.avatar : null);
</script>

{#if src}
  <img class="avatar {size}" {src} alt="" title={name} />
{:else if name}
  <span class="avatar {size}" title={name}>{initials(name)}</span>
{:else}
  <span class="avatar empty {size}" title="Unassigned"></span>
{/if}
