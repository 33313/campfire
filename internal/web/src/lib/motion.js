// Motion: the durations of DESIGN.md §8, for the transitions Svelte and the
// Web Animations API run. Those are scripted, so the reduced-motion rule in
// style.css never reaches them; this is what does.

const still = matchMedia("(prefers-reduced-motion: reduce)");

/** A duration, or none at all for someone who asked for less motion. */
export const ms = (n) => (still.matches ? 0 : n);

export const QUICK = 140;
export const GLIDE = 220;

/** Fast out, soft landing: the one easing curve everything moves on. */
export const easeOut = (t) => 1 - (1 - t) ** 3;

// Sparks off a card that has just landed in the final state: a handful of
// square embers rising off it and going out. Drawn on <body>, outside the
// zoomed #app, so the card's client rect is already in the right pixels.
const SPARK_COLOURS = ["var(--accent)", "var(--accent-hover)"];

export function sparks(el) {
  if (still.matches || !el) return;
  const box = el.getBoundingClientRect();
  for (let i = 0; i < 9; i++) {
    const s = document.createElement("i");
    const size = 2 + Math.round(Math.random() * 2);
    const x = box.left + box.width * (0.1 + Math.random() * 0.8);
    s.className = "spark";
    s.style.cssText = `left:${x}px;top:${box.top + 4}px;width:${size}px;height:${size}px;` +
      `background:${SPARK_COLOURS[i % 2]}`;
    document.body.append(s);
    const rise = 26 + Math.random() * 34;
    const drift = (Math.random() - 0.5) * 28;
    const a = s.animate([
      { transform: "translate(0, 0)", opacity: 1 },
      { transform: `translate(${drift}px, -${rise}px) rotate(${drift * 4}deg)`, opacity: 0 },
    ], { duration: 520 + Math.random() * 380, delay: Math.random() * 90, easing: "ease-out",
         fill: "backwards" });
    a.onfinish = a.oncancel = () => s.remove();
  }
}

// Svelte's crossfade, measured the way #app is drawn. Svelte's own takes the
// distance between two client rects and writes it into the node's transform;
// inside the zoomed #app that transform is multiplied by the zoom, so on a
// zoomed panel every card started its glide a quarter further away than it
// was (DESIGN.md §1). Its flip already divides; this does the same.
// ponytail: a port of svelte/transition's crossfade; drop it once upstream
// accounts for zoom the way flip does.
export function crossfade({ fallback, ...defaults }) {
  const sending = new Map();
  const receiving = new Map();

  function glide(from, node, params) {
    const { delay = 0, duration, easing } = { ...defaults, ...params };
    const a = from.getBoundingClientRect();
    const b = node.getBoundingClientRect();
    const zoom = node.currentCSSZoom || 1;
    const dx = (a.left - b.left) / zoom;
    const dy = (a.top - b.top) / zoom;
    const dw = a.width / b.width;
    const dh = a.height / b.height;
    const style = getComputedStyle(node);
    const transform = style.transform === "none" ? "" : style.transform;
    const opacity = +style.opacity;
    return {
      delay, easing,
      duration: typeof duration === "function" ? duration(Math.hypot(dx, dy)) : duration,
      css: (t, u) => `opacity: ${t * opacity}; transform-origin: top left; ` +
        `transform: ${transform} translate(${u * dx}px, ${u * dy}px) scale(${t + u * dw}, ${t + u * dh});`,
    };
  }

  const transition = (items, counterparts, intro) => (node, params) => {
    items.set(params.key, node);
    return () => {
      if (counterparts.has(params.key)) {
        const other = counterparts.get(params.key);
        counterparts.delete(params.key);
        return glide(other, node, params);
      }
      // Claimed by neither list: it is arriving from nowhere or leaving for it.
      items.delete(params.key);
      return fallback && fallback(node, params, intro);
    };
  };
  return [transition(sending, receiving, false), transition(receiving, sending, true)];
}
