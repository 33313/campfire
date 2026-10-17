import { test, expect } from "@playwright/test";

import { NAV, PRIORITY, BUCKET_ICON_GROUPS, BUCKET_ICONS, BUCKET_COLOURS }
  from "../internal/web/src/lib/icons.js";

// The glyph sets, checked as data. Three header actions once shared one mark,
// and `search` was the `lens` bucket icon; both were invisible in a diff and
// obvious on screen (DEV-11).

test("no two glyphs in a set share their path data", () => {
  for (const [name, set] of Object.entries({
    NAV, BUCKET_ICONS, PRIORITY: Object.fromEntries(
      Object.entries(PRIORITY).map(([k, p]) => [k, p.body])),
  })) {
    const byBody = new Map();
    for (const [key, body] of Object.entries(set)) {
      const seen = byBody.get(body);
      expect(seen, `${name}.${key} and ${name}.${seen} are the same glyph`).toBeUndefined();
      byBody.set(body, key);
    }
  }
});

// The keys are stored on every bucket and named by the API, so they match
// store.BucketIcons and store.BucketColours, in order (DESIGN.md §4).
test("the bucket sets are the built-in groups and colours, in order", () => {
  expect(Object.keys(BUCKET_ICON_GROUPS)).toEqual(["planning", "signals", "work", "places"]);
  expect(Object.keys(BUCKET_ICONS)).toEqual([
    "flag", "bookmark", "milestone", "target", "rocket", "hourglass", "chart", "inbox", "archive", "trophy",
    "lens", "bolt", "star", "sparkle", "bell", "heart", "chat", "bulb", "coin", "key", "lock",
    "check", "bug", "wrench", "pen", "page", "code", "terminal", "database", "beaker", "box", "layers",
    "flame", "tent", "mountain", "leaf", "sun", "moon", "globe", "map", "anchor", "home",
  ]);
  expect(Object.keys(BUCKET_COLOURS)).toEqual([
    "amber", "straw", "olive", "moss", "sage", "teal", "sky", "steel",
    "dusk", "plum", "mauve", "rose", "rust", "sand", "stone", "slate",
  ]);
});

test("every glyph is well-formed and off no colour but the token list", () => {
  const hex = /^#[0-9a-f]{6}$/;
  for (const body of [...Object.values(NAV), ...Object.values(BUCKET_ICONS)]) {
    expect(body).toMatch(/^<(path|rect|circle|ellipse)\b/);
    // Round caps are off-language (DESIGN.md §1), and a glyph never carries
    // its own colour: the component that draws it decides.
    expect(body).not.toContain("round");
    expect(body).not.toContain("fill=");
  }
  for (const value of Object.values(BUCKET_COLOURS)) {
    expect(value).toMatch(hex);
  }
});
