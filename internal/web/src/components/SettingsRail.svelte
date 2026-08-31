<script>
  let { sections, active, onpick, children } = $props();
</script>

<div class="settings-layout">
  <nav class="settings-rail">
    {#each sections as s}
      <button class:on={active === s.id} onclick={() => (s.onclick ? s.onclick() : onpick(s.id))}>
        {s.label}
      </button>
    {/each}
  </nav>
  <div class="settings-main">{@render children()}</div>
</div>
