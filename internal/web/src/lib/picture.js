// A picture — a person's or a team's — is scaled here and stored inline: an
// instance with no object store and no third party to fetch from is the whole
// point.

import { openModal } from "./ui.svelte.js";
import CropDialog from "../dialogs/CropDialog.svelte";

// 512 is the size a profile draws at on a dense screen; anything smaller was
// being scaled up there and looked it.
export const AVATAR = 512;

// What core.maxAvatar will store. A photograph at 512px is tens of kilobytes
// in WebP, but a busy one can run past this, and one step down in quality is
// better than an upload the server refuses.
const MAX_AVATAR = 256 * 1024;

function encode(canvas) {
  const data = canvas.toDataURL("image/webp", 0.85);
  return data.length <= MAX_AVATAR ? data : canvas.toDataURL("image/webp", 0.6);
}

function load(file) {
  return new Promise((resolve, reject) => {
    const img = new Image();
    // An SVG with only a viewBox loads with no size, and there is nothing to
    // crop or scale.
    img.onload = () => {
      if (img.naturalWidth && img.naturalHeight) return resolve(img);
      URL.revokeObjectURL(img.src);
      reject("that picture has no size; give the SVG a width and height");
    };
    img.onerror = () => { URL.revokeObjectURL(img.src); reject("that file is not an image"); };
    img.src = URL.createObjectURL(file);
  });
}

// The picture a file input's change event chose, cropped to the square picked
// in the crop dialog, as a data URL; null when nothing was chosen or the crop
// was cancelled. Rejects when the file is not an image.
//
// Square, because a picture is drawn at 20px beside a name, and letterboxing
// it there reads as a mistake.
export async function pickPicture(e) {
  const file = e.currentTarget.files?.[0];
  e.currentTarget.value = "";
  if (!file) return null;
  const img = await load(file);
  try {
    const square = await openModal(CropDialog, { img });
    if (!square) return null;
    const canvas = Object.assign(document.createElement("canvas"), { width: AVATAR, height: AVATAR });
    canvas.getContext("2d").drawImage(img, square.x, square.y, square.side, square.side,
      0, 0, AVATAR, AVATAR);
    return encode(canvas);
  } finally {
    URL.revokeObjectURL(img.src);
  }
}
