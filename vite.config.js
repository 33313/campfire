import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// The client is built into internal/web/app, where the Go binary embeds it.
// Filenames are fixed rather than hashed: the server serves them from an
// embedded filesystem with no cache headers, and stable names let the output
// directory be reused (emptyOutDir would take the .gitkeep with it).
export default defineConfig({
  root: "internal/web",
  publicDir: "static",
  plugins: [svelte()],
  build: {
    outDir: "app",
    emptyOutDir: false,
    rollupOptions: {
      output: {
        entryFileNames: "app.js",
        chunkFileNames: "[name].js",
        assetFileNames: "[name][extname]",
      },
    },
  },
});
