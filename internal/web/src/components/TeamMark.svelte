<script>
  // A team's picture where it has one, its short code where it has none; as
  // wide as the badge, so the names line up either way.
  let { team } = $props();
</script>

{#if team.avatar}
  <img class="team-avatar" style="width:24px;flex-basis:24px" src={team.avatar} alt="" />
{:else}
  <span class="code-badge">{team.shortCode}</span>
{/if}
