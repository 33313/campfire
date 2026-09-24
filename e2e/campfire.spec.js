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
  await expect(page.locator(".team-switch")).toContainText("Solstice");
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
  await expect(preview.getByLabel("Title")).toHaveValue("Replace the boiler");
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

  // One box, inline with the chips it makes, always there.
  const box = page.getByLabel("Labels");
  await box.fill("heating");
  await box.press("Enter");
  await expect(page.locator(".label-chip", { hasText: "heating" })).toBeVisible();
  await expect(box).toBeVisible(); // it does not come and go with the chips

  // SPEC.md §8.1: a card never carries a label.
  await page.goto("/t/1/board");
  await expect(page.locator(".card")).not.toContainText("heating");
});

test("a label that exists is offered before it is spelled a second way", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");

  // The create dialog adds labels through the ticket page's editor, not a
  // second one of its own.
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
  const form = page.getByRole("dialog");
  await expect(form.locator(".label-editor")).toBeVisible();

  const box = form.getByLabel("Labels");
  await box.fill("he");

  // The typed part is shown as typed, so it is clear how much of the offered
  // label is already there.
  const suggestion = form.locator(".label-suggestion").first();
  await expect(suggestion).toHaveText("heating");
  await expect(suggestion.locator("b")).toHaveText("he");

  await box.press("Enter");
  await expect(form.locator(".label-chip", { hasText: "heating" })).toBeVisible();

  // A label already picked is not offered back.
  await box.fill("he");
  await expect(form.locator(".label-suggestion")).toHaveCount(0);

  await form.getByRole("button", { name: "Cancel" }).click();
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
  await page.getByRole("button", { name: "Change bucket" }).click();
  await page.getByRole("dialog").getByRole("button", { name: /SOL-2\s+Winter readiness/ }).click();
  await expect(page.locator(".aside-control").last()).toContainText("Winter readiness");

  // The bucket on a ticket follows through to the bucket.
  await page.locator(".aside-control").last().click();
  await expect(page).toHaveURL(/\/SOL-2$/);

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

test("the log says what changed in words, and nothing where there was nothing", async ({ page }) => {
  await signIn(page);

  // A description change records that it changed and not the prose itself
  // (SPEC.md §10) — which used to reach the screen as "item.updated".
  await page.goto("/SOL-1");
  await page.getByRole("banner").getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Description").fill("Rewritten, so the log has something to say.");
  await page.getByRole("banner").getByRole("button", { name: "Save" }).click();

  await page.goto("/log");
  const change = (field) => page.locator(".table tbody tr")
    .filter({ has: page.locator("td.field-label", { hasText: field }) })
    .first().locator("td").nth(5);

  await expect(change("description")).toHaveText("description updated");

  // A row with no previous value starts with the new one: an em dash standing
  // for "there was nothing before" is on every creation and says nothing.
  await expect(change("created")).not.toContainText("—");
  await expect(change("state")).toContainText("→");
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

test("search suggests as you type, without a request per keystroke", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");

  let asked = 0;
  await page.route("**/api/items?*", (route) => { asked++; return route.continue(); });
  await page.waitForTimeout(300); // let the screen finish loading before counting
  asked = 0;

  const box = page.getByRole("banner").getByPlaceholder("Title and description");
  await box.pressSequentially("boiler", { delay: 25 });

  const hit = page.locator(".search-suggestion").first();
  await expect(hit).toContainText("SOL-1");
  // Six keystrokes is not six requests: one per pause in the typing, and
  // nothing at all under two characters.
  expect(asked).toBeLessThan(3);

  // A suggestion is a way through to the item, not a second filter.
  await hit.click();
  await expect(page).toHaveURL(/\/SOL-1$/);
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

test("a card dropped from the selection stops looking selected", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");

  const cards = page.locator(".card");
  await cards.first().click(); // previews it, and anchors the selection
  await cards.nth(1).click({ modifiers: ["Control"] });
  await cards.first().click({ modifiers: ["Control"] }); // and out again

  await expect(page.locator(".selection-bar .count")).toContainText("1");
  // The first card is still the one in the preview, which is a different fact
  // and a different mark: it must not go on wearing the selection's border.
  await expect(cards.first()).toHaveClass(/focused/);
  await expect(cards.first()).not.toHaveClass(/selected/);
  const border = (c) => c.evaluate((el) => getComputedStyle(el).borderTopColor);
  expect(await border(cards.first())).not.toBe(await border(cards.nth(1)));
});

test("a drag carries the whole selection, not just the card under the pointer", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");

  const todo = page.locator(".column", { has: page.getByRole("heading", { name: "To Do" }) });
  const progress = page.locator(".column", { has: page.getByRole("heading", { name: "In Progress" }) });
  const cards = todo.locator(".card");
  await expect(cards.first()).toBeVisible(); // the board is drawn before it is counted
  const before = await progress.locator(".card").count();
  const start = await cards.count();
  for (const title of ["Sweep the chimney", "Order the kindling"]) {
    await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
    const form = page.getByRole("dialog");
    await form.getByLabel("Title").fill(title);
    await form.getByLabel("Mark as accepted").check();
    await form.getByRole("button", { name: "Create" }).click();
  }
  await expect(cards).toHaveCount(start + 2);

  await cards.first().click({ modifiers: ["Control"] });
  await cards.nth(1).click({ modifiers: ["Control"] });
  await expect(page.locator(".selection-bar .count")).toContainText("2");

  await cards.first().dragTo(progress.locator(".column-body"));
  await expect(progress.locator(".card")).toHaveCount(before + 2);
});

test("right-clicking a card offers what the selection bar offers, and more", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");

  const card = page.locator(".card").first();
  await expect(card).toBeVisible();
  await card.click({ button: "right" });

  const menu = page.locator(".card-menu");
  await expect(menu).toBeVisible();
  // One card: the bulk actions plus the two that only mean anything alone.
  for (const name of ["Move to…", "Reject…", "Assignee…", "Priority…", "Bucket…",
                      "Clone", "Split…", "Open"]) {
    await expect(menu.getByRole("button", { name, exact: true })).toBeVisible();
  }
  // It acts on what it names, so the card it was opened on is the selection.
  await expect(page.locator(".selection-bar .count")).toContainText("1");

  // Move to… writes through the same bulk endpoint the bar uses.
  await menu.getByRole("button", { name: "Move to…" }).click();
  await expect(menu).toHaveCount(0); // the menu goes before the dialog it opened
  const dialog = page.getByRole("dialog");
  await dialog.locator(".option-row", { hasText: "In Progress" }).click();
  await dialog.getByRole("button", { name: "Move" }).click();

  const progress = page.locator(".column", { has: page.getByRole("heading", { name: "In Progress" }) });
  await expect(progress.locator(".card").first()).toBeVisible();
});

test("the menu opens at the pointer on a panel wide enough to zoom", async ({ page }) => {
  // A client coordinate written straight back into a style inside #app is
  // multiplied by the zoom, which opened the menu a quarter of a screen down
  // and to the right of the click.
  await page.setViewportSize({ width: 2560, height: 1440 });
  await signIn(page);
  await page.goto("/t/1/board");
  await expect(page.locator("#app")).toHaveCSS("zoom", "1.25");

  const card = page.locator(".card").first();
  await expect(card).toBeVisible();
  const box = await card.boundingBox();
  const at = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
  await page.mouse.click(at.x, at.y, { button: "right" });

  const menu = await page.locator(".card-menu").boundingBox();
  expect(Math.abs(menu.x - at.x)).toBeLessThan(2);
  expect(Math.abs(menu.y - at.y)).toBeLessThan(2);
});

test("a profile picture replaces the initials wherever the name appears", async ({ page }) => {
  await signIn(page);
  // The identity block at the foot of the sidebar goes to the profile, and the
  // profile is where the picture is changed.
  await page.locator(".whoami").click();
  await expect(page).toHaveURL(/\/users\/root$/);
  await page.getByRole("button", { name: "Edit profile" }).click();

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
  await page.getByRole("button", { name: "Remove picture" }).click();
  await expect(page.locator("img.avatar")).toHaveCount(0);
  await expect(page.locator(".whoami .avatar")).toHaveText("RO");
});

test("a 512px picture still fits inside what the server will store", async ({ page }) => {
  await signIn(page);
  await page.goto("/account");

  // The client's own encode, on noise — the worst case, compressing no better
  // than a photograph does — in the browser that does the scaling. What the
  // client sends must fit what core.maxAvatar stores, at any picture.
  const encoded = await page.evaluate(() => {
    const canvas = Object.assign(document.createElement("canvas"), { width: 512, height: 512 });
    const ctx = canvas.getContext("2d");
    const pixels = ctx.createImageData(512, 512);
    for (let i = 0; i < pixels.data.length; i++) {
      pixels.data[i] = i % 4 === 3 ? 255 : Math.floor(Math.random() * 256);
    }
    ctx.putImageData(pixels, 0, 0);
    const data = canvas.toDataURL("image/webp", 0.85);
    return (data.length <= 256 * 1024 ? data : canvas.toDataURL("image/webp", 0.6)).length;
  });
  expect(encoded).toBeLessThan(256 * 1024);
});

test("the detail page edits behind Edit; the preview edits in place", async ({ page }) => {
  await signIn(page);
  await page.goto("/SOL-1");

  // The fields are in the one column: no second sidebar beside the board's.
  await expect(page.locator(".detail-aside")).toHaveCount(0);
  await expect(page.locator(".fields")).toBeVisible();

  // Cancel throws the draft away.
  const header = page.getByRole("banner");
  await header.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Title").fill("nonsense");
  await header.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("heading", { name: "Replace the boiler", exact: true })).toBeVisible();

  // Save writes it.
  await header.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Title").fill("Replace the boiler and its flue");
  await header.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { name: "Replace the boiler and its flue" })).toBeVisible();

  // In the preview the title is the text it shows, typed into where it is
  // drawn. The floppy and the cross appear only once it has been changed.
  await page.goto("/t/1/board");
  await page.locator(".card", { hasText: "Replace the boiler and its flue" }).click();
  const preview = page.locator(".detail-aside");
  const save = preview.getByRole("button", { name: "Save title" });
  await expect(save).toHaveCount(0);

  await preview.getByLabel("Title").fill("nonsense");
  await preview.getByRole("button", { name: "Discard title" }).click();
  await expect(preview.getByLabel("Title")).toHaveValue("Replace the boiler and its flue");

  await preview.getByLabel("Title").fill("Replace the boiler");
  await save.click();
  await expect(save).toHaveCount(0);
  await page.goto("/SOL-1");
  await expect(page.getByRole("heading", { name: "Replace the boiler", exact: true })).toBeVisible();
});

test("a description is read back with the line breaks it was typed with", async ({ page }) => {
  await signIn(page);
  await page.goto("/SOL-1");

  const header = page.getByRole("banner");
  await header.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Description").fill("It is older than the building.\nThe flue is worse.");
  await header.getByRole("button", { name: "Save" }).click();

  // The break is drawn, not merely stored: two typed lines occupy two lines.
  const prose = page.locator(".prose");
  await expect(prose).toContainText("The flue is worse.");
  const lines = await prose.evaluate((el) =>
    el.getBoundingClientRect().height / parseFloat(getComputedStyle(el).lineHeight));
  expect(lines).toBeGreaterThan(1.5);
});

test("a comment shows its author's picture, not just their initials", async ({ page }) => {
  await signIn(page);
  await page.goto("/users/root");
  await page.getByRole("button", { name: "Edit profile" }).click();
  await page.getByLabel("Picture").setInputFiles({
    name: "me.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFUlEQVR4nGP8z8Dwn4GBgYkBBv" +
      "5DGAAmYgML8Ck1FQAAAABJRU5ErkJggg==", "base64"),
  });
  await expect(page.locator("img.profile-picture")).toBeVisible(); // stored before we leave

  await page.goto("/SOL-1");
  const picture = page.locator(".comment img").first();
  await expect(picture).toBeVisible();

  // The size class must not also match the block the picture sits in. As
  // "comment" it did, and the block's padding left the image no content box to
  // be drawn in — a full-height border box with nothing inside it.
  const drawn = await picture.evaluate((el) => {
    const s = getComputedStyle(el);
    return el.getBoundingClientRect().height
      - parseFloat(s.paddingTop) - parseFloat(s.paddingBottom)
      - parseFloat(s.borderTopWidth) - parseFloat(s.borderBottomWidth);
  });
  expect(drawn).toBeGreaterThan(20);

  await page.goto("/users/root");
  await page.getByRole("button", { name: "Edit profile" }).click();
  await page.getByRole("button", { name: "Remove picture" }).click();
});

test("the account page is a column at a measure, not a stretched page", async ({ page }) => {
  await signIn(page);
  await page.setViewportSize({ width: 1600, height: 900 });
  await page.goto("/account");

  const box = await page.locator(".body").evaluate((el) => {
    const own = el.getBoundingClientRect();
    const room = el.parentElement.getBoundingClientRect();
    return { width: own.width, left: own.left - room.left, right: room.right - own.right };
  });
  expect(box.width).toBeLessThan(900);
  // Centred in whatever space is left, rather than pinned to one side.
  expect(Math.abs(box.left - box.right)).toBeLessThan(2);
});

test("switching team is a drop-down, and the name follows the switch", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");

  // A drop-down under the name it changes, not a modal over the screen.
  await page.locator(".team-switch").click();
  await expect(page.locator(".team-list")).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  // Escape puts it away again, as a drop-down does.
  await page.keyboard.press("Escape");
  await expect(page.locator(".team-list")).toBeHidden();

  await page.locator(".team-switch").click();
  await page.locator(".team-option", { hasText: "New team" }).click();
  const form = page.getByRole("dialog");
  await form.getByLabel("Name").fill("Terra Nova");
  await form.getByLabel("Short code").fill("TERRA");
  await form.getByRole("button", { name: "Create" }).click();
  await expect(page.locator(".team-switch")).toContainText("Terra Nova");

  // The switch changed the board and left the selector naming the team it
  // switched away from. It names the team you are on.
  await page.locator(".team-switch").click();
  await page.locator(".team-option", { hasText: "Solstice" }).click();
  await expect(page).toHaveURL(/\/t\/1\/board$/);
  await expect(page.locator(".team-switch")).toContainText("Solstice");
});

test("a bucket's progress separates what is done from what is under way", async ({ page }) => {
  await signIn(page);
  await page.goto("/SOL-2");

  // SOL-1 is in this bucket, in an intermediate state: not in the backlog, not
  // finished. Open counts what is left; accepted counts what someone holds.
  const block = page.locator(".progress-block");
  await expect(block).toContainText("1 open, 1 accepted");

  const widths = () => page.locator(".progress > i").evaluateAll(
    (els) => els.map((e) => e.getBoundingClientRect().width));
  const [done, underway] = await widths();
  expect(done).toBe(0);
  expect(underway).toBeGreaterThan(0);

  // Finishing it moves the width from the one bar to the other.
  await page.goto("/SOL-1");
  await page.locator(".aside-control").first().click();
  const move = page.getByRole("dialog");
  await move.getByRole("button", { name: "Done" }).click();
  await move.getByRole("button", { name: "Move", exact: true }).click();

  await page.goto("/SOL-2");
  await expect(block).toContainText("0 open, 0 accepted");
  const [doneNow, underwayNow] = await widths();
  expect(doneNow).toBeGreaterThan(0);
  expect(underwayNow).toBe(0);
});

test("the preview carries the comments and the history, not just their counts", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await page.locator(".card", { hasText: "Replace the boiler" }).click();

  const preview = page.locator(".detail-aside");
  await expect(preview.locator(".comment-body")).toContainText("The part is on order.");

  // A comment is written from here, as every other field is edited from here.
  await preview.getByPlaceholder("Write a comment…").fill("Fitted this morning.");
  await preview.getByRole("button", { name: "Comment", exact: true }).click();
  await expect(preview.locator(".comment-body").last()).toHaveText("Fitted this morning.");

  // The history reads through the same helpers the log and the detail view
  // read through, so an assignee is a name and not the id it is stored as.
  await preview.getByRole("button", { name: /^History/ }).click();
  await expect(preview.locator(".aside-event").first()).toBeVisible();
  await expect(preview.locator(".aside-activity")).toContainText("state");
});

test("a profile is a page at a name, and it is edited where it is read", async ({ page }) => {
  await signIn(page);
  await page.goto("/users/root");

  await page.getByRole("button", { name: "Edit profile" }).click();
  await page.getByLabel("Email").fill("root@example.invalid");
  await page.getByLabel("Bio").fill("Keeps the boiler running.\nIn the workshop, most mornings.");
  await page.getByRole("banner").getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.locator(".toast")).toHaveText(/Saved/);
  await expect(page).toHaveURL(/\/users\/root$/);

  // Settings is its own page, reached from the profile and not the other way
  // about: the sidebar's identity block is the profile.
  await page.getByRole("banner").getByRole("link", { name: "Settings" }).click();
  await expect(page).toHaveURL(/\/account$/);
  await expect(page.getByRole("heading", { name: "Password" })).toBeVisible();
  await page.getByRole("banner").getByRole("link", { name: "Your profile" }).click();
  await expect(page).toHaveURL(/\/users\/root$/);

  // The picture at a size worth looking at, the name, the email, last seen,
  // the teams and the trail.
  await expect(page.locator(".profile-facts h1")).toHaveText("root");
  await expect(page.locator(".profile-email")).toHaveText("root@example.invalid");
  await expect(page.locator(".profile-head")).toContainText("Last seen");
  await expect(page.locator(".profile-bio")).toContainText("In the workshop, most mornings.");
  await expect(page.locator(".code-badge", { hasText: "SOL" }).last()).toBeVisible();
  await expect(page.locator(".relation-row").last()).toContainText(/\d{4}/);

  // The bio keeps the line breaks it was typed with, as a description does.
  const lines = await page.locator(".profile-bio").evaluate((el) =>
    el.getBoundingClientRect().height / parseFloat(getComputedStyle(el).lineHeight));
  expect(lines).toBeGreaterThan(1.5);

  // The page fits: only the activity panel scrolls, so the picture, the bio
  // and the teams stay on screen however long the trail gets.
  const scrolls = (sel) => page.locator(sel).evaluate((el) => el.scrollHeight > el.clientHeight + 1);
  expect(await scrolls(".profile-body")).toBe(false);
  await expect(page.locator(".profile-activity-rows")).toBeVisible();

  // A comment author is a way through to their profile.
  await page.goto("/SOL-1");
  await page.locator(".comment-meta a").first().click();
  await expect(page).toHaveURL(/\/users\/root$/);
});

test("a team has a profile: its picture, what it does, who is in it", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await page.locator(".sidebar-foot").getByRole("link", { name: "Team", exact: true }).click();
  await expect(page).toHaveURL(/\/t\/1\/profile$/);

  // Without a picture the short code stands in for one.
  await expect(page.locator("span.profile-picture")).toHaveText("SOL");

  await page.getByRole("button", { name: "Edit profile" }).click();
  await page.getByLabel("Picture").setInputFiles({
    name: "team.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFUlEQVR4nGP8z8Dwn4GBgYkBBv" +
      "5DGAAmYgML8Ck1FQAAAABJRU5ErkJggg==", "base64"),
  });
  await expect(page.locator("img.profile-picture")).toHaveAttribute("src", /^data:image\//);
  await page.getByLabel("Description").fill("Keeps the building warm.");
  await page.getByRole("banner").getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.locator(".toast")).toHaveText(/Saved/);

  await page.reload();
  await expect(page.locator(".profile-bio")).toHaveText("Keeps the building warm.");
  await expect(page.locator("img.profile-picture")).toBeVisible();

  // The team picker shows it in place of the short code; a team without one
  // gets a blank square.
  await page.request.post("/api/teams", { data: { name: "Umbra", shortCode: "UMB" } });
  await page.reload();
  await page.locator(".team-switch").click();
  const option = (code) => page.locator(".team-option", { hasText: code }).locator(".team-avatar");
  await expect(page.locator(".team-list .code-badge")).toHaveCount(0);
  await expect(option("Solstice")).toHaveAttribute("src", /^data:image\//);
  await expect(option("Umbra")).toBeEmpty();
  expect(await option("Umbra").evaluate((el) => el.tagName)).toBe("SPAN");
  await page.locator(".team-switch").click();
  // ...and so does the switch, for the team it shows.
  await expect(page.locator(".team-switch .team-avatar")).toHaveAttribute("src", /^data:image\//);
  await expect(page.locator(".team-switch .code-badge")).toHaveCount(0);

  // The members, each a way through to their own profile, and the trail.
  await page.locator(".profile-teams").getByRole("link", { name: "root" }).click();
  await expect(page).toHaveURL(/\/users\/root$/);
  // ...and a person's teams lead back.
  await page.locator(".profile-teams a[href='/t/1/profile']").click();
  await expect(page).toHaveURL(/\/t\/1\/profile$/);
  await expect(page.locator(".profile-activity-rows .relation-row").first()).toContainText("root");
});

test("a long bucket name makes the card taller, not wider", async ({ page }) => {
  await signIn(page);
  const bucket = await (await page.request.post("/api/items", { data: {
    kind: "bucket", teamId: 1, title: "Campfire - Post-release improvements, and then some more",
  } })).json();
  await page.request.post("/api/items", { data: {
    kind: "ticket", teamId: 1, title: "Short", bucketId: bucket.id, accepted: true,
  } });

  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/t/1/board");
  const card = page.locator(".card", { hasText: "Short" });
  const chip = card.locator(".card-bucket");
  await expect(chip).toBeVisible();

  // Every column keeps the width it shares with the others.
  const widths = await page.locator(".column").evaluateAll((cols) =>
    cols.map((c) => Math.round(c.getBoundingClientRect().width)));
  expect(new Set(widths).size, `column widths ${widths}`).toBe(1);

  // The name wraps inside the card, as a title does, rather than being cut.
  const [c, b] = [await card.boundingBox(), await chip.boundingBox()];
  expect(b.x + b.width).toBeLessThanOrEqual(c.x + c.width);
  expect(b.height).toBeGreaterThan(20);
  expect(await chip.locator(".name").evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true);
});

test("starting a selection moves nothing on the screen", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const card = page.locator(".card").first();
  const filter = page.locator(".filter-bar");
  const top = async () => (await card.boundingBox()).y;
  const before = await top();

  await card.click({ modifiers: ["Control"] });
  const bar = page.locator(".selection-bar");
  await expect(bar).toBeVisible();
  // The bar is where the filter bar was, and the filters behind it are out of
  // reach until the selection ends.
  expect(await bar.boundingBox()).toEqual(await filter.boundingBox());
  await expect(page.locator(".toolbar > [inert]")).toHaveCount(1);
  expect(await top()).toBe(before);

  await bar.getByRole("button", { name: "Clear" }).click();
  await expect(bar).toHaveCount(0);
  await expect(page.locator(".toolbar > [inert]")).toHaveCount(0);
  expect(await top()).toBe(before);
});
