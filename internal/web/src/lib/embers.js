// A finished card feeds the fire (DESIGN.md §8).
//
// The card kindles: heat washes up through it and its border runs amber, then
// olive. Embers leave it and rise, drift on a turbulent draft, flicker, and
// cool as they go — white-hot, amber, clay, ash. One brighter ember carries
// the work to the campfire in the sidebar, which roars up when it arrives.
//
// Everything that moves is drawn on one canvas over the page. It exists only
// while an ember is alive, so an idle board pays nothing for it.

const still = matchMedia("(prefers-reduced-motion: reduce)");

// The heat ramp, hottest first, in the palette's own colours: bright, the two
// ambers, clay, then the faint of ash. An ember is drawn between two of them.
const RAMP = ["--bright", "--accent-hover", "--accent", "--alert", "--faint"];
let ramp = null;
const rgb = (hex) => [1, 3, 5].map((i) => parseInt(hex.trim().slice(i, i + 2), 16));
function heat(t) { // t: 1 is white-hot, 0 is ash
  ramp ||= RAMP.map((v) => rgb(getComputedStyle(document.documentElement).getPropertyValue(v)));
  const x = (1 - Math.max(0, Math.min(1, t))) * (ramp.length - 1);
  const i = Math.min(ramp.length - 2, Math.floor(x));
  const f = x - i;
  return ramp[i].map((c, k) => Math.round(c + (ramp[i + 1][k] - c) * f));
}

let canvas = null, ctx = null, frame = 0, last = 0;
const embers = [];   // loose embers: { x, y, vx, vy, age, life, size, seed }
const carriers = []; // the ones carrying work to the fire: { from, to, bend, age, life, trail, done }

function surface() {
  if (canvas) return;
  canvas = document.createElement("canvas");
  canvas.className = "embers";
  document.body.append(canvas);
  ctx = canvas.getContext("2d");
  const fit = () => {
    const dpr = devicePixelRatio || 1;
    canvas.width = innerWidth * dpr; canvas.height = innerHeight * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.lineCap = "square"; // a resize resets it; round caps are off-language
  };
  fit();
  addEventListener("resize", fit);
  canvas.fit = fit;
}

function burst(x, y, w, h, n, power = 1) {
  for (let i = 0; i < n; i++) {
    embers.push({
      x: x + Math.random() * w, y: y + Math.random() * h,
      vx: (Math.random() - 0.5) * 90 * power,
      vy: -(90 + Math.random() * 190) * power,
      age: -Math.random() * 0.12, life: 0.6 + Math.random() * 0.8,
      size: Math.random() < 0.25 ? 1 : 1.5 + Math.random() * 2.2,
      seed: Math.random() * 100,
    });
  }
}

// A square, with a tighter square of its own colour behind it for the glow.
// Square like everything else: the ember is a pixel of the fire, not a bokeh
// dot.
function draw(x, y, size, t, alpha) {
  const [r, g, b] = heat(t);
  const glow = size * 2.4;
  ctx.fillStyle = `rgba(${r},${g},${b},${alpha * 0.25})`;
  ctx.fillRect(x - glow / 2, y - glow / 2, glow, glow);
  ctx.fillStyle = `rgba(${r},${g},${b},${alpha})`;
  ctx.fillRect(x - size / 2, y - size / 2, size, size);
}

function step(now) {
  const dt = Math.min(0.05, (now - last) / 1000 || 0.016);
  last = now;
  ctx.clearRect(0, 0, innerWidth, innerHeight);
  ctx.globalCompositeOperation = "lighter";

  for (let i = embers.length - 1; i >= 0; i--) {
    const e = embers[i];
    e.age += dt;
    if (e.age < 0) continue;
    if (e.age > e.life) { embers.splice(i, 1); continue; }
    // Buoyant, slowed by the air, pushed about by a draft that changes its
    // mind: each ember has its own seed, so no two drift together.
    e.vy -= 40 * dt;
    e.vx += Math.sin(e.age * 7 + e.seed) * 220 * dt;
    const drag = Math.pow(0.08, dt);
    e.vx *= drag; e.vy *= Math.pow(0.35, dt);
    e.x += e.vx * dt; e.y += e.vy * dt;
    // Born amber-gold, not white: a loose ember is already cooling. It goes
    // through amber and clay to ash, and shrinks as it goes.
    const t = 1 - e.age / e.life;
    const flicker = 0.6 + 0.4 * Math.sin(e.age * 38 + e.seed * 3);
    draw(e.x, e.y, e.size * (0.45 + 0.55 * t), 0.05 + 0.72 * t, Math.min(1, t * 2) * flicker);
  }

  for (let i = carriers.length - 1; i >= 0; i--) {
    const c = carriers[i];
    c.age += dt;
    if (c.age < 0) continue;
    const u = Math.min(1, c.age / c.life);
    // Slow to leave, quick to arrive: it is being drawn in by the fire.
    const k = u * u * (3 - 2 * u);
    // It rises off the card first, then sweeps over to the fire, and never
    // leaves the screen on the way: the mark sits at its top edge.
    const mx = c.to.x + (c.from.x - c.to.x) * 0.35;
    const my = Math.max(24, c.from.y - c.bend);
    const x = (1 - k) ** 2 * c.from.x + 2 * (1 - k) * k * mx + k ** 2 * c.to.x;
    const y = (1 - k) ** 2 * c.from.y + 2 * (1 - k) * k * my + k ** 2 * c.to.y;
    if (!c.done) c.trail.unshift([x, y]);
    // Kept to a length, not a count of frames: at full speed ten frames were a
    // third of the screen, a laser rather than an ember.
    for (let j = 1, run = 0; j < c.trail.length; j++) {
      run += Math.hypot(c.trail[j][0] - c.trail[j - 1][0], c.trail[j][1] - c.trail[j - 1][1]);
      if (run > 64) { c.trail.length = j + 1; break; }
    }
    // A streak, not a string of beads: a line that thins and cools behind a
    // white-hot head.
    for (let j = 1; j < c.trail.length; j++) {
      const f = 1 - j / c.trail.length;
      const [r, g, b] = heat(0.3 + 0.65 * f);
      ctx.strokeStyle = `rgba(${r},${g},${b},${0.85 * f})`;
      ctx.lineWidth = 0.6 + 2.8 * f;
      ctx.beginPath();
      ctx.moveTo(...c.trail[j - 1]);
      ctx.lineTo(...c.trail[j]);
      ctx.stroke();
    }
    if (!c.done) draw(x, y, 3.4, 1, 1);
    if (u >= 1 && !c.done) {
      c.done = true;
      roar();
      burst(c.to.x - 5, c.to.y - 4, 10, 6, 10, 0.55);
    }
    if (c.done) c.trail.pop();
    if (c.done && !c.trail.length) carriers.splice(i, 1);
  }

  if (embers.length || carriers.length) frame = requestAnimationFrame(step);
  else { removeEventListener("resize", canvas.fit); canvas.remove(); canvas = ctx = null; frame = 0; }
}

// The mark in the sidebar flares up, once per ember that reaches it.
function roar() {
  const mark = document.querySelector(".sidebar .mark");
  if (!mark) return;
  mark.classList.remove("roar");
  void mark.getBoundingClientRect(); // restart the animation
  mark.classList.add("roar");
  mark.addEventListener("animationend", () => mark.classList.remove("roar"), { once: true });
}

// A one-shot CSS animation on an element, restarted if it is already running.
function play(el, cls) {
  if (!el) return;
  el.classList.remove(cls);
  void el.offsetWidth;
  el.classList.add(cls);
  el.addEventListener("animationend", () => el.classList.remove(cls), { once: true });
}

/** A card has just been finished: kindle it, and carry the work to the fire. */
export function kindle(card, delay = 0) {
  if (still.matches || !card) return;
  play(card, "kindled");
  play(card.closest(".column")?.querySelector(".column-count"), "bump");

  const box = card.getBoundingClientRect();
  surface();
  burst(box.left + 8, box.top + box.height * 0.25, box.width - 16, box.height * 0.6, 26);

  const mark = document.querySelector(".sidebar .mark")?.getBoundingClientRect();
  if (mark) {
    carriers.push({
      from: { x: box.left + box.width / 2, y: box.top + 6 },
      to: { x: mark.left + mark.width / 2, y: mark.top + mark.height * 0.55 },
      bend: 110 + Math.random() * 50, age: -0.18 - delay, life: 0.62, trail: [], done: false,
    });
  }
  if (!frame) { last = performance.now(); frame = requestAnimationFrame(step); }
}
