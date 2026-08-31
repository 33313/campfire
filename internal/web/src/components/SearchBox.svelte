<script>
  import { untrack } from "svelte";
  import Icon from "./Icon.svelte";
  import { NAV } from "../lib/icons.js";

  let { value = "", placeholder = "Title and description", onsearch } = $props();
  let text = $state(untrack(() => value));
</script>

<div class="search-box">
  <Icon body={NAV.search} size={13} />
  <input class="input" {placeholder} bind:value={text}
         onkeydown={(e) => e.key === "Enter" && onsearch(text)} />
</div>
