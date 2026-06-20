// ui.test.mjs — Stage-B RED Node ESM smoke tests (issue #11).
//
// These drive the REAL compiled MoonBit->JS exports (../static/app.js) against a
// stubbed document/fetch (harness.mjs) and assert the produced DOM markup and
// the requests issued for the core surfaces: plan list, plan detail, calc
// trigger, BOM per-unit/fleet, wiring export, and the app bootstrap.
//
// RED: render.mbt builders return "" and download_wiring/main_entry issue no
// request, so every assertion below currently fails — the UI is not implemented
// yet. GREEN implements render.mbt + the wiring and turns these green. The
// FFI + request plumbing they ride on is real (spike-proven).

import { test } from "node:test";
import assert from "node:assert/strict";
import { dom, fetches, setResponder, reset, flush } from "./harness.mjs";
import * as app from "../static/app.js";

// Canned API responses mirroring the Stage-A contract.
const PLANS = JSON.stringify({
  plans: [{ id: "clos-small", name: "Small Clos Reference", status: "draft" }],
});

const DETAIL = JSON.stringify({
  id: "clos-small",
  name: "Small Clos Reference",
  status: "draft",
  yaml: "id: clos-small\nfabric_domains:\n  - fabric_id: frontend\n",
});

// F7b calc shape: CalcOutput flattened + is_valid (no more ir{nodes,edges,fabrics}).
const CALC = JSON.stringify({
  is_valid: true,
  errors: [],
  switch_quantity: [
    { class_id: "fe-leaf", quantity: 2 },
    { class_id: "be-spine", quantity: 2 },
  ],
  server_quantity: [{ class_id: "compute", quantity: 32 }],
  endpoints: [{}, {}, {}],
  transceiver_verdicts: [{ connection_id: "c1", outcome: "match", reason_code: "" }],
});

// F7b two-plane validation: calc constraint violations are DATA (is_valid:false).
const CALC_INVALID = JSON.stringify({
  is_valid: false,
  errors: [{ code: "ZONE_OVERFLOW", message: "zone scale_out_server_2x400 over-allocated" }],
  switch_quantity: [],
  server_quantity: [],
  endpoints: [],
  transceiver_verdicts: [],
});

// F7b structural failure (a 4xx body): {"error": ...} — distinct from calc data.
const CALC_STRUCTURAL = JSON.stringify({ error: "cannot resolve plan: ingest failed" });

// F7b BOM shape: flat rows[] (string cells) + suppressed_cable_assembly_count.
const BOM = JSON.stringify({
  suppressed_cable_assembly_count: 0,
  rows: [
    { section: "server", module_type_model: "OPG-256 Compute Server FE-BE", hedgehog_class: "compute", manufacturer: "Generic", quantity: "32" },
    { section: "switch", module_type_model: "celestica-ds5000", hedgehog_class: "be-rail-leaf", manufacturer: "Celestica", quantity: "4" },
    { section: "switch_transceiver", module_type_model: "QSFP112-200GBASE-SR2", hedgehog_class: "", manufacturer: "Generic", quantity: "528" },
  ],
});

test("render_plan_list: Bootstrap table with plan name + status badge", () => {
  reset();
  app.render_plan_list("app", PLANS);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /<table/i, "expected a Bootstrap table");
  assert.match(html, /Small Clos Reference/, "expected the plan name");
  assert.match(html, /badge/i, "expected a status badge");
  assert.match(html, /draft/i, "expected the status text");
});

test("render_plan_detail: cards for fabric + validation", () => {
  reset();
  app.render_plan_detail("app", DETAIL);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /card/i, "expected Bootstrap cards");
  assert.match(html, /frontend/, "expected the fabric id");
  assert.match(html, /Small Clos Reference/, "expected the plan name");
});

test("render_bom: flat rows[] with section/model/class/qty", () => {
  reset();
  app.render_bom("app", BOM);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /Bill of Materials/i, "expected the BOM heading");
  assert.match(html, /celestica-ds5000/, "expected the switch model row");
  assert.match(html, /be-rail-leaf/, "expected the hedgehog class");
  assert.match(html, />\s*4\s*</, "expected the switch quantity 4");
  assert.match(html, /OPG-256 Compute Server FE-BE/, "expected the server row");
  assert.match(html, />\s*32\s*</, "expected the server quantity 32");
  assert.match(html, /QSFP112-200GBASE-SR2/, "expected the transceiver row");
  assert.match(html, />\s*528\s*</, "expected the transceiver quantity 528");
});

test("load_plans: GET /api/plans and render the list", async () => {
  reset();
  setResponder(() => PLANS);
  app.load_plans("app");
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans" && f.method === "GET"),
    `expected GET /api/plans; got ${JSON.stringify(fetches)}`,
  );
  assert.match(dom["app"]?.innerHTML ?? "", /Small Clos Reference/, "expected the list rendered");
});

test("trigger_calc: POST .../calc → Valid badge + computed quantities", async () => {
  reset();
  setResponder(() => CALC);
  app.trigger_calc("result", "clos-small");
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans/clos-small/calc" && f.method === "POST"),
    `expected POST .../clos-small/calc; got ${JSON.stringify(fetches)}`,
  );
  const html = dom["result"]?.innerHTML ?? "";
  assert.match(html, /valid/i, "expected a validity badge");
  assert.match(html, /fe-leaf/, "expected a switch class id");
  assert.match(html, /compute/, "expected a server class id");
  assert.match(html, />\s*32\s*</, "expected the server quantity 32");
});

test("trigger_calc invalid: calc errors surfaced as data (is_valid:false + code)", async () => {
  reset();
  setResponder(() => CALC_INVALID);
  app.trigger_calc("result", "bad");
  await flush();
  const html = dom["result"]?.innerHTML ?? "";
  assert.match(html, /invalid/i, "expected the Invalid badge");
  assert.match(html, /ZONE_OVERFLOW/, "expected the calc error code");
  assert.match(html, /over-allocated/, "expected the calc error message");
});

test("trigger_calc structural error: distinct from calc-as-data", async () => {
  reset();
  setResponder(() => CALC_STRUCTURAL);
  app.trigger_calc("result", "broken");
  await flush();
  const html = dom["result"]?.innerHTML ?? "";
  assert.match(html, /cannot resolve plan/i, "expected the structural error message");
  assert.doesNotMatch(html, /badge[^>]*text-bg-success/, "structural error must not render a Valid badge");
});

test("download_wiring: GET /api/plans/{id}/wiring/{fabric}", async () => {
  reset();
  setResponder(() => "apiVersion: wiring.githedgehog.com/v1beta1\n");
  app.download_wiring("clos-small", "frontend");
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans/clos-small/wiring/frontend" && f.method === "GET"),
    `expected GET .../wiring/frontend; got ${JSON.stringify(fetches)}`,
  );
});

test("main_entry: bootstraps by loading the plan list", async () => {
  reset();
  setResponder(() => PLANS);
  app.main_entry();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans" && f.method === "GET"),
    `expected main_entry to GET /api/plans; got ${JSON.stringify(fetches)}`,
  );
});
