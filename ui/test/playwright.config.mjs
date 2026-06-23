// Playwright config for the AID GUI real-browser E2E harness.
//
// The `make ui-e2e` target owns the lifecycle: it builds the GUI + binary,
// boots `aid serve` on a free port with a temp plans dir, seeds the vendored
// oracle plans through the REAL REST API, exports BASE_URL, and tears the
// server down afterward. Hence there is NO `webServer` block here — Playwright
// only drives the browser against the already-running server.
//
// Offline browser: chromium is NOT installed by this package. It is resolved
// from the shared Playwright browser cache via PLAYWRIGHT_BROWSERS_PATH, which
// the make target points at the vendored cache (revision 1140 ==
// @playwright/test 1.48.2). CI must likewise provide a matching chromium cache
// + PLAYWRIGHT_BROWSERS_PATH; see ui/test/README.md.

import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.BASE_URL || "http://127.0.0.1:8080";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: 1,
  reporter: [["list"]],
  use: {
    baseURL,
    headless: true,
    actionTimeout: 15000,
    navigationTimeout: 15000,
    trace: "off",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
