<script>
  import Icon from "./Icon.svelte";
  import { NAV } from "../lib/icons.js";

  /** The keyset footer: "load older from 41192", never a page number. */
  let { label, onclick } = $props();
</script>

<button class="load-older" {onclick}>
  <span>{label}</span>
  <Icon body={NAV.chevron} size={10} />
</button>
