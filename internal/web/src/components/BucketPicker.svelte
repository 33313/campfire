<script>
  // The bucket field (DEV-92): a box that names the bucket, and the buckets
  // under it as soon as it is clicked. Typing narrows them, by identifier or
  // by title; arrows and Enter pick one, Escape puts the box back.
  //
  // The list is the catalog's, asked for once and filtered here, so typing
  // costs no query. The buckets picked most in this browser come first.

  import { untrack } from "svelte";
  import { buckets } from "../lib/catalog.svelte.js";
  import { byUse, usedBucket } from "../lib/options.js";
  import BucketIcon from "./BucketIcon.svelte";

  let { value = 0, rendering = "mono", invalid = false, label = "Bucket", onpick } = $props();

  let all = $state([]);
  buckets().then((list) => (all = byUse(list)));

  let input;
  let open = $state(false);
  let text = $state("");
  let active = $state(0);
  let place = $state({ left: 0, top: 0, width: 0 });
  const list = `bucket-offers-${Math.random().toString(36).slice(2)}`;

  let chosen = $derived(all.find((b) => b.id === value));
  // What the box says when it is not being typed in.
  const name = () => (chosen ? `${chosen.identifier}  ${chosen.title}` : "");
  // Where the list starts: on the bucket the box names, after "No bucket".
  const start = () => Math.max(0, all.findIndex((b) => b.id === value) + 1);
  // The box names the bucket unless it is being typed in, including when the
  // list arrives after the box opened, as it does in a dialog that focuses it.
  $effect(() => {
    const n = name();
    untrack(() => {
      if (open && text) return;
      text = n;
      if (open) { active = start(); queueMicrotask(() => input.select()); }
    });
  });

  // Nothing typed yet is every bucket, and the way back to none.
  let offers = $derived.by(() => {
    const q = text.trim().toLowerCase();
    if (!open || q === name().toLowerCase() || !q) return [{ id: 0 }, ...all];
    return all.filter((b) => `${b.identifier} ${b.title}`.toLowerCase().includes(q));
  });

  // Fixed, measured from the box: a dialog's body scrolls, and would clip it.
  function measure() {
    const r = input.getBoundingClientRect();
    place = { left: r.left, top: r.bottom + 4, width: r.width };
  }
  function show() {
    if (open) return;
    measure();
    // A dialog still rising is measured again where it comes to rest.
    const rising = input.closest(".dialog")?.getAnimations() || [];
    if (rising.length) Promise.all(rising.map((a) => a.finished)).then(measure, () => {});
    open = true;
    active = start();
    input.select(); // so typing replaces the name rather than adding to it
  }
  function hide() {
    open = false;
    text = name();
  }
  function pick(b) {
    hide();
    if (b.id) usedBucket(b.id);
    onpick?.(b.id);
  }

  function key(e) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) return show();
      const n = offers.length;
      if (n) active = (active + (e.key === "ArrowDown" ? 1 : n - 1)) % n;
      queueMicrotask(() => document.getElementById(list)?.children[active]?.scrollIntoView({ block: "nearest" }));
    } else if (e.key === "Enter" && open) {
      e.preventDefault();
      if (offers[active]) pick(offers[active]);
    } else if (e.key === "Escape" && open) {
      e.stopPropagation(); // the list closes, not the dialog it is in
      hide();
    }
  }
</script>

<div class="bucket-picker">
  {#if chosen}
    <span class="bucket-picker-icon"><BucketIcon name={chosen.icon} size={13} {rendering} color={chosen.color} /></span>
  {/if}
  <input class="input" class:error={invalid} class:iconed={!!chosen} bind:this={input} bind:value={text}
         aria-label={label} placeholder="None" autocomplete="off" spellcheck="false"
         role="combobox" aria-controls={list} aria-expanded={open} aria-autocomplete="list"
         onfocus={show} onclick={show} oninput={() => { if (!open) show(); active = 0; }}
         onkeydown={key} onblur={hide} />
</div>

{#if open}
  <div class="query-offers bucket-offers" id={list} role="listbox"
       style="left:{place.left}px;top:{place.top}px;width:{place.width}px">
    {#each offers as b, i (b.id)}
      <button class="search-suggestion" class:on={i === active} role="option" aria-selected={i === active}
              onmousedown={(e) => { e.preventDefault(); pick(b); }} onmouseenter={() => (active = i)}>
        {#if b.id}
          <BucketIcon name={b.icon} size={13} {rendering} color={b.color} />
          <span class="ident">{b.identifier}</span>
          <span class="title">{b.title}</span>
        {:else}
          <span class="title">No bucket</span>
        {/if}
      </button>
    {:else}
      <div class="explain" style="padding:6px">No bucket matches.</div>
    {/each}
  </div>
{/if}
