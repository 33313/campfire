<script>
  import { untrack } from "svelte";

  import Dialog from "../components/Dialog.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import Icon from "../components/Icon.svelte";
  import LabelEditor from "../components/LabelEditor.svelte";
  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { BUCKET_ICONS, BUCKET_COLOURS, PRIORITY, NAV } from "../lib/icons.js";

  /** Create a ticket or a bucket. A bucket carries an icon; a ticket does not. */
  let { team, states, kind = "ticket", close } = $props();

  let chosen = $state(untrack(() => kind));
  let icon = $state("bookmark");
  let color = $state("");
  let title = $state("");
  let description = $state("");
  let priority = $state(3);
  let assigneeId = $state("");
  let bucketId = $state("");
  let plannedEnd = $state("");
  let labels = $state([]);
  let labelDraft = $state("");
  let accepted = $state(false);
  let buckets = $state([]);
  let acceptedState = $derived(states.find((s) => s.role === "accepted"));

  api.listItems({ kinds: ["bucket"] }, "", 200)
    .then((page) => (buckets = page.items || []))
    .catch(() => {});

  // Labels are created by typing them: there is no separate label registry to
  // pick from first (SPEC.md §8.1). The editor is the ticket page's, so the two
  // look and behave the same.
  function addLabel(name) {
    const value = (name ?? labelDraft).trim();
    labelDraft = "";
    if (value && !labels.includes(value)) labels.push(value);
  }

  // A refusal marks the field it names and keeps the form up. Closing on a
  // refusal would throw away everything typed to get here, which is the one
  // thing the person cannot recover.
  let invalid = $state(null); // { field, message }

  const clearIf = (field) => () => {
    if (invalid?.field === field) invalid = null;
  };

  async function create() {
    addLabel(); // whatever is still in the box was meant to be one
    invalid = null;
    if (!title.trim()) {
      invalid = { field: "title", message: "An item needs a title." };
      return;
    }
    const body = {
      kind: chosen, teamId: team.id, title, description,
      priority: +priority, plannedEnd: plannedEnd || "", accepted,
      assigneeId: +assigneeId || 0, labels,
    };
    if (chosen === "bucket") { body.icon = icon; body.color = color; }
    else if (bucketId) body.bucketId = +bucketId;
    try {
      close(await api.createItem(body));
    } catch (err) {
      // The API names the offending field; without one the message still has
      // to land somewhere, so it lands under the footer.
      invalid = { field: err.field || "", message: err.message };
    }
  }
</script>

<!-- The complaint sits under the field it is about, and clears as that field is
     corrected. -->
{#snippet complaint(field)}
  {#if invalid?.field === field}<span class="field-error">{invalid.message}</span>{/if}
{/snippet}

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
      <input class="input" class:error={invalid?.field === "title"} placeholder="What needs doing?"
             bind:value={title} oninput={clearIf("title")} />
      {@render complaint("title")}
    </label>
    <label class="field">
      <span class="field-label">Description</span>
      <textarea class="input" rows="4" placeholder="Markdown…" bind:value={description}></textarea>
    </label>
    <div style="display:flex;gap:12px">
      <label class="field">
        <span class="field-label">Priority</span>
        <select class="input" class:error={invalid?.field === "priority"} bind:value={priority}
                onchange={clearIf("priority")}>
          {#each [1, 2, 3, 4] as p}<option value={p}>{PRIORITY[p].name}</option>{/each}
        </select>
        {@render complaint("priority")}
      </label>
      <label class="field">
        <span class="field-label">Planned end</span>
        <input class="input" class:error={invalid?.field === "plannedEnd"} type="date"
               bind:value={plannedEnd} oninput={clearIf("plannedEnd")} />
        {@render complaint("plannedEnd")}
      </label>
    </div>
    <label class="field">
      <!-- A bucket's assignee is its owner: the person accountable for the
           milestone (SPEC.md §6.2). -->
      <span class="field-label">{chosen === "bucket" ? "Owner" : "Assignee"}</span>
      <select class="input" bind:value={assigneeId}>
        <option value="">Unassigned</option>
        {#each app.users as u}<option value={u.id}>{u.username}</option>{/each}
      </select>
    </label>
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
        {@render complaint("icon")}
      </div>
      <div class="field">
        <span class="field-label">Colour</span>
        <div style="display:flex;gap:6px;flex-wrap:wrap">
          <button class="swatch" class:on={!color} title="No colour" aria-label="No colour"
                  onclick={() => (color = "")}></button>
          {#each Object.entries(BUCKET_COLOURS) as [name, hex]}
            <button class="swatch" class:on={color === name} title={name} aria-label={name}
                    style="background:{hex};border-color:{hex}"
                    onclick={() => (color = name)}></button>
          {/each}
        </div>
        <span class="explain">Optional. Its tickets wear it on the board.</span>
        {@render complaint("color")}
      </div>
    {:else}
      <label class="field">
        <span class="field-label">Bucket</span>
        <select class="input" class:error={invalid?.field === "bucketId"} bind:value={bucketId}
                onchange={clearIf("bucketId")}>
          <option value="">None</option>
          {#each buckets as b}<option value={b.id}>{b.identifier}  {b.title}</option>{/each}
        </select>
        {@render complaint("bucketId")}
      </label>
    {/if}
    <div class="field">
      <span class="field-label">Labels</span>
      <LabelEditor {labels} bind:draft={labelDraft} onadd={addLabel}
                   onremove={(name) => labels.splice(labels.indexOf(name), 1)} />
    </div>
    <label style="display:flex;gap:8px;align-items:center">
      <input class="check" type="checkbox" bind:checked={accepted} />
      <span>Mark as accepted</span>
      {#if acceptedState}<span class="eyebrow">→ {acceptedState.name}</span>{/if}
    </label>
  {/snippet}
  {#snippet footer()}
    {#if invalid && !invalid.field}
      <span class="field-error">{invalid.message}</span>
    {:else}
      <span class="consequence">{chosen === "bucket" ? "A new bucket" : "A new ticket"}</span>
    {/if}
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" onclick={create}>Create</button>
    </div>
  {/snippet}
</Dialog>
