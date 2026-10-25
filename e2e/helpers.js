// Every test works in a team of its own, so it sees nothing another test made
// and runs alone (-g) or in any order.

export const ROOT = { username: "root", password: "correct horse battery" };

// A 2×2 PNG.
export const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFUlEQVR4nGP8z8Dwn4GBgYkBBv" +
  "5DGAAmYgML8Ck1FQAAAABJRU5ErkJggg==", "base64");

// Signs the page in as root, bootstrapping the instance first if no test has,
// and makes a new team. page.request shares the page's cookies, so its writes
// are root's.
export async function newTeam(page) {
  await page.request.post("/api/instance/bootstrap", { data: ROOT }); // refused once there is an account
  await page.request.post("/api/auth/signin", { data: ROOT });
  const shortCode = Array.from({ length: 5 }, () => String.fromCharCode(65 + Math.floor(Math.random() * 26))).join("");
  const team = await (await page.request.post("/api/teams", { data: { name: `Team ${shortCode}`, shortCode } })).json();
  const states = await (await page.request.get(`/api/teams/${team.id}/states`)).json();
  return {
    ...team,
    board: `/t/${team.id}/board`,
    backlog: `/t/${team.id}/backlog`,
    todo: states.find((s) => s.role === "accepted"),
    doing: states.find((s) => !["default", "accepted", "final", "rejected"].includes(s.role)),
    done: states.find((s) => s.role === "final"),
    ticket: async (title, data = {}) => (await page.request.post("/api/items", {
      data: { kind: "ticket", teamId: team.id, title, ...data },
    })).json(),
  };
}

export const column = (page, name) =>
  page.locator(".column", { has: page.getByRole("heading", { name, exact: true }) });
