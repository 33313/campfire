// Modals and toasts: the two things that live outside whatever screen is up.

import { ApiError } from "./api.js";

export const modals = $state([]);
export const toasts = $state([]);

/** Opens a dialog component. Resolves to whatever the dialog closes with. */
export function openModal(component, props = {}) {
  return new Promise((resolve) => {
    modals.push({ id: ++seq, component, props, resolve });
  });
}

export function closeModal(entry, value) {
  const i = modals.indexOf(entry);
  if (i < 0) return; // already closed
  modals.splice(i, 1);
  entry.resolve(value);
}

let seq = 0;

export function toast(message, kind = "") {
  const id = ++seq;
  toasts.push({ id, message, kind });
  // Matched by id rather than by reference: $state deep-proxies whatever goes
  // into the array, so the object pushed is never the one indexOf would find
  // and the toast would sit there for the rest of the session.
  setTimeout(() => {
    const i = toasts.findIndex((t) => t.id === id);
    if (i >= 0) toasts.splice(i, 1);
  }, 6000);
}

export function failure(err) {
  toast(err instanceof ApiError ? err.message : String(err), "alert");
}
