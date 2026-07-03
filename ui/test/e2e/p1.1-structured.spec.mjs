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

  test("edit an existing server class's device type via the dropdown -> save -> persists", async ({ page }) => {
    await openMeshDetail(page);

    const dt = page.locator("#srv-compute_xpu-devtype");
    await expect(dt).toHaveValue("srv_xpu_generic_dt");
    await dt.selectOption("srv_storage_generic_dt");
    await page.locator("#save-srv-btn").click();

    await expect(page.locator("#srv-compute_xpu-devtype")).toHaveValue("srv_storage_generic_dt");
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

// #81: the three missing structured-create surfaces. These run on a THROWAWAY
// plan (a uniquely-named clone of the mesh-64 template) and delete it afterward,
// so they never mutate the shared seeded fixtures other specs calc against —
// notably add_switch_class introduces a new managed fabric, which would corrupt
// downstream calc if applied to the shared plan.
test.describe("P1.1 structured-create (#81)", () => {
  test.beforeEach(async ({ page }) => {
    page.on("dialog", (d) => d.accept()); // auto-accept the delete confirm
  });

  // openFreshMesh creates a throwaway mesh-64 plan (unique case_id) via the New-
  // from-template flow and opens its structured editor.
  async function openFreshMesh(page, id) {
    await page.goto("/");
    await expect(page.locator("#app")).not.toBeEmpty();
    const tplYaml = await page.evaluate(async () => {
      const r = await fetch("/api/templates/xoc-64-mesh");
      return (await r.json()).training;
    });
    const yaml = tplYaml
      .replace(/case_id:\s*training_xoc64_1xopg64_mesh_conv_ro/, "case_id: " + id)
      .replace(/name:\s*Training XOC-64 1x OPG-64 Mesh Converged RO/, "name: " + id);
    await page.getByRole("button", { name: "+ New plan" }).click();
    await page.locator("#new-yaml").evaluate((el, v) => {
      el.value = v;
      el.dispatchEvent(new Event("input", { bubbles: true }));
    }, yaml);
    await page.locator("#new-submit-btn").click();
    await expect(page.locator("#srv-compute_xpu-qty")).toBeVisible();
  }

  async function deletePlan(page, id) {
    await page.getByRole("button", { name: "← All plans" }).click();
    await page.locator(`#del-${id}`).click();
    await expect(page.getByText(id, { exact: true })).toHaveCount(0);
  }

  test("add a switch class via the form -> it appears in the editor", async ({ page }) => {
    const id = "e2e_swc_" + Date.now();
    await openFreshMesh(page, id);

    await page.fill("#add-swc-id", "extra_leaf");
    await page.fill("#add-swc-fabric-name", "extra-fabric");
    await page.locator("#add-swc-fabric-class").selectOption("managed");
    await page.locator("#add-swc-role").selectOption("server-leaf");
    await page.locator("#add-swc-devext").selectOption("sw_ds2000_inb_ext");
    await page.locator("#add-swc-topo").selectOption("mesh");
    await page.locator("#add-swc-btn").click();

    // The new switch class round-trips into the reloaded editor.
    await expect(page.locator("#sw-extra_leaf-topo")).toHaveValue("mesh");
    await deletePlan(page, id);
  });

  test("add a switch port zone via the form -> it appears as a target-zone option", async ({ page }) => {
    const id = "e2e_zone_" + Date.now();
    await openFreshMesh(page, id);

    await page.locator("#add-zone-swc").selectOption("soc_storage_scale_out_leaf");
    await page.fill("#add-zone-name", "extra_zone");
    await page.locator("#add-zone-type").selectOption("server");
    await page.fill("#add-zone-portspec", "1-4");
    await page.locator("#add-zone-breakout").selectOption("brk_2x400_osfp");
    await page.locator("#add-zone-xcvr").selectOption("osfp_400g_dr4");
    await page.locator("#add-zone-btn").click();

    // The new zone surfaces as a "switch_class/zone_name" connection target-zone
    // option in the reloaded editor.
    await expect(
      page.locator('option[value="soc_storage_scale_out_leaf/extra_zone"]').first(),
    ).toBeAttached();
    await deletePlan(page, id);
  });

  test("add a NIC via the form -> it appears on the server class", async ({ page }) => {
    const id = "e2e_nic_" + Date.now();
    await openFreshMesh(page, id);

    await page.locator("#add-nic-server").selectOption("compute_xpu");
    await page.fill("#add-nic-id", "extra_nic");
    await page.locator("#add-nic-module").selectOption("nic_dual_25g");
    await page.locator("#add-nic-btn").click();

    // The new NIC round-trips into the reloaded editor (keyed nic-<class>-<nic>).
    await expect(page.locator("#nic-compute_xpu-extra_nic")).toBeVisible();
    await deletePlan(page, id);
  });
});
