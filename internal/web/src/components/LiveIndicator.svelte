<script>
  import { stream } from "../lib/stream.svelte.js";
  let live = $derived(stream.status === "live");
</script>

<span style="display:flex;align-items:center;gap:6px">
  <span class="pip {live ? 'live' : 'waiting'}"></span>
  <span class="eyebrow">{live ? "Live" : stream.status}</span>
</span>
