// A box whose text wraps is as wide as the room it was given, not as its
// longest line: CSS sizes the box before the lines break. This measures the
// lines once they have broken and takes the slack back, so a two-line bucket
// chip hugs its name the way a one-line one does.
//
// `use:hug={text}` on the box; the text is the element's `.name` child. The
// argument is only there so a new name is measured again.
export function hug(node) {
  const fit = () => {
    node.style.width = "";
    const text = node.querySelector(".name");
    const lines = document.createRange();
    lines.selectNodeContents(text);
    const rects = [...lines.getClientRects()];
    if (rects.length < 2) return; // one line already hugs
    const slack = text.getBoundingClientRect().width - Math.max(...rects.map((r) => r.width));
    node.style.width = `${Math.ceil(node.getBoundingClientRect().width - slack)}px`;
  };

  // The card's width is what decides where the name breaks, so a change in it
  // is the one reason to measure again. Its height changes when we do; that is
  // not a reason.
  let width = 0;
  const watch = new ResizeObserver(([entry]) => {
    if (entry.contentRect.width !== width) { width = entry.contentRect.width; fit(); }
  });
  watch.observe(node.parentElement);
  document.fonts.ready.then(fit);

  return { update: fit, destroy: () => watch.disconnect() };
}
