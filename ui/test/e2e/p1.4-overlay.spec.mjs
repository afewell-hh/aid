// p1.4-overlay.spec.mjs — real-browser E2E for the optic/identity overlay tab
// (#70): presence visibility (present/none), a no-overlay plan's BOM optic
// columns are blank, and after saving an overlay the presence flips to "present"
// and the BOM carries the optical standard.
//
// The e2e server is shared + serial, and an earlier spec attaches an overlay to
// the seeded xoc-64 plan — so this test POSTs its OWN overlay-free plan (a
// renamed copy of the xoc-64 training) via the API, keeping it isolated.

import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";

const TRAINING = readFileSync(
  new URL("../../../tests/oracle/xoc-64-mesh-conv-ro/training.yaml", import.meta.url),
  "utf8",
)
  .replace(/training_xoc64_1xopg64_mesh_conv_ro/g, "p14_overlay_fixture")
  .replace(/Training XOC-64 1x OPG-64 Mesh Converged RO/g, "P14 Overlay Fixture");
const PLAN_NAME = "P14 Overlay Fixture";
const OVERLAY = readFileSync(
  new URL("../../../tests/fixtures/f3/optic-overlay.yaml", import.meta.url),
  "utf8",
);

test.describe("P1.4 overlay tab (#70)", () => {
  test("presence none + blank BOM optics; save overlay -> present + optics populate", async ({ page }) => {
    // create a fresh, overlay-free plan (isolated from shared seeded state).
    const created = await page.request.post("/api/plans", {
      headers: { "content-type": "text/yaml" },
      data: TRAINING,
    });
    expect(created.ok()).toBeTruthy();

    await page.goto("/");
    await expect(page.locator("#app")).not.toBeEmpty();
    await page.locator("tr", { hasText: PLAN_NAME }).getByRole("button", { name: "View" }).click();
    await expect(page.locator("#overlay-yaml")).toBeVisible();

    const overlay = page.locator("#overlay-section");
    const result = page.locator("#detail-result");

    // (1) no overlay yet -> presence "none".
    await expect(overlay.locator(".text-bg-secondary")).toContainText("none");

    // (2) BOM optic-standard column is blank without the overlay.
    await page.locator("#bom-btn").click();
    await expect(result.getByText("Bill of Materials")).toBeVisible();
    await expect(result).not.toContainText("400GBASE-DR4");

    // (3) attach the overlay + save -> presence flips to "present".
    await page.locator("#overlay-yaml").fill(OVERLAY);
    await page.locator("#overlay-save-btn").click();
    await expect(overlay.locator(".text-bg-success")).toContainText("present");

    // (4) View BOM now carries the optical standard (real SKUs).
    await page.locator("#bom-btn").click();
    await expect(result).toContainText("400GBASE-DR4");
  });
});
