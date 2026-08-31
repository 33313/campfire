// The router: the address bar, read reactively.
//
// Small enough to keep: the client has seven addresses and one wildcard for an
// item identifier.

export const route = $state({
  path: location.pathname,
  search: new URLSearchParams(location.search),
});

function read() {
  route.path = location.pathname;
  route.search = new URLSearchParams(location.search);
}

export function navigate(path, replace = false) {
  if (replace) history.replaceState({}, "", path);
  else history.pushState({}, "", path);
  read();
}

addEventListener("popstate", read);

// Any left-click on an internal link routes without a page load.
addEventListener("click", (e) => {
  const a = e.target.closest?.("a[href^='/']");
  if (!a || e.defaultPrevented || e.metaKey || e.ctrlKey || e.shiftKey
      || e.button !== 0 || a.target) return;
  e.preventDefault();
  navigate(a.getAttribute("href"));
});
