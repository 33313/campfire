import { test, expect } from "@playwright/test";
import { ROOT, PNG, newTeam, column } from "./helpers.js";

// What only a browser shows: the client's main flows end to end, drag and
// drop, live updates, and what the client renders. Rules the server enforces
// are tested in Go.

// First, so it meets the instance empty. Run after another test, it skips.
test("the first visit creates the first account, then the first team", async ({ page }) => {
  const { needsBootstrap } = await (await page.request.get("/api/instance")).json();
  test.skip(!needsBootstrap, "another test bootstrapped the instance first");
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Create the first account" })).toBeVisible();
  await page.getByLabel("Username").fill(ROOT.username);
  await page.getByLabel("Password").fill(ROOT.password);
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { name: "Buckets" })).toBeVisible();

  await page.locator(".team-switch").click();
  await page.getByRole("button", { name: "New team" }).click();
  const form = page.getByRole("dialog");
  await form.getByLabel("Name").fill("Solstice");
  await form.getByLabel("Short code").fill("SOL");
  await form.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
  await expect(page.locator(".team-switch")).toContainText("Solstice");
});

test("a new ticket lands in the backlog with what the form set", async ({ page }) => {
  const team = await newTeam(page);
  await page.goto(team.board);
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
  const form = page.getByRole("dialog");
  await form.getByLabel("Title").fill("Replace the boiler");
  await form.getByLabel("Description").fill("It is older than the building.");
  await form.getByLabel("Assignee").selectOption({ label: "root" });
  await form.getByRole("button", { name: "Add a label" }).click();
  await form.getByLabel("Labels").fill("plumbing");
  await form.getByRole("button", { name: "Create" }).click();

  // Creating leaves you on the board.
  const id = `${team.shortCode}-1`;
  await expect(page).toHaveURL(new RegExp(`${team.board}$`));
  await expect(page.locator(".toast")).toHaveText(new RegExp(`${id} created`));

  await page.goto(team.backlog);
  await expect(page.getByRole("link", { name: id })).toBeVisible();
  await page.goto(`/${id}`);
  await expect(page.locator(".label-chip", { hasText: "plumbing" })).toBeVisible();
  await expect(page.locator(".aside-control", { hasText: "root" })).toBeVisible();
});

test("a ticket moved on its page appears in that column", async ({ page }) => {
  const team = await newTeam(page);
  const item = await team.ticket("Replace the boiler");
  await page.goto(`/${item.identifier}`);
  await page.locator(".aside-control").first().click();
  const move = page.getByRole("dialog");
  await move.getByRole("button", { name: "In Progress" }).click();
  await move.getByRole("button", { name: "Move", exact: true }).click();
  await expect(page.locator(".aside-control").first()).toContainText("In Progress");

  await page.goto(team.board);
  await expect(column(page, "In Progress").locator(".card")).toHaveText(/Replace the boiler/);
});

test("a ticket is edited behind Edit on its page, and in place in the board's preview", async ({ page }) => {
  const team = await newTeam(page);
  const item = await team.ticket("Replace the boiler", { accepted: true, assigneeId: 1 });
  await page.goto(`/${item.identifier}`);

  const header = page.getByRole("banner");
  await header.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Title").fill("nonsense");
  await header.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("heading", { name: "Replace the boiler", exact: true })).toBeVisible();
  await header.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Title").fill("Replace the boiler and its flue");
  await header.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { name: "Replace the boiler and its flue" })).toBeVisible();

  await page.goto(team.board);
  await page.locator(".card", { hasText: "Replace the boiler and its flue" }).click();
  const preview = page.locator(".detail-aside");
  const title = preview.getByLabel("Title", { exact: true });
  await title.fill("nonsense");
  await preview.getByRole("button", { name: "Discard title" }).click();
  await expect(title).toHaveValue("Replace the boiler and its flue");
  await title.fill("Replace the boiler");
  await title.press("Enter");
  await expect(preview.getByRole("button", { name: "Save title" })).toHaveCount(0);

  await preview.getByRole("button", { name: /root/ }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Unassigned" }).click();
  await expect(preview.getByRole("button", { name: /Unassigned/ })).toBeVisible();

  await page.goto(`/${item.identifier}`);
  await expect(page.getByRole("heading", { name: "Replace the boiler", exact: true })).toBeVisible();
});

test("a comment and a label are added on the ticket page, the label from what exists", async ({ page }) => {
  const team = await newTeam(page);
  const other = await team.ticket("Bleed the radiators");
  await page.request.post(`/api/items/${other.id}/labels`, { data: { name: "heating" } });
  const item = await team.ticket("Replace the boiler");
  await page.goto(`/${item.identifier}`);

  await page.getByPlaceholder("Write a comment…").fill("The part is on order.");
  await page.getByRole("button", { name: "Comment", exact: true }).click();
  await expect(page.locator(".comment-body")).toHaveText("The part is on order.");

  await page.getByRole("button", { name: "Add a label" }).click();
  const box = page.getByLabel("Labels");
  await box.fill("he");
  await expect(page.locator(".label-suggestion").first()).toHaveText("heating");
  await box.press("Enter");
  await expect(page.locator(".label-chip", { hasText: "heating" })).toBeVisible();

  await page.reload();
  await expect(page.locator(".comment-body")).toHaveText("The part is on order.");
  await expect(page.locator(".label-chip", { hasText: "heating" })).toBeVisible();
});

test("a bucket is made with an icon, and a ticket put in it carries it on its card", async ({ page }) => {
  const team = await newTeam(page);
  const item = await team.ticket("Replace the boiler", { accepted: true });
  await page.goto(team.board);
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
  const form = page.getByRole("dialog");
  await form.getByRole("button", { name: "Bucket", exact: true }).click();
  await form.getByLabel("Title").fill("Winter readiness");
  await form.getByRole("button", { name: /bookmark/ }).click();
  const looks = page.getByRole("dialog", { name: "Icon and colour" });
  await looks.getByRole("button", { name: "rocket", exact: true }).click();
  await looks.getByRole("button", { name: "rust", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(form.getByRole("button", { name: /rocket.*rust/ })).toBeVisible();
  await form.getByRole("button", { name: "Create" }).click();
  await expect(form).toHaveCount(0);

  await page.goto(`/${item.identifier}`);
  await page.getByRole("button", { name: "Change bucket" }).click();
  await page.getByRole("dialog").getByRole("combobox", { name: "Bucket" }).fill("wint");
  await expect(page.getByRole("listbox").getByRole("option")).toHaveCount(1);
  await page.keyboard.press("Enter");
  await expect(page.locator(".aside-control").last()).toContainText("Winter readiness");

  await page.goto(team.board);
  await expect(page.locator(".card .card-bucket")).toContainText("Winter readiness");
  await page.goto(`/${team.shortCode}-2`);
  await expect(page.locator(".state-badge", { hasText: "bucket" })).toBeVisible();
  await expect(page.getByRole("link", { name: item.identifier })).toBeVisible();
});

test("a ticket is created with a relation, and both ends show it", async ({ page }) => {
  const team = await newTeam(page);
  const first = await team.ticket("Lay the fire");
  await page.goto(team.backlog);
  await page.getByRole("banner").getByRole("button", { name: "Create" }).click();
  const form = page.getByRole("dialog");
  await form.getByLabel("Title").fill("Strike the match");
  await form.getByLabel("Relation kind").selectOption({ label: "blocked by" });
  await form.getByLabel("Related item").fill("lay the f");
  await page.getByRole("listbox").getByRole("option", { name: new RegExp(`${first.identifier}\\s*Lay the fire`) }).click();
  await expect(form.locator(".relation-row")).toContainText(`blocked by ${first.identifier} Lay the fire`);
  await form.getByRole("button", { name: "Create" }).click();
  await expect(form).toHaveCount(0);

  await page.locator("tbody tr", { hasText: "Strike the match" }).click();
  await expect(page.locator(".detail-aside .relation-row")).toContainText(`blocked by ${first.identifier}`);
  await page.goto(`/${first.identifier}`);
  await expect(page.locator(".relation-row", { hasText: "Strike the match" })).toContainText("blocks");
});

test("chips and raw CSQL filter the board, and a refused query is not applied", async ({ page }) => {
  const team = await newTeam(page);
  const boiler = await team.ticket("Replace the boiler", { accepted: true, priority: 3 });
  await page.request.post(`/api/items/${boiler.id}/labels`, { data: { name: "plumbing" } });
  await team.ticket("Sweep the flue", { accepted: true, priority: 1 });
  await page.goto(team.board);
  const bar = page.locator(".filter-bar");
  const cards = page.locator(".card");
  await expect(cards).toHaveCount(2);

  await bar.getByRole("button", { name: "Priority" }).click();
  await page.locator(".filter-pop").getByRole("option", { name: /P1 · Highest/ }).click();
  await page.keyboard.press("Enter");
  await expect(cards).toHaveText([/Sweep the flue/]);

  // The chips' filter is where the raw query starts.
  await bar.getByRole("button", { name: "</>" }).click();
  const query = bar.getByLabel("Query");
  await expect(query).toHaveValue("priority = 1");
  await query.fill("label = plumbing");
  await query.press("Enter");
  await expect(cards).toHaveText([/Replace the boiler/]);

  // A syntax error is the client's to catch; a bad name is the server's.
  // Neither changes the board.
  await query.fill("priority = 9 and label = plumbing");
  await expect(query).toHaveAttribute("aria-invalid", "true");
  await query.fill("assignee = nobody-yet");
  await query.press("Enter");
  await expect(page.locator(".query-offers .query-error")).toHaveText(/no user is called "nobody-yet"/);
  await expect(cards).toHaveText([/Replace the boiler/]);
  await bar.getByRole("button", { name: "Discard" }).click();
  await expect(query).toHaveValue("label = plumbing");
});

test("saved views switch on in the sidebar, are and-ed, and stay on over a reload", async ({ page }) => {
  const team = await newTeam(page);
  const boiler = await team.ticket("Replace the boiler", { accepted: true, priority: 3 });
  await page.request.post(`/api/items/${boiler.id}/labels`, { data: { name: "plumbing" } });
  await team.ticket("Sweep the flue", { accepted: true, priority: 1 });
  await page.goto(team.board);
  const bar = page.locator(".filter-bar");
  const sidebar = page.locator(".sidebar");
  const cards = page.locator(".card");
  await expect(cards).toHaveCount(2);

  // Saving the bar's filter moves it into a view that is on.
  await bar.getByRole("button", { name: "Priority" }).click();
  await page.locator(".filter-pop").getByRole("option", { name: /P1 · Highest/ }).click();
  await page.keyboard.press("Enter");
  await bar.getByRole("button", { name: "Save as view" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill("Urgent");
  await dialog.getByRole("button", { name: "Save" }).click();
  const urgent = sidebar.getByRole("button", { name: "Urgent", exact: true });
  await expect(urgent).toHaveAttribute("aria-pressed", "true");
  await expect(bar.getByRole("button", { name: "Priority", exact: true })).toBeVisible();
  await expect(cards).toHaveText([/Sweep the flue/]);

  await urgent.click();
  await expect(cards).toHaveCount(2);

  await sidebar.getByRole("button", { name: "New view" }).click();
  await dialog.getByLabel("Name").fill("Plumbing");
  await dialog.getByRole("button", { name: "</>" }).click();
  await dialog.getByLabel("Query").fill("label = plumbing");
  await dialog.getByRole("button", { name: "Save" }).click();
  const plumbing = sidebar.getByRole("button", { name: "Plumbing", exact: true });
  await expect(plumbing).toHaveAttribute("aria-pressed", "true");
  await expect(cards).toHaveText([/Replace the boiler/]);
  await urgent.click();
  await expect(cards).toHaveCount(0);

  await page.reload();
  await expect(plumbing).toHaveAttribute("aria-pressed", "true");
  await expect(urgent).toHaveAttribute("aria-pressed", "true");
  await expect(cards).toHaveCount(0);
});

test("search suggests an item as you type, and the suggestion opens it", async ({ page }) => {
  const team = await newTeam(page);
  const item = await team.ticket("Descale the kettle");
  await page.goto(team.board);
  await page.getByRole("banner").getByPlaceholder("Title and description").pressSequentially("kettle");
  const hit = page.locator(".search-suggestion").first();
  await expect(hit).toContainText(item.identifier);
  await hit.click();
  await expect(page).toHaveURL(new RegExp(`/${item.identifier}$`));
});

test("a drag carries the whole selection, not just the card under the pointer", async ({ page }) => {
  const team = await newTeam(page);
  await team.ticket("Sweep the chimney", { accepted: true });
  await team.ticket("Order the kindling", { accepted: true });
  await page.goto(team.board);
  const cards = column(page, "To Do").locator(".card");
  await expect(cards).toHaveCount(2);

  await cards.first().click({ modifiers: ["Control"] });
  await cards.nth(1).click({ modifiers: ["Control"] });
  await expect(page.locator(".selection-bar .count")).toContainText("2");
  const progress = column(page, "In Progress");
  await cards.first().dragTo(progress.locator(".column-body"));
  await expect(progress.locator(".card")).toHaveCount(2);
});

test("a card dropped between two others lands where the line said, and Escape puts a held card back", async ({ page }) => {
  const team = await newTeam(page);
  for (const title of ["Split the logs", "Stack the logs", "Cover the logs"]) {
    await team.ticket(title, { accepted: true });
  }
  await page.goto(team.board);
  const todo = column(page, "To Do");
  const cards = todo.locator(".card");
  await expect(cards).toHaveCount(3);
  const titles = () => cards.locator(".card-title").allTextContents();
  const [first, second, last] = await titles();

  // Held over the top half of the second card, the line is in the gap above it.
  const from = await cards.last().boundingBox();
  const [a, b] = [await cards.nth(0).boundingBox(), await cards.nth(1).boundingBox()];
  await page.mouse.move(from.x + 20, from.y + 10);
  await page.mouse.down();
  await page.mouse.move(b.x + 20, b.y + 6, { steps: 4 });
  const line = await todo.locator(".drop-line").boundingBox();
  expect(line.y).toBeGreaterThanOrEqual(a.y + a.height - 1);
  expect(line.y + line.height).toBeLessThanOrEqual(b.y + 1);

  // Dropped, it is second, and the release opened no preview.
  await page.mouse.up();
  await expect.poll(titles).toEqual([first, last, second]);
  await expect(page.locator(".card.ghost")).toHaveCount(0);
  await expect(page.locator(".detail-aside")).toHaveCount(0);

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

// A held card near the top or bottom of the screen scrolls it, and near the
// left or right of the board scrolls the board (DEV-69).
test("a held card scrolls the screen at its edge, and the board at its side", async ({ page }) => {
  const team = await newTeam(page);
  // Short, so To Do runs past the bottom; narrow, so the board runs past the side.
  await page.setViewportSize({ width: 1000, height: 520 });
  for (let i = 0; i < 12; i++) await team.ticket(`Pile ${i}`, { accepted: true });
  await page.goto(team.board);
  const screen = page.locator(".body");
  const todo = column(page, "To Do").locator(".column-body");
  await expect(todo.locator(".card")).toHaveCount(12);
  expect(await screen.evaluate((b) => b.scrollHeight > b.clientHeight)).toBe(true);
  const card = todo.locator(".card").first();
  const id = await card.getAttribute("data-id");
  const box = await card.boundingBox();
  const frame = await screen.boundingBox();

  await page.mouse.move(box.x + 30, box.y + 10);
  await page.mouse.down();
  await page.mouse.move(box.x + 30, frame.y + frame.height - 4, { steps: 5 });
  await expect.poll(() => screen.evaluate((b) => b.scrollTop)).toBeGreaterThan(100);

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

test("the card menu sets a field on the whole selection", async ({ page }) => {
  const team = await newTeam(page);
  const items = [await team.ticket("Sweep the chimney", { accepted: true }),
    await team.ticket("Order the kindling", { accepted: true })];
  await page.goto(team.board);
  const cards = page.locator(".column-body .card");
  await expect(cards).toHaveCount(2);
  await cards.nth(0).click({ modifiers: ["Control"] });
  await cards.nth(1).click({ modifiers: ["Control"] });
  await cards.nth(1).click({ button: "right" });

  const menu = page.locator(".card-menu");
  await expect(menu.locator(".menu-head")).toHaveText("2 selected");
  await menu.getByRole("button", { name: "Priority" }).hover();
  await page.getByRole("menu", { name: "Priority" }).getByRole("menuitem", { name: "P1 · Highest" }).click();
  await expect(menu).toHaveCount(0);
  await expect.poll(() => Promise.all(items.map(async ({ id }) =>
    (await (await page.request.get(`/api/items/${id}`)).json()).priority))).toEqual([1, 1]);
});

// The rest of the suite runs with reduced motion; this one needs the motion.
test.describe(() => {
  test.use({ contextOptions: { reducedMotion: "no-preference" } });

  test("a card someone else finishes kindles in Done and feeds the fire", async ({ page }) => {
    const team = await newTeam(page);
    const item = await team.ticket("Light the fire", { accepted: true });
    await page.goto(team.board);
    await expect(page.locator(".card", { hasText: "Light the fire" })).toBeVisible();
    await expect(page.locator("canvas.embers")).toHaveCount(0);

    // Moved behind the board's back: it hears of it on the stream (DEV-71).
    await page.request.post(`/api/items/${item.id}/move`, { data: { stateId: team.done.id } });
    await expect(column(page, "Done").locator(".card.kindled", { hasText: "Light the fire" })).toBeAttached();
    await expect(page.locator("canvas.embers")).toBeAttached();
    await expect(page.locator(".sidebar .mark.roar")).toBeAttached();
    await expect(page.locator("canvas.embers")).toHaveCount(0, { timeout: 5000 });
  });
});

// The server closes a hidden tab's stream after two seconds (e2e/serve.sh).
test("a hidden tab sleeps, and catches up on what it missed when shown", async ({ page }) => {
  const team = await newTeam(page);
  await team.ticket("Here before", { accepted: true });
  await page.goto(team.board);
  const status = page.locator(".filter-bar");
  await expect(status.getByText("Live")).toBeVisible();
  const visible = (shown) => page.evaluate((shown) => {
    Object.defineProperty(document, "hidden", { configurable: true, get: () => !shown });
    Object.defineProperty(document, "visibilityState", { configurable: true, get: () => (shown ? "visible" : "hidden") });
    document.dispatchEvent(new Event("visibilitychange"));
  }, shown);

  await visible(false);
  await team.ticket("Written while you looked away", { accepted: true });
  await expect(status.getByText("asleep")).toBeAttached({ timeout: 4000 });
  await team.ticket("Written while it slept", { accepted: true });

  await visible(true);
  await expect(status.getByText("Live")).toBeVisible();
  await expect(page.locator(".card", { hasText: "Written while you looked away" })).toBeVisible();
  await expect(page.locator(".card", { hasText: "Written while it slept" })).toBeVisible();
});

// Descriptions and comments are GitHub-flavoured Markdown, and nothing in
// them runs (DEV-76).
test("Markdown is rendered, and nothing in it runs", async ({ page }) => {
  const team = await newTeam(page);
  const first = await team.ticket("Lay the fire");
  const code = team.shortCode;
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
    `See ${first.identifier} and [elsewhere](https://example.com), not \`${code}-2\`.`,
    "",
    "![far](https://example.com/x.png) <script>window.ran = 1</script>",
    "<img src=x onerror=\"window.ran = 1\"> [click](javascript:window.ran=1)",
  ].join("\n");
  const item = await team.ticket("Read as Markdown", { description: text });
  await page.request.post(`/api/items/${item.id}/comments`, { data: { body: "A comment with *emphasis*." } });

  await page.goto(`/${item.identifier}`);
  const prose = page.locator(".prose.markdown");
  await expect(prose.locator("strong")).toHaveText("bold");
  await expect(prose.locator("del")).toHaveText("gone");
  await expect(prose.locator("input[type=checkbox]:checked")).toHaveCount(1);
  await expect(prose.locator("td")).toHaveText(["1", "2"]);
  expect(await prose.locator("p").first().innerText()).toContain("gone.\nA second line");
  // An identifier links to its item, except in code.
  await expect(prose.getByRole("link", { name: first.identifier, exact: true })).toHaveAttribute("href", `/${first.identifier}`);
  await expect(prose.getByRole("link", { name: `${code}-2` })).toHaveCount(0);
  await expect(prose.getByRole("link", { name: "elsewhere" })).toHaveAttribute("target", "_blank");
  // A picture from elsewhere is a link, not a fetch; HTML is text; a script
  // address is no link.
  await expect(prose.locator("img")).toHaveCount(0);
  await expect(prose.getByRole("link", { name: "far" })).toHaveAttribute("href", "https://example.com/x.png");
  await expect(prose).toContainText("<script>");
  await expect(prose.locator("a[href^='javascript']")).toHaveCount(0);
  expect(await page.evaluate(() => window.ran)).toBeUndefined();
  await expect(page.locator(".comment-body em")).toHaveText("emphasis");

  // The preview renders it too, and a click opens the Markdown for editing.
  await page.goto(team.backlog);
  await page.locator("td.cell-title", { hasText: "Read as Markdown" }).click();
  const aside = page.locator(".detail-aside");
  await expect(aside.locator(".markdown strong")).toHaveText("bold");
  await aside.locator(".bare-read").click({ position: { x: 5, y: 5 } });
  await expect(aside.getByRole("textbox", { name: "Description" })).toHaveValue(text);
});

// A pasted picture is uploaded, and Markdown naming it goes in at the caret
// (DEV-40).
test("a picture pasted into a description or a comment is uploaded and drawn", async ({ page }) => {
  const team = await newTeam(page);
  const item = await team.ticket("Pictured");
  await page.goto(`/${item.identifier}`);

  const paste = (box, size = 0) => box.evaluate((el, [png, size]) => {
    const bytes = size ? new Uint8Array(size) : Uint8Array.from(atob(png), (c) => c.charCodeAt(0));
    const data = new DataTransfer();
    data.items.add(new File([bytes], "image.png", { type: "image/png" }));
    el.focus();
    el.dispatchEvent(new ClipboardEvent("paste", { clipboardData: data, bubbles: true, cancelable: true }));
  }, [PNG.toString("base64"), size]);

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
});

test("a person's and a team's pictures replace their initials", async ({ page }) => {
  const team = await newTeam(page);
  await page.goto(team.board);
  await page.locator(".whoami .who-link").click();
  await expect(page).toHaveURL(/\/users\/root$/);
  await page.getByRole("button", { name: "Edit profile" }).click();
  await page.getByLabel("Picture").setInputFiles({ name: "me.png", mimeType: "image/png", buffer: PNG });
  await expect(page.locator(".whoami img.avatar")).toHaveAttribute("src", /^data:image\//);
  await page.getByRole("button", { name: "Remove picture" }).click();
  await expect(page.locator(".whoami .avatar")).toHaveText("RO");

  await page.goto(`/t/${team.id}/profile`);
  await expect(page.locator("span.profile-picture")).toHaveText(team.shortCode);
  await page.getByRole("button", { name: "Edit profile" }).click();
  await page.getByLabel("Picture").setInputFiles({ name: "team.png", mimeType: "image/png", buffer: PNG });
  await page.getByRole("banner").getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.locator(".toast")).toHaveText(/Saved/);
  await page.reload();
  await expect(page.locator("img.profile-picture")).toHaveAttribute("src", /^data:image\//);

  // A member's profile lists the team with its picture (DEV-119).
  await page.goto("/users/root");
  const row = page.locator(".profile-teams .relation-row", { has: page.locator(`a[href='/t/${team.id}/profile']`) });
  await expect(row.locator("img.team-avatar")).toHaveAttribute("src", /^data:image\//);
});

// Preferences are a convenience: with storage blocked, nothing is remembered
// and the board still works.
test("the board works where the browser stores nothing", async ({ page }) => {
  await page.addInitScript(() => Object.defineProperty(window, "localStorage", {
    get() { throw new DOMException("blocked", "SecurityError"); },
  }));
  const team = await newTeam(page);
  await page.goto(team.board);
  await expect(page.locator(".column").first()).toBeVisible();
  const toggle = page.getByRole("button", { name: "Show buckets" });
  await toggle.click();
  await expect(toggle).toHaveClass(/on/);
});

test("the team switcher goes to the team picked and names it", async ({ page }) => {
  const here = await newTeam(page);
  const there = await newTeam(page);
  await page.goto(here.board);
  await expect(page.locator(".team-switch")).toContainText(here.name);
  await page.locator(".team-switch").click();
  await page.locator(".team-option", { hasText: there.name }).click();
  await expect(page).toHaveURL(new RegExp(`${there.board}$`));
  await expect(page.locator(".team-switch")).toContainText(there.name);
});

test("signing out from the sidebar returns to the sign-in card", async ({ page }) => {
  const team = await newTeam(page);
  await page.goto(team.board);
  await page.locator(".sidebar").getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
});
