<script>
  import Icon from "./Icon.svelte";
  import { NAV } from "../lib/icons.js";
  import { values, labelOf, ensure } from "../lib/catalog.svelte.js";
  import { dismiss, under } from "../lib/dismiss.js";

  // One field of the filter, as a chip. Its dropdown lists the field's
  // values, and a click cycles each one: [ ] not in the filter, [+] is,
  // [−] isn't. The dropdown applies when it closes; Escape leaves it as it
  // was. The tokens are how the query writes the values (SPEC.md §9.2).
  let { field, title, plus = [], minus = [], onchange } = $props();

  let chip = $state(null);
  let panel = $state(null);
  let open = $state(false);
  let at = $state({ left: 0, top: 0 });
  let draft = $state({ plus: [], minus: [] });
  let search = $state("");
  let focus = $state(0);

  const on = $derived(plus.length + minus.length > 0);
  const shown = $derived([...plus.map((t) => labelOf(field, t)), ...minus.map((t) => `−${labelOf(field, t)}`)]);
  const label = $derived(on
    ? `${title}: ${shown.slice(0, 2).join(", ")}${shown.length > 2 ? ` +${shown.length - 2}` : ""}` : title);

  // The field's values, and any the filter holds that the list does not (a
  // deleted one, written as #id), so it can still be taken out.
  const rows = $derived.by(() => {
    const all = values(field);
    const known = new Set(all.map((v) => v.token));
    const extra = [...draft.plus, ...draft.minus].filter((t) => !known.has(t))
      .map((t) => ({ token: t, label: labelOf(field, t) }));
    const q = search.trim().toLowerCase();
    return [...all, ...extra].filter((v) => !q || v.label.toLowerCase().includes(q)
      || (v.note || "").toLowerCase().includes(q));
  });
  const stateOf = (t) => (draft.plus.includes(t) ? "+" : draft.minus.includes(t) ? "-" : " ");

  function show() {
    ensure(field);
    draft = { plus: [...plus], minus: [...minus] };
    search = "";
    focus = 0;
    at = under(chip);
    open = true;
  }

  function close(keep) {
    if (!open) return;
    open = false;
    const same = draft.plus.join() === plus.join() && draft.minus.join() === minus.join();
    if (keep && !same) onchange(draft);
  }

  function cycle(t) {
    const s = stateOf(t);
    draft = {
      plus: s === " " ? [...draft.plus, t] : draft.plus.filter((x) => x !== t),
      minus: s === "+" ? [...draft.minus, t] : draft.minus.filter((x) => x !== t),
    };
  }

  function key(e) {
    if (e.key === "ArrowDown") { e.preventDefault(); focus = Math.min(focus + 1, rows.length - 1); }
    else if (e.key === "ArrowUp") { e.preventDefault(); focus = Math.max(focus - 1, 0); }
    else if (e.key === " " && e.target.tagName !== "INPUT" || e.key === "Enter" && e.target.tagName === "INPUT") {
      e.preventDefault();
      if (rows[focus]) cycle(rows[focus].token);
    } else if (e.key === "Enter") { e.preventDefault(); close(true); }
  }

  $effect(() => {
    if (!open) return;
    panel?.querySelector("input, button")?.focus();
    return dismiss(() => [panel, chip], () => close(true), () => close(false));
  });
  $effect(() => { panel?.querySelectorAll(".tri")[focus]?.scrollIntoView({ block: "nearest" }); });
</script>

<span class="chip-group" bind:this={chip}>
  <button class="chip" class:on aria-expanded={open} aria-haspopup="listbox"
          onclick={() => (open ? close(true) : show())}>
    <span>{label}</span>
    {#if !on}<Icon body={NAV.chevron} size={10} />{/if}
  </button>
  {#if on}
    <button class="chip on" aria-label="Clear {title}" onclick={() => onchange({ plus: [], minus: [] })}>
      <Icon body={NAV.close} size={10} />
    </button>
  {/if}
</span>

{#if open}
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div class="pop filter-pop" role="listbox" aria-label={title} aria-multiselectable="true" tabindex="-1"
       bind:this={panel} style="left:{at.left}px;top:{at.top}px" onkeydown={key}>
    {#if values(field).length > 7}
      <input class="input pop-search" placeholder="Find a {title.toLowerCase()}" bind:value={search}
             oninput={() => (focus = 0)} aria-label="Find" />
    {/if}
    <div class="pop-rows">
      {#each rows as v, i (v.token)}
        {@const s = stateOf(v.token)}
        <button class="tri" class:plus={s === "+"} class:minus={s === "-"} class:focus={i === focus}
                role="option" aria-selected={s !== " "} title={s === "+" ? "Is" : s === "-" ? "Isn't" : null}
                onclick={() => { focus = i; cycle(v.token); }}>
          <span class="tri-box">[{s === "+" ? "+" : s === "-" ? "−" : " "}]</span>
          <span class="tri-label">{v.label}</span>
          {#if v.note}<span class="tri-note">{v.note}</span>{/if}
        </button>
      {:else}
        <span class="pop-empty">Nothing matches.</span>
      {/each}
    </div>
    <span class="pop-hint">Click: [+] is · [−] isn't · Esc undoes</span>
  </div>
{/if}
