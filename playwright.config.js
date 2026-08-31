import { defineConfig } from "@playwright/test";

// The suite drives the real binary: build it, run it against a throwaway data
// directory, and talk to it over HTTP exactly as a browser does.
export default defineConfig({
  testDir: "e2e",
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  use: { baseURL: "http://127.0.0.1:8791", trace: "retain-on-failure" },
  webServer: {
    command: "./e2e/serve.sh",
    url: "http://127.0.0.1:8791/api/instance",
    reuseExistingServer: false,
    stdout: "pipe",
  },
});
