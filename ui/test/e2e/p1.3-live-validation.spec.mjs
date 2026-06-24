// p1.3-live-validation.spec.mjs — real-browser E2E for debounced live validation
// (#68): a meaningful edit (structured form OR raw-YAML) auto-validates after the
// debounce and renders the result inline in #live-validation, WITHOUT clicking
// Calculate. Three render states: Valid, Invalid-as-data, and the distinct
// structural "cannot compute" alert. Runs against the seeded `aid serve`.

import { test, expect } from "@playwright/test";

const MESH_64_NAME = "Training XOC-64 1x OPG-64 Mesh Converged RO";

async function openMeshDetail(page) {
  await page.goto("/");
  await expect(page.locator("#app")).not.toBeEmpty();
  await page
    .locator("tr", { hasText: MESH_64_NAME })
    .getByRole("button", { name: "View" })
    .click();
  await expect(page.locator("#srv-compute_xpu-qty")).toBeVisible();
}

test.describe("P1.3 live validation (#68)", () => {
  test("structured over-allocating edit -> inline Invalid (no Calculate click); fix -> Valid", async ({ page }) => {
    await openMeshDetail(page);
    const live = page.locator("#live-validation");

    // Bump compute servers far past the soc/storage leaf zone capacity.
    await page.fill("#srv-compute_xpu-qty", "12");
    // No Calculate click — the debounced live validator surfaces the violation.
    await expect(live.getByText("Invalid", { exact: true })).toBeVisible();
    await expect(live).toContainText("ZONE_OVERFLOW");

    // Fix it back to a valid count -> live validation flips to Valid.
    await page.fill("#srv-compute_xpu-qty", "8");
    await expect(live.getByText("Valid", { exact: true })).toBeVisible();
  });

  test("raw-YAML valid edit -> inline Valid after debounce", async ({ page }) => {
    await openMeshDetail(page);
    const live = page.locator("#live-validation");

    // A YAML comment keeps the plan valid; the textarea edit triggers live-validate.
    const cur = await page.locator("#edit-yaml").inputValue();
    await page.locator("#edit-yaml").fill(cur + "\n# touched by live-validation e2e\n");
    await expect(live.getByText("Valid", { exact: true })).toBeVisible();
  });

  test("raw-YAML parse-broken draft -> distinct 'cannot compute' alert", async ({ page }) => {
    await openMeshDetail(page);
    const live = page.locator("#live-validation");

    await page.locator("#edit-yaml").fill("this: : is: not: valid: yaml\n  - [");
    await expect(live.locator(".alert-danger")).toBeVisible();
    await expect(live).toContainText(/cannot/i);
    // A structural failure must NOT be rendered as a green Valid badge.
    await expect(live.locator(".text-bg-success")).toHaveCount(0);
  });
});
