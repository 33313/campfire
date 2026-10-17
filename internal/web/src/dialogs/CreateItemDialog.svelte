<script>
  import { untrack } from "svelte";

  import Dialog from "../components/Dialog.svelte";
  import IconPicker from "../components/IconPicker.svelte";
  import Icon from "../components/Icon.svelte";
  import LabelEditor from "../components/LabelEditor.svelte";
  import BucketPicker from "../components/BucketPicker.svelte";
  import RelationList from "../components/RelationList.svelte";
  import ItemSearch from "../components/ItemSearch.svelte";
  import { api } from "../lib/api.js";
  import { app } from "../lib/state.svelte.js";
  import { PRIORITY, NAV, RELATION_LABELS } from "../lib/icons.js";
  import { pictures } from "../lib/pictures.js";

  /** Create a ticket or a bucket. A bucket carries an icon; a ticket does not. */
  let { team, states, kind = "ticket", close } = $props();

  let chosen = $state(untrack(() => kind));
  let icon = $state("bookmark");
  let color = $state("");
  let title = $state("");
  let description = $state("");
  let priority = $state(3);
  let assigneeId = $state("");
  let bucketId = $state(0);
  let plannedEnd = $state("");
  let labels = $state([]);
  let labelDraft = $state("");
  let accepted = $state(false);
  let relations = $state([]); // the other ends, as found: { kind, id, identifier, title }
  let relationKind = $state("blocks");
  let relationDraft = $state("");
  let acceptedState = $derived(states.find((s) => s.role === "accepted"));

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

  // A relation is made with the item (DEV-94). The other end is picked from
  // what the search finds, so it always names an item that exists.
  function addRelation(other) {
    invalid = null;
    if (!relations.some((r) => r.id === other.id && r.kind === relationKind)) {
      relations.push({ kind: relationKind, id: other.id, identifier: other.identifier, title: other.title });
    }
  }

  async function create() {
    addLabel(); // whatever is still in the box was meant to be one
    invalid = null;
    // Text left in the search was meant to be a relation, but names nothing yet.
    if (relationDraft.trim()) {
      invalid = { field: "relations", message: "Pick the related item from the list, or clear the box." };
      return;
    }
    if (!title.trim()) {
      invalid = { field: "title", message: "An item needs a title." };
      return;
    }
    const body = {
      kind: chosen, teamId: team.id, title, description,
      priority: +priority, plannedEnd: plannedEnd || "", accepted,
      assigneeId: +assigneeId || 0, labels,
      relations: relations.map((r) => ({ kind: r.kind, toId: r.id })),
    };
    if (chosen === "bucket") { body.icon = icon; body.color = color; }
    else if (bucketId) body.bucketId = bucketId;
    try {
      close(await api.createItem(body));
    } catch (err) {
      // The API names the offending field; without one the message still has
      // to land somewhere, so it lands at the foot of the form.
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
      <textarea class="input" rows="4" placeholder="Markdown… paste a picture to add one"
                bind:value={description} use:pictures></textarea>
    </label>
    <!-- Priority, planned end and assignee on one row: there is room, and the
         form is a row shorter for it (DEV-101). -->
    <div style="display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px">
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
    </div>
    {#if chosen === "bucket"}
      <div class="field">
        <span class="field-label">Icon &amp; colour</span>
        <IconPicker {icon} {color} boxed onpick={(p) => { if ("icon" in p) icon = p.icon; else color = p.color; }} />
        <span class="explain">It stands in for the bucket wherever it is named, and its tickets wear the colour on the board.</span>
        {@render complaint("icon")}
        {@render complaint("color")}
      </div>
    {:else}
      <div class="field">
        <span class="field-label">Bucket</span>
        <BucketPicker value={bucketId} invalid={invalid?.field === "bucketId"}
                      onpick={(id) => { bucketId = id; clearIf("bucketId")(); }} />
        {@render complaint("bucketId")}
      </div>
    {/if}
    <div class="field">
      <span class="field-label">Labels</span>
      <LabelEditor {labels} bind:draft={labelDraft} onadd={addLabel}
                   onremove={(name) => labels.splice(labels.indexOf(name), 1)} />
    </div>
    <div class="field">
      <span class="field-label">Relations</span>
      {#if relations.length}
        <div>
          <RelationList {relations} compact links={false}
                        onremove={(r) => relations.splice(relations.indexOf(r), 1)} />
        </div>
      {/if}
      <div style="display:flex;gap:6px">
        <select class="input" aria-label="Relation kind" bind:value={relationKind}>
          {#each Object.entries(RELATION_LABELS) as [value, label]}<option {value}>{label}</option>{/each}
        </select>
        <ItemSearch bind:text={relationDraft} placeholder="A title, or {team.shortCode}-42"
                    invalid={invalid?.field === "relations"} onpick={addRelation} />
      </div>
      {@render complaint("relations")}
    </div>
    <!-- A refusal that names no field still has to land somewhere: here, at
         the foot of the form. -->
    {@render complaint("")}
  {/snippet}
  {#snippet footer()}
    <!-- Beside Create, where it decides where the item lands (DEV-101). -->
    <label style="display:flex;gap:8px;align-items:center">
      <input class="check" type="checkbox" bind:checked={accepted} />
      <span>Mark as accepted</span>
      {#if acceptedState}<span class="eyebrow">→ {acceptedState.name}</span>{/if}
    </label>
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" onclick={create}>Create</button>
    </div>
  {/snippet}
</Dialog>
