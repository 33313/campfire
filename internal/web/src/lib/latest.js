// One screen's requests for what it shows: the newest wins. Starting one
// cancels the one in flight, and a reply that arrives after a newer request
// was made is dropped, never drawn (SPEC.md §9.2).

/** Returns `ask(fetch)`, where fetch(signal) makes the request. It resolves
    to { value }, { error }, or { superseded: true }. */
export function latest() {
  let controller = null;
  let made = 0;
  return async (fetch) => {
    controller?.abort();
    const mine = (controller = new AbortController());
    const n = ++made;
    try {
      const value = await fetch(mine.signal);
      return n === made ? { value } : { superseded: true };
    } catch (error) {
      return n === made && error.name !== "AbortError" ? { error } : { superseded: true };
    }
  };
}
