<script>
  let { title, sub = null, children } = $props();
</script>

<section style="padding:18px 20px">
  <div style="display:flex;align-items:baseline;gap:10px;margin-bottom:10px">
    <h2 style="margin:0;font-size:17px;font-weight:600;color:var(--bright)">{title}</h2>
    {#if sub}<span class="explain">{sub}</span>{/if}
  </div>
  {@render children()}
</section>
