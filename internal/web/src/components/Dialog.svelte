<script>
  import Icon from "./Icon.svelte";
  import { NAV } from "../lib/icons.js";

  /** The frame every dialog wears. `close` dismisses it with a value. */
  let { title, close, destructive = false, wide = false, body, footer = null } = $props();

  let panel;

  // Only the dialog on top answers Escape: a picker opened from a dialog
  // closes alone, leaving the one beneath it up.
  function onKey(e) {
    if (e.key === "Escape" && panel === [...document.querySelectorAll(".dialog")].at(-1)) close(undefined);
  }

  // The first control in a dialog takes the focus, so it can be typed into or
  // dismissed without reaching for the mouse.
  function focusFirst(panel) {
    panel.querySelector("input, textarea, select, button.primary")?.focus();
  }
</script>

<svelte:window on:keydown={onKey} />

<div class="scrim" role="presentation" onclick={(e) => e.target === e.currentTarget && close(undefined)}>
  <div class="dialog" bind:this={panel} class:destructive class:wide use:focusFirst role="dialog" aria-modal="true">
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
