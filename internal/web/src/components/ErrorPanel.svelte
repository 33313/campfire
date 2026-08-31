<script>
  import { ApiError } from "../lib/api.js";

  let { error, retry = null } = $props();
  let message = $derived(error instanceof ApiError ? error.message : String(error));
</script>

<div class="signin">
  <div class="signin-card">
    <h1>That did not load</h1>
    <p class="explain">{message}</p>
    {#if retry}<button class="btn" onclick={retry}>Try again</button>{/if}
  </div>
</div>
