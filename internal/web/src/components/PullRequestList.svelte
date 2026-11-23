<script>
  // The pull requests whose titles name an item (DEV-13), drawn like its
  // relations: where, which, its title, and its state. Open is green and
  // merged purple, as on GitHub, in this palette's own shades.
  let { pullRequests = [], compact = false, onremove = null } = $props();

  const NAME = { open: "Open", draft: "Draft", merged: "Merged", closed: "Closed", unknown: "Unknown" };
</script>

{#each pullRequests as p (`${p.manual || false}:${p.id}`)}
  <div class="relation-row" class:compact>
    <!-- The preview has room for the repository's name, not its owner's. -->
    <a class="relation-kind pr-ref" href={p.url} target="_blank" rel="noopener"
       title={p.url}>{compact ? p.repo.split("/").pop() : p.repo}{p.number ? `#${p.number}` : ""}</a>
    <span class="relation-title">{p.title || (p.number ? "" : p.url)}</span>
    <span class="state-badge pr-{p.state}">
      {#if p.state !== "draft" && p.state !== "unknown"}<span class="pip"></span>{/if}
      <span>{NAME[p.state] || p.state}</span>
    </span>
    {#if p.manual && onremove}
      <button class="btn quiet small" aria-label="Remove pull request link" onclick={() => onremove(p)}>Remove</button>
    {/if}
  </div>
{/each}
