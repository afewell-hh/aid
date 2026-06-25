// p2.4-cleanup.spec.mjs — real-browser E2E for the P2.4 cleanup (#73):
// the detail action toolbar (one clear primary), clickable list rows, and the
// removal of the dead #result div. Runs against the seeded `aid serve`
// (BASE_URL), like golden-path.spec.mjs.

import { test, expect } from "@playwright/test";

const MESH_64_ID = "training_xoc64_1xopg64_mesh_conv_ro";

async function waitForApp(page) {
  await expect(page.locator("#app")).not.toBeEmpty();
}

test.describe("P2.4 cleanup (#73)", () => {
  test("the dead #result div is gone from the shell", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await expect(page.locator("#result")).toHaveCount(0);
  });

  test("clicking a row body (not the View button) opens the plan detail", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);

    // click the row's ID cell — a non-interactive area, so on_click_row fires.
    await page.locator(`#row-${MESH_64_ID} code`).click();
    await expect(page.locator("#edit-yaml")).toBeVisible(); // detail rendered
  });

  test("detail actions are grouped in one toolbar with a single primary (Calculate)", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${MESH_64_ID}`).click();
    await expect(page.locator("#edit-yaml")).toBeVisible();

    const toolbar = page.locator('[role="toolbar"][aria-label="Plan actions"]');
    await expect(toolbar).toBeVisible();
    // Calculate is the single primary within the action toolbar.
    await expect(toolbar.locator("#calc-btn")).toHaveClass(/btn-primary/);
    await expect(toolbar.locator(".btn-primary")).toHaveCount(1);
    await expect(toolbar.locator("#bom-btn")).toBeVisible();
    await expect(toolbar.locator("#detail-del-btn")).toBeVisible();

    // the grouped primary still triggers a real calc (no behavior regression).
    await page.locator("#calc-btn").click();
    await expect(page.locator("#detail-result").getByText("Valid", { exact: true })).toBeVisible();
  });
});
