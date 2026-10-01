<script>
  // Right-click on a view in the sidebar: where a view is changed or deleted,
  // apart from the screens that only use it. Same panel as the card menu.

  import { untrack } from "svelte";
  import { editView, deleteView } from "../lib/views.js";

  let { view, x, y, onclose } = $props();

  // Copied out of the props: the menu closes before the dialog it opens.
  const target = untrack(() => view);
  let el = $state(null);

  // Closes when you look away from it or press Escape, like the card menu.
  $effect(() => {
    el.querySelector("button").focus(); // so a menu opened from the keyboard is in reach
    const away = (e) => { if (!el?.contains(e.target)) onclose(); };
    const key = (e) => e.key === "Escape" && onclose();
    addEventListener("pointerdown", away, true);
    addEventListener("keydown", key);
    return () => {
      removeEventListener("pointerdown", away, true);
      removeEventListener("keydown", key);
    };
  });

  const top = $derived(Math.min(y, innerHeight - 110));

  function run(fn) {
    onclose();
    fn(target);
  }
</script>

<div class="card-menu" role="menu" bind:this={el} style="left:{x}px;top:{top}px">
  <span class="menu-head">{target.name}</span>
  <button class="team-option" role="menuitem" onclick={() => run(editView)}>Edit…</button>
  <span class="menu-rule"></span>
  <button class="team-option" role="menuitem" style="color:var(--alert)" onclick={() => run(deleteView)}>Delete…</button>
</div>
