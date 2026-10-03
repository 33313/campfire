// Preferences of this browser: they decide what is drawn, never what is true.
// Storage may be blocked — a private window, a strict setting — and then
// nothing is remembered and nothing breaks.

export function pref(key, fallback = null) {
  try { return JSON.parse(localStorage.getItem(key)) ?? fallback; } catch { return fallback; }
}

export function setPref(key, value) {
  try { localStorage.setItem(key, JSON.stringify(value)); } catch { /* not remembered */ }
}
