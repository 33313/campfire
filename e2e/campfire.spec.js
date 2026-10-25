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
  await form.getByRole("button", { name: "Add a label" }).click();
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

  // A quiet "+" inline with the chips opens the box already focused (DEV-100).
  const plus = page.getByRole("button", { name: "Add a label" });
  const box = page.getByLabel("Labels");
  await expect(box).toHaveCount(0);
  await plus.click();
  await expect(box).toBeFocused();
  await page.keyboard.type("heating");
  await box.press("Enter");
  await expect(page.locator(".label-chip", { hasText: "heating" })).toBeVisible();
  await expect(box).toBeFocused(); // it stays up for the next one
  // Escape puts the "+" back, focused.
  await box.press("Escape");
  await expect(box).toHaveCount(0);
  await expect(plus).toBeFocused();

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

  await form.getByRole("button", { name: "Add a label" }).click();
  const box = form.getByLabel("Labels");
  await box.fill("he");

  await expect(form.locator(".label-suggestion").first()).toHaveText("heating");

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
  // The icon and the colour hang from one field, and the panel stays up
  // between the two; Escape closes it and leaves the form.
  await bucket.getByRole("button", { name: /bookmark/ }).click();
  const looks = page.getByRole("dialog", { name: "Icon and colour" });
  await looks.getByRole("button", { name: "rocket", exact: true }).click();
  await looks.getByRole("button", { name: "rust", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(looks).toBeHidden();
  await expect(bucket.getByRole("button", { name: /rocket.*rust/ })).toBeVisible();
  await bucket.getByRole("button", { name: "Create" }).click();

  await expect(page).toHaveURL(/\/buckets$/);
  await expect(page.locator(".bucket-card", { hasText: "Winter readiness" })).toBeVisible();

  await page.goto("/SOL-2");
  await expect(page.locator(".state-badge", { hasText: "bucket" })).toBeVisible();

  // Put the ticket in it from the ticket's own aside.
  await page.goto("/SOL-1");
  await page.getByRole("button", { name: "Change bucket" }).click();
  // The buckets are offered at once, and typing narrows them (DEV-92).
  const picker = page.getByRole("dialog").getByRole("combobox", { name: "Bucket" });
  await expect(page.getByRole("listbox").getByRole("option", { name: /Winter readiness/ })).toBeVisible();
  await picker.fill("wint");
  await expect(page.getByRole("listbox").getByRole("option")).toHaveCount(1);
  await picker.press("Enter");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.locator(".aside-control").last()).toContainText("Winter readiness");

  // The bucket on a ticket follows through to the bucket.
  await page.locator(".aside-control").last().click();
  await expect(page).toHaveURL(/\/SOL-2$/);

  // The bucket rides on its tickets' cards.
  await page.goto("/t/1/board");
  await expect(page.locator(".card .card-bucket").first()).toContainText("Winter readiness");

  await page.goto("/SOL-2");
  await expect(page.getByRole("link", { name: "SOL-1" })).toBeVisible();
  await expect(page.locator(".progress-block .big")).toHaveText("0");
});

// DEV-92: the create form's bucket is a box to type in, with the buckets under
// it from the first click.
test("the create form's bucket is picked by typing, or from the list under it", async ({ page }) => {
  await signIn(page);
  const bucket = await (await page.request.post("/api/items", { data: {
    kind: "bucket", teamId: 1, title: "Kindling store",
  } })).json();
  await page.goto("/t/1/board");
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
  const form = page.getByRole("dialog");
  await form.getByLabel("Title").fill("Check the stove pipe");
  const picker = form.getByRole("combobox", { name: "Bucket" });
  await picker.click();
  const offers = page.getByRole("listbox");
  const options = offers.getByRole("option");
  await expect(options.first()).toHaveText("No bucket");
  await expect(offers.getByRole("option", { name: /Kindling store/ })).toBeVisible();

  // Typing narrows by identifier or title; nothing matching says so.
  await picker.fill("no such bucket");
  await expect(options).toHaveCount(0);
  await expect(offers).toContainText("No bucket matches.");
  // Escape puts the box back and leaves the form up.
  await picker.press("Escape");
  await expect(offers).toHaveCount(0);
  await expect(form).toBeVisible();
  await expect(picker).toHaveValue("");

  await picker.fill(bucket.identifier.toLowerCase());
  await picker.press("Enter");
  await expect(picker).toHaveValue(`${bucket.identifier}  Kindling store`);
  await form.getByRole("button", { name: "Create" }).click();
  await expect(form).toHaveCount(0);
  await page.goto("/t/1/backlog");
  await expect(page.locator("tbody tr", { hasText: "Check the stove pipe" })).toContainText("Kindling store");
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

// The filter chips: a click cycles a value [ ] → [+] → [−]; the dropdown
// applies when it closes and Escape leaves it as it was (SPEC.md §9.2).
test("the filter chips take [+] and [−] values, days and more fields, and clear", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const bar = page.locator(".filter-bar");
  const cards = page.locator(".card");
  const pop = page.locator(".filter-pop");
  await expect(cards).toHaveCount(1);

  // [+] P1: only P1, and the P3 boiler goes.
  await bar.getByRole("button", { name: "Priority" }).click();
  await pop.getByRole("option", { name: /P1 · Highest/ }).click();
  await expect(pop.getByRole("option", { name: /P1 · Highest/ })).toHaveText(/\[\+\]/);
  await page.keyboard.press("Enter");
  await expect(pop).toHaveCount(0);
  await expect(cards).toHaveCount(0);
  await expect(bar.getByRole("button", { name: "Priority: P1", exact: true })).toBeVisible();

  // Again: [−] P1, everything but P1.
  await bar.getByRole("button", { name: "Priority: P1", exact: true }).click();
  await pop.getByRole("option", { name: /P1 · Highest/ }).click();
  await page.keyboard.press("Enter");
  await expect(cards).toHaveCount(1);
  await expect(bar.getByRole("button", { name: "Priority: −P1", exact: true })).toBeVisible();

  // Escape undoes what the dropdown did.
  await bar.getByRole("button", { name: "Priority: −P1", exact: true }).click();
  await pop.getByRole("option", { name: /P3 · Medium/ }).click();
  await page.keyboard.press("Escape");
  await expect(pop).toHaveCount(0);
  await expect(bar.getByRole("button", { name: "Priority: −P1", exact: true })).toBeVisible();
  await expect(cards).toHaveCount(1);

  // A field from + More, and a day preset.
  await bar.getByRole("button", { name: "+ More" }).click();
  await page.locator(".pop").getByRole("menuitem", { name: "Created" }).click();
  await bar.getByRole("button", { name: "Created" }).click();
  await page.locator(".day-pop").getByRole("button", { name: "Last 7 days" }).click();
  await expect(bar.getByRole("button", { name: "Created: Last 7 days" })).toBeVisible();
  await expect(cards).toHaveCount(1);

  // Clear takes it all off.
  await bar.getByRole("button", { name: "Clear", exact: true }).click();
  await expect(bar.getByRole("button", { name: "Priority", exact: true })).toBeVisible();
  await expect(bar.getByRole("button", { name: "Created" })).toHaveCount(0);
});

test("raw CSQL offers what comes next, says what is wrong, and never applies a refused query", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const bar = page.locator(".filter-bar");
  const cards = page.locator(".card");

  // The chips' filter is the raw query's starting point.
  await bar.getByRole("button", { name: "Priority" }).click();
  await page.locator(".filter-pop").getByRole("option", { name: /P3 · Medium/ }).click();
  await page.keyboard.press("Enter");
  await bar.getByRole("button", { name: "</>" }).click();
  const query = bar.getByLabel("Query");
  await expect(query).toHaveValue("priority = 3");

  const toggle = bar.getByRole("button", { name: "</>" });

  // or, which the chips cannot say: applied on Enter, and the chips decline it,
  // offering to clear it rather than leaving you stuck.
  await query.fill("priority = 1 or label = plumbing");
  await query.press("Enter");
  await expect(cards).toHaveCount(1);
  await toggle.click();
  await expect(bar.getByRole("alert")).toHaveText("The chips can't show this query.");
  await expect(bar.getByRole("button", { name: "Clear filter" })).toBeVisible();
  await expect(query).toBeVisible();

  // A syntax error is the browser's to say, at once and where it is.
  await query.fill("priority = 9 and label = plumbing");
  await expect(query).toHaveAttribute("aria-invalid", "true");
  const offers = page.locator(".query-offers");
  await expect(offers.locator(".query-error")).toHaveText("priority is 1 to 4, or P1 to P4");
  // A name is the server's: refused, and the board stays as it was.
  await query.fill("label = plumbing and assignee = nobody-yet");
  await query.press("Enter");
  await expect(offers.locator(".query-error")).toHaveText(/no user is called "nobody-yet"/);
  // The board sends it bracketed beside its own clauses; the mark is still on the name.
  await expect(page.locator(".query-mirror mark")).toHaveText("nobody-yet");
  await expect(cards).toHaveCount(1);
  // Nothing has to be fixed: Discard puts back the query in use.
  await bar.getByRole("button", { name: "Discard" }).click();
  await expect(query).toHaveValue("priority = 1 or label = plumbing");
  await expect(query).toHaveAttribute("aria-invalid", "false");
  await expect(bar.getByRole("button", { name: "Discard" })).toHaveCount(0);

  // What can come next is offered: a field, its operators, its values.
  await query.fill("");
  await query.pressSequentially("pri");
  await expect(offers.getByRole("option", { name: /^priority/ })).toBeVisible();
  await query.press("Tab");
  await expect(query).toHaveValue("priority ");
  await expect(offers.getByRole("option", { name: /^<=/ })).toBeVisible();
  await query.press("Tab");
  await expect(query).toHaveValue("priority = ");
  await expect(offers.getByRole("option", { name: /^1 Highest$/ })).toBeVisible();
  await query.pressSequentially("3");
  await query.press("Tab");
  await expect(query).toHaveValue("priority = 3 ");
  await expect(offers.getByRole("option", { name: /^and/ })).toBeVisible();
  await query.press("Enter");
  await expect(cards).toHaveCount(1);

  // A value is found by any part of it, not only its start.
  await query.fill("");
  await query.pressSequentially("state = prog");
  await expect(offers.getByRole("option", { name: /^"In Progress"/ })).toBeVisible();
  await query.press("Tab");
  await expect(query).toHaveValue('state = "In Progress" ');
  await query.fill("priority = 3");

  // Back to the chips, which can show it.
  await toggle.click();
  await expect(bar.getByRole("button", { name: "Priority: P3", exact: true })).toBeVisible();
  await bar.getByRole("button", { name: "Clear", exact: true }).click();
});

// SPEC.md §9.2: typing costs no requests, each applied change exactly one,
// and a refused draft never rides along with the next request.
test("filtering costs one request per change and none per keystroke", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await expect(page.locator(".card")).toHaveCount(1);
  const bar = page.locator(".filter-bar");
  const sidebar = page.locator(".sidebar");
  let asked = [];
  page.on("request", (r) => {
    const u = new URL(r.url());
    if (u.pathname.startsWith("/api/") && u.pathname !== "/api/stream") asked.push(u.pathname + u.search);
  });
  // Our own writes come back on the stream, and refresh the board 300–700ms
  // later (SPEC.md §12.4): wait those out, so only what an action costs counts.
  const settle = async () => { await page.waitForTimeout(900); await page.waitForLoadState("networkidle"); };

  // A view to switch.
  await sidebar.getByRole("button", { name: "New view" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill("P1 only");
  await dialog.getByRole("button", { name: "</>" }).click();
  await dialog.getByLabel("Query").fill("priority = 1");
  await dialog.getByRole("button", { name: "Save" }).click();
  const p1 = sidebar.getByRole("button", { name: "P1 only", exact: true });
  await expect(p1).toHaveAttribute("aria-pressed", "true");
  await expect(page.locator(".card")).toHaveCount(0);
  await settle();

  // Typing: nothing.
  await bar.getByRole("button", { name: "</>" }).click();
  const query = bar.getByLabel("Query");
  asked = [];
  await query.pressSequentially("label in (plumbing, heating) and priority != 4", { delay: 20 });
  await page.waitForTimeout(600);
  expect(asked).toEqual([]);

  // Enter: one.
  await query.press("Enter");
  await settle();
  expect(asked.filter((u) => u.includes("/board"))).toHaveLength(1);
  expect(asked).toHaveLength(1);

  // A refused draft, then a view switched off: the board changes anyway, in
  // one request, and the refused draft is not in it.
  await query.fill("assignee = nobody-yet");
  await query.press("Enter");
  await expect(page.locator(".query-offers .query-error")).toBeVisible();
  await settle();
  asked = [];
  await p1.click();
  await expect(page.locator(".card")).toHaveCount(1);
  await settle();
  expect(asked).toHaveLength(1);
  expect(asked[0]).not.toContain("nobody-yet");

  // Leaving the refused draft for the chips is never blocked: it is offered
  // to Discard, which goes back to the chips with the filter in use.
  await bar.getByRole("button", { name: "</>" }).click();
  await expect(bar.getByRole("alert")).toHaveText("Not applied. Discard it to go back to the chips.");
  await bar.getByRole("button", { name: "Discard" }).click();
  await expect(bar.getByRole("button", { name: /^Label: plumbing, heating/ })).toBeVisible();
  await bar.getByRole("button", { name: "Clear", exact: true }).click();

  // A chip: opening it and picking twice is one request, on close.
  await settle();
  asked = [];
  await bar.getByRole("button", { name: "Priority" }).click();
  await page.locator(".filter-pop").getByRole("option", { name: /P1 · Highest/ }).click();
  await page.locator(".filter-pop").getByRole("option", { name: /P3 · Medium/ }).click();
  await page.keyboard.press("Enter");
  await expect(page.locator(".card")).toHaveCount(1);
  await settle();
  expect(asked).toHaveLength(1);
  await bar.getByRole("button", { name: "Clear", exact: true }).click();
});

// SPEC.md §9.2: a cached list goes stale on the events that change it and is
// fetched again when next needed.
test("a label made elsewhere is offered without a reload", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const bar = page.locator(".filter-bar");
  await bar.getByRole("button", { name: "Label" }).click();
  await expect(page.locator(".filter-pop").getByRole("option", { name: /ferns/ })).toHaveCount(0);
  await page.keyboard.press("Escape");

  // Someone else labels a ticket "ferns": the stream says so.
  await page.evaluate(async () => {
    await fetch("/api/items/1/labels", { method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "ferns" }) });
  });
  await expect.poll(async () => {
    await bar.getByRole("button", { name: "Label" }).click();
    const n = await page.locator(".filter-pop").getByRole("option", { name: /ferns/ }).count();
    await page.keyboard.press("Escape");
    return n;
  }).toBe(1);
  await page.evaluate(() => fetch("/api/items/1/labels/ferns", { method: "DELETE" }));
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

test("saved views are named queries, switched on in the sidebar and and-ed", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const bar = page.locator(".filter-bar");
  const sidebar = page.locator(".sidebar");
  const cards = page.locator(".card");

  // Saving the bar's filter moves it into a view that is on: the bar clears,
  // the sidebar lights the view, and Board stays the place you are.
  await bar.getByRole("button", { name: "Priority" }).click();
  await page.locator(".filter-pop").getByRole("option", { name: /P1 · Highest/ }).click();
  await page.keyboard.press("Enter");
  await expect(cards).toHaveCount(0);
  await bar.getByRole("button", { name: "Save as view" }).click();
  let dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "Priority: P1", exact: true })).toBeVisible();
  await dialog.getByLabel("Name").fill("Urgent");
  await dialog.getByRole("button", { name: "Save" }).click();
  const urgent = sidebar.getByRole("button", { name: "Urgent", exact: true });
  await expect(urgent).toHaveAttribute("aria-pressed", "true");
  await expect(bar.getByRole("button", { name: "Priority", exact: true })).toBeVisible();
  await expect(sidebar.getByRole("link", { name: "Board" })).toHaveClass(/\bon\b/);
  await expect(cards).toHaveCount(0);

  // A click switches it off, and on again.
  await urgent.click();
  await expect(urgent).toHaveAttribute("aria-pressed", "false");
  await expect(cards).toHaveCount(1);
  await urgent.click();
  await expect(cards).toHaveCount(0);

  // Views that are on are and-ed: P1 and plumbing is nothing; plumbing alone
  // is the boiler.
  await sidebar.getByRole("button", { name: "New view" }).click();
  await dialog.getByLabel("Name").fill("Plumbing");
  await dialog.getByRole("button", { name: "Label" }).click();
  await page.locator(".filter-pop").getByRole("option", { name: /plumbing/ }).click();
  await page.keyboard.press("Enter");
  await expect(dialog.getByRole("button", { name: "Label: plumbing", exact: true })).toBeVisible();
  await expect(cards).toHaveCount(0); // nothing runs until Save
  await dialog.getByRole("button", { name: "Save" }).click();
  const plumbing = sidebar.getByRole("button", { name: "Plumbing", exact: true });
  await expect(plumbing).toHaveAttribute("aria-pressed", "true");
  await expect(cards).toHaveCount(0);
  await urgent.click();
  await expect(cards).toHaveCount(1);

  // Which views are on outlasts a reload.
  await page.reload();
  await expect(plumbing).toHaveAttribute("aria-pressed", "true");
  await expect(urgent).toHaveAttribute("aria-pressed", "false");

  // A link from before views were switches still switches its view on.
  const id = await page.evaluate(async () =>
    (await (await fetch("/api/teams/1/views")).json()).find((v) => v.name === "Urgent").id);
  await page.goto(`/t/1/board?view=${id}`);
  await expect(page).toHaveURL(/board$/);
  await expect(urgent).toHaveAttribute("aria-pressed", "true");
  await expect(cards).toHaveCount(0);

  // Changing a view is a right-click on it, apart from using it. A query that
  // cannot be read keeps the dialog up and says why; Escape in the box closes
  // its list, not the dialog.
  await urgent.click({ button: "right" });
  await page.getByRole("menuitem", { name: "Edit…" }).click();
  await dialog.getByRole("button", { name: "</>" }).click();
  const q = dialog.getByLabel("Query");
  await expect(q).toHaveValue("priority = 1");
  await q.fill("priority = 9");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(q).toHaveAttribute("aria-invalid", "true");
  await q.press("Escape");
  await expect(dialog).toHaveCount(1);
  await q.fill("priority = 1 or priority = 3");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(cards).toHaveCount(1);

  // All off, then deleting one that is on switches it off with it.
  await sidebar.getByRole("button", { name: "All off" }).click();
  await expect(urgent).toHaveAttribute("aria-pressed", "false");
  await plumbing.click();
  await plumbing.click({ button: "right" });
  await page.getByRole("menuitem", { name: "Delete…" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Delete" }).click();
  await expect(plumbing).toHaveCount(0);
  await expect(cards).toHaveCount(1);
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

test("signing out from the sidebar returns to the sign-in card", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await page.locator(".sidebar").getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
});

// DEV-95: AGPL-3.0 §13 offers the source to everyone using it over the
// network, signed in or not: under the sign-in card, and at the end of the
// documentation.
test("the source is linked from the sign-in card and the documentation", async ({ page }) => {
  const source = "https://github.com/33313/campfire";
  await page.goto("/");
  await expect(page.getByRole("link", { name: "Source code" })).toHaveAttribute("href", source);
  await signIn(page);
  await page.goto("/docs#source-code");
  await expect(page.locator(".docs h2#source-code ~ p a").first()).toHaveAttribute("href", source);
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
  for (const name of ["Move to", "Reject…", "Assignee", "Priority", "Bucket",
                      "Clone", "Split…", "Open"]) {
    await expect(menu.getByRole("button", { name, exact: true })).toBeVisible();
  }
  // It acts on what it names, so the card it was opened on is the selection.
  await expect(page.locator(".selection-bar .count")).toContainText("1");

  // Move to opens its states beside it, and a state is applied as it is
  // clicked, through the same bulk endpoint the bar uses (DEV-81).
  await menu.getByRole("button", { name: "Move to" }).hover();
  await page.getByRole("menu", { name: "Move to" }).getByRole("menuitem", { name: "In Progress" }).click();
  await expect(menu).toHaveCount(0);

  const progress = page.locator(".column", { has: page.getByRole("heading", { name: "In Progress" }) });
  await expect(progress.locator(".card").first()).toBeVisible();
});

test("a profile picture replaces the initials wherever the name appears", async ({ page }) => {
  await signIn(page);
  // The identity block at the foot of the sidebar goes to the profile, and the
  // profile is where the picture is changed.
  await page.locator(".whoami .who-link").click();
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

  // Enter saves it.
  const long = "Replace the boiler and its flue, bleed every radiator, and lag the pipes in the loft";
  const title = preview.getByLabel("Title", { exact: true });
  await title.fill(long);
  await title.press("Enter");
  await expect(save).toHaveCount(0);
  await expect(title).toHaveValue(long);

  await preview.getByLabel("Title").fill("Replace the boiler");
  await save.click();
  await expect(save).toHaveCount(0);
  await page.goto("/SOL-1");
  await expect(page.getByRole("heading", { name: "Replace the boiler", exact: true })).toBeVisible();
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

  await page.goto("/SOL-1");
  await page.locator(".aside-control").first().click();
  const move = page.getByRole("dialog");
  await move.getByRole("button", { name: "Done" }).click();
  await move.getByRole("button", { name: "Move", exact: true }).click();

  await page.goto("/SOL-2");
  await expect(block).toContainText("0 open, 0 accepted");
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

test("a bucket made after the history was first read is named there, not numbered", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const card = page.locator(".card", { hasText: "Replace the boiler" });
  await card.click();
  const preview = page.locator(".detail-aside");
  await expect(preview).toBeVisible(); // the bucket list is read now

  await page.evaluate(async () => {
    const json = { "Content-Type": "application/json" };
    const icon = (await (await fetch("/api/instance")).json()).bucketIcons[0];
    const bucket = await (await fetch("/api/items", { method: "POST", headers: json,
      body: JSON.stringify({ kind: "bucket", teamId: 1, title: "Winter works", icon }) })).json();
    const q = encodeURIComponent('text ~ "Replace the boiler"');
    const [boiler] = (await (await fetch(`/api/items?csql=${q}`)).json()).items;
    await fetch(`/api/items/${boiler.id}`, { method: "PATCH", headers: json,
      body: JSON.stringify({ bucketId: bucket.id }) });
  });

  // Opened again, once the stream has said buckets changed.
  await expect(async () => {
    await preview.getByRole("button", { name: "Close" }).click();
    await card.click();
    await preview.getByRole("button", { name: /^History/ }).click();
    await expect(preview.locator(".aside-activity")).toContainText("Winter works", { timeout: 500 });
  }).toPass();
});

// Preferences are a convenience: with storage blocked, nothing is remembered
// and the board still works.
test("the board works where the browser stores nothing", async ({ page }) => {
  await page.addInitScript(() => Object.defineProperty(window, "localStorage", {
    get() { throw new DOMException("blocked", "SecurityError"); },
  }));
  await signIn(page);
  await page.goto("/t/1/board");
  await expect(page.locator(".column").first()).toBeVisible();
  const toggle = page.getByRole("button", { name: "Show buckets" });
  await toggle.click();
  await expect(toggle).toHaveClass(/on/);
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

  // What was saved reads back, with the teams and the trail.
  await expect(page.locator(".profile-facts h1")).toHaveText("root");
  await expect(page.locator(".profile-email")).toHaveText("root@example.invalid");
  await expect(page.locator(".profile-bio")).toContainText("In the workshop, most mornings.");
  await expect(page.locator(".code-badge", { hasText: "SOL" }).last()).toBeVisible();
  await expect(page.locator(".relation-row").last()).toContainText(/\d{4}/);

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

  // The members, each a way through to their own profile, and the trail.
  await page.locator(".profile-teams").getByRole("link", { name: "root" }).click();
  await expect(page).toHaveURL(/\/users\/root$/);
  // ...where the team wears the picture it was just given (DEV-119)...
  await expect(page.locator(".profile-teams img.team-avatar").first())
    .toHaveAttribute("src", /^data:image\//);
  // ...and a person's teams lead back.
  await page.locator(".profile-teams a[href='/t/1/profile']").click();
  await expect(page).toHaveURL(/\/t\/1\/profile$/);
  await expect(page.locator(".profile-activity-rows .relation-row").first()).toContainText("root");
});

test("the sidebar's backlog count follows a new ticket without a reload", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const count = page.locator(".sidebar .nav-item", { hasText: "Backlog" }).locator(".count");
  await expect(page.locator(".card").first()).toBeVisible(); // booted
  const before = (await count.count()) ? +(await count.textContent()) : 0; // 0 is not drawn

  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
  const form = page.getByRole("dialog");
  await form.getByLabel("Title").fill("Bleed the radiators");
  await form.getByRole("button", { name: "Create" }).click();
  await expect(count).toHaveText(String(before + 1));
});

test("the board hides old done items behind a toggle the backlog does not have", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const toggle = page.getByRole("button", { name: "Show old done" });
  await expect(toggle).not.toHaveClass(/\bon\b/);
  const asked = page.waitForRequest((r) => r.url().includes("/board?") && r.url().includes("allDone=1"));
  await toggle.click();
  await asked;
  await expect(toggle).toHaveClass(/\bon\b/);
  // A view preference: it survives a reload.
  await page.reload();
  await expect(page.getByRole("button", { name: "Show old done" })).toHaveClass(/\bon\b/);
  await page.getByRole("button", { name: "Show old done" }).click();

  await page.goto("/t/1/backlog");
  await expect(page.getByRole("button", { name: "Show buckets" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Show old done" })).toHaveCount(0);
});

// DEV-93: the backlog's columns say what they are, sit beside the title, and
// a row's right-click is a card's.
test("the backlog names its assignees, and a row's right-click is the card menu", async ({ page }) => {
  await signIn(page);
  const item = await (await page.request.post("/api/items", { data: {
    kind: "ticket", teamId: 1, title: "Rake the ashes", assigneeId: 1,
  } })).json();
  await page.goto("/t/1/backlog");
  const heads = page.locator("thead th");
  await expect(heads).toContainText(["", "", "Key", "Title", "Bucket", "Assignee", "Planned", ""]);
  const row = page.locator("tbody tr", { hasText: "Rake the ashes" });
  await expect(row.locator(".cell-assignee")).toHaveText(/\sroot$/); // after the avatar's initials

  await row.click({ button: "right" });
  const menu = page.locator(".card-menu");
  await expect(menu.locator(".menu-head")).toHaveText(item.identifier);
  await expect(row).toHaveClass(/selected/);
  await menu.getByRole("button", { name: "Priority" }).hover();
  await page.getByRole("menu", { name: "Priority" }).getByRole("menuitem", { name: "P1 · Highest" }).click();
  await expect.poll(async () =>
    (await (await page.request.get(`/api/items/${item.id}`)).json()).priority).toBe(1);
  await expect(page.locator(".selection-bar")).toHaveCount(0);
});

test("a card dropped between two others lands between them, where the line said", async ({ page }) => {
  await signIn(page);
  for (const title of ["Split the logs", "Stack the logs", "Cover the logs"]) {
    await page.request.post("/api/items", { data: { kind: "ticket", teamId: 1, title, accepted: true } });
  }
  await page.goto("/t/1/board");
  const todo = page.locator(".column", { has: page.getByRole("heading", { name: "To Do" }) });
  const cards = todo.locator(".card");
  await expect(cards.nth(2)).toBeVisible();
  const titles = () => cards.locator(".card-title").allTextContents();
  const [first, second] = await titles();
  const last = (await titles()).at(-1);

  // Held over the top half of the second card: the line sits in the gap
  // between the first and the second.
  const from = await cards.last().boundingBox();
  const [a, b] = [await cards.nth(0).boundingBox(), await cards.nth(1).boundingBox()];
  await page.mouse.move(from.x + 20, from.y + 10);
  await page.mouse.down();
  await page.mouse.move(b.x + 20, b.y + 6, { steps: 4 });
  const line = await todo.locator(".drop-line").boundingBox();
  expect(line.y).toBeGreaterThanOrEqual(a.y + a.height - 1);
  expect(line.y + line.height).toBeLessThanOrEqual(b.y + 1);

  // Dropped, it is second — not first, which is where naming only the card
  // below used to put it — and the click that ended the drag opened nothing.
  await page.mouse.up();
  await expect.poll(async () => (await titles()).slice(0, 3)).toEqual([first, last, second]);
  await expect(todo.locator(".drop-line")).toHaveCount(0);
  await expect(page.locator(".card.ghost")).toHaveCount(0);
  await expect(page.locator(".detail-aside")).toHaveCount(0);

  // Escape puts a held card back where it was.
  const order = await titles();
  const top = await cards.first().boundingBox();
  await page.mouse.move(top.x + 20, top.y + 10);
  await page.mouse.down();
  await page.mouse.move(top.x + 20, top.y + 300, { steps: 4 });
  await page.keyboard.press("Escape");
  await expect(page.locator(".card.ghost")).toHaveCount(0);
  await page.mouse.up();
  expect(await titles()).toEqual(order);
});

// The rest of the suite runs with reduced motion; this one needs the motion.
test.describe(() => {
  test.use({ contextOptions: { reducedMotion: "no-preference" } });

  test("a card finished on the board feeds the fire, even when someone else finished it", async ({ page }) => {
    await signIn(page);
    const item = await (await page.request.post("/api/items", { data: {
      kind: "ticket", teamId: 1, title: "Light the fire", accepted: true,
    } })).json();
    await page.goto("/t/1/board");
    await expect(page.locator(".card", { hasText: "Light the fire" })).toBeVisible();
    await expect(page.locator("canvas.embers")).toHaveCount(0);

    // Moved behind the board's back: it hears of it on the stream.
    const done = (await (await page.request.get("/api/teams/1/states")).json())
      .find((s) => s.role === "final");
    await page.request.post(`/api/items/${item.id}/move`, { data: { stateId: done.id } });
    // It kindles where it landed, embers are drawn, and the campfire in the
    // sidebar roars when the one carrying the work reaches it (DEV-71).
    const doneColumn = page.locator(".column", { has: page.getByRole("heading", { name: "Done" }) });
    await expect(doneColumn.locator(".card.kindled", { hasText: "Light the fire" })).toBeAttached();
    await expect(page.locator("canvas.embers")).toBeAttached();
    await expect(page.locator(".sidebar .mark.roar")).toBeAttached();
    // And the canvas goes once the last ember is out.
    await expect(page.locator("canvas.embers")).toHaveCount(0, { timeout: 5000 });
  });

  test("a card that leaves the filter in view kindles in Done and stays, faded, until the filter changes", async ({ page }) => {
    await signIn(page);
    const states = await (await page.request.get("/api/teams/1/states")).json();
    const done = states.find((s) => s.role === "final");
    const todo = states.find((s) => s.role === "accepted");
    const item = await (await page.request.post("/api/items", { data: {
      kind: "ticket", teamId: 1, title: "Sweep the hearth", accepted: true,
    } })).json();
    await page.goto("/t/1/board");
    const bar = page.locator(".filter-bar");
    const query = bar.getByLabel("Query");
    await bar.getByRole("button", { name: "</>" }).click();
    await query.fill(`state = "${todo.name}"`);
    await query.press("Enter");
    const doneColumn = page.locator(".column", { has: page.getByRole("heading", { name: "Done" }) });
    await expect(page.locator(".card", { hasText: "Sweep the hearth" })).toBeVisible();
    await expect(doneColumn.locator(".card")).toHaveCount(0);

    // Finished elsewhere: it no longer matches, and goes to Done anyway,
    // kindling there, and stays, kept.
    await page.request.post(`/api/items/${item.id}/move`, { data: { stateId: done.id } });
    await expect(doneColumn.locator(".card.kindled", { hasText: "Sweep the hearth" })).toBeAttached();
    const kept = doneColumn.locator(".card-slot.kept", { hasText: "Sweep the hearth" });
    await expect(kept).toBeVisible();

    // A later refresh keeps it: another write lands on the stream. Its title is
    // its own, so the wait is for it and not an earlier test's card.
    await page.request.post("/api/items", { data: { kind: "ticket", teamId: 1, title: "Split the kindling", accepted: true } });
    await expect(page.locator(".card", { hasText: "Split the kindling" })).toBeVisible();
    await expect(kept).toBeVisible();

    // A new filter starts from what matches it.
    await query.fill(`state = "${todo.name}" and priority != 4`);
    await query.press("Enter");
    await expect(page.locator(".card", { hasText: "Split the kindling" })).toBeVisible();
    await expect(page.locator(".card", { hasText: "Sweep the hearth" })).toHaveCount(0);
  });

  test("a card grabbed while it kindles is held in the hand, not dropped into the page", async ({ page }) => {
    await signIn(page);
    const item = await (await page.request.post("/api/items", { data: {
      kind: "ticket", teamId: 1, title: "Bank the coals", accepted: true,
    } })).json();
    await page.goto("/t/1/board");
    await expect(page.locator(".card", { hasText: "Bank the coals" })).toBeVisible();
    const done = (await (await page.request.get("/api/teams/1/states")).json())
      .find((s) => s.role === "final");
    await page.request.post(`/api/items/${item.id}/move`, { data: { stateId: done.id } });
    const card = page.locator(".column-body .card.kindled", { hasText: "Bank the coals" });
    await expect(card).toBeAttached();

    // Picked up mid-kindle (DEV-75): the copy follows the pointer, fixed over
    // the page, and no preview opens.
    const box = await card.boundingBox();
    await page.mouse.move(box.x + 20, box.y + 10);
    await page.mouse.down();
    await page.mouse.move(box.x + 60, box.y + 40, { steps: 4 });
    const ghost = page.locator(".card.ghost");
    await expect(ghost).toHaveCSS("position", "fixed");
    await expect(ghost).not.toHaveClass(/kindled/);
    // Called off with Escape and let go over the card: the release is not a
    // click on it.
    await page.keyboard.press("Escape");
    await expect(ghost).toHaveCount(0);
    const home = await card.boundingBox();
    await page.mouse.move(home.x + 20, home.y + 10);
    await page.mouse.up();
    await expect(page.locator(".detail-aside")).toHaveCount(0);
  });
});

// DEV-82: assigning a new ticket to yourself is one click beside the label.
test("(Me!) beside the assignee assigns the new ticket to you", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();

  const form = page.getByRole("dialog");
  await form.getByLabel("Title").fill("Mine from the start");
  await form.getByRole("button", { name: "(Me!)" }).click();
  await expect(form.getByLabel("Assignee").locator("option:checked")).toHaveText("root");
  await form.getByRole("button", { name: "Create" }).click();

  const created = (await page.locator(".toast").last().textContent()).match(/SOL-\d+/)[0];
  const item = await (await page.request.get(`/api/lookup/${created}`)).json();
  expect(item.assigneeName).toBe("root");
});

// DEV-81: the card menu's fields open sub-menus of their values, applied as
// they are clicked, to the whole selection.
test("the card menu's sub-menus set a field on the selection in one click", async ({ page }) => {
  await signIn(page);
  await page.goto("/t/1/board");
  const cards = page.locator(".column-body .card");
  await expect(cards.nth(1)).toBeVisible();
  const ids = [await cards.nth(0).getAttribute("data-id"), await cards.nth(1).getAttribute("data-id")];
  await cards.nth(0).click({ modifiers: ["Control"] });
  await cards.nth(1).click({ modifiers: ["Control"] });
  await cards.nth(1).click({ button: "right" });

  const menu = page.locator(".card-menu").first();
  await expect(menu.locator(".menu-head")).toHaveText("2 selected");
  await menu.getByRole("button", { name: "Priority" }).hover();
  const priority = page.getByRole("menu", { name: "Priority" });
  await expect(priority.getByRole("menuitem")).toHaveCount(4);
  // The pointer crossing Bucket on its way into the Priority sub-menu does not
  // swap it out.
  const box = await priority.getByRole("menuitem", { name: "P1 · Highest" }).boundingBox();
  const bucket = await menu.getByRole("button", { name: "Bucket" }).boundingBox();
  await page.mouse.move(bucket.x + bucket.width - 10, bucket.y + bucket.height / 2);
  await page.mouse.move(box.x + 20, box.y + box.height / 2, { steps: 3 });
  await expect(priority).toBeVisible();
  await page.mouse.click(box.x + 20, box.y + box.height / 2);
  await expect(page.locator(".card-menu")).toHaveCount(0);

  await expect.poll(async () => Promise.all(ids.map(async (id) =>
    (await (await page.request.get(`/api/items/${id}`)).json()).priority))).toEqual([1, 1]);
  // The selection outlives the change, so the next one can follow (DEV-89).
  const bar = page.locator(".selection-bar");
  await expect(bar.locator(".count")).toContainText("2");
  await bar.getByRole("button", { name: "Priority" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "P3 · Medium" }).click();
  await dialog.getByRole("button", { name: "Apply" }).click();
  await expect.poll(async () => Promise.all(ids.map(async (id) =>
    (await (await page.request.get(`/api/items/${id}`)).json()).priority))).toEqual([3, 3]);
  await expect(bar.locator(".count")).toContainText("2");
  await bar.getByRole("button", { name: "Clear" }).click();

  // One card: its value now is marked, and a click on Assignee opens the
  // sub-menu too.
  await cards.nth(0).click({ button: "right" });
  await menu.getByRole("button", { name: "Priority" }).click();
  await expect(page.getByRole("menu", { name: "Priority" }).locator(".team-option.on")).toContainText("P3 · Medium");
  await menu.getByRole("button", { name: "Assignee" }).click();
  await page.getByRole("menu", { name: "Assignee" }).getByRole("menuitem", { name: "Unassigned" }).click();
  await expect.poll(async () =>
    (await (await page.request.get(`/api/items/${ids[0]}`)).json()).assigneeId ?? 0).toBe(0);
});

// DEV-69: a card held near the top or bottom of the screen scrolls it, every
// column together, and near the left or right of the board scrolls the board,
// so it can be dropped where the screen did not reach. Moving off the edge
// stops it.
test("a held card scrolls the screen at its edge, and the board at its side", async ({ page }) => {
  await signIn(page);
  // Short, so To Do runs past the bottom; narrow, so the board runs past the side.
  await page.setViewportSize({ width: 1000, height: 520 });
  for (let i = 0; i < 12; i++) {
    await page.request.post("/api/items", { data: { kind: "ticket", teamId: 1, title: `Pile ${i}`, accepted: true } });
  }
  await page.goto("/t/1/board");
  const screen = page.locator(".body");
  expect(await screen.evaluate((b) => b.scrollHeight > b.clientHeight)).toBe(true);
  // Columns do not scroll on their own: the screen scrolls them together.
  const todo = page.locator(".column", { has: page.getByRole("heading", { name: "To Do" }) }).locator(".column-body");
  expect(await todo.evaluate((b) => b.scrollHeight <= b.clientHeight)).toBe(true);
  const card = todo.locator(".card").first();
  const id = await card.getAttribute("data-id");
  const box = await card.boundingBox();
  const frame = await screen.boundingBox();

  await page.mouse.move(box.x + 30, box.y + 10);
  await page.mouse.down();
  await page.mouse.move(box.x + 30, frame.y + frame.height - 4, { steps: 5 });
  await expect.poll(() => screen.evaluate((b) => b.scrollTop)).toBeGreaterThan(100);
  // The line keeps up with what scrolled under the still pointer.
  const line = await todo.locator(".drop-line").evaluate((l) => l.offsetTop);
  expect(line).toBeGreaterThan(frame.height);

  // Out of the zone, it stops.
  await page.mouse.move(box.x + 30, frame.y + frame.height / 2, { steps: 2 });
  const stopped = await screen.evaluate((b) => b.scrollTop);
  await page.waitForTimeout(300);
  expect(await screen.evaluate((b) => b.scrollTop)).toBe(stopped);

  // Sideways, to the last column, and the card lands there.
  const board = page.locator(".board").locator("..");
  expect(await board.evaluate((b) => b.scrollWidth > b.clientWidth)).toBe(true);
  const side = await board.boundingBox();
  await page.mouse.move(side.x + side.width - 30, frame.y + frame.height / 2, { steps: 5 });
  await expect.poll(() => board.evaluate((b) => b.scrollLeft + b.clientWidth >= b.scrollWidth - 1)).toBe(true);
  const last = page.locator(".column").last();
  await expect(last.locator(".drop-line")).toHaveCount(1);
  await page.mouse.up();
  await expect(last.locator(`.card[data-id="${id}"]`)).toBeVisible();
});

// DEV-76: descriptions and comments are read as Markdown, GitHub's dialect,
// and nothing in them runs.
test("a description and a comment are read as Markdown, and nothing in them runs", async ({ page }) => {
  await signIn(page);
  const text = [
    "Some **bold**, some `code`, and ~~gone~~.",
    "A second line, kept.",
    "",
    "- [x] done",
    "- [ ] not yet",
    "",
    "| a | b |",
    "|---|---|",
    "| 1 | 2 |",
    "",
    "See SOL-1 and [elsewhere](https://example.com), not `SOL-2`.",
    "",
    "![far](https://example.com/x.png) <script>window.ran = 1</script>",
    "<img src=x onerror=\"window.ran = 1\"> [click](javascript:window.ran=1)",
  ].join("\n");
  const item = await (await page.request.post("/api/items", {
    data: { kind: "ticket", teamId: 1, title: "Read as Markdown", description: text },
  })).json();
  await page.request.post(`/api/items/${item.id}/comments`, { data: { body: "A comment with *emphasis*." } });

  await page.goto(`/${item.identifier}`);
  const prose = page.locator(".prose.markdown");
  await expect(prose.locator("strong")).toHaveText("bold");
  await expect(prose.locator("code").first()).toHaveText("code");
  await expect(prose.locator("del")).toHaveText("gone");
  await expect(prose.locator("input[type=checkbox]:checked")).toHaveCount(1);
  await expect(prose.locator("td")).toHaveText(["1", "2"]);
  // A typed line break inside a paragraph is kept.
  expect(await prose.locator("p").first().innerText()).toContain("gone.\nA second line");
  // An identifier links to its item, except in code; a link away opens apart.
  await expect(prose.getByRole("link", { name: "SOL-1", exact: true })).toHaveAttribute("href", "/SOL-1");
  await expect(prose.getByRole("link", { name: "SOL-2" })).toHaveCount(0);
  await expect(prose.getByRole("link", { name: "elsewhere" })).toHaveAttribute("target", "_blank");
  // A picture from elsewhere is a link, not a fetch; HTML is text; a script
  // address is no link at all.
  await expect(prose.locator("img")).toHaveCount(0);
  await expect(prose.getByRole("link", { name: "far" })).toHaveAttribute("href", "https://example.com/x.png");
  await expect(prose).toContainText("<script>");
  await expect(prose.locator("a[href^='javascript']")).toHaveCount(0);
  expect(await page.evaluate(() => window.ran)).toBeUndefined();

  await expect(page.locator(".comment-body em")).toHaveText("emphasis");

  // The preview reads it rendered, and a click opens the Markdown to write.
  await page.goto("/t/1/backlog");
  await page.locator("td.cell-title", { hasText: "Read as Markdown" }).click();
  const aside = page.locator(".detail-aside");
  await expect(aside.locator(".markdown strong")).toHaveText("bold");
  await aside.locator(".bare-read").click({ position: { x: 5, y: 5 } });
  await expect(aside.getByRole("textbox", { name: "Description" })).toBeFocused();
  await expect(aside.getByRole("textbox", { name: "Description" })).toHaveValue(text);
});

// DEV-40: a picture pasted into a description or a comment is uploaded, and
// the Markdown naming it goes in at the caret; read back, it is drawn.
test("a picture pasted into a description or a comment is drawn", async ({ page }) => {
  await signIn(page);
  const item = await (await page.request.post("/api/items", {
    data: { kind: "ticket", teamId: 1, title: "Pictured" },
  })).json();
  await page.goto(`/${item.identifier}`);

  // A 2×2 PNG, pasted as a screenshot is: a file on the clipboard.
  const paste = (box, size = 0) => box.evaluate((el, size) => {
    const png = Uint8Array.from(atob("iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFUlEQVR4nGP8z8Dwn4GBgYkBBv" +
      "5DGAAmYgML8Ck1FQAAAABJRU5ErkJggg=="), (c) => c.charCodeAt(0));
    const data = new DataTransfer();
    data.items.add(new File([size ? new Uint8Array(size) : png], "image.png", { type: "image/png" }));
    el.focus();
    el.dispatchEvent(new ClipboardEvent("paste", { clipboardData: data, bubbles: true, cancelable: true }));
  }, size);

  await page.getByRole("banner").getByRole("button", { name: "Edit" }).click();
  const description = page.getByLabel("Description");
  await description.fill("Before. ");
  await paste(description);
  await expect(description).toHaveValue(/^Before\. !\[image\.png\]\(\/api\/images\/\d+\)$/);
  await page.getByRole("banner").getByRole("button", { name: "Save" }).click();
  const drawn = page.locator(".prose img");
  await expect(drawn).toBeVisible();
  expect(await drawn.evaluate((img) => img.complete && img.naturalWidth)).toBe(2);

  const comment = page.getByPlaceholder("Write a comment…");
  await paste(comment);
  await expect(comment).toHaveValue(/^!\[image\.png\]\(\/api\/images\/\d+\)$/);
  await page.getByRole("button", { name: "Comment", exact: true }).click();
  await expect(page.locator(".comment-body img")).toBeVisible();

  // Over the instance's limit, it is refused before it is sent.
  await paste(comment, 11 * 2 ** 20);
  await expect(page.locator(".toast").last()).toHaveText(/over the 10MB limit/);
  await expect(comment).toHaveValue("");
});

// DEV-80: the documentation is a page of the app, reached from the sidebar
// and from the query box's cheat sheet, and it names every field CSQL has.
test("the docs page is in the sidebar, the cheat sheet links into it, and it covers CSQL", async ({ page }) => {
  const { FIELDS } = await import("../internal/web/src/lib/csql.js");
  await signIn(page);
  await page.goto("/t/1/board");
  await page.locator(".sidebar").getByRole("link", { name: "Documentation" }).click();
  await expect(page).toHaveURL(/\/docs$/);
  const docs = page.locator(".docs");
  // From first steps to everyday use to connecting other tools, in that order.
  const rail = page.locator(".settings-rail button");
  await expect(rail.first()).toHaveText("Overview");
  await expect(rail.nth(1)).toHaveText("Getting started");
  await expect(rail.last()).toHaveText("Source code");
  for (const field of FIELDS) {
    await expect(docs.locator("td code", { hasText: new RegExp(`^${field}$`) }).first()).toBeVisible();
  }

  // The rail goes to a section and says which one is being read.
  await page.locator(".settings-rail").getByRole("button", { name: "Saved views" }).click();
  await expect(page).toHaveURL(/\/docs#saved-views$/);
  await expect(page.locator(".settings-rail button.on")).toHaveText("Saved views");

  // The cheat sheet beside the query box opens the full reference at Filtering.
  await page.goto("/t/1/board");
  const bar = page.locator(".filter-bar");
  await bar.getByTitle("Write the filter in CSQL").click();
  await bar.getByRole("button", { name: "CSQL help" }).click();
  await page.getByRole("link", { name: /The full reference/ }).click();
  await expect(page).toHaveURL(/\/docs#filtering$/);
  const heading = page.locator(".docs h2#filtering");
  await expect(heading).toBeInViewport();
  await expect(page.locator(".settings-rail button.on")).toHaveText("Filtering");
});

// DEV-94: relations are made on the create form, in any of the seven kinds,
// and read in the preview as well as on the ticket.
test("a ticket is created with its relations, and its preview lists them", async ({ page }) => {
  await signIn(page);
  const first = await (await page.request.post("/api/items", { data: {
    kind: "ticket", teamId: 1, title: "Lay the fire",
  } })).json();
  await page.goto("/t/1/backlog");
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
  const form = page.getByRole("dialog");
  await form.getByLabel("Title").fill("Strike the match");
  await form.getByLabel("Relation kind").selectOption({ label: "blocked by" });
  const other = form.getByLabel("Related item");
  // Text that was never picked names nothing, and Create says so.
  await other.fill("no such thing");
  await form.getByRole("button", { name: "Create" }).click();
  await expect(form.locator(".field-error")).toHaveText("Pick the related item from the list, or clear the box.");
  // Part of a title offers the items that match; a click picks one.
  await other.fill("lay the f");
  const offers = page.getByRole("listbox");
  await offers.getByRole("option", { name: new RegExp(`${first.identifier}\\s*Lay the fire`) }).click();
  await expect(other).toHaveValue("");
  await expect(form.locator(".relation-row")).toContainText(`blocked by ${first.identifier} Lay the fire`);
  await form.getByRole("button", { name: "Create" }).click();
  await expect(form).toHaveCount(0);

  // The preview lists it, as seen from the new ticket.
  await page.locator("tbody tr", { hasText: "Strike the match" }).click();
  const aside = page.locator(".detail-aside");
  await expect(aside.locator(".relation-row")).toContainText(`blocked by ${first.identifier}`);
  // And the far end sees the one edge the other way.
  await page.goto(`/${first.identifier}`);
  await expect(page.locator(".relation-row", { hasText: "Strike the match" })).toContainText("blocks");
});

// DEV-94: the ticket page's Add a relation finds the other item by search, by
// title or by identifier, as the create form does.
test("a relation is added from the ticket page by searching for the other item", async ({ page }) => {
  await signIn(page);
  const [a, b] = await Promise.all(["Fetch the kindling", "Dry the kindling"].map(async (title) =>
    (await page.request.post("/api/items", { data: { kind: "ticket", teamId: 1, title } })).json()));
  await page.goto(`/${a.identifier}`);
  await page.getByRole("button", { name: "Add", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Kind").selectOption({ label: "follows up" });
  // The item itself is never offered.
  const to = dialog.getByRole("combobox", { name: "To" });
  await to.fill("the kindling");
  const offers = page.getByRole("listbox");
  await expect(offers.getByRole("option", { name: /Dry the kindling/ })).toBeVisible();
  await expect(offers.getByRole("option", { name: /Fetch the kindling/ })).toHaveCount(0);
  // An identifier is looked up as one: typed and entered at once, it is picked.
  await to.fill(b.identifier);
  await to.press("Enter");
  await expect(dialog.locator(".relation-row")).toContainText(`follows up ${b.identifier} Dry the kindling`);
  await dialog.getByRole("button", { name: "Add", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.locator(".relation-row", { hasText: "Dry the kindling" })).toContainText("follows up");
});
