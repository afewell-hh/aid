// p1.5-nav.spec.mjs — real-browser E2E for the P1.5 navigation surface (#66):
// breadcrumb back-nav and the SPA navbar brand. Runs in headless chromium
// against the seeded `aid serve` (BASE_URL), like golden-path.spec.mjs.
//
// RED: the current GUI has a "← All plans" link but no breadcrumb, and the
// navbar brand is a hard <a href="/"> (full reload, no id). GREEN adds a
// breadcrumb with a clickable Plans crumb and an SPA brand (#nav-home) that
// returns to the list without a reload.

import { test, expect } from "@playwright/test";

const MESH_64_NAME = "Training XOC-64 1x OPG-64 Mesh Converged RO";

async function waitForApp(page) {
  await expect(page.locator("#app")).not.toBeEmpty();
}

async function openFirstDetail(page) {
  await page.goto("/");
  await waitForApp(page);
  await page.getByRole("button", { name: "View" }).first().click();
  await expect(page.locator("#edit-yaml")).toBeVisible(); // detail rendered
}

test.describe("P1.5 navigation (#66)", () => {
  test("detail shows a breadcrumb whose Plans crumb returns to the list", async ({ page }) => {
    await openFirstDetail(page);

    const crumb = page.locator('nav[aria-label="breadcrumb"]');
    await expect(crumb).toBeVisible();
    await page.locator("#crumb-plans").click();

    await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
  });

  test("SPA navbar brand returns to the plan list without a full reload", async ({ page }) => {
    await openFirstDetail(page);

    // Mark the live document; a real SPA nav (no reload) preserves the marker.
    await page.evaluate(() => {
      window.__aid_spa_marker = "alive";
    });

    await page.locator("#nav-home").click();
    await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
    await expect(page.getByText(MESH_64_NAME, { exact: true })).toBeVisible();

    const marker = await page.evaluate(() => window.__aid_spa_marker);
    expect(marker).toBe("alive"); // a full reload would have cleared it
  });
});
