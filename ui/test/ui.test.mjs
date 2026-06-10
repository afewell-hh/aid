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

const CALC = JSON.stringify({
  ir: { nodes: [{}, {}, {}], edges: [], fabrics: [{}] },
  validation: { is_valid: true, errors: [], warnings: [] },
});

// switch-bom BOM: a multi-level device class with per-unit AND fleet quantities.
const BOM = JSON.stringify({
  include_fleet_totals: true,
  boms: [
    {
      entry_id: "leaf-switches",
      plan_quantity: 2,
      device_class: { name: "Reference 64-port 800G Leaf Switch" },
      line_items: [
        { level: 0, name: "Reference 64-port 800G Leaf Switch", quantity_per_unit: 1, fleet_quantity: 2 },
        { level: 1, name: "OSFP 800G Transceiver", quantity_per_unit: 64, fleet_quantity: 128 },
      ],
    },
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

test("render_bom: per-unit AND fleet quantities", () => {
  reset();
  app.render_bom("app", BOM);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /per[\s-]?unit/i, "expected a per-unit column/label");
  assert.match(html, /fleet/i, "expected a fleet column/label");
  assert.match(html, /OSFP 800G Transceiver/, "expected the sub-component");
  // Per-unit 64 and fleet 128 for the transceiver must both appear.
  assert.match(html, /64/, "expected the per-unit quantity 64");
  assert.match(html, /128/, "expected the fleet quantity 128");
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

test("trigger_calc: POST /api/plans/{id}/calc and show validation", async () => {
  reset();
  setResponder(() => CALC);
  app.trigger_calc("result", "clos-small");
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans/clos-small/calc" && f.method === "POST"),
    `expected POST .../clos-small/calc; got ${JSON.stringify(fetches)}`,
  );
  assert.match(dom["result"]?.innerHTML ?? "", /valid/i, "expected a validation summary");
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
