// p2.1-facts.spec.mjs — real-browser E2E for at-a-glance derived facts (#71):
// list rows + detail header show engine-derived topology / GPU / server / switch
// totals + validity, and the BOM view downloads a real CSV file.
//
// Posts its OWN fresh mesh + Clos plans (the shared serial e2e server mutates the
// seeded ones), so the asserted facts are deterministic.

import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";

function fixture(rel, id, name) {
  return readFileSync(new URL(rel, import.meta.url), "utf8")
    .replace(/training_xoc\w+/g, id)
    .replace(/^(\s*name:).*/m, `$1 ${name}`);
}
const MESH = fixture("../../../tests/oracle/xoc-64-mesh-conv-ro/training.yaml", "p21_mesh", "P21 Mesh Fixture");
const CLOS = fixture("../../../tests/oracle/xoc-256-2xopg128-clos-ro/training.yaml", "p21_clos", "P21 Clos Fixture");

async function streamToString(stream) {
  const chunks = [];
  for await (const c of stream) chunks.push(Buffer.from(c));
  return Buffer.concat(chunks).toString("utf8");
}

test.describe("P2.1 derived facts + BOM CSV (#71)", () => {
  test("list + detail show derived facts; BOM downloads a real CSV", async ({ page }) => {
    for (const data of [MESH, CLOS]) {
      const r = await page.request.post("/api/plans", { headers: { "content-type": "text/yaml" }, data });
      expect(r.ok()).toBeTruthy();
    }

    await page.goto("/");
    await expect(page.locator("#app")).not.toBeEmpty();

    // (1) list rows show engine-derived facts.
    const meshRow = page.locator("tr", { hasText: "P21 Mesh Fixture" });
    await expect(meshRow).toContainText("mesh");
    await expect(meshRow).toContainText("64 GPU"); // 8 compute × 8 gpus
    await expect(meshRow).toContainText("17 servers");
    await expect(meshRow).toContainText("4 switches"); // soc 2 + inb 1 + oob 1
    await expect(meshRow.locator(".text-bg-success")).toContainText("Valid");

    const closRow = page.locator("tr", { hasText: "P21 Clos Fixture" });
    await expect(closRow).toContainText("Clos");
    await expect(closRow).toContainText("9 switches"); // be-rail-leaf 4 + be-spine 2 + fe-leaf 2 + fe-spine 1
    await expect(closRow.locator(".text-bg-success")).toContainText("Valid");

    // (2) detail header shows the same facts.
    await meshRow.getByRole("button", { name: "View" }).click();
    await expect(page.locator("#detail-facts")).toContainText("mesh");
    await expect(page.locator("#detail-facts")).toContainText("64 GPU");
    await expect(page.locator("#detail-facts .text-bg-success")).toContainText("Valid");

    // (3) Download BOM (CSV) → a real file download whose content is the CSV BOM.
    await page.locator("#bom-btn").click();
    await expect(page.locator("#detail-result").getByText("Bill of Materials")).toBeVisible();
    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.locator("#bom-csv-btn").click(),
    ]);
    expect(download.suggestedFilename()).toBe("p21_mesh-bom.csv");
    const csv = await streamToString(await download.createReadStream());
    expect(csv).toContain("section,"); // the CSV header
    expect(csv).toContain("server,"); // a real BOM row
    expect(csv).not.toContain('"error"'); // never a poisoned error body
  });
});
