// p3.1-library.spec.mjs — real-browser E2E (#80) for the read-only Library
// browse surface. Runs in headless chromium against a LIVE `aid serve` (managed
// by `make ui-e2e`). The Library is the built-in catalog union derived from the
// shipped reference templates — it must contain real items from BOTH the mesh
// (xoc-64) and Clos (xoc-256) compositions.
//
// RED (#80): the navbar Library button calls a stub loader that renders a
// placeholder and issues no request, so these fail at the intended seam. #80
// GREEN wires GET /api/catalog + library_html.

import { test, expect } from "@playwright/test";

// Wait until the initial load_plans has SETTLED (the plan-list heading rendered),
// not merely until #app is non-empty (true at the loading spinner). Otherwise a
// navbar click races the in-flight load_plans fetch, whose late callback would
// clobber #app.
async function waitForApp(page) {
  await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
}

test.describe("AID GUI #80 — Library browse (read-only)", () => {
  test("navbar Library opens the browse table with real built-in items", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);

    await page.locator("#nav-library").click();

    await expect(page.getByRole("heading", { name: /Library/i })).toBeVisible();
    // Coverage across shipped references: a Clos class (xoc-256) AND a mesh class
    // (xoc-64) are both present in the built-in union.
    await expect(page.getByText("fe-leaf", { exact: false })).toBeVisible();
    await expect(page.getByText("soc_storage_scale_out_leaf", { exact: false })).toBeVisible();
  });

  test("Library is read-only: no create/edit/delete controls", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator("#nav-library").click();
    await expect(page.getByRole("heading", { name: /Library/i })).toBeVisible();

    // No authoring affordances on the Library surface (slice-1 is browse only).
    await expect(page.getByRole("button", { name: /new|create|add|edit|delete/i })).toHaveCount(0);
  });

  test("clicking a Library item shows its detail", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator("#nav-library").click();
    await expect(page.getByRole("heading", { name: /Library/i })).toBeVisible();

    // Stable per-item control id the renderer emits (#80 GREEN contract).
    await page.locator("#catalog-item-fe-leaf").click();
    // Assert within the detail panel (the table also contains these strings).
    const detail = page.locator("#library-detail");
    await expect(detail.getByRole("heading")).toContainText("fe-leaf");
    await expect(detail).toContainText("class");
    await expect(detail).toContainText("celestica-ds5000");
  });
});
