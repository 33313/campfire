<script>
  // The pull requests whose titles name an item (DEV-13), drawn like its
  // relations: where, which, its title, and its state. Merged reads as done
  // and closed as rejected, in the badges the work states already use.
  import StateBadge from "./StateBadge.svelte";

  let { pullRequests = [], compact = false } = $props();

  const ROLE = { merged: "final", closed: "rejected" };
  const NAME = { open: "Open", draft: "Draft", merged: "Merged", closed: "Closed" };
</script>

{#each pullRequests as p (p.id)}
  <div class="relation-row" class:compact>
    <!-- The preview has room for the repository's name, not its owner's. -->
    <a class="relation-kind pr-ref" href={p.url} target="_blank" rel="noopener"
       title="{p.repo}#{p.number} by {p.author}">{compact ? p.repo.split("/").pop() : p.repo}#{p.number}</a>
    <span class="relation-title">{p.title}</span>
    <StateBadge name={NAME[p.state] || p.state} role={ROLE[p.state]} />
  </div>
{/each}
