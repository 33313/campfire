<script>
  import StatePip from "./StatePip.svelte";

  /** A work state named in a bordered mono box, with a pip on terminal states. */
  let { name, role = null } = $props();
</script>

<span class="state-badge {role === 'final' ? 'final' : role === 'rejected' ? 'rejected' : ''}">
  {#if role === "final" || role === "rejected"}<StatePip {role} />{/if}
  <span>{name}</span>
</span>
