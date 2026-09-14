<script>
  import { moveMany, editMany } from "../lib/bulk.js";
  import { assigneeOptions, priorityOptions, bucketOptions } from "../lib/options.js";

  // Move to…, Reject…, then Assignee, Priority, Bucket, then Clear
  // (DESIGN.md §7). All five are v1, and the card menu offers the same five
  // at the pointer.
  let { ids, states, onapplied, onclear } = $props();

  const move = async (only) => (await moveMany(ids, states, only)) && onapplied();
  const edit = async (field, options) => (await editMany(ids, field, options)) && onapplied();
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
