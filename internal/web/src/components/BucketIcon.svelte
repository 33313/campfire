<script>
  import Icon from "./Icon.svelte";
  import { BUCKET_ICONS, BUCKET_COLOURS } from "../lib/icons.js";

  // A bucket wears its own colour where it has one; otherwise the icon is
  // drawn as every other quiet glyph is.
  let { name, size = 11, color = null } = $props();
  let body = $derived(BUCKET_ICONS[name] || BUCKET_ICONS.bookmark);
  let colour = $derived(BUCKET_COLOURS[color] || "var(--muted-soft)");
</script>

<Icon {body} {size} stroke={1.5} color={colour} />
