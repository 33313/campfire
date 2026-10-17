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
test("the bucket sets are the server's, in order", async ({ request }) => {
  const { bucketIcons, bucketColors } = await (await request.get("/api/instance")).json();
  expect(Object.keys(BUCKET_ICONS)).toEqual(bucketIcons);
  expect(Object.values(BUCKET_ICON_GROUPS).flatMap(Object.keys)).toEqual(bucketIcons);
  expect(Object.keys(BUCKET_COLOURS)).toEqual(bucketColors);
});
