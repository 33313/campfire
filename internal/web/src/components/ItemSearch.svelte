<script>
  // Find an item to relate to (DEV-94): type part of its title or its
  // identifier, and the items that match are offered under the box, as the
  // header search offers them. Arrows and Enter pick one; Escape closes the
  // list and leaves the dialog up.
  //
  // Off the network the way the header search is: nothing under two
  // characters, one request per pause in the typing, and a late reply to an
  // older query dropped. Text search reads titles and descriptions, so a query
  // shaped like an identifier is also looked up as one, and comes first.

  import { api } from "../lib/api.js";
  import { quote } from "../lib/csql.js";

  let { text = $bindable(""), exclude = [], label = "Related item", placeholder = "",
        invalid = false, onpick } = $props();

  const WAIT = 200;
  const LEAST = 2;
  const IDENT = /^[a-z][a-z0-9]*-\d+$/i;

  let input;
  let hits = $state([]);
  let active = $state(0);
  let open = $state(false);
  let place = $state({ left: 0, top: 0, width: 0 });
  let timer = null, waiting = null, issued = 0;
  const list = `item-offers-${Math.random().toString(36).slice(2)}`;

  // Fixed, measured from the box: a dialog's body scrolls, and would clip it.
  function measure() {
    const r = input.getBoundingClientRect();
    place = { left: r.left, top: r.bottom + 4, width: r.width };
  }

  async function look(q) {
    const mine = ++issued;
    const [byId, page] = await Promise.all([
      IDENT.test(q) ? api.lookupItem(q).catch(() => null) : null,
      api.listItems({ csql: `text ~ ${quote(q)}` }, "", 6).catch(() => null),
    ]);
    const found = [byId, ...(page?.items || [])].filter((it, i, all) => it && !exclude.includes(it.id)
      && all.findIndex((o) => o?.id === it.id) === i).slice(0, 6);
    if (mine === issued) { hits = found; active = 0; }
    return found;
  }

  function typed() {
    clearTimeout(timer);
    measure();
    open = true;
    const q = text.trim();
    if (q.length < LEAST) { hits = []; waiting = null; return; }
    waiting = q;
    timer = setTimeout(() => { waiting = null; look(q); }, WAIT);
  }

  function pick(it) {
    close();
    text = "";
    hits = [];
    onpick(it);
  }

  function close() {
    clearTimeout(timer);
    open = false;
  }

  async function key(e) {
    if (e.key === "Enter") {
      e.preventDefault();
      // Enter before the pause ends asks at once rather than doing nothing.
      let offers = hits;
      if (waiting) { clearTimeout(timer); const q = waiting; waiting = null; offers = await look(q); }
      if (offers[active]) pick(offers[active]);
    } else if ((e.key === "ArrowDown" || e.key === "ArrowUp") && hits.length) {
      e.preventDefault();
      const n = hits.length;
      active = (active + (e.key === "ArrowDown" ? 1 : n - 1)) % n;
    } else if (e.key === "Escape" && open) {
      e.stopPropagation(); // the list closes, not the dialog it is in
      close();
    }
  }
</script>

<div class="item-search">
  <input class="input" class:error={invalid} bind:this={input} bind:value={text} aria-label={label}
         {placeholder} autocomplete="off" spellcheck="false" role="combobox" aria-controls={list}
         aria-expanded={open && hits.length > 0} aria-autocomplete="list"
         oninput={typed} onkeydown={key} onfocus={() => { if (hits.length) { measure(); open = true; } }}
         onblur={close} />
</div>

{#if open && hits.length}
  <div class="query-offers item-offers" id={list} role="listbox"
       style="left:{place.left}px;top:{place.top}px;width:{place.width}px">
    {#each hits as it, i (it.id)}
      <button class="search-suggestion" class:on={i === active} role="option" aria-selected={i === active}
              onmousedown={(e) => { e.preventDefault(); pick(it); }} onmouseenter={() => (active = i)}>
        <span class="ident">{it.identifier}</span>
        <span class="title">{it.title}</span>
      </button>
    {/each}
  </div>
{/if}
