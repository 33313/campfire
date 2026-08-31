// The glyphs of DESIGN.md, as path data. Every stroke is square and mitred;
// round caps are off-language.

// Navigation glyphs, taken from the artboards.
export const NAV = {
  board: '<rect x="2" y="2.5" width="3.5" height="11"/><rect x="6.25" y="2.5" width="3.5" height="7.5"/><rect x="10.5" y="2.5" width="3.5" height="9.5"/>',
  backlog: '<path d="M5.5 4h8.5M5.5 8h8.5M5.5 12h8.5"/><path d="M2 4h.01M2 8h.01M2 12h.01" stroke-width="2"/>',
  // The pail is the sidebar's Buckets section icon and nothing else. It is
  // never a bucket's own icon (DESIGN.md §4).
  buckets: '<path d="M3.2 5.2h9.6l-1 8a1.1 1.1 0 0 1-1.1 1H5.3a1.1 1.1 0 0 1-1.1-1Z"/><path d="M5.5 5.2a2.5 2.5 0 0 1 5 0"/>',
  log: '<circle cx="8" cy="8" r="6"/><path d="M8 4.6V8l2.4 1.5"/>',
  settings: '<circle cx="8" cy="8" r="2.4"/><path d="M8 1.8v2M8 12.2v2M1.8 8h2M12.2 8h2M3.6 3.6l1.4 1.4M11 11l1.4 1.4M12.4 3.6 11 5M5 11l-1.4 1.4"/>',
  admin: '<path d="M8 1.8 13.5 4v4.2c0 3-2.3 5.2-5.5 6-3.2-.8-5.5-3-5.5-6V4Z"/>',
  view: '<path d="M2.2 3.4h11.6L9.3 8.2v4.6L6.7 14V8.2Z"/>',
  search: '<circle cx="7" cy="7" r="4.2"/><path d="M10.1 10.1 13.6 13.6"/>',
  plus: '<path d="M8 3.2v9.6M3.2 8h9.6"/>',
  chevron: '<path d="M4.5 6.5 8 10l3.5-3.5"/>',
  close: '<path d="M4 4l8 8M12 4l-8 8"/>',
  agent: '<rect x="2.6" y="4.6" width="10.8" height="8" /><path d="M8 1.8v2.8M5.4 8.2h.01M10.6 8.2h.01" stroke-width="1.8"/>',
  copy: '<rect x="5.4" y="2.6" width="8" height="8"/><path d="M10.6 13.4h-8v-8"/>',
  split: '<path d="M8 2.6v5M8 7.6 3.6 12M8 7.6l4.4 4.4"/>',
  pen: '<path d="M2.9 13.1 4.5 8.5l5.8-5.8 2.9 2.9-5.8 5.8Z"/><path d="M9.3 3.7l2.9 2.9"/>',
};

// Priority is four glyphs *and* four colours. The shape carries the level on
// its own; colour only reinforces it. It is never collapsed to a dot.
export const PRIORITY = {
  1: { colour: "var(--accent)", body: '<path d="M4 8.5 8 4.5l4 4"/><path d="M4 12 8 8l4 4"/>', name: "P1 · Highest" },
  2: { colour: "var(--steel)", body: '<path d="M3.8 10.2 8 6l4.2 4.2"/>', name: "P2 · High" },
  3: { colour: "var(--muted)", body: '<path d="M3.8 8h8.4"/>', name: "P3 · Medium" },
  4: { colour: "var(--faint)", body: '<path d="M3.8 5.8 8 10l4.2-4.2"/>', name: "P4 · Low" },
};

// The fixed set of eight bucket icons (DESIGN.md §4). Not extensible, not
// uploadable. The icon is recognition and nothing more: nothing reads it,
// filters on it, or derives behaviour from it.
export const BUCKET_ICONS = {
  check: '<path d="M2.9 2.9h10.2v10.2H2.9Z"/><path d="M5.7 8.2 7.4 9.9l3-3.4"/>',
  bug: '<rect x="4.9" y="5" width="6.2" height="8" rx="3.1"/><path d="M4.9 8H2.5M13.5 8h-2.4M5.7 5.3 4.5 3.5M10.3 5.3l1.2-1.8"/>',
  bookmark: '<path d="M4.1 2.7h7.8v10.9L8 10.9l-3.9 2.7Z"/>',
  page: '<path d="M4.2 2.5h4.9L12 5.4v8.1H4.2Z"/><path d="M8.9 2.5v3h3.1"/>',
  pen: '<path d="M2.9 13.1 4.5 8.5l5.8-5.8 2.9 2.9-5.8 5.8Z"/><path d="M9.3 3.7l2.9 2.9"/>',
  wrench: '<path d="M10.5 2.7a3.3 3.3 0 0 0-3.9 4.3l-4 4a1.35 1.35 0 0 0 1.9 1.9l4-4a3.3 3.3 0 0 0 4.3-3.9l-1.9 1.9-1.6-.4-.4-1.6Z"/>',
  lens: '<circle cx="7" cy="7" r="4.2"/><path d="M10.1 10.1 13.6 13.6"/>',
  flag: '<path d="M4.1 2.5v11.1"/><path d="M4.1 3.3h7.5L10 5.9l1.6 2.6H4.1"/>',
};

// The fixed set of colours a bucket may be given, on the same terms as the
// icons: recognition only, nothing reads one. A bucket without a colour is
// drawn as it always was, so the set has no default member.
export const BUCKET_COLOURS = {
  amber: "#d9a05b", rust: "#c4675c", olive: "#a3a961", steel: "#7d94a8",
  mauve: "#b087a3", sage: "#8aa8a0", stone: "#8f8578",
};

// The colour rendering. A team picks which its surfaces use; monochrome is the
// default and which one is in force never changes the icon a bucket carries.
export const ICON_COLOURS = {
  check: "#7d94a8", bug: "#c4675c", bookmark: "#b087a3", page: "#8aa8a0",
  pen: "#d9a05b", wrench: "#8f8578", lens: "#a3a961", flag: "#7d94a8",
};

export const RELATION_LABELS = {
  blocks: "blocks", blocked_by: "blocked by",
  follows_up: "follows up", followed_up_by: "followed up by",
  duplicates: "duplicates", duplicated_by: "duplicated by",
  relates_to: "relates to",
};
