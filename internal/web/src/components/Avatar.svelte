<script>
  import { initials } from "../lib/format.js";
  import { app } from "../lib/state.svelte.js";

  /** Unassigned is a dashed square, never a person glyph. */
  //
  // Items carry the assignee's name rather than their picture, so the picture
  // is looked up in the user list the client already holds. Nobody without one
  // loses their initials.
  //
  // The size is prefixed rather than used bare: "comment" as a class on the
  // picture also matched the .comment block it sits in, which padded the image
  // down to no height at all and left only the initials showing.
  let { name = null, size = "" } = $props();
  let cls = $derived(size ? `avatar avatar-${size}` : "avatar");
  let src = $derived(name ? app.users.find((u) => u.username === name)?.avatar : null);
</script>

{#if src}
  <img class={cls} {src} alt="" title={name} />
{:else if name}
  <span class={cls} title={name}>{initials(name)}</span>
{:else}
  <span class="{cls} empty" title="Unassigned"></span>
{/if}
