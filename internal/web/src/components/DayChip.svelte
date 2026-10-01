<script>
  import Icon from "./Icon.svelte";
  import { NAV } from "../lib/icons.js";
  import { dismiss, under } from "../lib/dismiss.js";

  // A date field of the filter, as a chip: a preset, or a range of days typed
  // in. Days are the reader's (SPEC.md §9.2). `value` is the chips' state for
  // the field: { from, to }, { before } or { none }.
  let { field, title, value = null, onchange } = $props();

  const PRESETS = {
    common: [
      ["Today", { from: "today", to: "today" }],
      ["Last 7 days", { from: "-7d" }],
      ["Last 30 days", { from: "-30d" }],
    ],
    due: [
      ["Before today", { before: "today" }],
      ["Today", { from: "today", to: "today" }],
      ["Next 7 days", { from: "today", to: "+7d" }],
      ["No due date", { none: true }],
    ],
    started: [["Not started", { none: true }]],
    completed: [["Not completed", { none: true }]],
  };
  const presets = field === "due" ? PRESETS.due : [...PRESETS.common, ...(PRESETS[field] || [])];
  const same = (a, b) => JSON.stringify(a || {}) === JSON.stringify(b || {});

  let chip = $state(null);
  let panel = $state(null);
  let open = $state(false);
  let at = $state({ left: 0, top: 0 });
  let from = $state("");
  let to = $state("");

  const preset = $derived(presets.find(([, v]) => same(v, value))?.[0]);
  const label = $derived(!value ? title : `${title}: ${preset || describe(value)}`);

  function describe(v) {
    if (v.before) return `before ${v.before}`;
    if (v.from && v.to) return `${v.from} – ${v.to}`;
    return v.from ? `from ${v.from}` : `until ${v.to}`;
  }

  // Only a plain day goes into the inputs; a relative one is a preset's.
  const plain = (d) => (/^\d{4}-\d{2}-\d{2}$/.test(d || "") ? d : "");

  function show() {
    from = plain(value?.from);
    to = plain(value?.to);
    at = under(chip, 240);
    open = true;
  }

  function pick(v) {
    open = false;
    if (!same(v, value)) onchange(v);
  }

  $effect(() => {
    if (!open) return;
    panel?.querySelector("button")?.focus();
    return dismiss(() => [panel, chip], () => (open = false));
  });
</script>

<span class="chip-group" bind:this={chip}>
  <button class="chip" class:on={!!value} aria-expanded={open} aria-haspopup="dialog"
          onclick={() => (open ? (open = false) : show())}>
    <span>{label}</span>
    {#if !value}<Icon body={NAV.chevron} size={10} />{/if}
  </button>
  {#if value}
    <button class="chip on" aria-label="Clear {title}" onclick={() => onchange(null)}>
      <Icon body={NAV.close} size={10} />
    </button>
  {/if}
</span>

{#if open}
  <div class="pop filter-pop day-pop" role="dialog" aria-label={title} bind:this={panel}
       style="left:{at.left}px;top:{at.top}px">
    {#each presets as [name, v] (name)}
      <button class="tri" class:plus={same(v, value)} onclick={() => pick(v)}>
        <span class="tri-label">{name}</span>
      </button>
    {/each}
    <span class="menu-rule"></span>
    <div class="day-range">
      <label><span class="field-label">From</span><input class="input" type="date" bind:value={from} /></label>
      <label><span class="field-label">To</span><input class="input" type="date" bind:value={to} /></label>
      <button class="btn small" disabled={!from && !to}
              onclick={() => pick(from && to && from === to ? { from, to } : { ...(from && { from }), ...(to && { to }) })}>
        Apply range
      </button>
    </div>
  </div>
{/if}
