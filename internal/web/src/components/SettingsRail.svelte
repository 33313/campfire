<script>
  // `manage` caps the content for a screen of controls (style.css, .manage);
  // the docs keep their own measure.
  let { sections, active, onpick, manage = false, children } = $props();
</script>

<div class="settings-layout">
  <nav class="settings-rail">
    {#each sections as s}
      <button class:on={active === s.id} onclick={() => onpick(s.id)}>
        {s.label}
      </button>
    {/each}
  </nav>
  <div class="settings-main" class:manage>{@render children()}</div>
</div>
