// The caret, drawn by us where the browser will not blink it at our pace.
// Chromium hands its blink over through caret-animation (style.css). Firefox
// and Safari cannot: they blink at the desktop's rate, 600ms each way on GTK,
// which reads as sluggish (DEV-74). There the native caret is made
// transparent and a 1px bar is drawn in its place, on the same 500ms blink.

const FIELD = "textarea, input:is(:not([type]), [type=text], [type=search], [type=url], [type=tel], [type=password])";

// What decides where a character falls, copied onto the mirror that measures.
const COPIED = ["fontFamily", "fontSize", "fontWeight", "fontStyle", "fontStretch", "fontVariant",
  "fontFeatureSettings", "fontKerning", "lineHeight", "letterSpacing", "wordSpacing", "textTransform",
  "textIndent", "tabSize", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft"];

// How wide one character of a password field is drawn (DEV-99). The field
// shows a mask the desktop picks, U+25CF on GTK and U+2022 on a Mac among
// others, every one the same width. So it is measured off a hidden password
// input in the field's font rather than guessed: a hundred of them, since
// scrollWidth counts whole pixels. The probe is gone before anything paints,
// and empty, so no password manager finds it.
function maskWidth(cs) {
  const probe = document.createElement("input");
  probe.type = "password";
  for (const p of COPIED) probe.style[p] = cs[p];
  probe.style.cssText += "position:absolute;left:0;top:0;visibility:hidden;width:0;padding:0;border:0";
  probe.value = "x".repeat(100);
  document.body.append(probe);
  const w = probe.scrollWidth / 100;
  probe.value = "";
  probe.remove();
  return w;
}

export function drawCaret() {
  const bar = document.createElement("div");
  bar.className = "drawn-caret";
  bar.hidden = true;
  const mirror = document.createElement("div");
  mirror.setAttribute("aria-hidden", "true");
  mirror.style.cssText = "position:absolute;left:0;top:0;visibility:hidden;border:0;box-sizing:content-box;overflow:hidden";
  document.body.append(bar, mirror);

  let field = null, color = "", mask = 0, seen = "", at = null, frame = 0;

  // Where the caret sits, as an offset from the field's border box: a mirror
  // of the field's text with a mark at the caret. A word joiner has the
  // font's height and, unlike a zero-width space, offers no line break the
  // field would not take. Null when there is a selection and so no caret.
  // Borders come from the style: Firefox counts an input's padding into its
  // clientLeft and out of its clientWidth.
  const measure = () => {
    const { selectionStart: start, selectionEnd: end } = field;
    if (start == null || start !== end) return null;
    const cs = getComputedStyle(field);
    for (const p of COPIED) mirror.style[p] = cs[p];
    const area = field.tagName === "TEXTAREA";
    const px = (p) => parseFloat(cs[p]);
    mirror.style.whiteSpace = area ? "pre-wrap" : "pre";
    mirror.style.overflowWrap = area ? cs.overflowWrap : "normal";
    mirror.style.width = area ? `${field.clientWidth - px("paddingLeft") - px("paddingRight")}px` : "auto";
    const mark = document.createElement("span");
    mark.textContent = "\u2060";
    mirror.replaceChildren(field.value.slice(0, start), mark, field.value.slice(start));
    // Measured to the fraction: offsetLeft rounds to whole pixels. A password
    // field draws its mask, not its value.
    const m = mark.getBoundingClientRect(), o = mirror.getBoundingClientRect();
    const x = mask ? px("paddingLeft") + start * mask : m.left - o.left;
    const h = m.height;
    // An input centres its one line in its box; a textarea starts at the top.
    const inner = field.offsetHeight - px("borderTopWidth") - px("borderBottomWidth")
      - px("paddingTop") - px("paddingBottom");
    return {
      left: px("borderLeftWidth") + x - field.scrollLeft,
      top: px("borderTopWidth") + (area ? m.top - o.top - field.scrollTop : px("paddingTop") + (inner - h) / 2),
      h,
    };
  };

  // Every frame while a field has focus: re-measure when its text, caret,
  // scroll or width changed, starting the blink over lit, and follow the
  // field, so a dialog sliding in carries the caret with it. Hidden where
  // something covers the spot or a scroll has taken it out of view.
  const place = () => {
    if (!field) return;
    if (!field.isConnected) return blur();
    frame = requestAnimationFrame(place);
    const now = [field.value, field.selectionStart, field.selectionEnd,
      field.scrollLeft, field.scrollTop, field.offsetWidth].join("\0");
    if (now !== seen) {
      seen = now;
      at = measure();
      restart();
    }
    if (!at) { bar.hidden = true; return; }
    // On whole device pixels, one wide: a bar between two pixels is smeared
    // over both and reads bold, then thin again as typing moves it (DEV-99).
    const box = field.getBoundingClientRect(), dpr = devicePixelRatio;
    const snap = (v) => Math.round(v * dpr) / dpr;
    const x = snap(box.left + at.left), y = snap(box.top + at.top);
    const hit = document.elementFromPoint(x + 0.5, y + at.h / 2);
    bar.hidden = !field.contains(hit);
    bar.style.cssText = `transform:translate(${x}px,${y}px);width:${Math.max(1, Math.round(dpr)) / dpr}px;`
      + `height:${snap(at.h)}px;background:${color}`;
  };

  const restart = () => { for (const a of bar.getAnimations()) a.currentTime = 0; };

  const blur = () => {
    field?.classList.remove("caret-drawn");
    field = null;
    cancelAnimationFrame(frame);
    bar.hidden = true;
  };

  addEventListener("focusin", (e) => {
    if (!e.target.matches?.(FIELD) || e.target.readOnly) return;
    field = e.target;
    // The colour the field asks for, read before the native caret is hidden.
    const cs = getComputedStyle(field);
    color = cs.caretColor === "auto" ? cs.color : cs.caretColor;
    mask = field.type === "password" ? maskWidth(cs) : 0;
    field.classList.add("caret-drawn");
    seen = "";
    cancelAnimationFrame(frame);
    place();
  });
  addEventListener("focusout", blur);
  // A key or a click shows the caret at once, even one that does not move it.
  addEventListener("keydown", restart, true);
  addEventListener("pointerdown", restart, true);
}
