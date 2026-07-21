// p3.3-new-plan-choice.spec.mjs — real-browser E2E (#87) for the guided new-plan
// CHOICE surface + collision-safe cloning. Runs in headless chromium against a
// LIVE seeded `aid serve` (managed by `make ui-e2e`), like p3.2-reference-gallery.
//
// RED (#87): `+ New plan` still opens the raw-YAML form (no #choice-reference /
// #choice-import), and both clone paths derive colliding ids (verbatim seed /
// fixed `-copy`), so these fail at the intended seam. #87 GREEN adds the choice
// renderer + the shared collision-aware identity helper and turns these green.
//
// Fixture isolation (per #87): assertions never overwrite or count on exact
// totals of seeded oracle ids; created plans use suffixed NON-seed ids and are
// asserted by presence of their view-<id> control. The repeat tests each use a
// DISTINCT seeded reference (xoc-256 clone / xoc-128 duplicate) so they cannot
// race other specs (or each other) on the same id — the server-side duplicate
// guard is a deferred #87 non-goal, so same-id concurrency is avoided by design.

import { test, expect } from "@playwright/test";

// Seeded oracle plan ids (identity derived from meta.case_id; stable).
const SEED_64 = "training_xoc64_1xopg64_mesh_conv_ro";
const CLOS_256 = "training_xoc256_2xopg128_clos_ro";
const MESH_128 = "training_xoc128_2xopg64_mesh_conv_ro";

// Wait until the initial load_plans has SETTLED (plan-list heading rendered), not
// merely until #app is non-empty (true at the loading spinner). Otherwise a
// navbar/button click races the in-flight load_plans fetch.
async function waitForApp(page) {
  await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
}

async function openChoice(page) {
  await page.locator("#new-plan-btn").click();
  await expect(page.locator("#choice-reference")).toBeVisible();
  await expect(page.locator("#choice-import")).toBeVisible();
}

// The harness runs workers:1 / fullyParallel:false, so these run sequentially in
// one worker (no races). Each test navigates fresh and each state-mutating test
// uses a DISTINCT seeded reference (xoc-256 clone / xoc-128 duplicate), so they
// stay deterministic without describe.serial — and in RED every test is
// independently observable as failing at its own seam.
test.describe("AID GUI #87 — guided new-plan choice + collision-safe clone", () => {
  test("+ New plan opens an intentional choice surface (reference + import)", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);

    await page.locator("#new-plan-btn").click();

    // The choice surface, not the raw YAML textarea, is the default landing.
    await expect(page.locator("#choice-reference")).toBeVisible();
    await expect(page.locator("#choice-import")).toBeVisible();
    await expect(page.locator("#new-yaml")).toHaveCount(0);
  });

  test("primary reference path -> clone xoc-64 -> structured editor + valid calc", async ({ page }) => {
    page.on("dialog", (d) => d.accept());
    await page.goto("/");
    await waitForApp(page);
    await openChoice(page);

    // Primary path surfaces the shipped references; pick xoc-64.
    await page.locator("#choice-reference").click();
    await page.locator("#use-template-xoc-64-mesh").click();

    // Lands on the created (detached, identity-mutated) plan's detail: the
    // structured editor renders and the plan calculates valid. The clone's name
    // carries a " (copy)" suffix (identity mutation); assert the plan-name heading.
    await expect(page.getByRole("heading", { name: /Training XOC-64/i })).toBeVisible();
    await expect(page.locator("#structure-editor")).toBeVisible();
    await page.getByRole("button", { name: "Calculate" }).click();
    await expect(
      page.locator("#detail-result").getByText("Valid", { exact: true }),
    ).toBeVisible();

    // The seeded oracle plan was NOT overwritten by the clone.
    await page.locator("#nav-home").click();
    await waitForApp(page);
    await expect(page.locator(`#view-${SEED_64}`)).toBeVisible();
  });

  test("repeat reference clone -> distinct ids, seed + first clone preserved", async ({ page }) => {
    page.on("dialog", (d) => d.accept());
    await page.goto("/");
    await waitForApp(page);

    // Clone the xoc-256 reference twice (isolated from other specs).
    for (let i = 0; i < 2; i++) {
      await openChoice(page);
      await page.locator("#choice-reference").click();
      await page.locator("#use-template-xoc-256-clos").click();
      await expect(page.getByRole("heading", { name: /Training XOC-256/i })).toBeVisible();
      await page.locator("#nav-home").click();
      await waitForApp(page);
    }

    // Seed survives; at least two DISTINCT non-seed clones now exist.
    await expect(page.locator(`#view-${CLOS_256}`)).toBeVisible();
    const clones = page.locator(`[id^="view-${CLOS_256}-copy"]`);
    await expect(clones).toHaveCount(2, { timeout: 10000 });
  });

  test("repeat Duplicate -> distinct ids, first duplicate preserved", async ({ page }) => {
    page.on("dialog", (d) => d.accept());
    await page.goto("/");
    await waitForApp(page);

    // Duplicate the seeded xoc-128 plan twice (isolated from other specs).
    await page.locator(`#dup-${MESH_128}`).click();
    await expect(page.locator(`[id^="view-${MESH_128}-copy"]`)).toHaveCount(1);
    await page.locator(`#dup-${MESH_128}`).click();

    // Source survives; two DISTINCT duplicates exist (first not overwritten).
    await expect(page.locator(`#view-${MESH_128}`)).toBeVisible();
    await expect(page.locator(`[id^="view-${MESH_128}-copy"]`)).toHaveCount(2, { timeout: 10000 });
  });

  test("expert import path opens the paste form; malformed paste shows an error", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);
    await openChoice(page);

    await page.locator("#choice-import").click();
    // The raw-YAML escape hatch (D25) is one click away under the expert path.
    await expect(page.locator("#new-yaml")).toBeVisible();

    // A malformed body is rejected server-side: in-form error, no navigation.
    await page.locator("#new-yaml").fill("this: : not: valid");
    await page.locator("#new-submit-btn").click();
    await expect(page.locator("#new-error")).toContainText(/invalid|error|400/i);
    // Still on the create surface (no ghost plan / navigation).
    await expect(page.locator("#new-yaml")).toBeVisible();
  });
});
