// gen-evidence.mjs — regenerate the air-gapped F7c GUI evidence under
// ui/docs/evidence/. It drives the REAL compiled bundle (../static/app.js) via
// the same DOM/fetch stub as the Node smoke tests (../test/harness.mjs) against
// canned F7b-shape API responses, and writes each rendered surface as a
// self-contained HTML page that links the locally-bundled Bootstrap CSS (no CDN,
// no browser/network needed). Run: `node ui/docs/gen-evidence.mjs`.
//
// This is the air-gapped substitute for headless-Chromium PNG capture (no browser
// tooling in this env). The markup is byte-identical to what `aid serve` ships,
// since it comes from the same app.js.

import { writeFileSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { dom, setResponder, reset, flush } from "../test/harness.mjs";
import * as app from "../static/app.js";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = join(here, "evidence");
mkdirSync(outDir, { recursive: true });

// Canned F7b REST shapes (mirror ui/test/ui.test.mjs).
const PLANS = JSON.stringify({
  plans: [
    { id: "xoc-256", name: "XOC-256 (2x OPG-128) Clos RO", status: "active" },
    { id: "xoc-64", name: "XOC-64 Mesh Converged RO", status: "draft" },
  ],
});
const DETAIL = JSON.stringify({
  id: "xoc-256",
  name: "XOC-256 (2x OPG-128) Clos RO",
  status: "active",
  yaml: "meta:\n  case_id: training_xoc256_2xopg128_clos_ro\n  name: XOC-256 Clos RO\nswitch_classes:\n  - switch_class_id: fe-leaf\n    fabric_name: frontend\n",
});
const CALC = JSON.stringify({
  is_valid: true,
  errors: [],
  switch_quantity: [
    { class_id: "be-rail-leaf", quantity: 4 },
    { class_id: "be-spine", quantity: 2 },
    { class_id: "fe-leaf", quantity: 2 },
    { class_id: "fe-spine", quantity: 1 },
  ],
  server_quantity: [{ class_id: "compute", quantity: 32 }],
  endpoints: new Array(9).fill({}),
  transceiver_verdicts: [{ connection_id: "c1", outcome: "match", reason_code: "" }],
});
const CALC_INVALID = JSON.stringify({
  is_valid: false,
  errors: [{ code: "ZONE_OVERFLOW", message: "zone scale_out_server_2x400 over-allocated" }],
  switch_quantity: [],
  server_quantity: [],
  endpoints: [],
  transceiver_verdicts: [],
});
const CALC_STRUCTURAL = JSON.stringify({ error: "cannot resolve plan: ingest failed" });
const BOM = JSON.stringify({
  suppressed_cable_assembly_count: 0,
  rows: [
    { section: "server", module_type_model: "OPG-256 Compute Server FE-BE", hedgehog_class: "compute", manufacturer: "Generic", quantity: "32" },
    { section: "switch", module_type_model: "celestica-ds5000", hedgehog_class: "be-rail-leaf", manufacturer: "Celestica", quantity: "4" },
    { section: "switch", module_type_model: "celestica-ds5000", hedgehog_class: "fe-spine", manufacturer: "Celestica", quantity: "1" },
    { section: "switch_transceiver", module_type_model: "QSFP112-200GBASE-SR2", hedgehog_class: "", manufacturer: "Generic", quantity: "528" },
  ],
});

function page(title, inner) {
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
  <main class="container">${inner}</main>
</body>
</html>
`;
}

async function calcInto(target, response) {
  reset();
  setResponder(() => response);
  app.trigger_calc(target, "demo");
  await flush();
  return dom[target]?.innerHTML ?? "";
}

// 1. Plan list
reset();
app.render_plan_list("app", PLANS);
writeFileSync(join(outDir, "01-plan-list.html"), page("Plan list", dom["app"].innerHTML));

// 2. Plan detail
reset();
app.render_plan_detail("app", DETAIL);
writeFileSync(join(outDir, "02-plan-detail.html"), page("Plan detail", dom["app"].innerHTML));

// 3. Calc — valid (new CalcOutput shape: quantities + endpoint/verdict summary)
writeFileSync(join(outDir, "03-calc-valid.html"), page("Calc — valid", await calcInto("r", CALC)));

// 4. Calc — invalid (calc errors surfaced as data: is_valid:false + code)
writeFileSync(join(outDir, "04-calc-invalid.html"), page("Calc — invalid (errors as data)", await calcInto("r", CALC_INVALID)));

// 5. Calc — structural failure (4xx {"error":...}, distinct danger alert)
writeFileSync(join(outDir, "05-calc-structural.html"), page("Calc — structural failure", await calcInto("r", CALC_STRUCTURAL)));

// 6. BOM — flat rows[]
reset();
app.render_bom("app", BOM);
writeFileSync(join(outDir, "06-bom.html"), page("BOM", dom["app"].innerHTML));

console.log("wrote air-gapped evidence to ui/docs/evidence/");
