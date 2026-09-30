import { defineConfig } from "@playwright/test";

// The suite drives the real binary: build it, run it against a throwaway data
// directory, and talk to it over HTTP exactly as a browser does.
export default defineConfig({
  testDir: "e2e",
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  // Reduced motion: the suite checks where things end up, not the frames on
  // the way there, and a width or a position read mid-animation is neither.
  use: {
    baseURL: "http://127.0.0.1:8791", trace: "retain-on-failure",
    contextOptions: { reducedMotion: "reduce" },
  },
  webServer: {
    command: "./e2e/serve.sh",
    url: "http://127.0.0.1:8791/api/instance",
    reuseExistingServer: false,
    stdout: "pipe",
  },
});
