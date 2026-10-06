<script>
  // The pull requests whose titles name an item (DEV-13), drawn like its
  // relations: where, which, its title, and its state, coloured as GitHub
  // colours it.
  let { pullRequests = [], compact = false } = $props();

  const NAME = { open: "Open", draft: "Draft", merged: "Merged", closed: "Closed" };
</script>

{#each pullRequests as p (p.id)}
  <div class="relation-row" class:compact>
    <!-- The preview has room for the repository's name, not its owner's. -->
    <a class="relation-kind pr-ref" href={p.url} target="_blank" rel="noopener"
       title="{p.repo}#{p.number} by {p.author}">{compact ? p.repo.split("/").pop() : p.repo}#{p.number}</a>
    <span class="relation-title">{p.title}</span>
    <span class="state-badge pr-{p.state}">
      {#if p.state !== "draft"}<span class="pip"></span>{/if}
      <span>{NAME[p.state] || p.state}</span>
    </span>
  </div>
{/each}
