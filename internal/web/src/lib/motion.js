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
// square embers rising off it and going out, drawn on <body> at the card's
// client rect.
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
