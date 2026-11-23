import { api } from "./api.js";

const tokens = "page sunk surface bar border border-strong rule-faint text bright muted muted-soft dim faint ghost accent accent-hover alert ok pr-merged on-accent".split(" ");

function colors(s) {
  return Object.fromEntries(s.split(" ").map((color, i) => [tokens[i], `#${color}`]));
}

export const presets = [
  { key: "campfire", name: "Campfire", colors: {} },
  { key: "dracula", name: "Dracula", colors: colors("282a36 21222c 303241 262833 44475a 6272a4 393b4d f8f8f2 ffffff b1b8d5 c4c9e0 9aa4cb 9099ba 7581ad bd93f9 d8bfff ff5555 50fa7b ff79c6 21222c") },
  { key: "tokyo-night", name: "Tokyo Night", colors: colors("1a1b26 16161e 24283b 1f2335 363b54 565f89 2c3046 c0caf5 e1e8ff 9aa5ce a9b5df 838fbc 7984ad 626d97 7aa2f7 a3baff f7768e 9ece6a bb9af7 16161e") },
];

export const defaultColors = colors("131110 100e0d 1a1715 16130f 2c2621 3a332c 211d19 d6cfc4 ece5d8 8f8578 a89c8d 6b6157 5f574d 4a423a f0a84a f7bf6e e5604d b2bd52 a58cc4 131110");

export const themeState = $state({ instance: null, personal: null, preview: null });

export function choices(scope) {
  const scoped = (s) => (themeState[s]?.themes || []).map((t) => ({ ...t, key: `${s}:${t.key}` }));
  return [...presets, ...scoped("instance"), ...(scope === "personal" ? scoped("personal") : [])];
}

export async function loadThemes() {
  const result = await api.getThemes();
  themeState.instance = result.instance;
  themeState.personal = result.personal;
}

export function currentTheme() {
  const all = choices("personal");
  return themeState.preview || all.find((t) => t.key === themeState.personal?.selected)
    || all.find((t) => t.key === themeState.instance?.selected) || presets[0];
}

export function applyTheme(theme) {
  const style = document.documentElement.style;
  const values = theme.colors || {};
  for (const token of tokens) {
    if (values[token]) style.setProperty(`--${token}`, values[token]);
    else style.removeProperty(`--${token}`);
  }
  for (const [key, source, pct] of [["accent-tint", "accent", 18], ["accent-tint-weak", "accent", 8], ["alert-tint", "alert", 14]]) {
    if (values[source]) style.setProperty(`--${key}`, `color-mix(in srgb, var(--${source}) ${pct}%, transparent)`);
    else style.removeProperty(`--${key}`);
  }
}

export { tokens as themeTokens };
