<script>
  // Shared inline team picture; the sidebar leaves the fallback blank.
  let { team, size = 24, showCode = true } = $props();
  let dimensions = $derived(`width:${size}px;height:${size}px;flex-basis:${size}px`);
</script>

{#if team?.avatar}
  <img class="team-avatar" style={dimensions} src={team.avatar} alt="" />
{:else if showCode}
  <span class="code-badge">{team?.shortCode || ""}</span>
{:else}
  <span class="team-avatar" style={dimensions}></span>
{/if}
