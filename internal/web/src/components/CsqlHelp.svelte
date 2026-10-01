<script>
  import { dismiss } from "../lib/dismiss.js";

  // The language on one screen, beside the box it is typed in (SPEC.md §9.2).
  // The full reference is DEV-80's in-app documentation. The panel keeps to
  // the window: never wider than it, and scrolling inside rather than past
  // its foot.
  let anchor = $state(null);
  let panel = $state(null);
  let open = $state(false);
  let at = $state({ left: 0, top: 0, width: 0, height: 0 });

  const WIDTH = 520;
  const MARGIN = 8;

  function place() {
    const r = anchor.getBoundingClientRect();
    const width = Math.min(WIDTH, innerWidth - 2 * MARGIN);
    const left = Math.max(MARGIN, Math.min(r.right - width, innerWidth - width - MARGIN));
    const top = r.bottom + 4;
    at = { left, top, width, height: innerHeight - top - MARGIN };
  }

  $effect(() => {
    if (!open) return;
    addEventListener("resize", place);
    const undo = dismiss(() => [panel, anchor], () => (open = false));
    return () => { removeEventListener("resize", place); undo(); };
  });

  // A list of keys, comma between each.
  const FIELDS = [
    [["assignee", "reporter"], ["a username", "me", "none"]],
    [["label"], ["a label's name", "none"]],
    [["bucket"], ["an identifier such as DEV-6", "none"]],
    [["priority"], ["1 (highest) to 4"]],
    [["state"], ["a work state's name"]],
    [["team"], ["a team's short code"]],
    [["created", "started", "completed", "due"], ["2026-09-01", "today", "-7d", "+2w", "none"]],
    [["text"], ["a phrase, with ~ or !~"]],
  ];
  const CODE = /^(me|none|today|-7d|\+2w|2026-09-01|1|4|~|!~)$/;
</script>

<button class="btn quiet small" bind:this={anchor} aria-label="CSQL help" title="How to write a query"
        aria-expanded={open} onclick={() => { place(); open = !open; }}>?</button>

{#if open}
  <div class="pop csql-help" role="dialog" aria-label="How to write a query" bind:this={panel}
       style="left:{at.left}px;top:{at.top}px;width:{at.width}px;max-height:{at.height}px">
    <p class="csql-lead">
      A condition is <code>field</code> <code>operator</code> <code>value</code>. Join conditions
      with <code>and</code>, <code>or</code>, <code>not</code>, and group them with <code>( )</code>.
    </p>

    <section>
      <h3 class="eyebrow">Fields</h3>
      <dl class="csql-fields">
        {#each FIELDS as [keys, vals]}
          <dt>{#each keys as k, i}{i ? ", " : ""}<code>{k}</code>{/each}</dt>
          <dd>{#each vals as v, i}{i ? ", " : ""}{#if CODE.test(v)}<code>{v}</code>{:else}{v}{/if}{/each}</dd>
        {/each}
      </dl>
    </section>

    <section>
      <h3 class="eyebrow">Operators</h3>
      <p>
        <code>=</code>, <code>!=</code>, <code>in (a, b)</code>, <code>not in (a, b)</code>.
        Priorities and days also take <code>&lt;</code>, <code>&lt;=</code>, <code>&gt;</code>,
        <code>&gt;=</code>.
      </p>
      <p>Quote a value with spaces: <code>"needs review"</code>.</p>
    </section>

    <section>
      <h3 class="eyebrow">Examples</h3>
      <ul class="csql-examples">
        <li><code>assignee in (marta, me) and state != Done</code></li>
        <li><code>label = api or label = none</code></li>
        <li><code>due &lt; today and completed = none</code></li>
      </ul>
    </section>
  </div>
{/if}
