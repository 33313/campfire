<script>
  // Right-click on a card: the actions that were already on the selection bar,
  // at the pointer instead of at the top of the screen, plus the two that only
  // mean anything for one ticket.
  //
  // It acts on whatever is selected. Right-clicking a card outside the
  // selection selects that card first, so the menu always names what it will
  // touch and never quietly acts on something else.
  //
  // Move to, Assignee, Priority and Bucket open a sub-menu of their values
  // beside the row, and a value is applied as it is clicked, with no dialog
  // between (DEV-81). Reject keeps its dialog: it asks for a reason.

  import { untrack, tick } from "svelte";

  import { api } from "../lib/api.js";
  import { navigate } from "../lib/router.svelte.js";
  import { openModal, toast, failure } from "../lib/ui.svelte.js";
  import { moveMany, applyToAll } from "../lib/bulk.js";
  import { assigneeOptions, priorityOptions, bucketOptions } from "../lib/options.js";
  import { plural } from "../lib/format.js";
  import { NAV } from "../lib/icons.js";
  import Icon from "./Icon.svelte";
  import OptionIcon from "../dialogs/OptionIcon.svelte";
  import SplitDialog from "../dialogs/SplitDialog.svelte";

  let { ids, item, states, x, y, onapplied, onclose } = $props();

  // What the menu named when it opened, copied out of the props: the actions
  // outlive the menu — it closes before the dialog it opens — and a prop read
  // after that is a read of a board state that has already moved on.
  const [picked, target] = untrack(() => [[...ids], item]);

  let el = $state(null);
  let one = picked.length === 1;

  // Each sub-menu: its field, and its values. For one card the value it has
  // now is marked.
  const SUBS = {
    "Move to": { field: "state", now: target.stateId, options: () => states
      .filter((s) => s.role !== "rejected")
      .map((s) => ({ value: s.id, label: s.name, icon: { pip: s.role } })) },
    Assignee: { field: "assignee", now: target.assigneeId || 0, options: assigneeOptions },
    Priority: { field: "priority", now: target.priority, options: priorityOptions },
    Bucket: { field: "bucket", now: target.bucketId || 0, options: bucketOptions },
  };

  // The sub-menu that is open: its name, its values, and the row it hangs
  // from. A pointer crossing other rows on its way into the sub-menu does not
  // swap it for theirs: a change waits a moment, and reaching the sub-menu
  // calls it off.
  let sub = $state(null);
  let wait = 0;
  async function openSub(name, row, keyboard = false) {
    if (sub?.name !== name) {
      const options = await SUBS[name].options();
      sub = { name, options, top: row.getBoundingClientRect().top, row };
    }
    if (keyboard) {
      await tick();
      el.querySelector(".sub-menu button")?.focus();
    }
  }
  function closeSub() {
    sub?.row.focus();
    sub = null;
  }
  function hover(name, row) {
    clearTimeout(wait);
    wait = setTimeout(() => (name ? openSub(name, row) : (sub = null)), sub ? 160 : 60);
  }

  // Closes when you look away from it or press Escape, like the team
  // drop-down. The pointer check is on the way down and ignores its own panel.
  $effect(() => {
    el.querySelector("button").focus(); // so a menu opened from the keyboard is in reach
    const away = (e) => { if (!el?.contains(e.target)) onclose(); };
    const key = (e) => e.key === "Escape" && onclose();
    addEventListener("pointerdown", away, true);
    addEventListener("keydown", key);
    return () => {
      clearTimeout(wait);
      removeEventListener("pointerdown", away, true);
      removeEventListener("keydown", key);
    };
  });

  // Kept on screen: a card near the right edge or the bottom would otherwise
  // open a menu half of which cannot be reached. A sub-menu opens to the
  // right, or to the left where the right has no room, and scrolls within
  // itself when its list is longer than the window.
  const WIDTH = 186, SUB = 220;
  const left = $derived(Math.min(x, innerWidth - WIDTH - 10));
  const top = $derived(Math.min(y, innerHeight - (one ? 250 : 190)));
  const subLeft = $derived(left + WIDTH + SUB + 8 > innerWidth ? left - SUB + 1 : left + WIDTH - 1);
  const subTop = $derived(sub
    ? Math.max(8, Math.min(sub.top - 4, innerHeight - 8 - Math.min(sub.options.length * 29 + 8, 340)))
    : 0);

  // The menu goes before the dialog it opens: leaving it up behind a modal is
  // two things asking for the same answer.
  async function run(fn) {
    onclose();
    if (await fn()) onapplied();
  }

  const reject = () => run(() => moveMany(picked, states, (s) => s.role === "rejected"));
  function apply(value) {
    const { field } = SUBS[sub.name];
    run(() => applyToAll(picked, field, value));
  }

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

{#snippet parent(name)}
  <button class="team-option" class:open={sub?.name === name} aria-haspopup="menu"
          aria-expanded={sub?.name === name}
          onpointerenter={(e) => hover(name, e.currentTarget)}
          onclick={(e) => { clearTimeout(wait); openSub(name, e.currentTarget); }}
          onkeydown={(e) => e.key === "ArrowRight" && openSub(name, e.currentTarget, true)}>
    <span>{name}</span>
    <span class="sub-chevron"><Icon body={NAV.chevron} size={11} /></span>
  </button>
{/snippet}

{#snippet plain(label, fn)}
  <button class="team-option" onpointerenter={() => hover(null)} onclick={fn}>{label}</button>
{/snippet}

<div class="card-menu" role="menu" bind:this={el} style="left:{left}px;top:{top}px">
  <span class="menu-head">{one ? target.identifier : `${picked.length} selected`}</span>
  {@render parent("Move to")}
  {@render plain("Reject…", reject)}
  <span class="menu-rule"></span>
  {@render parent("Assignee")}
  {@render parent("Priority")}
  {@render parent("Bucket")}
  {#if one}
    <span class="menu-rule"></span>
    {@render plain("Clone", clone)}
    {@render plain("Split…", split)}
    {@render plain("Open", () => { onclose(); navigate(`/${target.identifier}`); })}
  {/if}

  {#if sub}
    <div class="card-menu sub-menu" role="menu" tabindex="-1" aria-label={sub.name}
         style="left:{subLeft}px;top:{subTop}px" onpointerenter={() => clearTimeout(wait)}
         onkeydown={(e) => e.key === "ArrowLeft" && closeSub()}>
      {#each sub.options as o}
        <button class="team-option" class:on={one && o.value === SUBS[sub.name].now}
                role="menuitem" onclick={() => apply(o.value)}>
          <OptionIcon icon={o.icon} />
          <span class="sub-label">{o.label}</span>
        </button>
      {/each}
    </div>
  {/if}
</div>
