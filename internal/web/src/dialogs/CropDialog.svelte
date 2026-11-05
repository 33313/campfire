<script>
  import Dialog from "../components/Dialog.svelte";

  // Which square of a picture becomes the avatar (DEV-110). Drag to move it,
  // the slider or the wheel to zoom. The picture always covers the square, so
  // there is no empty edge to crop by mistake. Closes with the square in the
  // picture's own pixels: { x, y, side }.
  let { img, close } = $props();

  const VIEW = 320;           // the square, in CSS pixels
  const MAX_ZOOM = 4;
  const base = VIEW / Math.min(img.naturalWidth, img.naturalHeight); // cover at zoom 1

  let zoom = $state(1);
  let x = $state(0);          // the picture's top-left inside the square
  let y = $state(0);
  let scale = $derived(base * zoom);

  // Keeps the picture over the whole square.
  function clamp() {
    x = Math.min(0, Math.max(VIEW - img.naturalWidth * scale, x));
    y = Math.min(0, Math.max(VIEW - img.naturalHeight * scale, y));
  }
  x = (VIEW - img.naturalWidth * base) / 2;
  y = (VIEW - img.naturalHeight * base) / 2;

  // Zooms about the square's centre, so what is in the middle stays there.
  function setZoom(z) {
    const cx = (VIEW / 2 - x) / scale, cy = (VIEW / 2 - y) / scale;
    zoom = Math.min(MAX_ZOOM, Math.max(1, z));
    x = VIEW / 2 - cx * scale;
    y = VIEW / 2 - cy * scale;
    clamp();
  }

  let drag = null;
  function down(e) {
    e.currentTarget.setPointerCapture(e.pointerId);
    drag = { px: e.clientX, py: e.clientY, x, y };
  }
  function move(e) {
    if (!drag) return;
    x = drag.x + e.clientX - drag.px;
    y = drag.y + e.clientY - drag.py;
    clamp();
  }

  // Not onwheel: Svelte adds that listener as passive, and the wheel would
  // scroll the page behind the dialog as well as zoom.
  let view;
  $effect(() => {
    const zoomBy = (e) => { e.preventDefault(); setZoom(zoom * (e.deltaY < 0 ? 1.1 : 1 / 1.1)); };
    view.addEventListener("wheel", zoomBy, { passive: false });
    return () => view.removeEventListener("wheel", zoomBy);
  });

  const done = () => close({ x: -x / scale, y: -y / scale, side: VIEW / scale });
</script>

<Dialog title="Crop the picture" {close}>
  {#snippet body()}
    <p class="explain">Drag the picture to place it; zoom with the slider or the wheel.</p>
    <div class="crop" style:width="{VIEW}px" style:height="{VIEW}px" role="presentation" bind:this={view}
         onpointerdown={down} onpointermove={move}
         onpointerup={() => (drag = null)} onpointercancel={() => (drag = null)}>
      <img src={img.src} alt="" draggable="false"
           style:width="{img.naturalWidth * scale}px" style:transform="translate({x}px, {y}px)" />
    </div>
    <label class="field" style="max-width:{VIEW}px">
      <span class="field-label">Zoom</span>
      <input type="range" min="1" max={MAX_ZOOM} step="0.01" value={zoom}
             oninput={(e) => setZoom(+e.currentTarget.value)} />
    </label>
  {/snippet}
  {#snippet footer()}
    <div class="actions">
      <button class="btn" onclick={() => close(undefined)}>Cancel</button>
      <button class="btn primary" onclick={done}>Use this</button>
    </div>
  {/snippet}
</Dialog>
