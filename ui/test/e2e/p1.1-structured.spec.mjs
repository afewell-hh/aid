// p1.1-structured.spec.mjs — real-browser E2E for the structured editor (#67):
// edit a server-class quantity via the form -> save -> re-Calculate reflects it;
// flip a switch class mesh->clos via the selector -> save -> persists + recomputes;
// add a server class via the form -> it appears. Runs against the seeded
// `aid serve` (BASE_URL) like golden-path.spec.mjs.

import { test, expect } from "@playwright/test";

const MESH_64_NAME = "Training XOC-64 1x OPG-64 Mesh Converged RO";

async function openMeshDetail(page) {
  await page.goto("/");
  await expect(page.locator("#app")).not.toBeEmpty();
  await page
    .locator("tr", { hasText: MESH_64_NAME })
    .getByRole("button", { name: "View" })
    .click();
  // the structured editor loads asynchronously into #structure-editor
  await expect(page.locator("#srv-compute_xpu-qty")).toBeVisible();
}

test.describe("P1.1 structured editor (#67)", () => {
  test("edit a server-class quantity via the form -> save -> Calculate reflects it", async ({ page }) => {
    await openMeshDetail(page);

    // Bump a small infra class (no zone-capacity risk): hh_controller 1 -> 2.
    await expect(page.locator("#srv-hh_controller-qty")).toHaveValue("1");
    await page.fill("#srv-hh_controller-qty", "2");
    await page.locator("#save-srv-btn").click();

    // After save the detail reloads; the structured editor shows the persisted 2.
    await expect(page.locator("#srv-hh_controller-qty")).toHaveValue("2");

    // Re-Calculate: the computed server quantity for hh_controller reflects 2.
    await page.locator("#calc-btn").click();
    const result = page.locator("#detail-result");
    await expect(result.getByText("Valid", { exact: true })).toBeVisible();
    await expect(result.locator("tr", { hasText: "hh_controller" })).toContainText("2");
  });

  test("flip a switch class mesh->clos via the selector -> save -> persists + recomputes", async ({ page }) => {
    await openMeshDetail(page);

    const topo = page.locator("#sw-soc_storage_scale_out_leaf-topo");
    await expect(topo).toHaveValue("mesh");
    await topo.selectOption("clos");
    await page.locator("#save-sw-btn").click();

    // The flip round-trips: after the reload the selector shows clos.
    await expect(page.locator("#sw-soc_storage_scale_out_leaf-topo")).toHaveValue("clos");

    // Recompute runs on the edited plan (a validation result renders).
    await page.locator("#calc-btn").click();
    await expect(page.locator("#detail-result").getByText("Validation")).toBeVisible();
  });

  test("add a server class via the form -> it appears in the editor", async ({ page }) => {
    await openMeshDetail(page);

    await page.fill("#add-srv-id", "extra_compute");
    await page.fill("#add-srv-qty", "2");
    await page.fill("#add-srv-gpus", "8");
    await page.locator("#add-srv-devtype").selectOption("srv_xpu_generic_dt");
    await page.locator("#add-srv-btn").click();

    // The new class round-trips and shows up in the reloaded structured editor.
    await expect(page.locator("#srv-extra_compute-qty")).toHaveValue("2");
    await expect(page.locator("#srv-extra_compute-gpus")).toHaveValue("8");
  });
});
