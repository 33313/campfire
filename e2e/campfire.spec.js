import { test, expect } from "@playwright/test";

// One instance, driven from an empty database through the flows the client
// exists for. The tests run in order and share the state they build, because
// that is the shape of the product: an instance is bootstrapped once.

const ROOT = { username: "root", password: "correct horse battery" };

test.describe.configure({ mode: "serial" });

async function signIn(page, who = ROOT) {
  await page.goto("/");
  const heading = page.getByRole("heading");
  await expect(heading).toBeVisible();
  if ((await heading.textContent()) === "Sign in") {
    await page.getByLabel("Username").fill(who.username);
    await page.getByLabel("Password").fill(who.password);
    await page.getByRole("button", { name: "Sign in" }).click();
  }
  await expect(page.locator(".sidebar")).toBeVisible();
}

test("the first visit offers to create the first account", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Create the first account" })).toBeVisible();

  await page.getByLabel("Username").fill(ROOT.username);
  await page.getByLabel("Password").fill(ROOT.password);
  await page.getByRole("button", { name: "Create account" }).click();

  // With no team yet, the redirect lands on the bucket index.
  await expect(page.locator(".sidebar")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Buckets" })).toBeVisible();
});

test("an administrator creates the first team", async ({ page }) => {
  await signIn(page);
  await page.locator(".team-switch").click();
  await page.getByRole("button", { name: "New team" }).click();

  const team = page.getByRole("dialog");
  await team.getByLabel("Name").fill("Solstice");
  await team.getByLabel("Short code").fill("SOL");
  await team.getByRole("button", { name: "Create" }).click();

  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
  await expect(page.locator(".code-badge").first()).toHaveText("SOL");
});

test("a new ticket lands in the backlog and leaves you where you were", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();

  const form = page.getByRole("dialog");
  await form.getByLabel("Title").fill("Replace the boiler");
  await form.getByLabel("Description").fill("It is older than the building.");
  // Every field the item has is on the form, so creating is not followed by
  // three edits to finish it.
  await form.getByLabel("Assignee").selectOption({ label: "root" });
  await form.getByLabel("Labels").fill("plumbing");
  await form.getByRole("button", { name: "Create" }).click();

  // Creating does not leave the board for the new item's detail view.
  await expect(page).toHaveURL(/\/t\/1\/board$/);
  await expect(page.locator(".toast")).toHaveText(/SOL-1 created/);

  await page.goto("/t/1/backlog");
  await expect(page.getByRole("link", { name: "SOL-1" })).toBeVisible();

  await page.goto("/SOL-1");
  await expect(page.locator(".label-chip", { hasText: "plumbing" })).toBeVisible();
  await expect(page.locator(".aside-control", { hasText: "root" })).toBeVisible();
});

test("a create that cannot go through keeps the form and marks the field", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();

  const form = page.getByRole("dialog");
  await form.getByLabel("Description").fill("Typed at some length, and worth keeping.");
  await form.getByRole("button", { name: "Create" }).click();

  await expect(form).toBeVisible();
  await expect(form.getByLabel("Title")).toHaveClass(/error/);
  await expect(form.locator(".field-error")).toHaveText("An item needs a title.");
  await expect(form.getByLabel("Description")).toHaveValue(/worth keeping/);

  // Correcting the field clears the mark.
  await form.getByLabel("Title").fill("Bleed the radiators");
  await expect(form.getByLabel("Title")).not.toHaveClass(/error/);
  await expect(form.locator(".field-error")).toHaveCount(0);

  await form.getByRole("button", { name: "Cancel" }).click();
});

test("a ticket moves to a board state and appears in that column", async ({ page }) => {
  await signIn(page);
  await page.goto("/SOL-1");

  await page.locator(".aside-control").first().click();
  const move = page.getByRole("dialog");
  await move.getByRole("button", { name: "In Progress" }).click();
  await move.getByRole("button", { name: "Move", exact: true }).click();

  await expect(page.locator(".aside-control").first()).toContainText("In Progress");

  await page.goto("/t/1/board");
  const column = page.locator(".column", { has: page.getByRole("heading", { name: "In Progress" }) });
  await expect(column.locator(".card")).toHaveText(/Replace the boiler/);
});

test("a card previews beside the board and its identifier opens it", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");

  await page.locator(".card").first().click();
  const preview = page.locator(".detail-aside");
  await expect(preview.getByRole("heading", { name: "Replace the boiler" })).toBeVisible();
  await expect(page).toHaveURL(/\/t\/1\/board$/);

  // The preview edits in place: a pick lands on the item and the board redraws.
  await preview.getByRole("button", { name: /root/ }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Unassigned" }).click();
  await expect(preview.getByRole("button", { name: /Unassigned/ })).toBeVisible();

  await preview.getByRole("link", { name: "SOL-1" }).click();
  await expect(page).toHaveURL(/\/SOL-1$/);
});

test("a comment is written and read back", async ({ page }) => {
  await signIn(page);
  await page.goto("/SOL-1");

  await page.getByPlaceholder("Write a comment…").fill("The part is on order.");
  await page.getByRole("button", { name: "Comment", exact: true }).click();

  await expect(page.locator(".comment-body")).toHaveText("The part is on order.");
  await expect(page.getByRole("button", { name: /^Comments/ })).toContainText("1");
});

test("a label is added on the detail view and nowhere else", async ({ page }) => {
  await signIn(page);
  await page.goto("/SOL-1");

  await page.getByRole("button", { name: "Label" }).click();
  await page.getByPlaceholder("Type to create").fill("heating");
  await page.getByPlaceholder("Type to create").press("Enter");

  await expect(page.locator(".label-chip", { hasText: "heating" })).toBeVisible();

  // SPEC.md §8.1: a card never carries a label.
  await page.goto("/t/1/board");
  await expect(page.locator(".card")).not.toContainText("heating");
});

test("a bucket carries an icon and its tickets", async ({ page }) => {
  await signIn(page);
  await page.goto("/buckets");
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();

  const bucket = page.getByRole("dialog");
  await bucket.getByRole("button", { name: "Bucket", exact: true }).click();
  await bucket.getByLabel("Title").fill("Winter readiness");
  await bucket.getByRole("button", { name: "rust", exact: true }).click();
  await bucket.getByRole("button", { name: "Create" }).click();

  await expect(page).toHaveURL(/\/buckets$/);
  await expect(page.locator(".bucket-card", { hasText: "Winter readiness" })).toBeVisible();

  await page.goto("/SOL-2");
  await expect(page.locator(".state-badge", { hasText: "bucket" })).toBeVisible();

  // Put the ticket in it from the ticket's own aside.
  await page.goto("/SOL-1");
  await page.locator(".aside-control").last().click();
  await page.getByRole("dialog").getByRole("button", { name: /SOL-2\s+Winter readiness/ }).click();
  await expect(page.locator(".aside-control").last()).toContainText("Winter readiness");

  // The bucket's colour rides on its tickets' cards, in the foot beside the
  // identifier rather than above the title.
  await page.goto("/t/1/board");
  const chip = page.locator(".card-foot .card-bucket");
  await expect(chip).toContainText("Winter readiness");
  await expect(chip).toHaveCSS("border-color", "rgb(196, 103, 92)");

  await page.goto("/SOL-2");
  await expect(page.getByRole("link", { name: "SOL-1" })).toBeVisible();
  await expect(page.locator(".progress-block .big")).toHaveText("0");
});

test("the log records what happened, newest first", async ({ page }) => {
  await signIn(page);
  await page.goto("/log");

  const rows = page.locator(".table tbody tr");
  await expect(rows.first()).toBeVisible();

  // Ordered by sequence, newest first (SPEC.md §10).
  const seqs = await page.locator(".table tbody td.num").first().textContent();
  expect(Number(seqs)).toBeGreaterThan(1);

  await page.getByRole("button", { name: "Field" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "bucket" }).click();
  await expect(page.locator(".table tbody td.field-label").first()).toHaveText("bucket");
});

test("the filter bar narrows the board and clears again", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await expect(page.locator(".card")).toHaveCount(1);

  await page.locator(".filter-bar").getByRole("button", { name: "Priority" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "P1 · Highest" }).click();
  await expect(page.locator(".card")).toHaveCount(0);

  await page.locator(".filter-bar").getByRole("button", { name: "P1", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Any" }).click();
  await expect(page.locator(".card")).toHaveCount(1);
});

test("a saved view is stored as clauses and appears in the sidebar", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");

  await page.locator(".filter-bar").getByRole("button", { name: "Priority" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "P3 · Medium" }).click();

  await page.getByRole("button", { name: "Save as view" }).click();
  const view = page.getByRole("dialog");
  await view.getByLabel("Name").fill("Medium work");
  await view.getByRole("button", { name: "Save" }).click();

  await expect(page.locator(".sidebar").getByRole("link", { name: "Medium work" })).toBeVisible();
  await page.locator(".sidebar").getByRole("link", { name: "Medium work" }).click();
  await expect(page).toHaveURL(/view=\d+$/);
  await expect(page.locator(".card")).toHaveCount(1);
});

test("work states can be added and the team renamed", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/settings");

  await page.getByRole("button", { name: "Add state" }).click();
  const state = page.getByRole("dialog");
  await state.getByLabel("Name").fill("In review");
  await state.getByRole("button", { name: "Save" }).click();
  await expect(page.locator(".table tbody tr", { hasText: "In review" })).toBeVisible();

  await page.getByRole("button", { name: "Identity" }).click();
  const main = page.locator(".settings-main");
  await main.getByLabel("Name").fill("Solstice Crew");
  await main.getByRole("button", { name: "Save" }).click();
  await expect(page.locator(".screen-header .sub")).toHaveText("Solstice Crew · SOL");
});

test("an invitation link is shown once", async ({ page }) => {
  await signIn(page);
  await page.goto("/admin");

  await page.getByRole("button", { name: "Invitation link" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Shown once");
  await expect(dialog.locator("input")).toHaveValue(/\/invite\/.+/);
  await dialog.getByRole("button", { name: "Done" }).click();
});

test("an unknown address says so and offers the way back", async ({ page }) => {
  await signIn(page);
  await page.goto("/nowhere");
  await expect(page.getByRole("heading", { name: "Nothing here" })).toBeVisible();
  await page.getByRole("link", { name: "Back to the board" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
});

test("a toast says its piece and then goes away", async ({ page }) => {
  await signIn(page);
  await page.goto("/SOL-1");
  await page.getByRole("button", { name: "Copy link" }).click();
  await expect(page.locator(".toast")).toHaveText("Link copied");
  // It clears itself: nothing here dismisses it.
  await expect(page.locator(".toast")).toHaveCount(0, { timeout: 10000 });
});

test("signing out returns to the sign-in card", async ({ page }) => {
  await signIn(page);
  await page.goto("/account");
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
});

test("the previewed card joins the selection a modifier-click starts", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
  const form = page.getByRole("dialog");
  await form.getByLabel("Title").fill("Service the flue");
  await form.getByLabel("Mark as accepted").check(); // so it lands on the board
  await form.getByRole("button", { name: "Create" }).click();

  const cards = page.locator(".card");
  await expect(cards).toHaveCount(2);
  await cards.first().click();
  await expect(page.locator(".detail-aside")).toBeVisible();

  // The card being previewed is in the selection the modifier-click starts,
  // rather than needing a second click of its own.
  await cards.nth(1).click({ modifiers: ["Control"] });
  await expect(page.locator(".selection-bar .count")).toContainText("2");
});

test("a profile picture replaces the initials wherever the name appears", async ({ page }) => {
  await signIn(page);
  await page.goto("/account");

  // A 2x2 PNG, scaled and re-encoded by the client before it is stored.
  await page.getByLabel("Picture").setInputFiles({
    name: "me.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFUlEQVR4nGP8z8Dwn4GBgYkBBv" +
      "5DGAAmYgML8Ck1FQAAAABJRU5ErkJggg==", "base64"),
  });

  const picture = page.locator("img.avatar").first();
  await expect(picture).toBeVisible();
  await expect(picture).toHaveAttribute("src", /^data:image\//);

  // Removing it puts the initials back.
  await page.getByRole("button", { name: "Remove" }).click();
  await expect(page.locator("img.avatar")).toHaveCount(0);
  await expect(page.locator(".whoami .avatar")).toHaveText("RO");
});
