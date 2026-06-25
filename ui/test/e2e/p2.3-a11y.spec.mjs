// p2.3-a11y.spec.mjs — real-browser E2E for the accessibility pass (#72). No
// behavior changes: this asserts the a11y CONTRACT in a live DOM — table headers
// carry scope, async regions are exposed as live regions, the Valid/Invalid cue
// is not color-only (it has spoken text), and key controls are keyboard-operable.
//
// @axe-core/playwright is not vendored (air-gapped, no network install), so this
// falls back to explicit attribute assertions rather than a full axe scan. Runs
// against the seeded `aid serve` (BASE_URL), like golden-path.spec.mjs.

import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";

async function waitForApp(page) {
  await expect(page.locator("#app")).not.toBeEmpty();
}

// A fresh, valid mesh plan (the seeded list also holds an INVALID over-allocated
// plan whose BOM 422s — so the BOM test must target a known-good plan, not
// whatever sorts first in the shared serial store).
function meshFixture(id, name) {
  return readFileSync(
    new URL("../../../tests/oracle/xoc-64-mesh-conv-ro/training.yaml", import.meta.url),
    "utf8",
  ).replace(/training_xoc\w+/g, id).replace(/^(\s*name:).*/m, `$1 ${name}`);
}

test.describe("P2.3 accessibility (#72)", () => {
  test("plan-list table exposes scope=col headers + a caption", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);

    const table = page.locator("#app table").first();
    await expect(table.locator("caption")).toHaveText(/Topology plans/);
    // every visible column header carries scope="col".
    const headers = table.locator("thead th");
    const total = await headers.count();
    const scoped = await table.locator('thead th[scope="col"]').count();
    expect(scoped).toBe(total);
    expect(total).toBeGreaterThan(0);

    // the per-action error region is an assertive live region.
    await expect(page.locator("#list-error")).toHaveAttribute("aria-live", "assertive");
  });

  test("detail async regions are polite live regions; the validity cue has text", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.getByRole("button", { name: "View" }).first().click();
    await expect(page.locator("#edit-yaml")).toBeVisible();

    // live-validation + detail-result announce updates without stealing focus.
    await expect(page.locator("#live-validation")).toHaveAttribute("role", "status");
    await expect(page.locator("#live-validation")).toHaveAttribute("aria-live", "polite");
    await expect(page.locator("#detail-result")).toHaveAttribute("aria-live", "polite");
    // the edit error region announces assertively.
    await expect(page.locator("#edit-error")).toHaveAttribute("role", "alert");

    // the derived-facts validity badge is not color-only: it carries spoken text.
    const facts = page.locator("#detail-facts");
    await expect(facts).toContainText(/Valid|Invalid|not computable/);
    // the glyph inside the badge is decorative (hidden from assistive tech).
    await expect(facts.locator('.badge span[aria-hidden="true"]').first()).toBeAttached();
  });

  test("BOM table exposes scope=col headers + a caption", async ({ page }) => {
    // seed a fresh, valid plan so the BOM computes (avoids the seeded invalid one).
    const r = await page.request.post("/api/plans", {
      headers: { "content-type": "text/yaml" },
      data: meshFixture("p23_mesh", "P23 A11y Mesh"),
    });
    expect(r.ok()).toBeTruthy();

    await page.goto("/");
    await waitForApp(page);
    await page.locator("tr", { hasText: "P23 A11y Mesh" }).getByRole("button", { name: "View" }).click();
    await page.locator("#bom-btn").click();
    await expect(page.locator("#detail-result").getByText("Bill of Materials")).toBeVisible();

    const table = page.locator("#detail-result table");
    await expect(table.locator("caption")).toHaveText(/Line items:/);
    const total = await table.locator("thead th").count();
    const scoped = await table.locator('thead th[scope="col"]').count();
    expect(scoped).toBe(total);
    expect(total).toBeGreaterThan(0);
  });

  test("structured-editor controls have programmatic labels (aria-label / label-for)", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.getByRole("button", { name: "View" }).first().click();
    await expect(page.locator("#structure-editor")).not.toBeEmpty();

    // every <select> in the structured editor has an accessible name.
    const selects = page.locator("#structure-editor select");
    const n = await selects.count();
    expect(n).toBeGreaterThan(0);
    for (let i = 0; i < n; i++) {
      const label = await selects.nth(i).getAttribute("aria-label");
      expect(label, `select #${i} must have an aria-label`).toBeTruthy();
    }
  });

  test("key controls are keyboard-operable: focus + Enter opens a plan", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);

    const view = page.getByRole("button", { name: "View" }).first();
    await view.focus();
    await expect(view).toBeFocused(); // a real, tabbable <button>
    await page.keyboard.press("Enter");

    // keyboard activation navigated to the detail (no mouse used).
    await expect(page.locator("#edit-yaml")).toBeVisible();
  });
});
