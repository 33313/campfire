<script>
  import Icon from "./Icon.svelte";
  import { PRIORITY } from "../lib/icons.js";

  let { level, size = 14 } = $props();
  let p = $derived(PRIORITY[level] || PRIORITY[3]);
</script>

<Icon body={p.body} {size} stroke={2} color={p.colour} label={p.name} />
