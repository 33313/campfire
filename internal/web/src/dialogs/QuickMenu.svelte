<script>
  import Dialog from "../components/Dialog.svelte";
  import Icon from "../components/Icon.svelte";
  import BucketIcon from "../components/BucketIcon.svelte";
  import { NAV } from "../lib/icons.js";
  import { api } from "../lib/api.js";
  import { ranked } from "../lib/matches.js";

  let { team, admin, username, close } = $props();
  let input = $state("");
  let active = $state(0);
  let hits = $state([]);
  let pending = $state(false);
  let results;
  let pages = $derived([
    ...(team ? [
      { title: "Board", path: `/t/${team.id}/board`, icon: NAV.board },
      { title: "Backlog", path: `/t/${team.id}/backlog`, icon: NAV.backlog },
      { title: "Team", path: `/t/${team.id}/profile`, icon: NAV.team },
      { title: "Create new ticket…", kind: "ticket", icon: NAV.plus },
      { title: "Create new bucket…", kind: "bucket", icon: NAV.plus },
    ] : []),
    { title: "Buckets", path: "/buckets", icon: NAV.buckets },
    { title: "Log", path: "/log", icon: NAV.log },
    { title: "Profile", path: `/users/${encodeURIComponent(username)}`, icon: NAV.team },
    { title: "Account", path: "/account", icon: NAV.settings },
    { title: "Documentation", path: "/docs", icon: NAV.docs },
    ...(admin ? [{ title: "Administration", path: "/admin", icon: NAV.admin }] : []),
  ]);
  let options = $derived([...ranked(pages, input), ...hits]);

  $effect(() => {
    const q = input.trim();
    active = 0;
    hits = [];
    pending = q.length >= 2;
    if (q.length < 2) return;
    let alive = true;
    const timer = setTimeout(async () => {
      const items = await api.suggestItems(q).catch(() => []);
      if (alive) {
        hits = items.map((it) => ({ ...it, path: `/${it.identifier}` }));
        pending = false;
      }
    }, 180);
    return () => { alive = false; clearTimeout(timer); };
  });

  function key(e) {
    const n = options.length;
    if (!n) return;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      active = (active + (e.key === "ArrowDown" ? 1 : n - 1)) % n;
      results.children[active]?.scrollIntoView({ block: "nearest" });
    } else if (e.key === "Enter") {
      e.preventDefault();
      close(options[active]);
    }
  }
</script>

<Dialog title="Quick menu" {close}>
  {#snippet body()}
    <input class="input" aria-label="Find a page, ticket or bucket" placeholder="Find a page, ticket or bucket"
           role="combobox" aria-expanded={options.length > 0} aria-controls="quick-options"
           aria-activedescendant={options[active] ? `quick-option-${active}` : undefined}
           autocomplete="off" spellcheck="false" bind:value={input} onkeydown={key} />
    <div id="quick-options" role="listbox" aria-label="Quick menu results" bind:this={results}>
      {#each options as option, i (option.path || option.kind)}
        <button id="quick-option-{i}" class="option-row" class:on={active === i}
                role="option" aria-selected={active === i} onclick={() => close(option)}>
          {#if option.identifier}
            {#if option.kind === "bucket"}<BucketIcon name={option.icon} color={option.color} size={15} />{/if}
            <span class="ident">{option.identifier}</span>
          {:else}<Icon body={option.icon} size={15} />{/if}
          <span>{option.title}</span>
        </button>
      {/each}
    </div>
    {#if options.length === 0 && !pending}<p class="explain">No matches.</p>{/if}
  {/snippet}
</Dialog>
