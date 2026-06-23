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

// ---------------------------------------------------------------------------
// Issue #65 — P0.3: per-fabric wiring download + fabric discovery.
//
// After a VALID Calculate, the calc response carries the plan's managed fabric
// names (server-derived from switch_classes.fabric_name where fabric_class ==
// managed). The GUI renders one "Download wiring: <fabric>" button per fabric
// (stable id `wiring-<fabric>`); clicking it streams GET .../wiring/<fabric> and
// triggers a real browser file download (Blob + anchor in ffi.mbt save_file).
// These tests assert the buttons are populated from real data AND that the
// downloaded file is wiring CRDs, not an error body.
// ---------------------------------------------------------------------------
test.describe("AID GUI P0.3 — wiring download + fabric discovery", () => {
  // mesh xoc-64 managed fabrics (derived server-side): inb-mgmt, soc-storage-scale-out.
  test("mesh (xoc-64): per-fabric Download buttons appear and download real wiring YAML", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${MESH_64_ID}`).click();
    await page.getByRole("button", { name: "Calculate" }).click();

    const result = page.locator("#detail-result");
    await expect(result.getByText("Valid", { exact: true })).toBeVisible();
    // Buttons populated from real managed-fabric names (NOT guessed).
    const soc = result.locator("#wiring-soc-storage-scale-out");
    await expect(soc).toBeVisible();
    await expect(soc).toContainText("Download wiring: soc-storage-scale-out");
    await expect(result.locator("#wiring-inb-mgmt")).toBeVisible();

    // Clicking triggers a real browser download; capture it and assert the file
    // name + that the content is wiring CRDs, not an {"error":...} body.
    const [download] = await Promise.all([page.waitForEvent("download"), soc.click()]);
    expect(download.suggestedFilename()).toBe(`${MESH_64_ID}-soc-storage-scale-out.yaml`);
    const stream = await download.createReadStream();
    const yaml = await streamToString(stream);
    expect(yaml).toContain("wiring.githedgehog.com");
    expect(yaml).not.toContain('"error"');
  });

  // Clos xoc-256 managed fabrics (derived server-side): backend, frontend.
  test("Clos (xoc-256): per-fabric Download buttons appear and download real wiring YAML", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${CLOS_256_ID}`).click();
    await page.getByRole("button", { name: "Calculate" }).click();

    const result = page.locator("#detail-result");
    await expect(result.getByText("Valid", { exact: true })).toBeVisible();
    const frontend = result.locator("#wiring-frontend");
    await expect(frontend).toBeVisible();
    await expect(result.locator("#wiring-backend")).toBeVisible();

    const [download] = await Promise.all([page.waitForEvent("download"), frontend.click()]);
    expect(download.suggestedFilename()).toBe(`${CLOS_256_ID}-frontend.yaml`);
    const stream = await download.createReadStream();
    const yaml = await streamToString(stream);
    expect(yaml).toContain("wiring.githedgehog.com");
    expect(yaml).not.toContain('"error"');
  });

  // An over-allocated (invalid) calc must NOT offer wiring downloads — quantities
  // are unreliable, so there is nothing valid to export.
  test("invalid calc offers no wiring download buttons", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${OVERFLOW_ID}`).click();
    await page.getByRole("button", { name: "Calculate" }).click();

    const result = page.locator("#detail-result");
    await expect(result.getByText("Invalid", { exact: true })).toBeVisible();
    await expect(result.getByText(/Download wiring/)).toHaveCount(0);
  });
});

// ---------------------------------------------------------------------------
// Issue #65 — P0.4: HTTP / network error handling.
//
// The fetch FFI now delivers {ok, status, body} (+ a .catch for rejections), and
// load_plans/load_detail/load_bom render a shared error alert for a non-2xx
// status, an {"error":...} body, or a network failure — instead of a misleading
// empty/ghost view. Calc-as-data (is_valid:false) is NOT an error and still shows
// the Invalid badge (asserted above + re-confirmed here). download_wiring never
// saves a non-OK body to a .yaml.
// ---------------------------------------------------------------------------
test.describe("AID GUI P0.4 — HTTP / network error handling", () => {
  // Network failure: a stopped/unreachable server. Simulated by aborting the API
  // request at the network layer, exercising the FFI .catch path for real. The
  // GUI must show a clear error, NOT an empty "0 plan(s)" table.
  test("network failure renders an error alert, not an empty account", async ({ page }) => {
    await page.route("**/api/plans", (route) => route.abort());
    await page.goto("/");
    await waitForApp(page); // #app gets the error alert, so it is non-empty

    const app = page.locator("#app");
    await expect(app.locator(".alert-danger")).toBeVisible();
    await expect(app.getByText(/Network error/i)).toBeVisible();
    await expect(app.getByText("0 plan(s)", { exact: true })).toHaveCount(0);
    await expect(app.getByRole("heading", { name: "Topology Plans" })).toHaveCount(0);
  });

  // A 404 on the plan-detail GET must render the error alert, NOT a ghost detail
  // card with live-but-broken Calculate/View-BOM buttons.
  test("404 on plan detail renders an error alert, not a ghost card", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    // Intercept only the detail GET (leave the list intact) and force a 404.
    await page.route(`**/api/plans/${MESH_64_ID}`, (route) =>
      route.fulfill({
        status: 404,
        contentType: "application/json",
        body: JSON.stringify({ error: "plan not found" }),
      }),
    );
    await page.locator(`#view-${MESH_64_ID}`).click();

    const app = page.locator("#app");
    await expect(app.locator(".alert-danger")).toBeVisible();
    await expect(app.getByText(/404/)).toBeVisible();
    await expect(page.getByRole("button", { name: "Calculate" })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "View BOM" })).toHaveCount(0);
  });

  // Bad fabric: the REAL server returns 404 + the valid-fabric list (P0.3 server
  // fix), and the download_wiring guard surfaces the error instead of silently
  // saving an {"error":...} body to a .yaml. Driven against the LIVE server via
  // the exported download_wiring (no UI affordance produces a bad fabric, by
  // design — buttons come from real data). Asserts: error shown (not silent), and
  // NO download fired.
  test("bad fabric: real 404 + valid list, error shown, no file saved", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${MESH_64_ID}`).click();
    // Need #detail-result in the DOM (the guard renders the error there).
    await expect(page.locator("#detail-result")).toBeAttached();

    // Fail the test if any download fires (a non-OK body must NEVER be saved).
    let downloaded = false;
    page.on("download", () => {
      downloaded = true;
    });

    // First confirm the server really answers 404 + valid_fabrics for a bad fabric.
    const resp = await page.request.get(`/api/plans/${MESH_64_ID}/wiring/nonsuch`);
    expect(resp.status()).toBe(404);
    const errBody = await resp.json();
    expect(errBody.error).toBeTruthy();
    expect(errBody.valid_fabrics).toEqual(["inb-mgmt", "soc-storage-scale-out"]);

    // Now drive the real GUI download_wiring against the bad fabric.
    await page.evaluate(async (id) => {
      const m = await import("/static/app.js");
      m.download_wiring(id, "nonsuch");
    }, MESH_64_ID);

    const result = page.locator("#detail-result");
    await expect(result.locator(".alert-danger")).toBeVisible();
    await expect(result.getByText(/unknown fabric/i)).toBeVisible();
    // Give any (erroneous) download a beat to fire, then assert none did.
    await page.waitForTimeout(300);
    expect(downloaded).toBe(false);
  });

  // Re-confirm the two-plane boundary end-to-end: a 200 is_valid:false calc is
  // DATA (the Invalid badge + ZONE_OVERFLOW list), NOT the generic error alert.
  // (Complements the P0.3 "no wiring buttons" assertion on the same plan.)
  test("calc-error (422-like) stays calc-as-data: Invalid badge, not the error alert", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#view-${OVERFLOW_ID}`).click();
    await page.getByRole("button", { name: "Calculate" }).click();

    const result = page.locator("#detail-result");
    await expect(result.getByText("Invalid", { exact: true })).toBeVisible();
    await expect(result.getByText("ZONE_OVERFLOW").first()).toBeVisible();
    // The generic HTTP error alert must NOT appear (this is data, not an error).
    await expect(result.getByText(/Network error/i)).toHaveCount(0);
    await expect(result.getByText(/Request failed \(HTTP/)).toHaveCount(0);
  });
});

// ---------------------------------------------------------------------------
// Issue #65 — P0.2: in-browser authoring (create / edit / delete / duplicate).
//
// These drive the REAL authoring UI against the LIVE seeded server: New-from-
// template, raw-YAML edit, delete, and the create error path. They CREATE new
// plans at runtime (distinct ids) and clean up via the Delete affordance so the
// seeded read-only fixtures are unaffected. window.confirm is auto-accepted via a
// dialog handler so the delete-confirm step does not hang headless chromium.
// ---------------------------------------------------------------------------
test.describe("AID GUI P0.2 — create / edit / delete authoring", () => {
  // Auto-accept the native confirm() (delete) dialog for every test here.
  test.beforeEach(async ({ page }) => {
    page.on("dialog", (d) => d.accept());
  });

  // setTextarea sets a (large) textarea's value directly + fires input, instead of
  // page.fill — fill() is pathologically slow (~12s) on the 24KB plan YAML because
  // it re-verifies the value char-by-char. This is still a real DOM mutation the
  // app reads back via get_value(). Small inputs still use fill() for fidelity.
  async function setTextarea(page, selector, value) {
    await page.locator(selector).waitFor();
    await page.locator(selector).evaluate((el, v) => {
      el.value = v;
      el.dispatchEvent(new Event("input", { bubbles: true }));
    }, value);
  }

  // (a) New-from-template (mesh xoc-64): pick the starter -> the YAML textarea
  // prefills -> Create -> lands on the new plan's detail -> Calculate shows Valid
  // + the known quantities -> View BOM shows non-blank optic identity (the
  // template's overlay was attached, so the BOM is full, not blank optics).
  //
  // NOTE: a template-created plan's derived id == the template's case_id, which is
  // the SAME id as the seeded mesh-64 fixture; creating it re-writes the identical
  // plan + overlay (idempotent). We deliberately do NOT delete here — that would
  // remove the shared read-only fixture other tests rely on. (The dedicated delete
  // coverage uses throwaway unique-id plans below.)
  test("New from template (mesh) -> create -> Calculate Valid + full BOM (optics populated)", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForApp(page);

    await page.getByRole("button", { name: "+ New plan" }).click();
    await expect(page.getByRole("heading", { name: "New plan" })).toBeVisible();

    // Choose the xoc-64 mesh starter; the textarea prefills from GET the template.
    await page.locator("#new-template").selectOption("xoc-64-mesh");
    await expect(page.locator("#new-yaml")).toHaveValue(/case_id:\s*training_xoc64/);

    await page.locator("#new-submit-btn").click();

    // Landed on the created plan's detail (derived id == the template's case_id).
    await expect(page.getByRole("heading", { name: MESH_64_NAME })).toBeVisible();
    await expect(page.getByRole("button", { name: "Calculate" })).toBeVisible();

    // Calculate: Valid + the known mesh quantities (proves the created plan really
    // computes against the live kernel).
    await page.getByRole("button", { name: "Calculate" }).click();
    const result = page.locator("#detail-result");
    await expect(result.getByText("Valid", { exact: true })).toBeVisible();
    await expect(result.locator("tr", { hasText: "soc_storage_scale_out_leaf" })).toContainText("2");
    await expect(result.locator("tr", { hasText: "compute_xpu" })).toContainText("8");

    // View BOM: the attached template overlay populates optic identity — assert a
    // non-blank optical standard cell (blank without the overlay).
    await page.getByRole("button", { name: "View BOM" }).click();
    await expect(result.getByRole("heading", { name: "Bill of Materials" })).toBeVisible();
    await expect(result.getByText(/400GBASE-DR4|200GBASE-SR2/).first()).toBeVisible();
    // Still present in the list afterward (the seeded fixture remains intact).
    await page.getByRole("button", { name: "← All plans" }).click();
    await expect(page.getByText(MESH_64_ID, { exact: true })).toBeVisible();
  });

  // (b) Edit: create a throwaway plan from pasted YAML, open it, bump a
  // server-class quantity in the textarea, Save, re-Calculate -> the changed
  // quantity is reflected. Then delete it.
  test("Edit the YAML -> Save -> re-Calculate reflects the change, then delete", async ({
    page,
  }) => {
    const id = "e2e_edit_" + Date.now();
    // A minimal real DIET plan: clone xoc-64 training via the template, but with a
    // unique case_id so it does not collide with the seeded fixture. We fetch the
    // template YAML through the page, rewrite identity + a quantity, and paste it.
    await page.goto("/");
    await waitForApp(page);
    const tplYaml = await page.evaluate(async () => {
      const r = await fetch("/api/templates/xoc-64-mesh");
      return (await r.json()).training;
    });
    const yaml = tplYaml
      .replace(/case_id:\s*training_xoc64_1xopg64_mesh_conv_ro/, "case_id: " + id)
      .replace(/name:\s*Training XOC-64 1x OPG-64 Mesh Converged RO/, "name: E2E Edit Plan");

    await page.getByRole("button", { name: "+ New plan" }).click();
    await setTextarea(page, "#new-yaml", yaml);
    await page.locator("#new-submit-btn").click();
    await expect(page.getByRole("heading", { name: "E2E Edit Plan" })).toBeVisible();

    // Baseline calc: compute_xpu == 8.
    await page.getByRole("button", { name: "Calculate" }).click();
    const result = page.locator("#detail-result");
    await expect(result.locator("tr", { hasText: "compute_xpu" })).toContainText("8");

    // Edit: drop compute_xpu quantity 8 -> 4 in the editable textarea, Save.
    const edited = yaml.replace(/quantity:\s*8/, "quantity: 4");
    await setTextarea(page, "#edit-yaml", edited);
    await page.locator("#save-btn").click();
    // Re-render of detail after save; re-Calculate and assert the new quantity.
    await expect(page.getByRole("button", { name: "Calculate" })).toBeVisible();
    await page.getByRole("button", { name: "Calculate" }).click();
    await expect(result.locator("tr", { hasText: "compute_xpu" })).toContainText("4");

    // Clean up.
    await page.getByRole("button", { name: "Delete" }).click();
    await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
    await expect(page.getByText(id, { exact: true })).toHaveCount(0);
  });

  // (c) Delete from the list row: create a throwaway plan, then delete it via the
  // per-row Delete button and assert it disappears from the table.
  test("Delete from the list row removes the plan", async ({ page }) => {
    const id = "e2e_del_" + Date.now();
    await page.goto("/");
    await waitForApp(page);
    const tplYaml = await page.evaluate(async () => {
      const r = await fetch("/api/templates/xoc-64-mesh");
      return (await r.json()).training;
    });
    const yaml = tplYaml.replace(/case_id:\s*training_xoc64_1xopg64_mesh_conv_ro/, "case_id: " + id);

    await page.getByRole("button", { name: "+ New plan" }).click();
    await setTextarea(page, "#new-yaml", yaml);
    await page.locator("#new-submit-btn").click();
    // Back to the list (via the detail Back link) to use the row Delete.
    await page.getByRole("button", { name: "← All plans" }).click();
    await expect(page.getByText(id, { exact: true })).toBeVisible();

    await page.locator(`#del-${id}`).click();
    await expect(page.getByText(id, { exact: true })).toHaveCount(0);
  });

  // (d) Create error path: submit malformed YAML -> the shared error alert renders
  // IN the form and NO ghost plan appears in the list.
  test("malformed YAML create shows the error alert, no ghost plan", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.getByRole("button", { name: "+ New plan" }).click();
    await page.locator("#new-yaml").fill("this: : not: valid: yaml\n  - [");
    await page.locator("#new-submit-btn").click();

    // Error alert in the form; still on the New-plan screen (no navigation).
    await expect(page.locator("#new-error .alert-danger")).toBeVisible();
    await expect(page.getByRole("heading", { name: "New plan" })).toBeVisible();

    // Cancel back to the list and confirm no ghost plan was created.
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
    // The seeded count is 4; a failed create must not have added a row.
    await expect(page.getByText("4 plan(s)", { exact: true })).toBeVisible();
  });

  // (e) Duplicate: clone a seeded plan; the copy appears with a -copy id suffix,
  // computes Valid, then delete the clone (leaving the original).
  test("Duplicate a plan -> clone appears, computes Valid, then delete the clone", async ({
    page,
  }) => {
    await page.goto("/");
    await waitForApp(page);
    await page.locator(`#dup-${MESH_64_ID}`).click();

    const cloneId = `${MESH_64_ID}-copy`;
    await expect(page.getByText(cloneId, { exact: true })).toBeVisible();

    // The clone calculates Valid (identity-suffixed copy of a valid plan).
    await page.locator(`#view-${cloneId}`).click();
    await page.getByRole("button", { name: "Calculate" }).click();
    await expect(page.locator("#detail-result").getByText("Valid", { exact: true })).toBeVisible();

    // Clean up the clone.
    await page.getByRole("button", { name: "← All plans" }).click();
    await page.locator(`#del-${cloneId}`).click();
    await expect(page.getByText(cloneId, { exact: true })).toHaveCount(0);
  });
});

// streamToString drains a Node Readable (download.createReadStream) to a string.
async function streamToString(stream) {
  const chunks = [];
  for await (const chunk of stream) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks).toString("utf8");
}
