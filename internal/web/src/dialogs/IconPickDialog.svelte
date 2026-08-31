<script>
  import Dialog from "../components/Dialog.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import { BUCKET_ICONS } from "../lib/icons.js";

  let { close } = $props();
</script>

<Dialog title="Bucket icon" {close}>
  {#snippet body()}
    <div style="display:flex;gap:6px;flex-wrap:wrap">
      {#each Object.keys(BUCKET_ICONS) as name}
        <button class="chip" title={name} onclick={() => close(name)}>
          <BucketIcon {name} size={16} />
        </button>
      {/each}
    </div>
  {/snippet}
</Dialog>
