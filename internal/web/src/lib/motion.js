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
