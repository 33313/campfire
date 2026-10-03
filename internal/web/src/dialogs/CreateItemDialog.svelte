<script>
  import { untrack } from "svelte";

  import Dialog from "../components/Dialog.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import Icon from "../components/Icon.svelte";
  import LabelEditor from "../components/LabelEditor.svelte";
  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { buckets as allBuckets } from "../lib/catalog.svelte.js";
  import { BUCKET_ICONS, BUCKET_COLOURS, PRIORITY, NAV } from "../lib/icons.js";
  import { plural } from "../lib/format.js";

  /** Create a ticket or a bucket, or several: one per line of titles, each
      with the rest of the form. Closes with what was created, as a list. */
  let { team, states, kind = "ticket", close } = $props();

  let chosen = $state(untrack(() => kind));
  let icon = $state("bookmark");
  let color = $state("");
  let title = $state("");
  let several = $state(false);
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

  allBuckets().then((list) => (buckets = list));

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

  // Several at once is the one create, once per line, in order (DEV-44):
  // no batch to go half through on the server. A list pasted into the single
  // title is taken as several.
  let titles = $derived(several ? title.split("\n").map((t) => t.trim()).filter(Boolean) : [title]);

  // The box that takes over from the single title is typed into where that
  // left off.
  function focusEnd(el) {
    el.focus();
    el.selectionStart = el.value.length;
  }

  function pasted(e) {
    const text = e.clipboardData.getData("text");
    if (!text.trim().includes("\n")) return;
    e.preventDefault();
    title = (title ? title + "\n" : "") + text.trim();
    several = true;
  }

  async function create() {
    addLabel(); // whatever is still in the box was meant to be one
    invalid = null;
    if (!titles.some((t) => t.trim())) {
      invalid = { field: "title", message: "An item needs a title." };
      return;
    }
    const body = {
      kind: chosen, teamId: team.id, description,
      priority: +priority, plannedEnd: plannedEnd || "", accepted,
      assigneeId: +assigneeId || 0, labels,
    };
    if (chosen === "bucket") { body.icon = icon; body.color = color; }
    else if (bucketId) body.bucketId = +bucketId;
    const made = [];
    for (const t of titles) {
      try {
        made.push(await api.createItem({ ...body, title: t }));
      } catch (err) {
        // The API names the offending field; without one the message still
        // has to land somewhere, so it lands under the footer. What was
        // created stays created, and leaves the box: what is left in it is
        // what a second Create would make.
        if (made.length) title = titles.slice(made.length).join("\n");
        invalid = { field: err.field || "",
          message: made.length ? `${made.length} created. ${err.message}` : err.message };
        return;
      }
    }
    close(made);
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
    <div class="field">
      <span class="field-label">
        {several ? "Titles, one per line" : "Title"}
        <button type="button" class="pick-me" onclick={() => {
          several = !several;
          if (!several) title = titles[0] || "";
        }}>{several ? "(One)" : "(Several)"}</button>
      </span>
      {#if several}
        <textarea class="input" class:error={invalid?.field === "title"} rows="5" aria-label="Titles"
                  placeholder={"One item per line\nAnother"} bind:value={title} oninput={clearIf("title")}
                  use:focusEnd></textarea>
      {:else}
        <input class="input" class:error={invalid?.field === "title"} placeholder="What needs doing?"
               aria-label="Title" bind:value={title} oninput={clearIf("title")} onpaste={pasted} />
      {/if}
      {@render complaint("title")}
    </div>
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
    <!-- A div, not a label: a label would name the shortcut inside it too. -->
    <div class="field">
      <!-- A bucket's assignee is its owner: the person accountable for the
           milestone (SPEC.md §6.2). -->
      <span class="field-label">
        {chosen === "bucket" ? "Owner" : "Assignee"}
        <!-- The one assignee picked most often, without the drop-down (DEV-82). -->
        <button type="button" class="pick-me" onclick={() => (assigneeId = app.user.id)}>(Me!)</button>
      </span>
      <select class="input" aria-label={chosen === "bucket" ? "Owner" : "Assignee"} bind:value={assigneeId}>
        <option value="">Unassigned</option>
        {#each app.users as u}<option value={u.id}>{u.username}</option>{/each}
      </select>
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
      <span class="consequence">
        {titles.length > 1 ? plural(titles.length, `new ${chosen}`)
          : chosen === "bucket" ? "A new bucket" : "A new ticket"}
      </span>
    {/if}
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" onclick={create}>Create</button>
    </div>
  {/snippet}
</Dialog>
