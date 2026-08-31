<script>
  let { role = null } = $props();
</script>

{#if role}
  <span class="role-badge {role === 'final' ? 'final' : role === 'rejected' ? 'rejected' : ''}">{role}</span>
{:else}
  <span class="role-badge none">intermediate</span>
{/if}
