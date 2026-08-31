<script>
  import { api } from "../lib/api.js";
  import { openModal, toast, failure } from "../lib/ui.svelte.js";
  import { assigneeOptions, priorityOptions, bucketOptions } from "../lib/options.js";
  import MoveDialog from "../dialogs/MoveDialog.svelte";
  import BulkEditDialog from "../dialogs/BulkEditDialog.svelte";

  // Move to…, Reject…, then Assignee, Priority, Bucket, then Clear
  // (DESIGN.md §7). All five are v1.
  let { ids, states, onapplied, onclear } = $props();

  async function apply(field, value, reason, note) {
    try {
      const res = await api.bulkEditItems({ itemIds: ids, field, value, reason, note });
      // An item the actor may not edit is refused on its own; the rest go
      // through, and the interface reports what was refused (SPEC.md §6.5).
      if (res.refused?.length) toast(`${res.refused.length} refused: ${res.refused[0].reason}`, "alert");
      await onapplied();
    } catch (err) { failure(err); }
  }

  async function move(only) {
    const res = await openModal(MoveDialog, {
      states: states.filter(only), count: ids.length,
    });
    if (res) apply("state", res.stateId, res.reason, res.note);
  }

  async function edit(field, options) {
    const res = await openModal(BulkEditDialog, { field, count: ids.length, options });
    if (res) apply(field.toLowerCase(), res.value);
  }
</script>

<div class="selection-bar">
  <span class="count">{ids.length} selected</span>
  <button class="btn primary small" onclick={() => move((s) => s.role !== "rejected")}>Move to…</button>
  <button class="btn small" onclick={() => move((s) => s.role === "rejected")}>Reject…</button>
  <button class="btn small" onclick={() => edit("Assignee", assigneeOptions())}>Assignee</button>
  <button class="btn small" onclick={() => edit("Priority", priorityOptions())}>Priority</button>
  <button class="btn small" onclick={async () => edit("Bucket", await bucketOptions())}>Bucket</button>
  <button class="btn quiet small" style="margin-left:auto" onclick={onclear}>Clear</button>
</div>
