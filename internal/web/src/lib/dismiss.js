// A floating panel closes when you look away from it or press Escape, like
// the card menu. Escape is caught on the way down, so a panel inside a dialog
// closes alone and the dialog stays.

/** For an $effect: `els()` are what count as inside (the panel and what opened it). */
export function dismiss(els, away, escape = away) {
  const down = (e) => { if (!els().some((el) => el?.contains(e.target))) away(); };
  const key = (e) => {
    if (e.key !== "Escape") return;
    e.stopPropagation();
    e.preventDefault();
    escape();
  };
  addEventListener("pointerdown", down, true);
  addEventListener("keydown", key, true);
  return () => {
    removeEventListener("pointerdown", down, true);
    removeEventListener("keydown", key, true);
  };
}

/** Where a panel hangs under what opened it, kept on screen. */
export function under(anchor, width = 260) {
  const r = anchor.getBoundingClientRect();
  return { left: Math.max(8, Math.min(r.left, innerWidth - width - 8)), top: r.bottom + 4 };
}
