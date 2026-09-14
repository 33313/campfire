<script>
  // Right-click on a card: the actions that were already on the selection bar,
  // at the pointer instead of at the top of the screen, plus the two that only
  // mean anything for one ticket.
  //
  // It acts on whatever is selected. Right-clicking a card outside the
  // selection selects that card first, so the menu always names what it will
  // touch and never quietly acts on something else.

  import { api } from "../lib/api.js";
  import { navigate } from "../lib/router.svelte.js";
  import { openModal, toast, failure } from "../lib/ui.svelte.js";
  import { moveMany, editMany } from "../lib/bulk.js";
  import { assigneeOptions, priorityOptions, bucketOptions } from "../lib/options.js";
  import { plural } from "../lib/format.js";
  import SplitDialog from "../dialogs/SplitDialog.svelte";

  let { ids, item, states, x, y, onapplied, onclose } = $props();

  // What the menu named when it opened, copied out of the props: the actions
  // outlive the menu — it closes before the dialog it opens — and a prop read
  // after that is a read of a board state that has already moved on.
  const picked = [...ids];
  const target = item;

  let el = $state(null);
  let one = picked.length === 1;

  // Closes when you look away from it or press Escape, like the team
  // drop-down. The pointer check is on the way down and ignores its own panel.
  $effect(() => {
    const away = (e) => { if (!el?.contains(e.target)) onclose(); };
    const key = (e) => e.key === "Escape" && onclose();
    addEventListener("pointerdown", away, true);
    addEventListener("keydown", key);
    return () => {
      removeEventListener("pointerdown", away, true);
      removeEventListener("keydown", key);
    };
  });

  // The pointer arrives in viewport pixels, but this menu is inside #app,
  // where a CSS length is multiplied by the zoom of a large panel (DESIGN.md
  // §1) — written back raw, the menu opened a quarter of the screen away from
  // the click. Everything below is in the app's own pixels.
  //
  // Kept on screen: a card near the right edge or the bottom would otherwise
  // open a menu half of which cannot be reached.
  const zoom = document.getElementById("app")?.currentCSSZoom ?? 1;
  const left = Math.min(x / zoom, innerWidth / zoom - 196);
  const top = Math.min(y / zoom, innerHeight / zoom - (one ? 250 : 190));

  // The menu goes before the dialog it opens: leaving it up behind a modal is
  // two things asking for the same answer.
  async function run(fn) {
    onclose();
    if (await fn()) onapplied();
  }

  const move = (only) => run(() => moveMany(picked, states, only));
  const edit = (field, options) => run(async () => editMany(picked, field, await options));

  const clone = () => run(async () => {
    try {
      navigate(`/${(await api.cloneItem(target.id)).identifier}`);
      return false; // the clone is its own screen; there is nothing to reload
    } catch (err) { failure(err); return false; }
  });

  const split = () => run(async () => {
    const res = await openModal(SplitDialog, { item: target });
    if (!res) return false;
    try {
      await api.splitItem(target.id, res.titles, res.rejectSource);
      toast(`${plural(res.titles.length, "new ticket")}`);
      return true;
    } catch (err) { failure(err); return false; }
  });
</script>

<div class="card-menu" role="menu" bind:this={el} style="left:{left}px;top:{top}px">
  <span class="menu-head">{one ? target.identifier : `${picked.length} selected`}</span>
  <button class="team-option" onclick={() => move((s) => s.role !== "rejected")}>Move to…</button>
  <button class="team-option" onclick={() => move((s) => s.role === "rejected")}>Reject…</button>
  <span class="menu-rule"></span>
  <button class="team-option" onclick={() => edit("Assignee", assigneeOptions())}>Assignee…</button>
  <button class="team-option" onclick={() => edit("Priority", priorityOptions())}>Priority…</button>
  <button class="team-option" onclick={() => edit("Bucket", bucketOptions())}>Bucket…</button>
  {#if one}
    <span class="menu-rule"></span>
    <button class="team-option" onclick={clone}>Clone</button>
    <button class="team-option" onclick={split}>Split…</button>
    <button class="team-option" onclick={() => { onclose(); navigate(`/${target.identifier}`); }}>
      Open
    </button>
  {/if}
</div>
