import { mount } from "svelte";
import "./fonts.css";
import "./style.css";
import App from "./App.svelte";
import { drawCaret } from "./lib/caret.js";

// A key or a click shows the caret at once and starts its blink over, as the
// browser's own blink does (style.css, DEV-74).
for (const type of ["keydown", "pointerdown"]) {
  addEventListener(type, (e) => {
    for (const a of e.target.getAnimations?.() || []) {
      if (a.animationName === "caret") a.currentTime = 0;
    }
  }, true);
}
if (!CSS.supports("caret-animation", "manual")) drawCaret();

export default mount(App, { target: document.getElementById("app") });
