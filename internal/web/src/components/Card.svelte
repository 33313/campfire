<script>
  import PriorityGlyph from "./PriorityGlyph.svelte";
  import BucketIcon from "./BucketIcon.svelte";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";
  import { BUCKET_COLOURS, NAV } from "../lib/icons.js";
  import { whenDay, daysUntil } from "../lib/format.js";

  // A card carries six fields and no seventh: bucket, title, deadline,
  // priority, identifier, assignee. Labels never appear on one (SPEC.md §8.1).
  //
  // Rows: the title, the bucket, the deadline, then the identifier and the
  // assignee. The bucket and the deadline are drawn only when there is one.
  // The bucket wears its colour where it has one, so a column of cards is
  // sorted by eye without reading a title on each: on its border and its name,
  // never as a fill. A filled chip on most cards outshouts the one P1 tile.
  let { item, selected = false, focused = false, rendering = "mono",
         ontoggle, ondrag, onpreview, onmenu } = $props();

  let dragging = $state(false);
  let tint = $derived(BUCKET_COLOURS[item.bucketColor] || null);
  // A deadline three days away or less is drawn in the accent so it is seen
  // before it is missed, and one already past in the alert colour. A done
  // ticket's deadline is greyed out: there is nothing left to be late for.
  let done = $derived(item.stateRole === "final");
  let days = $derived(item.plannedEnd && !done ? daysUntil(item.plannedEnd) : null);
  let soon = $derived(days !== null && days >= 0 && days <= 3);
  let overdue = $derived(days !== null && days < 0);
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
  {#if item.bucketIdentifier}
    <span class="card-bucket" title="{item.bucketIdentifier}  {item.bucketTitle}"
          style={tint ? `border-color:${tint};color:${tint}` : ""}>
      <BucketIcon name={item.bucketIcon} size={10} {rendering} color={item.bucketColor} />
      <span class="name">{item.bucketTitle}</span>
    </span>
  {/if}
  {#if item.plannedEnd}
    <span class="card-deadline" class:soon class:overdue class:done title="Planned end">
      <Icon body={NAV.calendar} size={12} /><span>{whenDay(item.plannedEnd)}</span>
    </span>
  {/if}
  <div class="card-foot">
    <PriorityGlyph level={item.priority} />
    <span class="ident">{item.identifier}</span>
    <span style="margin-left:auto"><Avatar name={item.assigneeName} /></span>
  </div>
</a>
