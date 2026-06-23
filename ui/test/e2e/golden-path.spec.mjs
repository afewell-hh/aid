// golden-path.spec.mjs — real-browser E2E for the AID GUI read-only flow.
//
// These run in headless chromium against a LIVE `aid serve` (managed by the
// `make ui-e2e` target), which has already seeded the vendored oracle plans via
// the real `POST /api/plans`. Assertions are on the VISIBLE rendered DOM (real
// clicks + text), never on internals — selectors stay resilient (roles, text,
// stable ids the renderer emits in render.mbt / app.mbt).
//
// Env:
//   BASE_URL   — seeded server (the 3 oracle plans)
//   EMPTY_URL  — a second server with an empty plans dir (empty-state case)
//
// Plan identity is derived from meta.case_id, so the ids are stable:
//   xoc-64  (mesh) -> training_xoc64_1xopg64_mesh_conv_ro
//   xoc-256 (Clos) -> training_xoc256_2xopg128_clos_ro
//   xoc-128 (mesh) -> training_xoc128_2xopg64_mesh_conv_ro

import { test, expect } from "@playwright/test";

const MESH_64_ID = "training_xoc64_1xopg64_mesh_conv_ro";
const MESH_64_NAME = "Training XOC-64 1x OPG-64 Mesh Converged RO";
const CLOS_256_ID = "training_xoc256_2xopg128_clos_ro";
const CLOS_256_NAME = "Training XOC-256 2x OPG-128 Clos RO";
// Seeded by the harness: a copy of xoc-64 with compute_xpu inflated past the
// soc/storage leaf server-zone capacity, so the live kernel calc returns
// is_valid:false with ZONE_OVERFLOW errors as DATA (HTTP 200). Identity derived
// from meta.case_id (see Makefile seed step).
const OVERFLOW_ID = "invalid_zone_overflow";
const OVERFLOW_NAME = "Invalid Zone Overflow (xoc-64 over-allocated)";

// Waits for the MoonBit bundle to have populated #app (main_entry -> load_plans
// is async over fetch).
async function waitForApp(page) {
  await expect(page.locator("#app")).not.toBeEmpty();
}

test.describe("AID GUI golden path (read-only)", () => {
  // (a) initial load renders the plan list from GET /api/plans.
  test("initial load renders the plan list from the API", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);

    await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
    // The vendored oracle plans are present by name and by id (rendered in a
    // <code> cell). Three plans seeded.
    await expect(page.getByText(MESH_64_NAME, { exact: true })).toBeVisible();
    await expect(page.getByText(CLOS_256_NAME, { exact: true })).toBeVisible();
    await expect(page.getByText(MESH_64_ID, { exact: true })).toBeVisible();
    await expect(page.getByText("4 plan(s)", { exact: true })).toBeVisible();
    // Each row has a View action button (resilient: by visible name).
    await expect(page.getByRole("button", { name: "View" })).toHaveCount(4);
  });

  // (b) empty-state: an empty store renders the table shell with "0 plan(s)"
  // and no View buttons (the current GUI has no dedicated empty message; this
  // asserts the actual rendered behavior, not an aspirational one).
  test("empty store renders zero plans and no rows", async ({ page }) => {
    const emptyURL = process.env.EMPTY_URL;
    test.skip(!emptyURL, "EMPTY_URL not provided by the harness");
    await page.goto(emptyURL + "/");
    await waitForApp(page);

    await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
    await expect(page.getByText("0 plan(s)", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "View" })).toHaveCount(0);
  });

  // (c) click a plan's View -> detail renders.
  test("View navigates to the plan detail", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);

    // The mesh-64 row's View button has a stable id `view-<plan id>`.
    await page.locator(`#view-${MESH_64_ID}`).click();

    // Detail renders the plan name as a heading + the Calculate / View BOM
    // actions and the plan id.
    await expect(page.getByRole("heading", { name: MESH_64_NAME })).toBeVisible();
    await expect(page.getByRole("button", { name: "Calculate" })).toBeVisible();
    await expect(page.getByRole("button", { name: "View BOM" })).toBeVisible();
    await expect(page.locator("#detail-result")).toBeAttached();
  });

  // (d) Calculate on a valid MESH plan (xoc-64) -> Valid badge + quantities.
  test("Calculate on the mesh plan (xoc-64) shows Valid + switch/server quantities", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${MESH_64_ID}`).click();
    await page.getByRole("button", { name: "Calculate" }).click();

    const result = page.locator("#detail-result");
    await expect(result.getByText("Valid", { exact: true })).toBeVisible();
    await expect(result.getByText("Invalid", { exact: true })).toHaveCount(0);

    // Validation card structure.
    await expect(result.getByText("Switch quantities", { exact: true })).toBeVisible();
    await expect(result.getByText("Server quantities", { exact: true })).toBeVisible();

    // Specific computed quantities (from the live kernel calc):
    //   switch soc_storage_scale_out_leaf = 2 ; server compute_xpu = 8.
    const switchRow = result.locator("tr", { hasText: "soc_storage_scale_out_leaf" });
    await expect(switchRow).toContainText("2");
    const serverRow = result.locator("tr", { hasText: "compute_xpu" });
    await expect(serverRow).toContainText("8");
  });

  // (d') Calculate on a valid CLOS plan (xoc-256) -> Valid badge + quantities.
  test("Calculate on the Clos plan (xoc-256) shows Valid + switch/server quantities", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${CLOS_256_ID}`).click();
    await page.getByRole("button", { name: "Calculate" }).click();

    const result = page.locator("#detail-result");
    await expect(result.getByText("Valid", { exact: true })).toBeVisible();
    await expect(result.getByText("Invalid", { exact: true })).toHaveCount(0);

    // Clos fabric: switch fe-leaf = 2 ; server compute = 32.
    const switchRow = result.locator("tr", { hasText: "fe-leaf" });
    await expect(switchRow).toContainText("2");
    const serverRow = result.locator("tr", { hasText: /^compute/ });
    await expect(serverRow).toContainText("32");
  });

  // (e) Calc-errors-as-data: Invalid badge + error text.
  //
  // No VENDORED oracle plan resolves to is_valid:false, so the harness seeds a
  // synthetic over-allocation: a copy of xoc-64 with compute_xpu inflated past
  // the soc/storage leaf server-zone capacity. The live kernel surfaces this as
  // DATA — HTTP 200, is_valid:false, populated ZONE_OVERFLOW errors — which the
  // GUI renders as the Invalid badge + a danger error list (calc_summary_html /
  // render.mbt). This is the two-plane "validation as data" contract, distinct
  // from a structural 4xx danger alert.
  test("Calculate on an over-allocated plan shows Invalid + ZONE_OVERFLOW error text", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForApp(page);
    await expect(page.getByText(OVERFLOW_NAME, { exact: true })).toBeVisible();
    await page.locator(`#view-${OVERFLOW_ID}`).click();
    await expect(page.getByRole("heading", { name: OVERFLOW_NAME })).toBeVisible();
    await page.getByRole("button", { name: "Calculate" }).click();

    const result = page.locator("#detail-result");
    await expect(result.getByText("Invalid", { exact: true })).toBeVisible();
    await expect(result.getByText("Valid", { exact: true })).toHaveCount(0);
    // The error list renders the constraint violations as danger list items.
    await expect(result.locator(".list-group-item-danger").first()).toBeVisible();
    await expect(result.getByText("ZONE_OVERFLOW").first()).toBeVisible();
  });

  // (f) View BOM -> rows render.
  test("View BOM renders the bill-of-materials rows", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${MESH_64_ID}`).click();
    await page.getByRole("button", { name: "View BOM" }).click();

    const result = page.locator("#detail-result");
    await expect(result.getByRole("heading", { name: "Bill of Materials" })).toBeVisible();
    // Header columns + at least several data rows (the live BOM has 21 rows).
    await expect(result.locator("thead").getByText("Section", { exact: true })).toBeVisible();
    const dataRows = result.locator("tbody tr");
    expect(await dataRows.count()).toBeGreaterThan(5);
    await expect(result.getByText(/Suppressed cable assemblies:/)).toBeVisible();
  });
});
