// Pictures pasted or dropped into a description or a comment (DEV-40). Each
// is uploaded, and the Markdown that shows it goes in at the caret:
// ![name](/api/images/12). While it travels a placeholder holds its place, so
// typing on is safe, and it is swapped for the real thing wherever it has got
// to by then.

import { api } from "./api.js";
import { app } from "./state.svelte.js";
import { toast, failure } from "./ui.svelte.js";

/** An action for a textarea; `on` false leaves it a plain one. */
export function pictures(el, on = true) {
  if (!on) return;
  const files = (list) => [...(list || [])].filter((f) => f.type.startsWith("image/"));
  const paste = (e) => take(el, files(e.clipboardData?.files), e);
  const drop = (e) => take(el, files(e.dataTransfer?.files), e);
  const over = (e) => e.dataTransfer?.types.includes("Files") && e.preventDefault();
  el.addEventListener("paste", paste);
  el.addEventListener("drop", drop);
  el.addEventListener("dragover", over);
  return {
    destroy() {
      el.removeEventListener("paste", paste);
      el.removeEventListener("drop", drop);
      el.removeEventListener("dragover", over);
    },
  };
}

async function take(el, images, e) {
  if (!images.length) return;
  e.preventDefault();
  const max = app.imageMaxSize;
  if (!max) {
    toast("This instance does not take pictures", "alert");
    return;
  }
  await Promise.all(images.map(async (file) => {
    const name = (file.name || "picture").replace(/[[\]()]/g, "");
    if (file.size > max) {
      toast(`${name} is over ${Math.round(max / 2 ** 20)}MB, the most this instance takes`, "alert");
      return;
    }
    const holder = `![Uploading ${name}…]()`;
    edit(el, holder);
    try {
      const { url } = await api.uploadImage(file);
      edit(el, `![${name}](${url})`, holder);
    } catch (err) {
      edit(el, "", holder);
      failure(err);
    }
  }));
}

// Writes `text` over `from` where it now is, or at the caret, and tells the
// binding, which only hears what is typed.
function edit(el, text, from) {
  const at = from === undefined ? el.selectionStart : el.value.indexOf(from);
  if (at < 0) return; // its placeholder was edited away
  const end = from === undefined ? el.selectionEnd : at + from.length;
  el.setRangeText(text, at, end, from === undefined ? "end" : "preserve");
  el.dispatchEvent(new Event("input", { bubbles: true }));
}
