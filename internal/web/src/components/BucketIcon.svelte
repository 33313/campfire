<script>
  import Icon from "./Icon.svelte";
  import { BUCKET_ICONS, ICON_COLOURS } from "../lib/icons.js";

  let { name, size = 11, rendering = "mono" } = $props();
  let body = $derived(BUCKET_ICONS[name] || BUCKET_ICONS.bookmark);
  let colour = $derived(rendering === "colour"
    ? ICON_COLOURS[name] || "var(--muted-soft)"
    : "var(--muted-soft)");
</script>

<Icon {body} {size} stroke={1.5} color={colour} />
