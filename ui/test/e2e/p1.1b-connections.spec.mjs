// p1.1b-connections.spec.mjs — real-browser E2E for the structured connections
// editor (#69): retarget a connection's target_zone via the dropdown -> save ->
// persists + live-validates; add a connection via the form -> persists + computes;
// remove a connection -> reflected. Runs against the seeded `aid serve`.

import { test, expect } from "@playwright/test";

const MESH_64_NAME = "Training XOC-64 1x OPG-64 Mesh Converged RO";
const NEW_ZONE = "soc_storage_scale_out_leaf/soc_storage_server_4x200";

async function openMeshDetail(page) {
  await page.goto("/");
  await expect(page.locator("#app")).not.toBeEmpty();
  await page
    .locator("tr", { hasText: MESH_64_NAME })
    .getByRole("button", { name: "View" })
    .click();
  await expect(page.locator("#conn-0-target_zone")).toBeVisible();
}

test.describe("P1.1b connections editor (#69)", () => {
  test("retarget a connection's target_zone via dropdown -> live-validates + persists", async ({ page }) => {
    await openMeshDetail(page);

    await page.locator("#conn-0-target_zone").selectOption(NEW_ZONE);
    // The edit triggers the debounced live validation (no Calculate click).
    await expect(page.locator("#live-validation").getByText("Validation")).toBeVisible();

    await page.locator("#save-conn-btn").click();
    // Persisted: after the reload the connection's dropdown shows the new zone.
    await expect(page.locator("#conn-0-target_zone")).toHaveValue(NEW_ZONE);
  });

  test("add a connection via the form -> it appears and the plan computes", async ({ page }) => {
    await openMeshDetail(page);

    await page.fill("#addconn-hh_controller-id", "extra-inb");
    await page.locator("#addconn-hh_controller-target_zone").selectOption("inb_mgmt_leaf/inb_mgmt_server_25g");
    await page.locator("#addconn-hh_controller-nic").selectOption("inb_mgmt");
    await page.fill("#addconn-hh_controller-speed", "25");
    await page.locator("#addconn-hh_controller").click();

    // The new connection round-trips into the reloaded structured editor.
    await expect(page.locator("#structure-editor")).toContainText("extra-inb");
    // And the edited plan still computes.
    await page.locator("#calc-btn").click();
    await expect(page.locator("#detail-result").getByText("Valid", { exact: true })).toBeVisible();
  });

  test("remove a connection -> reflected in the editor", async ({ page }) => {
    await openMeshDetail(page);
    // connection index 0 is scale-out-rail-0 (first server_connections row).
    await expect(page.locator("#structure-editor")).toContainText("scale-out-rail-0");
    await page.locator("#conn-rm-0").click();
    await expect(page.locator("#conn-0-target_zone")).toBeVisible(); // editor reloaded
    await expect(page.locator("#structure-editor")).not.toContainText("scale-out-rail-0");
  });
});
