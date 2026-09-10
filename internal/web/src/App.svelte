<script>
  // The client: boot, routing and the shell.

  import { api, ApiError } from "./lib/api.js";
  import { app, reloadContext } from "./lib/state.svelte.js";
  import { route, navigate } from "./lib/router.svelte.js";
  import { stream, openStream } from "./lib/stream.svelte.js";

  import Sidebar from "./components/Sidebar.svelte";
  import Modals from "./components/Modals.svelte";
  import Toasts from "./components/Toasts.svelte";
  import ErrorPanel from "./components/ErrorPanel.svelte";
  import SignIn from "./routes/SignIn.svelte";
  import Board from "./routes/Board.svelte";
  import Backlog from "./routes/Backlog.svelte";
  import Item from "./routes/Item.svelte";
  import Buckets from "./routes/Buckets.svelte";
  import Log from "./routes/Log.svelte";
  import TeamSettings from "./routes/TeamSettings.svelte";
  import Admin from "./routes/Admin.svelte";
  import Account from "./routes/Account.svelte";
  import Profile from "./routes/Profile.svelte";

  const IDENTIFIER = /^\/([A-Za-z][A-Za-z0-9]*-\d+)$/;

  let booted = $state(false);
  let bootError = $state(null);
  let needsBootstrap = $state(false);

  async function boot() {
    bootError = null;
    try {
      const instance = await api.getInstance();
      if (instance.needsBootstrap) {
        needsBootstrap = true;
        booted = true;
        return;
      }
      try {
        // Subscribe first, then fetch. Fetching first leaves a window in which
        // changes are silently lost.
        const session = await api.getSession();
        app.user = session.user;
        openStream();
        await reloadContext();
      } catch (err) {
        if (!(err instanceof ApiError) || err.status !== 401) throw err;
        app.user = null;
      }
      booted = true;
    } catch (err) {
      bootError = err;
      booted = true;
    }
  }

  async function afterSignIn() {
    needsBootstrap = false;
    openStream();
    await reloadContext();
    navigate("/", true);
  }

  boot();

  // The team in context follows the address. Everything per-team the sidebar
  // draws — the name in the switcher, the views, the backlog count — is read
  // from app.team, which otherwise only ever held the team the session booted
  // on, so switching changed the board and nothing around it.
  //
  // The team has to be one we know of: reloading a team the instance does not
  // have leaves app.team null, and the effect would ask again forever. Asking
  // only for a known team ends it, because loading that team is what makes the
  // condition false.
  $effect(() => {
    const id = +(route.path.match(/^\/t\/(\d+)\//)?.[1] || 0);
    if (id && app.user && id !== app.team?.id && app.allTeams.some((t) => t.id === id)) {
      reloadContext(id);
    }
  });

  // The one address that is a redirect: land on the first team's board, or on
  // the bucket index when there is no team yet.
  $effect(() => {
    if (app.user && route.path === "/") {
      const first = app.teams[0];
      navigate(first ? `/t/${first.teamId}/board` : "/buckets", true);
    }
  });

  let screen = $derived.by(() => {
    const path = route.path;
    let m;
    if ((m = path.match(/^\/t\/(\d+)\/board$/))) return { c: Board, p: { teamId: +m[1] } };
    if ((m = path.match(/^\/t\/(\d+)\/backlog$/))) return { c: Backlog, p: { teamId: +m[1] } };
    if ((m = path.match(/^\/t\/(\d+)\/settings$/))) return { c: TeamSettings, p: { teamId: +m[1] } };
    if (path === "/buckets") return { c: Buckets, p: {} };
    if (path === "/log") return { c: Log, p: {} };
    if (path === "/admin") return { c: Admin, p: {} };
    if (path === "/account") return { c: Account, p: {} };
    // A profile is addressed by name: /users/marta is the link a person hands over.
    if ((m = path.match(/^\/users\/([^/]+)$/))) {
      return { c: Profile, p: { username: decodeURIComponent(m[1]) } };
    }
    // The old /bucket/<id> address: a bucket's detail is the item screen.
    if ((m = path.match(/^\/bucket\/(\d+)$/))) return { c: Item, p: { bucketId: +m[1] } };
    if ((m = path.match(IDENTIFIER))) return { c: Item, p: { identifier: m[1] } };
    return null;
  });
</script>

{#if !booted}
  <div class="signin"><div class="eyebrow">Loading</div></div>
{:else if bootError}
  <ErrorPanel error={bootError} retry={boot} />
{:else if needsBootstrap}
  <SignIn mode="bootstrap" onsignedin={afterSignIn} />
{:else if !app.user}
  {#if route.path.startsWith("/invite/")}
    <SignIn mode="redeem" token={route.path.slice("/invite/".length)} onsignedin={afterSignIn} />
  {:else}
    <SignIn onsignedin={afterSignIn} />
  {/if}
{:else}
  <Sidebar />
  <div class="main">
    {#if stream.status === "stale"}
      <!-- A cursor past retention gets an explicit instruction, never a silent
           partial replay (SPEC.md §12.2). -->
      <div class="banner alert">
        <span class="pip rejected"></span>
        <span class="grow">This board is out of date. You were away longer than the log we keep.</span>
        <button class="btn small" onclick={() => location.reload()}>Reload</button>
      </div>
    {:else if stream.status === "reconnecting"}
      <div class="banner">
        <span class="pip waiting"></span>
        <span class="grow explain">
          Reconnecting. Everything on screen is still correct; live updates resume on their own.
        </span>
      </div>
    {/if}

    {#if screen}
      {@const Screen = screen.c}
      {#key route.path}
        <Screen {...screen.p} />
      {/key}
    {:else if route.path !== "/"}
      <div class="signin">
        <div class="signin-card">
          <h1>Nothing here</h1>
          <p class="explain">{route.path}</p>
          <a href="/">Back to the board</a>
        </div>
      </div>
    {/if}
  </div>
{/if}

<Modals />
<Toasts />
