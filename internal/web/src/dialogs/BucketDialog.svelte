<script>
  import Dialog from "../components/Dialog.svelte";
  import BucketPicker from "../components/BucketPicker.svelte";

  /** One item's bucket: the create form's field, open, in a dialog of its
      own. Picking closes it with the bucket's id, 0 for none. */
  let { value = 0, rendering = "mono", close } = $props();
</script>

<Dialog title="Bucket" {close}>
  {#snippet body()}
    <BucketPicker {value} {rendering} onpick={(id) => close(id)} />
  {/snippet}
</Dialog>
