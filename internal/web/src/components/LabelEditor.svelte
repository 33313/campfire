<script>
  // Labels, added the same way wherever they are added: the chips and the box
  // you type in on one line, and the labels that already exist offered as you
  // type so a second spelling of one is not the easiest thing to do. The
  // ticket page and the create dialog both use it.
  //
  // The box waits behind a quiet "+" (DEV-100): the same one click, since the
  // "+" opens it already focused, and no empty box sitting among the chips.

  import { tick } from "svelte";
  import { app } from "../lib/state.svelte.js";
  import { NAV } from "../lib/icons.js";
  import Icon from "./Icon.svelte";

  // `draft` is bindable so a form can add whatever is still in the box when it
  // is submitted: what was typed and not confirmed was still meant to be one.
  let { labels = [], draft = $bindable(""), onadd, onremove } = $props();

  let active = $state(0); // which suggestion the keyboard is on
  let open = $state(false); // the box is up, not the "+"
  let plus = $state();

  // Escape puts the "+" back and the focus on it, so the keyboard is not
  // dropped on the page.
  async function shut() {
    draft = "";
    open = false;
    await tick();
    plus?.focus();
  }

  // Labels are created by typing them: there is no registry to pick from first
  // (SPEC.md §8.1). The list is a shortcut to an existing one, never a gate.
  let matches = $derived.by(() => {
    const typed = draft.trim().toLowerCase();
    if (!typed) return [];
    return app.labels
      .map((l) => l.name)
      .filter((n) => !labels.includes(n) && n.toLowerCase().startsWith(typed))
      .slice(0, 6);
  });

  const focus = (el) => el.focus();

  function add(name) {
    const value = (name ?? draft).trim();
    draft = "";
    active = 0;
    if (value && !labels.includes(value)) onadd(value);
  }

  function key(e) {
    if (e.key === "Enter") {
      e.preventDefault();
      add(matches[active]);
    } else if (e.key === "ArrowDown" && matches.length) {
      e.preventDefault();
      active = (active + 1) % matches.length;
    } else if (e.key === "ArrowUp" && matches.length) {
      e.preventDefault();
      active = (active - 1 + matches.length) % matches.length;
    } else if (e.key === "Escape") {
      shut();
    } else if (e.key === "Backspace" && !draft && labels.length) {
      onremove(labels[labels.length - 1]); // the box is empty: take the last chip
    } else {
      active = 0;
    }
  }
</script>

<div class="label-editor">
  {#each labels as name}
    <span class="label-chip">
      <span>{name}</span>
      <button class="btn quiet" style="padding:0;height:auto;border:0"
              aria-label="Remove {name}" onclick={() => onremove(name)}>
        <Icon body={NAV.close} size={9} />
      </button>
    </span>
  {/each}
  <!-- No placeholder: the box sits among the chips it makes, which says what
       it is better than grey words inside it do. It stays up while it holds
       text, which a form still adds when it is submitted. -->
  {#if open || draft}
    <input class="label-input" aria-label="Labels" bind:value={draft} onkeydown={key}
           use:focus onblur={() => setTimeout(() => { active = 0; open = false; }, 0)} />
  {:else}
    <button type="button" class="btn quiet label-add" aria-label="Add a label" title="Add a label"
            bind:this={plus} onclick={() => (open = true)}>
      <Icon body={NAV.plus} size={11} />
    </button>
  {/if}
  {#if matches.length}
    <div class="label-suggestions" role="listbox">
      {#each matches as name, i}
        <!-- The typed part is shown as typed, so it is obvious which existing
             label is being offered and how much of it is already there. -->
        <button class="label-suggestion" class:on={i === active} role="option"
                aria-selected={i === active} onmousedown={() => add(name)}>
          <b>{name.slice(0, draft.trim().length)}</b><span>{name.slice(draft.trim().length)}</span>
        </button>
      {/each}
    </div>
  {/if}
</div>
