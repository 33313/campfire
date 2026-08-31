<script>
  import Icon from "./Icon.svelte";
  import { NAV } from "../lib/icons.js";

  /** The frame every dialog wears. `close` dismisses it with a value. */
  let { title, close, destructive = false, wide = false, body, footer = null } = $props();

  function onKey(e) {
    if (e.key === "Escape") close(undefined);
  }

  // The first control in a dialog takes the focus, so it can be typed into or
  // dismissed without reaching for the mouse.
  function focusFirst(panel) {
    panel.querySelector("input, textarea, select, button.primary")?.focus();
  }
</script>

<svelte:window on:keydown={onKey} />

<div class="scrim" role="presentation" onclick={(e) => e.target === e.currentTarget && close(undefined)}>
  <div class="dialog" class:destructive class:wide use:focusFirst role="dialog" aria-modal="true">
    <div class="dialog-title">
      {title}
      <button class="btn quiet small" style="margin-left:auto" aria-label="Close"
              onclick={() => close(undefined)}>
        <Icon body={NAV.close} size={13} />
      </button>
    </div>
    <div class="dialog-body">{@render body()}</div>
    {#if footer}<div class="dialog-foot">{@render footer()}</div>{/if}
  </div>
</div>
