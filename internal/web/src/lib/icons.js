// The glyphs of DESIGN.md, as path data. Every stroke is square and mitred;
// round caps are off-language.

// Navigation glyphs, taken from the artboards in the design directory. Those
// are the source: where a glyph here disagreed with the one drawn there, the
// artboard won.
//
// Copy link, Clone and Export each have their own glyph. They shared one, and
// three header actions wearing the same mark is three actions nobody can tell
// apart at a glance.
export const NAV = {
  board: '<rect x="2" y="2.5" width="3.5" height="11"/><rect x="6.25" y="2.5" width="3.5" height="7.5"/><rect x="10.5" y="2.5" width="3.5" height="9.5"/>',
  backlog: '<path d="M5 4.5h6M5 8h6M5 11.5h6"/>',
  // The pail is the sidebar's Buckets section icon and nothing else. It is
  // never a bucket's own icon (DESIGN.md §4).
  buckets: '<path d="M3.2 5.2h9.6l-1 8a1.1 1.1 0 0 1-1.1 1H5.3a1.1 1.1 0 0 1-1.1-1Z"/><path d="M5.5 5.2a2.5 2.5 0 0 1 5 0"/>',
  log: '<circle cx="8" cy="8" r="6"/><path d="M8 4.6V8l2.4 1.5"/>',
  settings: '<path d="M2 5h12"/><circle cx="6" cy="5" r="1.9"/><path d="M2 11h12"/><circle cx="10.5" cy="11" r="1.9"/>',
  team: '<circle cx="6" cy="5.6" r="2.3"/><path d="M1.9 13.4c.4-2.4 2-3.9 4.1-3.9s3.7 1.5 4.1 3.9"/><path d="M10.4 3.5a2.3 2.3 0 0 1 0 4.2M12 9.9c1.2.6 1.9 1.8 2.1 3.5"/>',
  admin: '<path d="M8 1.9 13.4 4.1v4.2c0 3-2.3 5.2-5.4 5.9-3.1-.7-5.4-2.9-5.4-5.9V4.1L8 1.9Z"/>',
  view: '<path d="M2.5 3.5h11l-4.3 5v4.3l-2.4-1.4V8.5Z"/>',
  // Distinct from the `lens` bucket icon, which the two used to share.
  search: '<circle cx="7" cy="7" r="4.6"/><path d="M10.4 10.4 14 14"/>',
  plus: '<path d="M8 3v10M3 8h10"/>',
  chevron: '<path d="M3.5 6 8 10.5 12.5 6"/>',
  close: '<path d="M4.5 4.5 11.5 11.5M11.5 4.5 4.5 11.5"/>',
  agent: '<rect x="2.5" y="5" width="11" height="8.5"/><path d="M5.5 8.2 7 9.7l-1.5 1.5M8.8 11.2h2.2M8 5V2.6"/>',
  link: '<path d="M6.5 9.5 9.5 6.5"/><path d="M9 4.6 10.4 3.2a2.7 2.7 0 0 1 3.8 3.8l-1.4 1.4"/><path d="M7 11.4 5.6 12.8a2.7 2.7 0 0 1-3.8-3.8l1.4-1.4"/>',
  clone: '<rect x="5.5" y="5.5" width="8" height="8"/><path d="M10.5 2.5h-8v8"/>',
  split: '<path d="M2.5 8h4l3-4h4"/><path d="M9.5 12h4"/><path d="M6.5 8l3 4"/><path d="M11.5 2.5 13.5 4l-2 1.5"/><path d="M11.5 10.5 13.5 12l-2 1.5"/>',
  export: '<path d="M8 2v8M4.5 6.5 8 10l3.5-3.5"/><path d="M2.5 13h11"/>',
  // Distinct from the `pen` bucket icon, which the two used to share.
  pen: '<path d="M10.6 2.9 13.1 5.4 5.5 13H3v-2.5Z"/>',
  // The floppy: what a save wears beside the field it writes.
  save: '<path d="M2.6 2.6h8.4l2.4 2.4v8.4H2.6Z"/><path d="M5.4 2.6v3.8h5.2V2.6"/><path d="M4.8 13.4V9h6.4v4.4"/>',
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
