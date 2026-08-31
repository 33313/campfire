<script>
  import PriorityGlyph from "./PriorityGlyph.svelte";
  import BucketIcon from "./BucketIcon.svelte";
  import Avatar from "./Avatar.svelte";

  // A card carries five fields and no sixth: bucket, title, priority,
  // identifier, assignee. Labels never appear on one (SPEC.md §8.1).
  let { item, selected = false, focused = false, rendering = "mono",
         ontoggle, ondrag, onpreview } = $props();

  let dragging = $state(false);
</script>

<a class="card" class:selected class:focused class:dragging data-id={item.id} draggable="true"
  href="/{item.identifier}"
  onclick={(e) => {
    // A modifier picks the card into the selection instead of opening it; the
    // router leaves modified clicks alone, so only the navigation is stopped.
    if (e.metaKey || e.ctrlKey) { e.preventDefault(); ontoggle(item.id); }
    // A plain click previews the ticket beside the board; the identifier in
    // the preview is what opens the full view.
    else if (onpreview) { e.preventDefault(); onpreview(item); }
  }}
  ondragstart={(e) => {
    dragging = true;
    ondrag(item);
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", item.identifier);
  }}
  ondragend={() => { dragging = false; ondrag(null); }}>

  {#if item.bucketIdentifier}
    <div class="card-bucket">
      <BucketIcon name={item.bucketIcon} size={11} {rendering} />
      <span>{item.bucketTitle}</span>
    </div>
  {/if}
  <p class="card-title">{item.title}</p>
  <div class="card-foot">
    <PriorityGlyph level={item.priority} />
    <span class="ident">{item.identifier}</span>
    <span style="margin-left:auto"><Avatar name={item.assigneeName} /></span>
  </div>
</a>
