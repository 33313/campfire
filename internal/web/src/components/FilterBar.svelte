<script>
  import Icon from "./Icon.svelte";
  import LiveIndicator from "./LiveIndicator.svelte";
  import FilterEditor from "./FilterEditor.svelte";
  import { NAV } from "../lib/icons.js";
  import { saveNewView } from "../lib/views.js";

  // setAllDone is the board's alone: the backlog has no final state.
  let { teamId, applied, apply, showBuckets, setShowBuckets, allDone, setAllDone } = $props();

  // The bar is your own filter, unsaved. Views switched on in the sidebar are
  // and-ed with it and are not repeated here: a view is in one place.
</script>

<div class="filter-bar">
  <span class="eyebrow">Filter</span>

  <FilterEditor value={applied.csql} onapply={(csql) => apply({ ...applied, csql })} />

  {#if applied.csql}
    <!-- Saved, the filter moves from the bar into the view it became, which is on. -->
    <button class="btn quiet small" onclick={async () => {
      if (await saveNewView(teamId, applied.csql)) apply({ ...applied, csql: "" });
    }}>
      <Icon body={NAV.view} size={13} />
      <span>Save as view</span>
    </button>
  {/if}

  <div class="right">
    {#if setAllDone}
      <span style="font-size:12px;color:var(--muted)" title="Done for more than three days">Show old done</span>
      <button class="toggle" class:on={allDone} aria-label="Show old done"
              onclick={() => setAllDone(!allDone)}><i></i></button>
      <span class="rule-v"></span>
    {/if}
    <span style="font-size:12px;color:var(--muted)">Show buckets</span>
    <button class="toggle" class:on={showBuckets} aria-label="Show buckets"
            onclick={() => setShowBuckets(!showBuckets)}><i></i></button>
    <span class="rule-v"></span>
    <LiveIndicator />
  </div>
</div>
