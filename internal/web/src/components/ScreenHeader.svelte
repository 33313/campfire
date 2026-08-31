<script>
  let { title = null, sub = null, badge = null, crumbs = null, right = null } = $props();
</script>

<header class="screen-header">
  {#if crumbs}{@render crumbs()}{/if}
  {#if title}<h1>{title}</h1>{/if}
  {#if sub}<span class="sub">{sub}</span>{/if}
  {#if badge}<span class="header-badge">{badge}</span>{/if}
  {#if right}<div class="right">{@render right()}</div>{/if}
</header>
