<script>
  // Campfire's mark: a flame over two crossed logs. It is lit while the live
  // stream is up and burns down to embers while it is not, so the one piece of
  // branding on screen is also the one status light nobody has to look for.
  //
  // `burning` keeps it flickering: the loader and the sign-in screen, where
  // there is nothing else to look at. `crackle` flares it once — the sidebar
  // keys it on the stream, so it flares when anything changes.
  let { size = 16, lit = true, burning = false, crackle = false } = $props();
</script>

<svg class="mark" class:lit class:burning class:crackle width={size} height={size}
     viewBox="0 0 16 16" aria-hidden="true">
  <path class="flame" d="M8 1.2 12.6 9.2 10.8 12H5.2L3.4 9.2Z" />
  <path class="core" d="M8 5.6 10.4 9.6 9.4 12H6.6L5.6 9.6Z" />
  <path class="logs" d="M2.8 12.6 13.2 14.8M2.8 14.8 13.2 12.6" />
</svg>
