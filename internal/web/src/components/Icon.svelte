<script>
  // Every stroke here is square and mitred; round caps are off-language.
  let { body, size = 15, stroke = 1.4, color = null, label = null, viewBox = "0 0 16 16" } = $props();
</script>

<svg width={size} height={size} {viewBox} fill="none" stroke="currentColor"
     stroke-width={stroke} stroke-linecap="square" stroke-linejoin="miter"
     style:flex="0 0 {size}px" style:color aria-label={label}
     role={label ? "img" : "presentation"}>
  {#if label}<title>{label}</title>{/if}
  {@html body}
</svg>
