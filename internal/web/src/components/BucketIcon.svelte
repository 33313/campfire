<script>
  import Icon from "./Icon.svelte";
  import { BUCKET_ICONS, colourOf } from "../lib/icons.js";
  import { app } from "../lib/state.svelte.js";

  // A bucket wears its own colour where it has one; otherwise the icon is
  // drawn as every other quiet glyph is. An icon of the instance's own sets is
  // drawn on its own grid, with the stroke scaled to weigh what a built-in
  // one does; a key no set has any more is drawn as the default.
  let { name, size = 11, color = null } = $props();
  let custom = $derived(BUCKET_ICONS[name] ? null : app.customIcons[name]);
  let body = $derived(custom?.body || BUCKET_ICONS[name] || BUCKET_ICONS.bookmark);
  let viewBox = $derived(custom?.viewBox || "0 0 16 16");
  let stroke = $derived(1.5 * Number(viewBox.split(" ")[2]) / 16);
  let colour = $derived(colourOf(color) || "var(--muted-soft)");
</script>

<Icon {body} {size} {stroke} {viewBox} color={colour} />
