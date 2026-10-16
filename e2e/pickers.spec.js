import { test, expect } from "@playwright/test";

// What the bucket pickers offer is the instance administrator's (DEV-79):
// hiding leaves the picker only, and hex colours come and go with the mode.
//
// Self-contained, like network.spec.js: it brings up its own team, and puts
// the instance's appearance back when it is done.

const ROOT = { username: "root", password: "correct horse battery" };
test.describe.configure({ mode: "serial" });

let bucket;

test.beforeAll(async ({ request }) => {
  await request.post("/api/instance/bootstrap", { data: ROOT }); // refused once there is an account
  await request.post("/api/auth/signin", { data: ROOT });
  const shortCode = Array.from({ length: 5 }, () => String.fromCharCode(65 + Math.floor(Math.random() * 26))).join("");
  const team = await (await request.post("/api/teams", { data: { name: `Looks ${shortCode}`, shortCode } })).json();
  bucket = await (await request.post("/api/items", {
    data: { kind: "bucket", teamId: team.id, title: "Hearth", icon: "flame", color: "rose" },
  })).json();
});

test.afterAll(async ({ request }) => {
  await request.post("/api/auth/signin", { data: ROOT });
  await request.put("/api/instance/appearance", { data: { colourMode: "palette", hiddenGroups: [], hiddenColours: [] } });
});

test("a hidden group and colour leave the picker, and a bucket that wears them keeps them", async ({ page }) => {
  await page.request.post("/api/auth/signin", { data: ROOT });
  await page.goto("/admin?tab=appearance");
  await page.getByRole("button", { name: "Offer Places" }).click();
  await expect(page.getByRole("button", { name: "Offer Places" })).toHaveAttribute("aria-pressed", "false");
  await page.getByRole("button", { name: "Offer rose" }).click();
  await expect(page.getByRole("button", { name: "Offer rose" })).toHaveAttribute("aria-pressed", "false");
  await page.getByRole("button", { name: "Palette and hex" }).click();

  await page.goto(`/${bucket.identifier}`);
  await page.getByRole("button", { name: /flame.*rose/ }).click();
  const looks = page.getByRole("dialog", { name: "Icon and colour" });
  // Places stays, because this bucket wears one of its icons; rose too.
  await expect(looks.getByRole("button", { name: "flame", exact: true })).toBeVisible();
  await expect(looks.getByRole("button", { name: "rose", exact: true })).toBeVisible();

  await looks.getByRole("button", { name: "rocket", exact: true }).click();
  await looks.getByRole("button", { name: "sky", exact: true }).click();
  await expect(page.getByRole("button", { name: /rocket.*sky/ })).toBeVisible();
  // Worn by nothing now, both are gone from the panel.
  await expect(looks.getByRole("button", { name: "flame", exact: true })).toHaveCount(0);
  await expect(looks.getByRole("button", { name: "rose", exact: true })).toHaveCount(0);

  await looks.getByLabel("Hex colour").fill("#FF88CC");
  await looks.getByRole("button", { name: "Use" }).click();
  await expect(page.getByRole("button", { name: /rocket.*#ff88cc/ })).toBeVisible();
});
