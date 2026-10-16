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
  calendar: '<rect x="2.5" y="3.5" width="11" height="10"/><path d="M2.5 6.5h11M5.5 2v3M10.5 2v3"/>',
  log: '<circle cx="8" cy="8" r="6"/><path d="M8 4.6V8l2.4 1.5"/>',
  settings: '<path d="M2 5h12"/><circle cx="6" cy="5" r="1.9"/><path d="M2 11h12"/><circle cx="10.5" cy="11" r="1.9"/>',
  team: '<circle cx="6" cy="5.6" r="2.3"/><path d="M1.9 13.4c.4-2.4 2-3.9 4.1-3.9s3.7 1.5 4.1 3.9"/><path d="M10.4 3.5a2.3 2.3 0 0 1 0 4.2M12 9.9c1.2.6 1.9 1.8 2.1 3.5"/>',
  admin: '<path d="M8 1.9 13.4 4.1v4.2c0 3-2.3 5.2-5.4 5.9-3.1-.7-5.4-2.9-5.4-5.9V4.1L8 1.9Z"/>',
  docs: '<path d="M1.8 3h4.4L8 4.4 9.8 3h4.4v9.8H9.8L8 14.2l-1.8-1.4H1.8Z"/><path d="M8 4.4v9.8"/>',
  view: '<path d="M2.5 3.5h11l-4.3 5v4.3l-2.4-1.4V8.5Z"/>',
  // Distinct from the `lens` bucket icon, so search never reads as a bucket.
  search: '<circle cx="7" cy="7" r="4.6"/><path d="M10.4 10.4 14 14"/>',
  plus: '<path d="M8 3v10M3 8h10"/>',
  chevron: '<path d="M3.5 6 8 10.5 12.5 6"/>',
  close: '<path d="M4.5 4.5 11.5 11.5M11.5 4.5 4.5 11.5"/>',
  agent: '<rect x="3" y="5" width="10" height="8.5"/><path d="M8 5V2.5M6 8.2v1.6M10 8.2v1.6M1.5 8v3M14.5 8v3"/>',
  link: '<path d="M6.5 9.5 9.5 6.5"/><path d="M9 4.6 10.4 3.2a2.7 2.7 0 0 1 3.8 3.8l-1.4 1.4"/><path d="M7 11.4 5.6 12.8a2.7 2.7 0 0 1-3.8-3.8l1.4-1.4"/>',
  clone: '<rect x="5.5" y="5.5" width="8" height="8"/><path d="M10.5 2.5h-8v8"/>',
  split: '<path d="M2.5 8h4l3-4h4"/><path d="M9.5 12h4"/><path d="M6.5 8l3 4"/><path d="M11.5 2.5 13.5 4l-2 1.5"/><path d="M11.5 10.5 13.5 12l-2 1.5"/>',
  signout: '<path d="M7 2.5H2.5v11H7"/><path d="M6.5 8h6.5M10 5l3 3-3 3"/>',
  export: '<path d="M8 2v8M4.5 6.5 8 10l3.5-3.5"/><path d="M2.5 13h11"/>',
  // Distinct from the `pen` bucket icon, so editing never reads as a bucket.
  pen: '<path d="M10.6 2.9 13.1 5.4 5.5 13H3v-2.5Z"/>',
  // The floppy: what a save wears beside the field it writes.
  save: '<path d="M2.6 2.6h8.4l2.4 2.4v8.4H2.6Z"/><path d="M5.4 2.6v3.8h5.2V2.6"/><path d="M4.8 13.4V9h6.4v4.4"/>',
};

// Priority is four glyphs and two signals. The shape carries the level on its
// own; colour marks P1 and P2 only (DESIGN.md §3). P3 is the default and on
// most cards, so it is silent.
export const PRIORITY = {
  1: { colour: "var(--alert)", body: '<path d="M4 8.5 8 4.5l4 4"/><path d="M4 12 8 8l4 4"/>', name: "P1 · Highest" },
  2: { colour: "var(--accent)", body: '<path d="M3.8 10.2 8 6l4.2 4.2"/>', name: "P2 · High" },
  3: { colour: "var(--muted)", body: '<path d="M3.8 8h8.4"/>', name: "P3 · Medium" },
  4: { colour: "var(--faint)", body: '<path d="M3.8 5.8 8 10l4.2-4.2"/>', name: "P4 · Low" },
};

// The bucket icons (DESIGN.md §4), in the groups a picker draws them in and in
// that order. A key is stored on every bucket and named by the API, so after
// v1 a key may be added but never renamed or removed. The icon is recognition
// and nothing more: nothing reads it, filters on it, or derives behaviour from
// it.
export const BUCKET_ICON_GROUPS = {
  planning: {
    flag: '<path d="M4.1 2.5v11.1"/><path d="M4.1 3.3h7.5L10 5.9l1.6 2.6H4.1"/>',
    bookmark: '<path d="M4.1 2.7h7.8v10.9L8 10.9l-3.9 2.7Z"/>',
    milestone: '<path d="M8 2 14 8 8 14 2 8Z"/><path d="M8 5.5 10.5 8 8 10.5 5.5 8Z"/>',
    target: '<circle cx="8" cy="8" r="5.6"/><circle cx="8" cy="8" r="2.8"/><path d="M8 7.6v.8"/>',
    rocket: '<path d="M8 1.8c2.2 1.6 3.2 4 3.2 6.8L9.6 11H6.4L4.8 8.6C4.8 5.8 5.8 3.4 8 1.8Z"/><circle cx="8" cy="6.3" r="1.2"/><path d="M4.8 8.6 3 10.5v2.3l3.4-1.8M11.2 8.6 13 10.5v2.3L9.6 11M7 13.2 8 14.6l1-1.4"/>',
    hourglass: '<path d="M4 2.5h8M4 13.5h8M5 2.5v1.8L11 11.7v1.8M11 2.5v1.8L5 11.7v1.8"/>',
    chart: '<path d="M2.5 13.5h11M4.5 13.5V9M8 13.5V4.5M11.5 13.5V7"/>',
    inbox: '<path d="M2.5 9 4.2 3h7.6l1.7 6v4.5h-11Z"/><path d="M2.5 9h3l.8 1.6h3.4l.8-1.6h3"/>',
    archive: '<path d="M2 3h12v3H2Z"/><path d="M3 6v7.5h10V6M6.5 8.5h3"/>',
    trophy: '<path d="M5 2.5h6v3.5a3 3 0 0 1-6 0Z"/><path d="M5 3.5H2.8c0 2 1 3 2.4 3.2M11 3.5h2.2c0 2-1 3-2.4 3.2M8 9v4M5.5 13.5h5"/>',
  },
  signals: {
    lens: '<circle cx="7" cy="7" r="4.2"/><path d="M10.1 10.1 13.6 13.6"/>',
    bolt: '<path d="M9.2 1.8 3.8 9h4l-1 5.2L12.2 7h-4Z"/>',
    star: '<path d="M8 2.2l1.8 3.7 4 .6-2.9 2.8.7 4L8 11.4l-3.6 1.9.7-4L2.2 6.5l4-.6Z"/>',
    sparkle: '<path d="M8 1.8 9.4 6.6 14.2 8 9.4 9.4 8 14.2 6.6 9.4 1.8 8 6.6 6.6Z"/>',
    bell: '<path d="M4 11V7a4 4 0 0 1 8 0v4l1.2 1.5H2.8Z"/><path d="M6.6 14h2.8"/>',
    heart: '<path d="M8 13.5 2.7 8.2a3 3 0 0 1 4.3-4.3L8 4.9l1-1a3 3 0 0 1 4.3 4.3Z"/>',
    chat: '<path d="M2.5 3h11v7.5H7L4 13v-2.5H2.5Z"/>',
    bulb: '<path d="M6 12h4M6.5 14h3M6 12c0-2-2.5-3-2.5-6a4.5 4.5 0 0 1 9 0c0 3-2.5 4-2.5 6"/>',
    coin: '<circle cx="8" cy="8" r="5.8"/><path d="M10 5.8H7.2a1.1 1.1 0 0 0 0 2.2h1.6a1.1 1.1 0 0 1 0 2.2H6M8 4.5v1.3M8 10.2v1.3"/>',
    key: '<circle cx="5.3" cy="10.7" r="2.7"/><path d="M7.2 8.8 13.5 2.5M11 5l1.8 1.8M12.4 3.6l1.4 1.4"/>',
    lock: '<path d="M3.5 7h9v6.5h-9Z"/><path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2M8 9.5v1.6"/>',
  },
  work: {
    check: '<path d="M2.9 2.9h10.2v10.2H2.9Z"/><path d="M5.7 8.2 7.4 9.9l3-3.4"/>',
    bug: '<path d="M5 6h6v4.5a3 3 0 0 1-6 0Z"/><path d="M6 6a2 2 0 0 1 4 0M5 8.5H2.5M13.5 8.5H11M5 11.5l-2 1.5M11 11.5l2 1.5M5.3 6.4 3.5 5M10.7 6.4 12.5 5M8 9v4.5"/>',
    wrench: '<path d="M10.5 2.7a3.3 3.3 0 0 0-3.9 4.3l-4 4a1.35 1.35 0 0 0 1.9 1.9l4-4a3.3 3.3 0 0 0 4.3-3.9l-1.9 1.9-1.6-.4-.4-1.6Z"/>',
    pen: '<path d="M3 13l1-3.6 6.6-6.6 2.6 2.6L6.6 12Z"/><path d="M9.3 4.1l2.6 2.6M3 13l1.8-1.8"/>',
    page: '<path d="M4.2 2.5h4.9L12 5.4v8.1H4.2Z"/><path d="M8.9 2.5v3h3.1"/>',
    code: '<path d="M5.5 4.5 2 8l3.5 3.5M10.5 4.5 14 8l-3.5 3.5M9.2 3 6.8 13"/>',
    terminal: '<path d="M2 3h12v10H2Z"/><path d="M4.5 6.2 6.6 8.3 4.5 10.4M8 10.5h3.5"/>',
    database: '<ellipse cx="8" cy="4" rx="5" ry="1.8"/><path d="M3 4v8c0 1 2.2 1.8 5 1.8s5-.8 5-1.8V4M3 8c0 1 2.2 1.8 5 1.8s5-.8 5-1.8"/>',
    beaker: '<path d="M5.8 2.5h4.4M6.8 2.5v4L3 13.5h10L9.2 6.5v-4"/><path d="M4.6 10.5h6.8"/>',
    box: '<path d="M2.5 4.8 8 2.3l5.5 2.5v6.6L8 13.9l-5.5-2.5Z"/><path d="M2.5 4.8 8 7.3l5.5-2.5M8 7.3v6.6"/>',
    layers: '<path d="M8 2 14 5 8 8 2 5Z"/><path d="M2 8l6 3 6-3M2 11l6 3 6-3"/>',
  },
  places: {
    flame: '<path d="M8 14c-2.6 0-4.3-1.7-4.3-4 0-2.4 2-3.6 2.4-6.2C8 5 8.6 6.4 8.6 7.6c.8-.5 1.2-1.4 1.2-2.4 1.7 1.3 2.5 3 2.5 4.8 0 2.3-1.7 4-4.3 4Z"/>',
    tent: '<path d="M1.8 13.5h12.4M3 13.5 8 3l5 10.5M6.3 13.5 8 10l1.7 3.5"/>',
    mountain: '<path d="M1.5 13.5 6 6l2.5 3.5L10.5 7l4 6.5Z"/>',
    leaf: '<path d="M3 13c0-6 4-10 10-10 0 6-4 10-10 10Z"/><path d="M3 13 9 7"/>',
    sun: '<circle cx="8" cy="8" r="2.8"/><path d="M8 1.5v2M8 12.5v2M1.5 8h2M12.5 8h2M3.4 3.4l1.4 1.4M11.2 11.2l1.4 1.4M3.4 12.6l1.4-1.4M11.2 4.8l1.4-1.4"/>',
    moon: '<path d="M12.8 10.2A5.5 5.5 0 0 1 5.8 3.2a5.5 5.5 0 1 0 7 7Z"/>',
    globe: '<circle cx="8" cy="8" r="5.8"/><path d="M2.2 8h11.6M8 2.2c-2 1.8-2.6 3.8-2.6 5.8s.6 4 2.6 5.8M8 2.2c2 1.8 2.6 3.8 2.6 5.8s-.6 4-2.6 5.8"/>',
    map: '<path d="M2 4 6 2.5l4 1.5 4-1.5V12l-4 1.5-4-1.5-4 1.5Z"/><path d="M6 2.5V12M10 4v9.5"/>',
    anchor: '<circle cx="8" cy="4" r="1.6"/><path d="M8 5.6V14M5.5 8h5M3 9.5c0 2.6 2.2 4.5 5 4.5s5-1.9 5-4.5"/>',
    home: '<path d="M2.5 7.5 8 2.8l5.5 4.7v6h-11Z"/><path d="M6.5 13.5V10h3v3.5"/>',
  },
};

// Every bucket icon by key, in the groups' order.
export const BUCKET_ICONS = Object.assign({}, ...Object.values(BUCKET_ICON_GROUPS));

// The colours a bucket may be given, on the same terms as the icons:
// recognition only, nothing reads one. In hue order. No red, which means late
// or P1 on a card, and no accent orange, which means selected. A bucket
// without a colour is drawn as it always was, so the set has no default.
export const BUCKET_COLOURS = {
  amber: "#d9a05b", straw: "#c8b45e", olive: "#a3a961", moss: "#7fa86a",
  sage: "#8aa8a0", teal: "#5fa7a3", sky: "#6fa3c7", steel: "#7d94a8",
  dusk: "#8a8fc9", plum: "#9d82c2", mauve: "#b087a3", rose: "#c48098",
  rust: "#a27459", sand: "#b7a587", stone: "#8f8578", slate: "#737d87",
};

// What a bucket's colour draws in: a palette key's hex, or a hex of the
// instance's own where it takes them. Anything else is no colour.
export const colourOf = (c) => BUCKET_COLOURS[c] || (/^#[0-9a-f]{6}$/.test(c || "") ? c : null);

export const RELATION_LABELS = {
  blocks: "blocks", blocked_by: "blocked by",
  follows_up: "follows up", followed_up_by: "followed up by",
  duplicates: "duplicates", duplicated_by: "duplicated by",
  relates_to: "relates to",
};
