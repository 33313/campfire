<script>
  // A titled block. `class` is for the caller that needs the block itself to
  // take part in its parent's layout — the profile's activity panel is the
  // one that scrolls while the rest of the page stays put.
  let { title, sub = null, class: klass = "", children } = $props();
</script>

<section class={klass} style="padding:18px 20px">
  <div style="display:flex;align-items:baseline;gap:10px;margin-bottom:10px">
    <h2 style="margin:0;font-size:17px;font-weight:600;color:var(--bright)">{title}</h2>
    {#if sub}<span class="explain">{sub}</span>{/if}
  </div>
  {@render children()}
</section>
