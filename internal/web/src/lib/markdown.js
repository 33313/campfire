// Descriptions and comments are Markdown (SPEC.md §6.1), and are read
// rendered (DEV-76).
//
// micromark is CommonMark plus GitHub's tables, strikethrough, task lists and
// bare links. It escapes any HTML in the text and drops a link whose address
// would run script, so what it returns is safe to set as HTML. Then three
// things are done to it:
//   - an identifier of a team on this instance, SOL-42, links to its item;
//   - a link off the instance opens in a new tab;
//   - a picture from off the instance becomes a link to it. Drawing it would
//     have every reader's browser fetch from a third party, which is why an
//     avatar is never a URL either. One pasted here (DEV-40) is drawn, and
//     opens at full size in a new tab.

import { micromark } from "micromark";
import { gfm, gfmHtml } from "micromark-extension-gfm";
import { app } from "./state.svelte.js";

const OPTIONS = { extensions: [gfm()], htmlExtensions: [gfmHtml()] };

const away = (url) => new URL(url, location.href).origin !== location.origin;

/** The HTML for a Markdown text. */
export function markdown(text) {
  const t = document.createElement("template"); // inert: nothing in it loads
  t.innerHTML = micromark(text || "", OPTIONS);
  const page = t.content;

  for (const img of page.querySelectorAll("img")) {
    const a = Object.assign(document.createElement("a"), { href: img.src });
    if (away(img.getAttribute("src"))) {
      a.textContent = img.alt || img.src;
      img.replaceWith(a);
    } else if (!img.closest("a")) {
      img.loading = "lazy";
      Object.assign(a, { target: "_blank", title: img.alt });
      img.replaceWith(a);
      a.append(img);
    }
  }
  for (const a of page.querySelectorAll("a[href]")) {
    if (away(a.getAttribute("href"))) Object.assign(a, { target: "_blank", rel: "noopener noreferrer" });
  }

  const codes = app.allTeams.map((team) => team.shortCode);
  if (codes.length) {
    const ident = new RegExp(`\\b(?:${codes.join("|")})-\\d+\\b`, "g");
    const walk = document.createTreeWalker(page, NodeFilter.SHOW_TEXT);
    const found = [];
    while (walk.nextNode()) {
      const node = walk.currentNode;
      if (!node.parentElement?.closest("a, code, pre") && ident.test(node.data)) found.push(node);
      ident.lastIndex = 0;
    }
    for (const node of found) {
      const parts = node.data.split(new RegExp(`(${ident.source})`));
      node.replaceWith(...parts.map((part, i) => {
        if (i % 2 === 0) return part;
        const a = Object.assign(document.createElement("a"), { href: `/${part}` });
        a.textContent = part;
        return a;
      }));
    }
  }
  return t.innerHTML;
}
