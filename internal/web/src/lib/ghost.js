// The card in the hand while a drag is on: a copy of it that follows the
// pointer from the spot it was picked up by, sways with the speed it is
// carried at, and settles onto where it lands.
//
// It lives inside the zoomed #app, so every client coordinate written into it
// is divided by the zoom first (DESIGN.md §1).

import { ms } from "./motion.js";

const SWAY = 6;   // degrees, at most
const LIFT = 1.03;

export function lift(card, x, y, count = 1) {
  const box = card.getBoundingClientRect();
  const zoom = card.currentCSSZoom || 1;
  const still = ms(1) === 0;
  const grip = { x: x - box.left, y: y - box.top };

  const el = card.cloneNode(true);
  el.classList.remove("selected", "focused", "dragging");
  el.classList.add("ghost");
  el.removeAttribute("href");
  el.style.width = `${box.width / zoom}px`;
  // It hangs from the finger: it turns about the point it is held by.
  el.style.transformOrigin = `${grip.x / zoom}px ${grip.y / zoom}px`;
  if (count > 1) el.insertAdjacentHTML("beforeend", `<span class="ghost-count">${count}</span>`);
  document.getElementById("app").append(el);

  let px = x, py = y, speed = 0, angle = 0, scale = 1, frame = 0;
  const place = () => {
    el.style.transform = `translate(${(px - grip.x) / zoom}px, ${(py - grip.y) / zoom}px) ` +
      `rotate(${angle}deg) scale(${scale})`;
  };
  // Carried right, the bottom trails left: the tilt follows the speed and
  // swings back to rest when the hand stops.
  const step = () => {
    if (!still) {
      angle += (Math.max(-SWAY, Math.min(SWAY, speed * 0.5)) - angle) * 0.2;
      scale += (LIFT - scale) * 0.3;
      speed *= 0.82;
    }
    place();
    frame = requestAnimationFrame(step);
  };
  step();

  return {
    move(x, y) { speed += x - px; px = x; py = y; },

    /** Settles onto `rect` (client pixels) and goes; fades out if there is none. */
    async land(rect) {
      cancelAnimationFrame(frame);
      const to = rect
        ? [{ transform: `translate(${rect.left / zoom}px, ${rect.top / zoom}px) rotate(0deg) scale(1)` }]
        : [{ opacity: 0 }];
      // Straight from the hand to its place, on the ease-out everything else
      // moves on: an overshoot carried it past the slot and back.
      await el.animate(to, { duration: ms(rect ? 180 : 120), fill: "forwards",
        easing: "cubic-bezier(0.33, 1, 0.68, 1)" }).finished.catch(() => {});
      el.remove();
    },
  };
}
