<script>
  import Avatar from "../components/Avatar.svelte";
  import PriorityGlyph from "../components/PriorityGlyph.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import StatePip from "../components/StatePip.svelte";

  /** The little mark on an option row, named rather than passed as a node. */
  let { icon } = $props();
</script>

{#if icon}
  {#if "avatar" in icon}<Avatar name={icon.avatar} />
  {:else if "priority" in icon}<PriorityGlyph level={icon.priority} />
  {:else if "bucket" in icon}<BucketIcon name={icon.bucket} size={13} />
  {:else if "pip" in icon}<StatePip role={icon.pip} />
  {/if}
{/if}
