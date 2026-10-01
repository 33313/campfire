<script>
  import { untrack } from "svelte";
  import { FIELDS, FIELD, parse, complete, spanOf } from "../lib/csql.js";
  import { values, ensure } from "../lib/catalog.svelte.js";

  // A CSQL query, typed (SPEC.md §9.2). The browser's parser says what can
  // come next at the caret — a field, its operators, its values, then and or
  // or — and where the query is wrong, at no request. Nothing runs until
  // Enter: `onenter` is the caller's to apply, and an error the server gives
  // back (`error`, its span in bytes) is underlined where it points.
  let { value = $bindable(""), error = null, onenter = null, ontype = null, label = "Query",
        placeholder = "assignee = me and not state = Done" } = $props();

  // An offer carries a note only when it says what the offer does not: what
  // a relative day means, a bucket's title, a team's name, a priority's word.
  const DAYS = [["today", ""], ["-7d", "7 days ago"], ["-30d", "30 days ago"], ["+7d", "in 7 days"]];

  const list = $props.id();
  let input = $state(null);
  let mirror = $state(null);
  let focused = $state(false);
  let caret = $state(untrack(() => value.length));
  let active = $state(0);
  let chose = $state(false); // an offer was moved to with the arrows: Enter takes it
  let closed = $state(false); // Escape shuts the list until the next keystroke
  let entered = $state(false); // Enter was pressed on this text: say what is wrong, all of it

  // What is wrong, from here or from the server. An error the caret is in is
  // a word still being typed, not a wrong one, until Enter says it is done.
  let local = $derived.by(() => {
    try { parse(value); return null; } catch (err) { return err; }
  });
  let problem = $derived.by(() => {
    // Submitted — by Enter, or by a Save the server answered — it is done.
    if (local && (entered || error || caret < local.pos || caret > local.end)) {
      return { message: local.message, span: [local.pos, local.end] };
    }
    if (!local && error) return { message: error.message, span: spanOf(value, error.span) };
    return null;
  });

  let at = $derived(complete(value, caret));
  // Fetching a field's values is a side effect, so it lives here and not in
  // the offers below, which only read.
  $effect(() => { if (focused && at.want === "value") ensure(at.field); });

  let offers = $derived.by(() => {
    const { want, field, prefix = "", depth } = at;
    let all = [];
    if (want === "field") {
      all = FIELDS.map((f) => ({ label: f, insert: `${f} ` }));
      all.push({ label: "not", insert: "not " }, { label: "(", insert: "(" });
    } else if (want === "op") {
      all = FIELD[field].ops.map((op) => ({ label: op, insert: op.endsWith("in") ? `${op} (` : `${op} ` }));
    } else if (want === "value") {
      const tail = at.inList ? "" : " ";
      const vals = FIELD[field].day
        ? [...DAYS.map(([token, note]) => ({ token, note })), ...(FIELD[field].nullable ? [{ token: "none" }] : [])]
        : values(field).map((v) => ({ token: v.token,
          note: field === "priority" ? v.label.split(" · ")[1] : v.token === "none" ? "" : v.note }));
      all = vals.map((v) => ({ label: v.token, note: v.note, insert: v.token + tail }));
    } else if (want === "open") {
      all = [{ label: "(", insert: "(" }];
    } else if (want === "next") {
      all = [{ label: ",", insert: ", " }, { label: ")", insert: ") " }];
    } else if (want === "joiner") {
      all = [{ label: "and", insert: "and " }, { label: "or", insert: "or " }];
      if (depth > 0) all.push({ label: ")", insert: ") " });
    }
    // A value is found by any part of it — rev finds "In Review" — and what
    // begins with what was typed comes first. Fields and keywords are found
    // by their start, as they are typed from it.
    const p = prefix.toLowerCase().replace(/^["']/, "");
    const name = (o) => o.label.toLowerCase().replace(/^"/, "");
    const anywhere = want === "value";
    const hit = (o) => name(o).startsWith(p) ? 0
      : anywhere && (name(o).includes(p) || (p.length > 1 && (o.note || "").toLowerCase().includes(p))) ? 1
      : 2;
    return all.map((o) => [hit(o), o]).filter(([h]) => h < 2)
      .sort((a, b) => a[0] - b[0]).map(([, o]) => o).slice(0, 8);
  });
  let open = $derived(focused && !closed && (offers.length > 0 || !!problem));

  // Where the list hangs: under the box, outside any bar that scrolls.
  let place = $state({ left: 0, top: 0, width: 0 });
  function measure() {
    const r = input?.getBoundingClientRect();
    if (r) place = { left: r.left, top: r.bottom + 4, width: Math.max(r.width, 280) };
  }

  function sync() {
    caret = input.selectionStart ?? value.length;
    if (mirror) mirror.scrollLeft = input.scrollLeft;
  }

  function typed() {
    ontype?.();
    closed = false;
    entered = false;
    active = 0;
    chose = false;
    sync();
    measure();
  }

  function take(o) {
    ontype?.();
    // The word being typed, an opening quote and all, is what the offer replaces.
    const end = caret;
    value = value.slice(0, at.from) + o.insert + value.slice(end).replace(/^ /, "");
    const to = at.from + o.insert.length;
    active = 0;
    chose = false;
    queueMicrotask(() => { input.focus(); input.setSelectionRange(to, to); sync(); });
  }

  function key(e) {
    const listed = open && offers.length;
    // Tab takes the first offer, or the one moved to; Enter takes one only
    // when it was moved to, and otherwise runs the query as typed.
    if (listed && (e.key === "Tab" || (e.key === "Enter" && chose))) {
      e.preventDefault();
      take(offers[active]);
    } else if (e.key === "Enter") {
      e.preventDefault();
      entered = true;
      closed = false;
      if (!local) onenter?.(value);
    } else if (e.key === "ArrowDown" && listed) {
      e.preventDefault();
      active = chose ? (active + 1) % offers.length : 0;
      chose = true;
    } else if (e.key === "ArrowUp" && listed) {
      e.preventDefault();
      active = (active - 1 + offers.length) % offers.length;
      chose = true;
    } else if (e.key === "Escape" && open) {
      e.stopPropagation(); // the list closes, not the dialog the box may be in
      closed = true;
    }
  }

  // The problem's span, cut out of the text for the underline.
  let cut = $derived(problem?.span
    ? [value.slice(0, problem.span[0]), value.slice(...problem.span) || " ", value.slice(problem.span[1])] : null);
  // An error from the server opens the list to say so.
  $effect(() => { if (error) { closed = false; measure(); } });
</script>

<div class="query-box" class:bad={!!problem}>
  {#if cut}
    <div class="query-mirror" aria-hidden="true" bind:this={mirror}>{cut[0]}<mark>{cut[1]}</mark>{cut[2]}</div>
  {/if}
  <input class="input" bind:this={input} bind:value aria-label={label} {placeholder}
         spellcheck="false" autocomplete="off" title={problem?.message || null}
         aria-invalid={!!problem} role="combobox" aria-controls={list} aria-expanded={open}
         aria-autocomplete="list"
         oninput={typed} onkeydown={key} onclick={sync} onkeyup={sync} onscroll={sync}
         onfocus={() => { focused = true; measure(); sync(); }}
         onblur={() => setTimeout(() => {
           // Gone, unless it came back in the meantime: a blur and a focus in
           // quick succession leave the box focused, offers and all.
           focused = document.activeElement === input;
           // A box that loses focus scrolls back to its start, with no scroll
           // event: the underline follows it there.
           if (mirror) mirror.scrollLeft = input.scrollLeft;
         }, 120)} />
</div>

{#if open}
  <div class="query-offers" id={list} role="listbox"
       style="left:{place.left}px;top:{place.top}px;width:{place.width}px">
    {#if problem}<div class="query-error">{problem.message}</div>{/if}
    {#each offers as o, i}
      <button class="search-suggestion" class:on={i === active && chose} role="option"
              aria-selected={i === active && chose}
              onmousedown={(e) => { e.preventDefault(); take(o); }}>
        <span class="ident">{o.label}</span>
        {#if o.note}<span class="title">{o.note}</span>{/if}
      </button>
    {/each}
  </div>
{/if}
