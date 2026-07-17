// p3.2-reference-gallery.spec.mjs — real-browser E2E (#80) for the Reference-
// topology gallery. Runs in headless chromium against a LIVE `aid serve`
// (`make ui-e2e`). The gallery reuses the existing /api/templates family and the
// existing template clone chain: "Use as starting point" creates a detached
// plan (not a live link) and lands on its detail, where it calculates valid.
//
// RED (#80): the navbar Reference button calls a stub loader (placeholder, no
// request), so these fail at the intended seam. #80 GREEN wires
// GET /api/templates + reference_gallery_html + the clone action.

import { test, expect } from "@playwright/test";

// Template-created plan ids derive from meta.case_id (stable). #87: the clone is
// now identity-mutated (collision-safe), so the created plan's name carries a
// " (copy)" suffix and its id a "-copy" suffix — the clone no longer overwrites
// the seeded reference plan.
const MESH_64_NAME = "Training XOC-64 1x OPG-64 Mesh Converged RO (copy)";

// Wait until the initial load_plans has SETTLED (the plan-list heading rendered),
// not merely until #app is non-empty (true at the loading spinner). Otherwise a
// navbar click races the in-flight load_plans fetch, whose late callback would
// clobber #app.
async function waitForApp(page) {
  await expect(page.getByRole("heading", { name: "Topology Plans" })).toBeVisible();
}

test.describe("AID GUI #80 — Reference-topology gallery", () => {
  test("navbar Reference opens the gallery with the shipped references", async ({ page }) => {
    await page.goto("/");
    await waitForApp(page);

    await page.locator("#nav-reference").click();

    await expect(page.getByRole("heading", { name: /Reference topologies/i })).toBeVisible();
    // The three shipped reference compositions appear as cards.
    await expect(page.getByText("XOC-64", { exact: false })).toBeVisible();
    await expect(page.getByText("XOC-256", { exact: false })).toBeVisible();
    await expect(page.getByText("XOC-128", { exact: false })).toBeVisible();
  });

  test("Use as starting point clones a reference into a new valid plan", async ({ page }) => {
    page.on("dialog", (d) => d.accept());
    await page.goto("/");
    await waitForApp(page);
    await page.locator("#nav-reference").click();
    await expect(page.getByRole("heading", { name: /Reference topologies/i })).toBeVisible();

    // Stable per-reference control id the gallery emits (#80 GREEN contract).
    await page.locator("#use-template-xoc-64-mesh").click();

    // Lands on the created plan's detail (detached clone) and computes valid.
    await expect(page.getByRole("heading", { name: MESH_64_NAME })).toBeVisible();
    await page.getByRole("button", { name: "Calculate" }).click();
    await expect(
      page.locator("#detail-result").getByText("Valid", { exact: true }),
    ).toBeVisible();
  });
});
