// A picture — a person's or a team's — is scaled here and stored inline: an
// instance with no object store and no third party to fetch from is the whole
// point.

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

// The picture a file input's change event chose, as a data URL; null when
// nothing was chosen. Rejects when the file is not an image.
export function pickPicture(e) {
  const file = e.currentTarget.files?.[0];
  e.currentTarget.value = "";
  if (!file) return Promise.resolve(null);
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      // Square crop from the centre, then scale: a picture is drawn at 20px
      // beside a name, and letterboxing it there reads as a mistake.
      const side = Math.min(img.width, img.height);
      const canvas = Object.assign(document.createElement("canvas"),
        { width: AVATAR, height: AVATAR });
      canvas.getContext("2d").drawImage(img, (img.width - side) / 2, (img.height - side) / 2,
        side, side, 0, 0, AVATAR, AVATAR);
      URL.revokeObjectURL(img.src);
      resolve(encode(canvas));
    };
    img.onerror = () => reject("that file is not an image");
    img.src = URL.createObjectURL(file);
  });
}
