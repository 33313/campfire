<script>
  import BucketIcon from "./BucketIcon.svelte";
  import Icon from "./Icon.svelte";
  import { BUCKET_ICON_GROUPS, BUCKET_COLOURS, NAV } from "../lib/icons.js";
  import { dismiss, under } from "../lib/dismiss.js";

  // A bucket's icon and colour, picked together because they are seen
  // together (DEV-79). The field names both; under it hang the icons in their
  // groups and the colours along the foot. A click applies, and the panel
  // stays up so the other can be picked too.
  let { icon, color = "", boxed = false, onpick } = $props();

  const NAMES = { planning: "Planning", signals: "Signals", work: "Work", places: "Places" };
  const WIDTH = 374; // eleven icons a row: no group wraps

  let field = $state(null);
  let panel = $state(null);
  let open = $state(false);
  let at = $state({ left: 0, top: 0 });

  function show() {
    at = under(field, WIDTH);
    open = true;
  }

  $effect(() => {
    if (!open) return;
    // Hung above the field where there is no room under it, so the colours
    // at its foot are never off the screen.
    const h = panel.offsetHeight;
    if (at.top + h > innerHeight - 8) {
      at = { ...at, top: Math.max(8, field.getBoundingClientRect().top - 4 - h) };
    }
    panel?.querySelector(".icon-option.on, .icon-option")?.focus();
    return dismiss(() => [panel, field], () => (open = false));
  });
</script>

<button class={boxed ? "input icon-field" : "aside-control"} bind:this={field}
        aria-haspopup="dialog" aria-expanded={open} onclick={() => (open ? (open = false) : show())}>
  <BucketIcon name={icon} size={16} {color} />
  <span>{icon}</span>
  <span class="icon-field-colour">· {color || "no colour"}</span>
  <span style="margin-left:auto;display:flex"><Icon body={NAV.chevron} size={11} color="var(--faint)" /></span>
</button>

{#if open}
  <div class="pop icon-pop" role="dialog" aria-label="Icon and colour" bind:this={panel}
       style="left:{at.left}px;top:{at.top}px;width:{WIDTH}px">
    <div class="icon-groups">
      {#each Object.entries(BUCKET_ICON_GROUPS) as [group, set] (group)}
        <span class="menu-head">{NAMES[group]}</span>
        <div class="icon-grid">
          {#each Object.keys(set) as name (name)}
            <button class="icon-option" class:on={icon === name} title={name} aria-label={name}
                    aria-pressed={icon === name} onclick={() => onpick({ icon: name })}>
              <BucketIcon {name} size={16} color={icon === name ? color : null} />
            </button>
          {/each}
        </div>
      {/each}
    </div>
    <div class="icon-colours">
      <button class="swatch" class:on={!color} title="No colour" aria-label="No colour"
              onclick={() => onpick({ color: "" })}></button>
      {#each Object.entries(BUCKET_COLOURS) as [name, hex] (name)}
        <button class="swatch" class:on={color === name} title={name} aria-label={name}
                style="background:{hex};border-color:{hex}" onclick={() => onpick({ color: name })}></button>
      {/each}
    </div>
  </div>
{/if}
