import { defineConfig } from "@playwright/test";
import { createHash } from "node:crypto";

// One port per checkout, so suites in parallel worktrees never meet. Set
// E2E_PORT to choose it. The config is evaluated again in each worker, so the
// port is derived, not random, and set in the environment serve.sh reads.
process.env.E2E_PORT ||= String(20000 + createHash("sha1").update(process.cwd()).digest().readUInt16BE(0) % 10000);
const base = `http://127.0.0.1:${process.env.E2E_PORT}`;

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
    baseURL: base, trace: "retain-on-failure",
    contextOptions: { reducedMotion: "reduce" },
    // CI runners ship Google Chrome; using it skips a browser download.
    ...(process.env.CI && { channel: "chrome" }),
  },
  webServer: {
    command: "./e2e/serve.sh",
    url: `${base}/api/instance`,
    reuseExistingServer: false,
    stdout: "pipe",
  },
});
