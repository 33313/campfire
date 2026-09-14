<script>
  import { untrack } from "svelte";
  import Icon from "./Icon.svelte";
  import { NAV } from "../lib/icons.js";
  import { api } from "../lib/api.js";
  import { navigate } from "../lib/router.svelte.js";

  // Search, with the items that match offered as you type.
  //
  // The suggestions are a way through to one item, not a second way to filter:
  // Enter still runs the search the screen is showing, exactly as before.
  //
  // What keeps it off the network: nothing until two characters, one request
  // per pause in the typing rather than one per keystroke, and a reply that
  // arrives after a newer one was sent is dropped rather than drawn. The query
  // is the ordinary filtered list of SPEC.md §9.2 — no second index, nothing
  // kept in memory between keystrokes.
  let { value = "", placeholder = "Title and description", teamId = null, onsearch } = $props();

  const WAIT = 200; // ms of quiet before asking
  const LEAST = 2;  // shorter than this matches most of the instance

  let text = $state(untrack(() => value));
  let hits = $state([]);
  let active = $state(-1);
  let open = $state(false);
  let timer = null;
  let issued = 0;

  function typed() {
    clearTimeout(timer);
    active = -1;
    const q = text.trim();
    open = true;
    if (q.length < LEAST) { hits = []; return; }
    timer = setTimeout(() => look(q), WAIT);
  }

  async function look(q) {
    const mine = ++issued;
    const filter = { text: q };
    if (teamId) filter.teamIds = [teamId];
    try {
      const page = await api.listItems(filter, "", 6);
      if (mine === issued) hits = page.items || [];
    } catch {
      // A suggestion that cannot be fetched is no suggestion. The search itself
      // still runs on Enter and reports its own failures.
      if (mine === issued) hits = [];
    }
  }

  function go(item) {
    close();
    navigate(`/${item.identifier}`);
  }

  function close() {
    clearTimeout(timer);
    open = false;
    active = -1;
  }

  function key(e) {
    if (e.key === "Enter") {
      if (active >= 0 && hits[active]) { e.preventDefault(); go(hits[active]); return; }
      close();
      onsearch(text);
    } else if (e.key === "ArrowDown" && hits.length) {
      e.preventDefault();
      active = (active + 1) % hits.length;
    } else if (e.key === "ArrowUp" && hits.length) {
      e.preventDefault();
      active = (active - 1 + hits.length) % hits.length;
    } else if (e.key === "Escape") {
      close();
    }
  }
</script>

<div class="search-box">
  <Icon body={NAV.search} size={13} />
  <input class="input" {placeholder} bind:value={text} oninput={typed} onkeydown={key}
         onfocus={() => (open = true)}
         onblur={() => setTimeout(close, 120)} />
  {#if open && hits.length}
    <div class="search-suggestions" role="listbox">
      {#each hits as item, i}
        <button class="search-suggestion" class:on={i === active} role="option"
                aria-selected={i === active} onmousedown={() => go(item)}>
          <span class="ident">{item.identifier}</span>
          <span class="title">{item.title}</span>
        </button>
      {/each}
    </div>
  {/if}
</div>
