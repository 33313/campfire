<script>
  import { untrack } from "svelte";
  import FilterChip from "./FilterChip.svelte";
  import DayChip from "./DayChip.svelte";
  import QueryBox from "./QueryBox.svelte";
  import CsqlHelp from "./CsqlHelp.svelte";
  import { FIELD, parse, toBasic, fromBasic } from "../lib/csql.js";
  import { staleAll } from "../lib/catalog.svelte.js";
  import { dismiss, under } from "../lib/dismiss.js";

  // A CSQL query, edited two ways (SPEC.md §9.2): chips, which build the
  // common query without typing, and raw text, for anything else. Both write
  // the one query; neither is a second filter.
  //
  // Two queries are kept apart. `value` is the one in use: it changes only
  // when the server accepts another. `draft` is the one in the box: it is
  // yours, and nothing you type in it blocks anything. When the two differ,
  // Discard puts the one in use back in the box.
  //
  // `onapply(query)` applies a query and resolves to what became of it:
  // { ok }, { error } when the server refused it, or { superseded } when
  // something newer was asked first. A dialog binds `draft` to save it.
  let { value = "", onapply, draft = $bindable(""), error: given = null } = $props();

  const SHOWN = ["assignee", "state", "label", "bucket", "priority"];
  const MORE = ["reporter", "team", "created", "started", "completed", "due"];
  const TITLES = { assignee: "Assignee", reporter: "Reporter", label: "Label", bucket: "Bucket",
    priority: "Priority", state: "State", team: "Team", created: "Created", started: "Started",
    completed: "Completed", due: "Due" };

  const chipsOf = (q) => { try { return toBasic(parse(q)); } catch { return null; } };

  let basic = $derived(chipsOf(value));
  let mode = $state(untrack(() => (chipsOf(value) ? "chips" : "raw")));
  let refused = $state(null); // the server's word on the last query tried, and that query
  let notice = $state("");    // why the box has not gone back to the chips
  let pending = false;        // back to the chips once the draft is discarded
  let added = $state([]);     // fields brought in from + More
  // A refusal stands while its query does: edit the text and it is gone.
  let error = $derived(refused && (mode === "chips" || refused.query === draft) ? refused : given);

  // The applied query is what the editor holds, whatever changed it: a chip,
  // Enter, or the caller (a saved view taking it over clears it).
  $effect(() => {
    const v = value;
    untrack(() => {
      draft = v;
      refused = null;
      if (mode === "chips" && !chipsOf(v)) mode = "raw";
    });
  });

  let unapplied = $derived(mode === "raw" && draft.trim() !== value.trim());

  let fields = $derived([...SHOWN, ...MORE.filter((f) => basic?.[f] || added.includes(f))]);
  let more = $derived(MORE.filter((f) => !fields.includes(f)));

  async function apply(q) {
    const outcome = await onapply(q);
    if (outcome.error) {
      // A name the server did not know may be one the chips offered from a
      // list that has gone stale: refetch them all when next needed.
      staleAll();
      refused = { message: outcome.error.message, span: outcome.error.span, query: q };
    } else if (outcome.ok) {
      refused = null;
    }
    return outcome;
  }

  function set(field, v) {
    const next = { ...basic };
    const empty = !v || (v.plus && !v.plus.length && !v.minus.length);
    if (empty) delete next[field];
    else next[field] = v;
    apply(fromBasic(next));
  }

  // Back to the chips, never blocked. A draft is applied on the way when it
  // can be; a draft that cannot is offered to Discard, and a query in use
  // that the chips cannot show is offered to Clear.
  async function toChips() {
    if (unapplied) {
      const showable = chipsOf(draft);
      const outcome = showable ? await apply(draft) : null;
      if (!outcome?.ok) {
        notice = "Not applied. Discard it to go back to the chips.";
        pending = true;
        return;
      }
    } else if (!chipsOf(value)) {
      notice = "The chips can't show this query.";
      return;
    }
    notice = "";
    mode = "chips";
  }

  function discard() {
    const back = pending;
    draft = value;
    refused = null;
    if (back) toChips();
  }

  async function clearToChips() {
    if ((await apply("")).ok) { notice = ""; mode = "chips"; }
  }

  // + More: the fields not on the bar yet.
  let moreButton = $state(null);
  let morePanel = $state(null);
  let moreOpen = $state(false);
  let moreAt = $state({ left: 0, top: 0 });
  $effect(() => {
    if (!moreOpen) return;
    return dismiss(() => [morePanel, moreButton], () => (moreOpen = false));
  });
</script>

<button class="btn quiet small mode" aria-pressed={mode === "raw"}
        title={mode === "raw" ? "Back to the filter chips" : "Write the filter in CSQL"}
        onclick={() => (mode === "raw" ? toChips() : ((draft = value), (mode = "raw")))}>
  &lt;/&gt;
</button>

{#if mode === "chips"}
  {#each fields as f (f)}
    {#if FIELD[f].day}
      <DayChip field={f} title={TITLES[f]} value={basic?.[f] || null} onchange={(v) => set(f, v)} />
    {:else}
      <FilterChip field={f} title={TITLES[f]} plus={basic?.[f]?.plus || []} minus={basic?.[f]?.minus || []}
                  onchange={(v) => set(f, v)} />
    {/if}
  {/each}
  {#if more.length}
    <button class="btn quiet small" bind:this={moreButton} aria-expanded={moreOpen}
            onclick={() => { moreAt = under(moreButton, 180); moreOpen = !moreOpen; }}>+ More</button>
  {/if}
  {#if value}
    <button class="btn quiet small" onclick={() => { added = []; apply(""); }}>Clear</button>
  {/if}
  {#if error}<span class="filter-error" role="alert">{error.message}</span>{/if}
{:else}
  <!-- Typing again means fixing the draft rather than leaving it. -->
  <QueryBox bind:value={draft} {error} onenter={apply} ontype={() => { notice = ""; pending = false; }} />
  <CsqlHelp />
  {#if unapplied}
    <button class="btn quiet small" title="Put back the filter in use" onclick={discard}>Discard</button>
  {/if}
  {#if notice}
    <span class="filter-error" role="alert">{notice}</span>
    {#if !unapplied && value}
      <button class="btn quiet small" onclick={clearToChips}>Clear filter</button>
    {/if}
  {/if}
{/if}

{#if moreOpen}
  <div class="pop filter-pop" role="menu" bind:this={morePanel} style="left:{moreAt.left}px;top:{moreAt.top}px">
    {#each more as f (f)}
      <button class="tri" role="menuitem" onclick={() => { added = [...added, f]; moreOpen = false; }}>
        <span class="tri-label">{TITLES[f]}</span>
      </button>
    {/each}
  </div>
{/if}
