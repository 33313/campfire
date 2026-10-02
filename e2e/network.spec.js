import { test, expect } from "@playwright/test";

// What live updates cost on the wire (SPEC.md §12.4): a notification is a
// refetch only where it can change what is shown, a hidden tab refetches
// nothing, a refetch that finds nothing new is a 304, and a screen you come
// back to is drawn before the server answers.
//
// Self-contained: it brings up its own teams, so it runs alone or after the
// main suite. The server it runs against closes a hidden tab's stream after
// two seconds (e2e/serve.sh).

const ROOT = { username: "root", password: "correct horse battery" };
test.describe.configure({ mode: "serial" });

let here, there, doing;

test.beforeAll(async ({ request }) => {
  await request.post("/api/instance/bootstrap", { data: ROOT }); // refused once there is an account
  await request.post("/api/auth/signin", { data: ROOT });
  const code = () => Array.from({ length: 5 }, () => String.fromCharCode(65 + Math.floor(Math.random() * 26))).join("");
  const team = async (name) => {
    const shortCode = code();
    return (await request.post("/api/teams", { data: { name: `${name} ${shortCode}`, shortCode } })).json();
  };
  here = await team("Here");
  there = await team("There");
  const states = await (await request.get(`/api/teams/${here.id}/states`)).json();
  doing = states.find((s) => !["default", "accepted", "final", "rejected"].includes(s.role));
});

// Signs the page in, so that page.request writes as the same account.
async function signIn(page) {
  await page.request.post("/api/auth/signin", { data: ROOT });
}

async function open(page) {
  await signIn(page);
  await page.goto(`/t/${here.id}/board`);
}

// Every API request the page makes but the stream, as "status path".
function count(page) {
  const asked = [];
  page.on("request", (r) => {
    const u = new URL(r.url());
    if (!u.pathname.startsWith("/api/") || u.pathname === "/api/stream") return;
    const entry = { path: u.pathname, status: 0 };
    asked.push(entry);
    r.response().then((res) => { entry.status = res?.status(); }).catch(() => {});
  });
  return asked;
}

const ticket = (request, team, title) => request.post("/api/items",
  { data: { kind: "ticket", teamId: team.id, title, accepted: true } }).then((r) => r.json());

// Past the debounce of SPEC.md §12.4 and the refetch it causes.
const settle = (page) => page.waitForTimeout(1200);

const visible = (page, shown) => page.evaluate((shown) => {
  Object.defineProperty(document, "hidden", { configurable: true, get: () => !shown });
  Object.defineProperty(document, "visibilityState", { configurable: true, get: () => (shown ? "visible" : "hidden") });
  document.dispatchEvent(new Event("visibilitychange"));
}, shown);

test("another team's change costs nothing; this team's one refetch, and nothing new is a 304", async ({ page }) => {
  await signIn(page);
  const first = await ticket(page.request, here, "Sweep the chimney");
  await open(page);
  await expect(page.locator(".card", { hasText: "Sweep the chimney" })).toBeVisible();
  await settle(page);
  const asked = count(page);

  const away = await ticket(page.request, there, "Elsewhere entirely");
  await page.request.post(`/api/items/${away.id}/move`, { data: { stateId: (await (await page.request
    .get(`/api/teams/${there.id}/states`)).json()).find((s) => s.role === "final").id } });
  await settle(page);
  expect(asked).toEqual([]);

  // Someone else moves a card here: the board and the sidebar's count, once.
  await page.request.post(`/api/items/${first.id}/move`, { data: { stateId: doing.id } });
  await expect(page.locator(`.column-body[data-state="${doing.id}"] .card`, { hasText: "Sweep the chimney" })).toBeVisible();
  await settle(page);
  expect(asked.map((a) => a.path).sort()).toEqual([`/api/teams/${here.id}/backlog`, `/api/teams/${here.id}/board`]);

  // Asked again with nothing changed, the board is a 304.
  asked.length = 0;
  await page.request.post(`/api/items/${away.id}/comments`, { data: { body: "unrelated" } });
  await page.request.post(`/api/items/${first.id}/comments`, { data: { body: "the card is on screen" } });
  await settle(page);
  expect(asked.filter((a) => a.path.endsWith("/board")).map((a) => a.status)).toEqual([304]);
});

test("a hidden tab refetches nothing, sleeps, and catches up once when shown", async ({ page }) => {
  await open(page);
  await expect(page.locator(".card").first()).toBeVisible();
  await expect(page.locator(".filter-bar").getByText("Live")).toBeVisible();
  await settle(page);
  const asked = count(page);
  const streams = [];
  page.on("request", (r) => r.url().includes("/api/stream") && streams.push(r.url()));

  await visible(page, false);
  await ticket(page.request, here, "Written while you looked away");
  await settle(page);
  expect(asked).toEqual([]);

  // Past the grace the stream closes; what happens meanwhile is replayed from
  // the cursor on return, not lost.
  await expect(page.locator(".filter-bar").getByText("asleep")).toBeAttached({ timeout: 4000 });
  await ticket(page.request, here, "Written while it slept");
  await page.waitForTimeout(500);
  expect(asked).toEqual([]);

  await visible(page, true);
  await expect(page.locator(".filter-bar").getByText("Live")).toBeVisible();
  await expect(page.locator(".card", { hasText: "Written while you looked away" })).toBeVisible();
  await expect(page.locator(".card", { hasText: "Written while it slept" })).toBeVisible();
  expect(streams).toHaveLength(1);
  expect(streams[0]).toMatch(/cursor=\d+/);
});

test("a tab too far behind to replay catches up without a reload", async ({ page }) => {
  await open(page);
  await expect(page.locator(".card").first()).toBeVisible();
  await page.evaluate(() => { window.stayed = true; });
  await visible(page, false);
  await expect(page.locator(".filter-bar").getByText("asleep")).toBeAttached({ timeout: 4000 });

  // More changes than the log replays (MaxStreamReplay): 100 tickets, their
  // priority flipped 21 times.
  const ids = [];
  for (let i = 0; i < 100; i++) ids.push((await ticket(page.request, there, `Flood ${i}`)).id);
  for (let i = 0; i < 21; i++) {
    await page.request.post("/api/items/bulk", { data: { itemIds: ids, field: "priority", value: 1 + (i % 2) } });
  }
  await ticket(page.request, here, "After the flood");

  await visible(page, true);
  await expect(page.locator(".card", { hasText: "After the flood" })).toBeVisible();
  await expect(page.locator(".filter-bar").getByText("Live")).toBeVisible();
  expect(await page.evaluate(() => window.stayed)).toBe(true);
});

test("coming back to the board draws it before the server answers", async ({ page }) => {
  await open(page);
  await expect(page.locator(".card").first()).toBeVisible();
  const sidebar = page.locator(".sidebar");
  await sidebar.getByRole("link", { name: "Backlog" }).click();
  await expect(page.getByRole("heading", { name: "Backlog" })).toBeVisible();

  let release;
  const held = new Promise((r) => { release = r; });
  await page.route("**/board?*", async (route) => { await held; await route.continue(); });
  await sidebar.getByRole("link", { name: "Board" }).click();
  await expect(page.locator(".card", { hasText: "After the flood" })).toBeVisible({ timeout: 500 });
  release();
  await page.unroute("**/board?*");
});
