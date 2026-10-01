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
  // `value` is the query as applied. `onapply(query)` applies another and
  // resolves to what became of it: { ok }, { error } when the server refused
  // it, or { superseded } when something newer was asked first. `draft` is
  // what the editor holds now, for a dialog that saves it.
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
  let notice = $state("");    // why the chips cannot show a query
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

  async function toChips() {
    let tree;
    try { tree = parse(draft); } catch (err) { notice = `Fix the query first: ${err.message}.`; return; }
    if (!toBasic(tree)) { notice = "This query uses CSQL the chips can't show."; return; }
    if (draft !== value && !(await apply(draft)).ok) return;
    notice = "";
    mode = "chips";
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
  <QueryBox bind:value={draft} {error} onenter={apply} />
  <CsqlHelp />
{/if}
{#if notice && mode === "raw"}<span class="filter-error" role="alert">{notice}</span>{/if}
<button class="btn quiet small mode" aria-pressed={mode === "raw"}
        title={mode === "raw" ? "Back to the filter chips" : "Write the filter in CSQL"}
        onclick={() => (mode === "raw" ? toChips() : ((draft = value), (notice = ""), (mode = "raw")))}>
  &lt;/&gt;
</button>

{#if moreOpen}
  <div class="pop filter-pop" role="menu" bind:this={morePanel} style="left:{moreAt.left}px;top:{moreAt.top}px">
    {#each more as f (f)}
      <button class="tri" role="menuitem" onclick={() => { added = [...added, f]; moreOpen = false; }}>
        <span class="tri-label">{TITLES[f]}</span>
      </button>
    {/each}
  </div>
{/if}
