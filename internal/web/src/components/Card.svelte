<script>
  import PriorityGlyph from "./PriorityGlyph.svelte";
  import BucketIcon from "./BucketIcon.svelte";
  import Avatar from "./Avatar.svelte";
  import { BUCKET_COLOURS } from "../lib/icons.js";

  // A card carries five fields and no sixth: bucket, title, priority,
  // identifier, assignee. Labels never appear on one (SPEC.md §8.1).
  //
  // The bucket sits in the foot between the identifier and the assignee, and
  // wears the bucket's colour where it has one: a column of cards is then
  // sorted by eye without reading a title on each.
  let { item, selected = false, focused = false, rendering = "mono",
         ontoggle, ondrag, onpreview, onmenu } = $props();

  let dragging = $state(false);
  let tint = $derived(BUCKET_COLOURS[item.bucketColor] || null);
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
  oncontextmenu={(e) => {
    // The browser's own menu is about a link; this one is about a ticket.
    if (!onmenu) return;
    e.preventDefault();
    onmenu(item, e.clientX, e.clientY);
  }}
  ondragstart={(e) => {
    dragging = true;
    ondrag(item);
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", item.identifier);
  }}
  ondragend={() => { dragging = false; ondrag(null); }}>

  <p class="card-title">{item.title}</p>
  <div class="card-foot">
    <PriorityGlyph level={item.priority} />
    <span class="ident">{item.identifier}</span>
    {#if item.bucketIdentifier}
      <span class="card-bucket" title="{item.bucketIdentifier}  {item.bucketTitle}"
            style={tint ? `border-color:${tint};color:${tint};` +
              `background:color-mix(in srgb, ${tint} 13%, transparent)` : ""}>
        <BucketIcon name={item.bucketIcon} size={10} {rendering} color={item.bucketColor} />
        <span class="name">{item.bucketTitle}</span>
      </span>
    {/if}
    <span style="margin-left:auto"><Avatar name={item.assigneeName} /></span>
  </div>
</a>
