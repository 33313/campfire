<script>
  import { untrack } from "svelte";

  import Dialog from "../components/Dialog.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import { api } from "../lib/api.js";
  import { BUCKET_ICONS, PRIORITY } from "../lib/icons.js";

  /** Create a ticket or a bucket. A bucket carries an icon; a ticket does not. */
  let { team, states, kind = "ticket", close } = $props();

  let chosen = $state(untrack(() => kind));
  let icon = $state("bookmark");
  let title = $state("");
  let description = $state("");
  let priority = $state(3);
  let bucketId = $state("");
  let plannedEnd = $state("");
  let accepted = $state(false);
  let buckets = $state([]);
  let acceptedState = $derived(states.find((s) => s.role === "accepted"));

  api.listItems({ kinds: ["bucket"] }, "", 200)
    .then((page) => (buckets = page.items || []))
    .catch(() => {});

  async function create() {
    const body = {
      kind: chosen, teamId: team.id, title, description,
      priority: +priority, plannedEnd: plannedEnd || "", accepted,
    };
    if (chosen === "bucket") body.icon = icon;
    else if (bucketId) body.bucketId = +bucketId;
    try { close(await api.createItem(body)); } catch (err) { close({ error: err }); }
  }
</script>

<Dialog title="New item in {team.name}" {close}>
  {#snippet body()}
    <div style="display:flex;gap:6px">
      {#each ["ticket", "bucket"] as k}
        <button class="chip" class:on={chosen === k} onclick={() => (chosen = k)}>
          {k === "ticket" ? "Ticket" : "Bucket"}
        </button>
      {/each}
    </div>
    <label class="field">
      <span class="field-label">Title</span>
      <input class="input" placeholder="What needs doing?" bind:value={title} />
    </label>
    <label class="field">
      <span class="field-label">Description</span>
      <textarea class="input" rows="4" placeholder="Markdown…" bind:value={description}></textarea>
    </label>
    <div style="display:flex;gap:12px">
      <label class="field">
        <span class="field-label">Priority</span>
        <select class="input" bind:value={priority}>
          {#each [1, 2, 3, 4] as p}<option value={p}>{PRIORITY[p].name}</option>{/each}
        </select>
      </label>
      <label class="field">
        <span class="field-label">Planned end</span>
        <input class="input" type="date" bind:value={plannedEnd} />
      </label>
    </div>
    {#if chosen === "bucket"}
      <div class="field">
        <span class="field-label">Icon</span>
        <div style="display:flex;gap:6px;flex-wrap:wrap">
          {#each Object.keys(BUCKET_ICONS) as name}
            <button class="chip" class:on={icon === name} title={name} onclick={() => (icon = name)}>
              <BucketIcon {name} size={16} rendering={team.iconRendering} />
            </button>
          {/each}
        </div>
        <span class="explain">One of the eight. It stands in for the bucket wherever it is named.</span>
      </div>
    {:else}
      <label class="field">
        <span class="field-label">Bucket</span>
        <select class="input" bind:value={bucketId}>
          <option value="">None</option>
          {#each buckets as b}<option value={b.id}>{b.identifier}  {b.title}</option>{/each}
        </select>
      </label>
    {/if}
    <label style="display:flex;gap:8px;align-items:center">
      <input class="check" type="checkbox" bind:checked={accepted} />
      <span>Mark as accepted</span>
      {#if acceptedState}<span class="eyebrow">→ {acceptedState.name}</span>{/if}
    </label>
  {/snippet}
  {#snippet footer()}
    <span class="consequence">{chosen === "bucket" ? "A new bucket" : "A new ticket"}</span>
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" onclick={create}>Create</button>
    </div>
  {/snippet}
</Dialog>
