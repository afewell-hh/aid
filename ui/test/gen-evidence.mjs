// gen-evidence.mjs — regenerate the air-gapped GUI evidence under ui/docs/ by
// driving the REAL REST API + GUI in headless chromium against a live `aid serve`.
//
// This replaces the old fabricated generator (which rendered the bundle against
// canned JSON): every surface here is captured AFTER a real request round-trip
// (POST plan / PUT overlay were done by the seeder; calc / bom / wiring / facts /
// validation are computed by the live engine), so the evidence doubles as an
// end-to-end smoke test. It also records every network request the page issues
// and FAILS if any is non-same-origin (proves the air-gapped, no-CDN claim).
//
// Lifecycle is owned by ui/test/run-evidence.sh (boots a seeded server, exports
// BASE_URL + PLAYWRIGHT_BROWSERS_PATH, links node_modules, tears down). Invoked
// by `make ui-evidence`.

import { chromium } from "@playwright/test";
import { writeFileSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const BASE = process.env.BASE_URL;
if (!BASE) {
  console.error("gen-evidence: BASE_URL not set (run via `make ui-evidence`)");
  process.exit(1);
}

const here = dirname(fileURLToPath(import.meta.url)); // ui/test
const docsDir = join(here, "..", "docs"); // ui/docs
const evidenceDir = join(docsDir, "evidence");
const shotsDir = join(docsDir, "screenshots");
const updateScreenshots = process.env.AID_UPDATE_SCREENSHOTS === "1";
mkdirSync(evidenceDir, { recursive: true });
if (updateScreenshots) mkdirSync(shotsDir, { recursive: true });

// Start clean so a renamed/dropped surface never leaves an orphaned artifact
// (the evidence dir must reflect EXACTLY this run, not a union with prior runs).
for (const f of readdirSync(evidenceDir)) {
  if (f.endsWith(".html") || f === "requests.txt") rmSync(join(evidenceDir, f));
}
if (updateScreenshots) {
  for (const f of readdirSync(shotsDir)) {
    if (f.endsWith(".png")) rmSync(join(shotsDir, f));
  }
}

// Seeded oracle plan ids (deterministic, from ui/test/seed-oracle-plans.sh).
const MESH = "training_xoc64_1xopg64_mesh_conv_ro";
const CLOS = "training_xoc256_2xopg128_clos_ro"; // has an optic overlay attached
const OVERFLOW = "invalid_zone_overflow"; // over-allocated → Invalid-as-data

// Wrap a live-rendered DOM fragment in a self-contained, air-gapped page that
// links the locally-bundled Bootstrap CSS (relative to ui/docs/evidence/), so the
// file opens offline and looks like `aid serve` shipped it.
function page(title, note, inner) {
  return `<!doctype html>
<html lang="en" data-bs-theme="dark">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>AID — ${title}</title>
  <link rel="stylesheet" href="../../static/bootstrap.min.css">
</head>
<body class="bg-body">
  <nav class="navbar navbar-expand bg-dark border-bottom border-secondary mb-3">
    <div class="container"><span class="navbar-brand">AID — ${title}</span></div>
  </nav>
  <main class="container">
    <p class="text-muted small">${note}</p>
    ${inner}
  </main>
</body>
</html>
`;
}

const requests = []; // every URL the page fetches (air-gapped proof)

async function main() {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    baseURL: BASE,
    viewport: { width: 1280, height: 900 },
  });
  const p = await ctx.newPage();
  p.on("request", (r) => requests.push(r.url()));

  const writeSurface = (file, title, note, html) =>
    writeFileSync(join(evidenceDir, file), page(title, note, html));
  const captureApp = () => p.locator("#app").innerHTML();
  const captureRegion = (id) => p.locator(`#${id}`).innerHTML();
  const screenshot = async (name) => {
    if (updateScreenshots) {
      await p.screenshot({ path: join(shotsDir, name), fullPage: true });
    }
  };

  // 1. Plan list (GET /api/plans) — with the P2.1 derived-facts summary column.
  await p.goto("/");
  await p.locator("#app table").first().waitFor();
  writeSurface("01-plan-list.html", "Plan list",
    "GET /api/plans — each row shows the engine-derived facts (topology / GPU / servers / switches) and a validity cue.",
    await captureApp());
  await screenshot("01-plan-list.png");

  // 2. New-plan guided CHOICE surface (#87) — the create entry (GET /api/templates).
  await p.locator("#new-plan-btn").click();
  await p.locator("#choice-reference").waitFor();
  writeSurface("02-new-plan.html", "New plan (guided choice)",
    "The guided create surface (#87): choose a starting point — clone a built-in reference topology (primary path → structured editor), or import / paste DIET YAML (the expert escape hatch). The collision-safe clone gives each new plan a distinct id so repeat clones never overwrite the seed or a prior copy.",
    await captureApp());
  await screenshot("02-new-plan.png");

  // 3. Plan detail (GET /api/plans/{id}) — facts header, action toolbar, the
  //    structured editor (GET .../structure), and the optic-overlay tab.
  await p.goto("/");
  await p.locator(`#view-${MESH}`).click();
  await p.locator("#edit-yaml").waitFor();
  await p.locator("#structure-editor select").first().waitFor(); // structure loaded
  await p.locator("#overlay-section").waitFor();
  writeSurface("03-plan-detail.html", "Plan detail",
    "GET /api/plans/{id}: the derived-facts header, the action toolbar (one primary — Calculate), the structured server/switch/connection editor (dropdowns sourced from the plan's catalog), the raw-YAML escape hatch, and the optic-overlay tab.",
    await captureApp());
  await screenshot("03-plan-detail.png");

  // 3b. Optic / identity overlay tab in isolation (present, prefilled from PUT).
  writeSurface("07-overlay.html", "Optic / identity overlay",
    "GET .../overlay: presence badge + the attached overlay YAML (PUT .../overlay). Without it the BOM's optic columns are blank.",
    await captureRegion("overlay-section"));

  // 3c. Live validation (no persist): edit the YAML to trigger the debounced
  //     dry-run POST /api/validate and capture the inline result.
  await p.locator("#edit-yaml").focus();
  await p.locator("#edit-yaml").press("End");
  await p.locator("#edit-yaml").pressSequentially("\n");
  await p.locator("#live-validation .card, #live-validation .alert").first().waitFor();
  writeSurface("08-live-validation.html", "Live validation (as you design)",
    "Editing the plan fires a debounced dry-run (POST /api/validate) that validates WITHOUT persisting — the inline result shows the live Valid/Invalid verdict and computed quantities.",
    await captureRegion("live-validation"));

  // 4. Calc — valid (POST /api/plans/{id}/calc) + per-fabric wiring downloads.
  await p.locator("#calc-btn").click();
  await p.locator("#detail-result").getByText("Valid", { exact: true }).waitFor();
  writeSurface("04-calc-valid.html", "Calculate — valid",
    "POST /api/plans/{id}/calc: a green Valid cue, the engine-computed switch/server quantities per class, an endpoint/verdict summary, and a Download-wiring button per managed fabric (hhfab CRDs).",
    await captureRegion("detail-result"));
  await screenshot("04-calc.png");

  // 5. BOM (GET /api/plans/{id}/bom) — use the Clos plan (overlay attached →
  //    the optic columns are populated). View BOM, then capture.
  await p.goto("/");
  await p.locator(`#view-${CLOS}`).click();
  await p.locator("#bom-btn").click();
  await p.locator("#detail-result").getByText("Bill of Materials").waitFor();
  writeSurface("06-bom.html", "Bill of materials",
    "GET /api/plans/{id}/bom: a flat line-item table (section / model / class / manufacturer / quantity + optic standard) with a Download BOM (CSV) export. Optic columns are populated because the Clos plan carries an optic overlay.",
    await captureRegion("detail-result"));
  await screenshot("05-bom.png");

  // 6. Calc — invalid (calc-as-data): the over-allocated plan returns is_valid:
  //    false + ZONE_OVERFLOW as DATA (HTTP 200), rendered as the Invalid cue.
  await p.goto("/");
  await p.locator(`#view-${OVERFLOW}`).click();
  await p.locator("#calc-btn").click();
  await p.locator("#detail-result").getByText("Invalid", { exact: true }).waitFor();
  writeSurface("05-calc-invalid.html", "Calculate — invalid (errors as data)",
    "An over-allocated plan: the engine returns is_valid:false + ZONE_OVERFLOW as DATA (HTTP 200, not an error), rendered as a red Invalid cue with the constraint-violation list. No wiring is offered for an invalid calc.",
    await captureRegion("detail-result"));

  await browser.close();

  // Air-gapped proof: every request must be same-origin (served from the binary).
  const origin = new URL(BASE).origin;
  const external = [...new Set(requests)].filter((u) => !u.startsWith(origin) && !u.startsWith("data:") && !u.startsWith("blob:"));
  const manifest = [...new Set(requests)]
    .map((u) => u.replace(origin, ""))
    .sort()
    .join("\n");
  writeFileSync(join(evidenceDir, "requests.txt"),
    `# Requests issued by the GUI during evidence capture (driven against <same-origin aid serve>).\n` +
    `# external (non-same-origin) requests: ${external.length}\n\n${manifest}\n`);
  if (external.length > 0) {
    console.error("gen-evidence: FAIL — external requests detected (not air-gapped):\n" + external.join("\n"));
    process.exit(1);
  }

  const files = readdirSync(evidenceDir).filter((f) => f.endsWith(".html")).sort();
  console.log(`gen-evidence: wrote ${files.length} evidence pages from live ${origin}`);
  if (updateScreenshots) {
    console.log("gen-evidence: screenshots refreshed (AID_UPDATE_SCREENSHOTS=1)");
  } else {
    console.log("gen-evidence: screenshots unchanged (set AID_UPDATE_SCREENSHOTS=1 to refresh)");
  }
  console.log("  " + files.join("\n  "));
  console.log(`gen-evidence: air-gapped OK — 0 external requests across ${new Set(requests).size} unique URLs`);
}

main().catch((e) => {
  console.error("gen-evidence: ERROR", e);
  process.exit(1);
});
