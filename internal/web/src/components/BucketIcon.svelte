<script>
  import Icon from "./Icon.svelte";
  import { BUCKET_ICONS, BUCKET_COLOURS, ICON_COLOURS } from "../lib/icons.js";

  // A bucket's own colour, where it has one, beats both the team's rendering
  // and the icon's default: it was chosen for this bucket.
  let { name, size = 11, rendering = "mono", color = null } = $props();
  let body = $derived(BUCKET_ICONS[name] || BUCKET_ICONS.bookmark);
  let colour = $derived(BUCKET_COLOURS[color]
    || (rendering === "colour" ? ICON_COLOURS[name] || "var(--muted-soft)" : "var(--muted-soft)"));
</script>

<Icon {body} {size} stroke={1.5} color={colour} />
