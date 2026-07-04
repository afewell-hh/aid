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
import { dom, el, fetches, saved, setResponder, setConfirm, reset, flush } from "./harness.mjs";
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

// --- P0.3: per-fabric wiring download buttons + download guard ----------------

// Calc with managed_fabrics renders a Download-wiring button per fabric, each
// with a stable id `wiring-<fabric>`, and clicking one issues the real
// GET .../wiring/<fabric> and saves the YAML.
const CALC_WITH_FABRICS = JSON.stringify({
  is_valid: true,
  errors: [],
  managed_fabrics: ["backend", "frontend"],
  switch_quantity: [{ class_id: "fe-leaf", quantity: 2 }],
  server_quantity: [{ class_id: "compute", quantity: 32 }],
  endpoints: [{}],
  transceiver_verdicts: [{}],
});

test("trigger_calc: renders a Download-wiring button per managed fabric", async () => {
  reset();
  setResponder(() => CALC_WITH_FABRICS);
  app.trigger_calc("result", "clos-small");
  await flush();
  const html = dom["result"]?.innerHTML ?? "";
  assert.match(html, /Download wiring: backend/, "expected a backend download button");
  assert.match(html, /Download wiring: frontend/, "expected a frontend download button");
  assert.match(html, /id="wiring-backend"/, "expected the stable backend button id");
  assert.match(html, /id="wiring-frontend"/, "expected the stable frontend button id");
});

test("wiring button click: GET .../wiring/<fabric> and saves the YAML", async () => {
  reset();
  setResponder((url) =>
    url.endsWith("/calc")
      ? CALC_WITH_FABRICS
      : "apiVersion: wiring.githedgehog.com/v1beta1\nkind: Switch\n",
  );
  app.trigger_calc("result", "clos-small");
  await flush();
  // The rendered button is wired to download_wiring; click it.
  dom["wiring-frontend"].click();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans/clos-small/wiring/frontend" && f.method === "GET"),
    `expected GET .../wiring/frontend; got ${JSON.stringify(fetches)}`,
  );
  assert.equal(saved.length, 1, "expected exactly one file download");
  assert.match(saved[0].content, /wiring\.githedgehog\.com/, "expected real wiring CRDs saved");
});

test("invalid calc renders no wiring buttons (nothing to download)", async () => {
  reset();
  setResponder(() => CALC_INVALID);
  app.trigger_calc("result", "bad");
  await flush();
  const html = dom["result"]?.innerHTML ?? "";
  assert.doesNotMatch(html, /Download wiring/, "invalid calc must not offer wiring downloads");
});

// download_wiring GUARD: a non-OK response (e.g. 404 bad fabric) is NEVER saved
// to a .yaml — it surfaces the error alert in #detail-result instead.
test("download_wiring guard: 404 bad fabric shows error, saves nothing", async () => {
  reset();
  setResponder(() => ({
    ok: false,
    status: 404,
    body: JSON.stringify({ error: "unknown fabric: nope", valid_fabrics: ["backend"] }),
  }));
  app.download_wiring("clos-small", "nope");
  await flush();
  assert.equal(saved.length, 0, "a non-OK wiring response must NOT be saved to a file");
  const html = dom["detail-result"]?.innerHTML ?? "";
  assert.match(html, /alert-danger/, "expected an error alert");
  assert.match(html, /unknown fabric/, "expected the server error message surfaced");
});

// --- P0.4: HTTP / network error handling --------------------------------------

test("load_plans: HTTP 500 renders the error alert, NOT an empty table", async () => {
  reset();
  setResponder(() => ({ ok: false, status: 500, body: JSON.stringify({ error: "internal error" }) }));
  app.load_plans("app");
  await flush();
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /alert-danger/, "expected an error alert");
  assert.match(html, /500/, "expected the HTTP status surfaced");
  assert.doesNotMatch(html, /0 plan\(s\)/, "an outage must not look like an empty account");
});

test("load_plans: network failure (rejected fetch) renders a network error", async () => {
  reset();
  setResponder(() => {
    throw new Error("connection refused");
  });
  app.load_plans("app");
  await flush();
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /alert-danger/, "expected an error alert on network failure");
  assert.match(html, /Network error/i, "expected a network-failure message");
});

test("load_bom: 422 (calc-gated) renders error, NOT a misleading empty BOM", async () => {
  // Navigate list -> detail (which wires the BOM button), then click BOM and get
  // a 422. load_bom is internal, so it is exercised through the real button.
  reset();
  const route = (url) => {
    if (url === "/api/plans") return PLANS;
    if (url.endsWith("/bom"))
      return { ok: false, status: 422, body: JSON.stringify({ error: "cannot compute BOM: plan has calc errors" }) };
    return DETAIL; // GET /api/plans/{id}
  };
  setResponder(route);
  app.load_plans("app");
  await flush();
  dom["view-clos-small"].click();
  await flush();
  dom["bom-btn"].click();
  await flush();
  const html = dom["detail-result"]?.innerHTML ?? "";
  assert.match(html, /alert-danger/, "expected an error alert, not an empty BOM");
  assert.match(html, /422/, "expected the HTTP status surfaced");
  assert.doesNotMatch(html, /Suppressed cable assemblies: 0/, "must not render a misleading empty BOM");
});

test("load_detail: 404 renders the error alert, NOT a ghost detail card", async () => {
  reset();
  setResponder((url) =>
    url === "/api/plans"
      ? PLANS
      : { ok: false, status: 404, body: JSON.stringify({ error: "plan not found" }) },
  );
  app.load_plans("app");
  await flush();
  dom["view-clos-small"].click();
  await flush();
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /alert-danger/, "expected an error alert");
  assert.match(html, /404/, "expected the HTTP status surfaced");
  assert.doesNotMatch(html, /id="calc-btn"/, "a 404 must not render a ghost detail card with live buttons");
});

// --- P0.2: create / edit / delete / duplicate authoring -----------------------
//
// These drive the REAL compiled exports + the wired on_click/on_change handlers
// against the harness DOM, asserting (a) the request SHAPE for the new verbs
// (POST create, PUT edit, DELETE, PUT overlay), and (b) the rendered authoring
// surfaces (New-plan form, delete-confirm). The full create->calc round-trip
// against the live kernel is covered by the Playwright E2E; here we assert the
// client wiring deterministically.

const TEMPLATES = JSON.stringify({
  templates: [
    { id: "xoc-64-mesh", name: "XOC-64 Mesh", topology: "mesh" },
    { id: "xoc-256-clos", name: "XOC-256 Clos", topology: "clos" },
  ],
});
const TEMPLATE_64 = JSON.stringify({
  id: "xoc-64-mesh",
  name: "XOC-64 Mesh",
  topology: "mesh",
  training: "meta:\n  case_id: training_xoc64_1xopg64_mesh_conv_ro\n  name: XOC-64\n",
  overlay: "module_types:\n  - id: x\n    cage_type: OSFP\n",
});

// new_plan_form_html: renders a name field, a template <select> populated from the
// catalog, and a YAML textarea (the raw-YAML escape hatch).
test("new_plan_form_html: name + template select + YAML textarea", () => {
  reset();
  const html = app.new_plan_form_html(TEMPLATES);
  assert.match(html, /New plan/, "expected the form heading");
  assert.match(html, /id="new-name"/, "expected the name field");
  assert.match(html, /id="new-template"/, "expected the template picker");
  assert.match(html, /XOC-64 Mesh \(mesh\)/, "expected a template option from the catalog");
  assert.match(html, /Blank/, "expected the Blank (paste your own) option");
  assert.match(html, /id="new-yaml"/, "expected the YAML textarea");
  assert.match(html, /id="new-submit-btn"/, "expected the Create button");
});

// new_plan_form_html tolerates an empty/garbage catalog (paste flow still works).
test("new_plan_form_html: empty catalog still renders the Blank option + textarea", () => {
  reset();
  const html = app.new_plan_form_html("{}");
  assert.match(html, /id="new-yaml"/, "expected the YAML textarea even with no templates");
  assert.match(html, /Blank/, "expected the Blank option");
});

// "New plan" -> form -> choose template -> textarea prefilled -> submit POSTs the
// YAML and (template chosen) PUTs the overlay, then routes to the new detail.
test("create from template: prefill + POST plan + PUT overlay + route to detail", async () => {
  reset();
  const route = (url, opts) => {
    if (url === "/api/plans" && (!opts.method || opts.method === "GET")) return "{\"plans\":[]}";
    if (url === "/api/templates") return TEMPLATES;
    if (url === "/api/templates/xoc-64-mesh") return TEMPLATE_64;
    if (url === "/api/plans" && opts.method === "POST")
      return JSON.stringify({ id: "training_xoc64_1xopg64_mesh_conv_ro", name: "XOC-64", status: "" });
    if (url.endsWith("/overlay") && opts.method === "PUT") return { status: 204, body: "" };
    // GET the created plan detail after routing.
    return JSON.stringify({ id: "training_xoc64_1xopg64_mesh_conv_ro", name: "XOC-64", status: "", yaml: "meta:\n  case_id: training_xoc64_1xopg64_mesh_conv_ro\n" });
  };
  setResponder(route);
  app.main_entry();
  await flush();
  dom["new-plan-btn"].click(); // open the form (GET /api/templates)
  await flush();
  // Choose a template -> on_change prefills the textarea from GET /api/templates/{id}.
  dom["new-template"].change("xoc-64-mesh");
  await flush();
  assert.match(dom["new-yaml"].value, /case_id: training_xoc64/, "textarea prefilled from the template");
  // Submit -> POST /api/plans, then PUT overlay, then route to detail.
  dom["new-submit-btn"].click();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans" && f.method === "POST"),
    `expected POST /api/plans; got ${JSON.stringify(fetches.map((f) => f.method + " " + f.url))}`,
  );
  assert.ok(
    fetches.some(
      (f) => f.url === "/api/plans/training_xoc64_1xopg64_mesh_conv_ro/overlay" && f.method === "PUT",
    ),
    "expected the template overlay to be PUT so the BOM is complete",
  );
  // Landed on the detail (heading rendered into #app).
  assert.match(dom["app"]?.innerHTML ?? "", /XOC-64/, "expected to route to the created plan detail");
});

// Create error path: a malformed POST body returns a 400 error -> the error alert
// renders in the form and NO navigation/ghost plan happens.
test("create error: malformed YAML shows the error alert, no ghost navigation", async () => {
  reset();
  const route = (url, opts) => {
    if (url === "/api/plans" && (!opts.method || opts.method === "GET")) return "{\"plans\":[]}";
    if (url === "/api/templates") return TEMPLATES;
    if (url === "/api/plans" && opts.method === "POST")
      return { ok: false, status: 400, body: JSON.stringify({ error: "planstore: invalid plan: bad yaml" }) };
    return "{}";
  };
  setResponder(route);
  app.main_entry();
  await flush();
  dom["new-plan-btn"].click();
  await flush();
  el("new-yaml").value = "this: : not: valid";
  dom["new-submit-btn"].click();
  await flush();
  const err = dom["new-error"]?.innerHTML ?? "";
  assert.match(err, /alert-danger/, "expected the error alert in the form");
  assert.match(err, /invalid plan|400/i, "expected the server error surfaced");
});

// Edit: detail textarea is editable; Save PUTs the edited YAML to /api/plans/{id}.
test("edit: Save PUTs the edited YAML to /api/plans/{id}", async () => {
  reset();
  const detail = JSON.stringify({ id: "clos-small", name: "Small Clos", status: "draft", yaml: "meta:\n  case_id: clos-small\n  name: Small Clos\n" });
  const route = (url, opts) => {
    if (url === "/api/plans" && (!opts.method || opts.method === "GET")) return PLANS;
    if (url === "/api/plans/clos-small" && opts.method === "PUT")
      return JSON.stringify({ id: "clos-small", name: "Renamed", status: "draft" });
    return detail; // GET /api/plans/clos-small (before + after save)
  };
  setResponder(route);
  app.load_plans("app");
  await flush();
  dom["view-clos-small"].click();
  await flush();
  // The detail rendered an editable textarea seeded with the YAML.
  assert.match(dom["app"]?.innerHTML ?? "", /id="edit-yaml"/, "expected an editable YAML textarea");
  el("edit-yaml").value = "meta:\n  case_id: clos-small\n  name: Renamed\n";
  dom["save-btn"].click();
  await flush();
  const put = fetches.find((f) => f.url === "/api/plans/clos-small" && f.method === "PUT");
  assert.ok(put, `expected PUT /api/plans/clos-small; got ${JSON.stringify(fetches.map((f) => f.method + " " + f.url))}`);
  assert.match(put.body, /Renamed/, "expected the edited YAML in the PUT body");
});

// Edit error: a 400 on Save shows the error and keeps the editor (no nav away).
test("edit error: 400 on Save renders the error alert in #edit-error", async () => {
  reset();
  const detail = JSON.stringify({ id: "clos-small", name: "Small Clos", status: "draft", yaml: "meta:\n  case_id: clos-small\n" });
  setResponder((url, opts) => {
    if (url === "/api/plans" && (!opts.method || opts.method === "GET")) return PLANS;
    if (url === "/api/plans/clos-small" && opts.method === "PUT")
      return { ok: false, status: 400, body: JSON.stringify({ error: "invalid plan" }) };
    return detail;
  });
  app.load_plans("app");
  await flush();
  dom["view-clos-small"].click();
  await flush();
  dom["save-btn"].click();
  await flush();
  assert.match(dom["edit-error"]?.innerHTML ?? "", /alert-danger/, "expected an error alert on a failed save");
});

// Delete (confirmed): DELETE /api/plans/{id} then refresh the list.
test("delete confirmed: DELETE /api/plans/{id} then reloads the list", async () => {
  reset();
  setConfirm(true);
  let deleted = false;
  setResponder((url, opts) => {
    if (url === "/api/plans/clos-small" && opts.method === "DELETE") {
      deleted = true;
      return { status: 204, body: "" };
    }
    // After delete the list is empty.
    if (url === "/api/plans") return deleted ? "{\"plans\":[]}" : PLANS;
    return PLANS;
  });
  app.load_plans("app");
  await flush();
  dom["del-clos-small"].click();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans/clos-small" && f.method === "DELETE"),
    "expected DELETE /api/plans/clos-small",
  );
  assert.match(dom["app"]?.innerHTML ?? "", /id="empty-state"/, "expected the empty-state panel after deleting the last plan (P1.5)");
});

// Delete (cancelled): the confirm step returns false -> NO DELETE is issued.
test("delete cancelled: confirm=false issues no DELETE", async () => {
  reset();
  setConfirm(false);
  setResponder(() => PLANS);
  app.load_plans("app");
  await flush();
  dom["del-clos-small"].click();
  await flush();
  assert.ok(
    !fetches.some((f) => f.method === "DELETE"),
    "a cancelled confirm must NOT issue a DELETE",
  );
});

// Duplicate: GET source YAML -> POST a clone (identity suffixed) -> refresh.
test("duplicate: clones the source plan as a new POST and refreshes", async () => {
  reset();
  const detail = JSON.stringify({ id: "clos-small", name: "Small Clos", status: "draft", yaml: "meta:\n  case_id: clos-small\n  name: Small Clos\n" });
  let posted = null;
  setResponder((url, opts) => {
    if (url === "/api/plans" && opts && opts.method === "POST") {
      posted = opts.body;
      return JSON.stringify({ id: "clos-small-copy", name: "Small Clos (copy)", status: "draft" });
    }
    if (url === "/api/plans/clos-small/overlay") return { ok: false, status: 404, body: JSON.stringify({ error: "no overlay" }) };
    if (url === "/api/plans/clos-small") return detail;
    return PLANS;
  });
  app.load_plans("app");
  await flush();
  dom["dup-clos-small"].click();
  await flush();
  assert.ok(posted, "expected a POST /api/plans for the clone");
  assert.match(posted, /clos-small-copy/, "expected the cloned case_id to be suffixed");
  assert.match(posted, /\(copy\)/, "expected the cloned name to be suffixed");
});

// clone_yaml_identity: suffixes meta.case_id (id-safe) and meta.name (human).
test("clone_yaml_identity: suffixes meta.case_id and meta.name", () => {
  const src = "meta:\n  case_id: training_xoc64_1xopg64_mesh_conv_ro\n  name: XOC 64\n  version: 1\n";
  const out = app.clone_yaml_identity(src, "-copy");
  assert.match(out, /case_id: training_xoc64_1xopg64_mesh_conv_ro-copy/, "case_id suffixed (id-safe)");
  assert.match(out, /name: XOC 64 \(copy\)/, "name suffixed (human)");
  assert.match(out, /version: 1/, "unrelated lines untouched");
});

// api_put / api_delete request shape (the new client wrappers).
test("api wrappers: PUT and DELETE issue the right method/url", async () => {
  reset();
  const detail = JSON.stringify({ id: "p1", name: "P1", status: "draft", yaml: "meta:\n  case_id: p1\n" });
  setResponder((url, opts) => {
    if (url === "/api/plans" && (!opts.method || opts.method === "GET")) return JSON.stringify({ plans: [{ id: "p1", name: "P1", status: "draft" }] });
    if (url === "/api/plans/p1" && opts.method === "PUT") return JSON.stringify({ id: "p1", name: "P1b", status: "draft" });
    if (url === "/api/plans/p1" && opts.method === "DELETE") return { status: 204, body: "" };
    return detail;
  });
  app.load_plans("app");
  await flush();
  // PUT via Save.
  dom["view-p1"].click();
  await flush();
  dom["save-btn"].click();
  await flush();
  assert.ok(fetches.some((f) => f.url === "/api/plans/p1" && f.method === "PUT"), "expected a PUT");
  // DELETE via the detail Delete button.
  setConfirm(true);
  dom["detail-del-btn"].click();
  await flush();
  assert.ok(fetches.some((f) => f.url === "/api/plans/p1" && f.method === "DELETE"), "expected a DELETE");
});

// --- P1.5 (Issue #66): navigation + loading/disabled + first-run empty state ---
// RED: these pin the P1.5 behaviors the current app.js lacks (a bare table when
// plans==0, no breadcrumb, no loading feedback on the list/detail navigation
// transitions). GREEN implements them in render.mbt + app.mbt.

const EMPTY = JSON.stringify({ plans: [] });

test("P1.5 empty state: plans==0 renders a first-run guidance panel + CTA, not a bare table", () => {
  reset();
  app.render_plan_list("app", EMPTY);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /id="empty-state"/, "expected a dedicated empty-state panel");
  assert.match(html, /No topology plans yet|Design your first/i, "expected first-run guidance copy");
  assert.match(html, /id="new-plan-btn"/, "empty state must offer the New-plan CTA");
  assert.doesNotMatch(html, /<tbody>\s*<\/tbody>/, "empty state should replace the bare empty table");
});

test("P1.5 breadcrumb: plan detail shows a breadcrumb with a clickable Plans crumb", () => {
  reset();
  app.render_plan_detail("app", DETAIL);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /aria-label="breadcrumb"/, "expected a breadcrumb nav");
  assert.match(html, /id="crumb-plans"/, "expected a clickable Plans crumb");
});

test("P1.5 loading: load_plans shows a spinner before the response resolves", () => {
  reset();
  setResponder(() => PLANS);
  app.load_plans("app"); // no await — inspect the synchronous pre-fetch state
  const loading = dom["app"]?.innerHTML ?? "";
  assert.match(loading, /spinner-border/, "expected a spinner while the list loads");
  assert.match(loading, /Loading/i, "expected a loading label");
});

test("P1.5 loading: load_detail (View) shows a spinner before the detail resolves", async () => {
  reset();
  setResponder((url) => (url === "/api/plans" ? PLANS : DETAIL));
  app.load_plans("app");
  await flush();
  dom["view-clos-small"].click(); // triggers load_detail
  const loading = dom["app"]?.innerHTML ?? "";
  assert.match(loading, /spinner-border/, "expected a spinner while the detail loads");
  assert.match(loading, /Loading/i, "expected a loading label");
});

// --- P1.1 (#67): structured editor render --------------------------------------
const STRUCTURE = JSON.stringify({
  server_classes: [
    { id: "compute_xpu", quantity: 8, gpus_per_server: 8, server_device_type: "srv_xpu_generic_dt",
      nics: [{ nic_id: "scale_out", module_type: "nic_xpu_scale_out_8x400" }] },
  ],
  switch_classes: [
    { id: "soc_storage_scale_out_leaf", topology_mode: "mesh", device_type_extension: "sw_ds5000_soc_storage_scale_out_ext", override_quantity: 2 },
  ],
  catalog: {
    module_types: ["nic_xpu_scale_out_8x400", "nic_dual_200g"],
    device_types: ["srv_xpu_generic_dt"],
    device_type_extensions: ["sw_ds5000_soc_storage_scale_out_ext"],
    breakout_options: ["b_1x800"],
  },
});

test("P1.1 structured editor: data-derived forms for server + switch classes", () => {
  reset();
  const html = app.structure_editor_html(STRUCTURE);
  // server class: quantity + gpus inputs, NIC module-type select with options.
  assert.match(html, /id="srv-compute_xpu-qty"[^>]*value="8"/, "server quantity input");
  assert.match(html, /id="srv-compute_xpu-gpus"[^>]*value="8"/, "gpus input");
  assert.match(html, /id="srv-compute_xpu-devtype"/, "server device-type select (editable on existing classes)");
  assert.match(html, /id="nic-compute_xpu-scale_out"/, "NIC module-type select");
  assert.match(html, /<option value="nic_dual_200g"/, "NIC dropdown is data-derived from the catalog");
  // switch class: explicit mesh|clos selector (the headline capability).
  assert.match(html, /id="sw-soc_storage_scale_out_leaf-topo"/, "topology selector");
  assert.match(html, /<option value="clos"/, "clos option present");
  assert.match(html, /<option value="mesh" selected/, "current mesh mode selected");
  assert.match(html, /id="sw-soc_storage_scale_out_leaf-override"[^>]*value="2"/, "override qty input");
  // add-server-class form + save buttons + the hidden id carrier.
  assert.match(html, /id="add-srv-id"/, "add-class id input");
  assert.match(html, /id="save-srv-btn"/, "save server classes button");
  assert.match(html, /id="save-sw-btn"/, "save switch classes button");
  assert.match(html, /id="structure-data"/, "hidden structure-data carrier");
});

// --- P1.3 (#68): live (dry-run) validation handler ----------------------------
const VALID_CALC = JSON.stringify({
  is_valid: true, errors: [],
  switch_quantity: [{ class_id: "fe-leaf", quantity: 2 }],
  server_quantity: [{ class_id: "compute", quantity: 8 }],
  endpoints: [{}], transceiver_verdicts: [{}], managed_fabrics: ["frontend"],
});

test("P1.3 live validate: POSTs /api/validate and renders the Valid summary inline", async () => {
  reset();
  el("edit-yaml").value = "meta:\n  case_id: x\n";
  setResponder((url) => (url === "/api/validate" ? VALID_CALC : ""));
  app.validate_raw();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/validate" && f.method === "POST"),
    `expected POST /api/validate; got ${JSON.stringify(fetches)}`,
  );
  const html = dom["live-validation"]?.innerHTML ?? "";
  assert.match(html, /Valid/, "expected the inline Valid badge");
  assert.match(html, /fe-leaf/, "expected computed quantities inline");
});

test("P1.3 live validate: a structural 4xx renders the distinct 'cannot compute' alert", async () => {
  reset();
  el("edit-yaml").value = "broken: : yaml";
  setResponder(() => ({ status: 422, body: JSON.stringify({ error: "cannot resolve plan: parse" }) }));
  app.validate_raw();
  await flush();
  const html = dom["live-validation"]?.innerHTML ?? "";
  assert.match(html, /alert-danger/, "structural failure must be the error alert");
  assert.doesNotMatch(html, /badge[^>]*text-bg-success/, "must not show a Valid badge for a structural failure");
});

// --- P1.3 (#68) devb review: stored-base + stale-response guard ---------------
const STRUCT_PROJ = JSON.stringify({
  server_classes: [{ id: "compute_xpu", quantity: 8, gpus_per_server: 8, server_device_type: "srv_xpu_generic_dt", nics: [] }],
  switch_classes: [],
  catalog: { module_types: [], device_types: ["srv_xpu_generic_dt"], device_type_extensions: [], breakout_options: [] },
});

test("P1.3 validate_structured uses the STORED plan YAML (not the editable textarea)", async () => {
  reset();
  el("stored-plan-yaml").value = "STORED_CANONICAL";
  el("edit-yaml").value = "RAW_DIVERGENT_DRAFT"; // user's separate raw edits
  el("structure-data").value = STRUCT_PROJ;
  setResponder(() => VALID_CALC);
  app.validate_structured();
  await flush();
  const sent = fetches.find((f) => f.url === "/api/validate");
  assert.ok(sent, "expected POST /api/validate");
  assert.match(sent.body, /STORED_CANONICAL/, "structured dry-run must use the stored plan as base");
  assert.doesNotMatch(sent.body, /RAW_DIVERGENT_DRAFT/, "must NOT use the editable textarea as base");
  assert.match(sent.body, /"ops":\[/, "must include the structured ops");
});

test("P1.3 stale-response guard: an older slower response never overwrites a newer one", async () => {
  reset();
  el("edit-yaml").value = "draft";
  let n = 0;
  setResponder(() => {
    n += 1;
    // 1st (older) response is SLOW + Invalid; 2nd (newer) is FAST + Valid.
    return n === 1
      ? { delay: 40, body: JSON.stringify({ is_valid: false, errors: [{ code: "STALE_BADGE" }], switch_quantity: [], server_quantity: [], endpoints: [], transceiver_verdicts: [] }) }
      : { delay: 0, body: VALID_CALC };
  });
  app.validate_raw(); // seq 1 (slow)
  app.validate_raw(); // seq 2 (fast)
  await new Promise((r) => setTimeout(r, 80)); // let both resolve (fast then slow)
  const html = dom["live-validation"]?.innerHTML ?? "";
  assert.match(html, /Valid/, "newest (fast) result must stand");
  assert.doesNotMatch(html, /STALE_BADGE/, "the older slow response must be dropped");
});

// --- P1.1b (#69): structured connections editor render ------------------------
const CONN_STRUCT = JSON.stringify({
  server_classes: [{
    id: "compute_xpu", quantity: 8, gpus_per_server: 8, server_device_type: "srv_xpu_generic_dt",
    nics: [{ nic_id: "scale_out", module_type: "nic_x" }, { nic_id: "inb_mgmt", module_type: "nic_y" }],
    connections: [{ index: 0, connection_id: "scale-out-rail-0", connection_name: "scale-out", target_zone: "leaf/zoneA", nic: "scale_out", ports_per_connection: 1, hedgehog_conn_type: "unbundled", distribution: "rail-optimized", speed: 400, rail: 0 }],
  }],
  switch_classes: [],
  catalog: { module_types: ["nic_x", "nic_y"], device_types: ["srv_xpu_generic_dt"], device_type_extensions: [], breakout_options: [], target_zones: ["leaf/zoneA", "leaf/zoneB"] },
});

test("P1.1b structured connections: target_zone dropdown + add/remove, data-derived", () => {
  reset();
  const html = app.structure_editor_html(CONN_STRUCT);
  // connection row keyed by index, target_zone is a dropdown over the plan's zones.
  assert.match(html, /id="conn-0-target_zone"/, "per-connection target_zone select");
  assert.match(html, /<option value="leaf\/zoneB"/, "target_zone options are data-derived from the plan");
  assert.match(html, /<option value="leaf\/zoneA" selected/, "current target_zone selected");
  assert.match(html, /id="conn-0-nic"/, "connection NIC dropdown");
  assert.match(html, /id="conn-rm-0"/, "per-connection remove button");
  // per-class add-connection form + the Save connections button.
  assert.match(html, /id="addconn-compute_xpu-id"/, "add-connection id input");
  assert.match(html, /id="addconn-compute_xpu-target_zone"/, "add-connection target_zone dropdown");
  assert.match(html, /id="save-conn-btn"/, "save connections button");
});

// --- P1.4 (#70): optic overlay tab ------------------------------------------
test("P1.4 overlay section: present badge + prefilled textarea + save", () => {
  reset();
  const html = app.overlay_section_html(true, "items:\n  - id: { name: o }\n");
  assert.match(html, /present/, "present badge");
  assert.match(html, /id="overlay-yaml"/, "overlay textarea");
  assert.match(html, /items:/, "textarea prefilled with the overlay");
  assert.match(html, /id="overlay-save-btn"/, "save button");
});

test("P1.4 overlay section: none badge when absent", () => {
  reset();
  const html = app.overlay_section_html(false, "");
  assert.match(html, /none/, "none badge when no overlay");
});

test("P1.4 load_overlay: 404 -> presence none; 200 -> present + content", async () => {
  reset();
  setResponder(() => ({ status: 404, body: JSON.stringify({ error: "not found" }) }));
  app.load_overlay("p");
  await flush();
  assert.match(dom["overlay-section"]?.innerHTML ?? "", /none/, "404 -> presence none");

  reset();
  setResponder(() => "items:\n  - id: { name: present-optic }\n");
  app.load_overlay("p");
  await flush();
  const html = dom["overlay-section"]?.innerHTML ?? "";
  assert.match(html, /present/, "200 -> presence present");
  assert.match(html, /present-optic/, "200 -> overlay content shown");
});

test("P1.4 save_overlay: PUT /api/plans/{id}/overlay", async () => {
  reset();
  el("overlay-yaml").value = "items:\n  - id: { name: o }\n";
  setResponder(() => ({ status: 204, body: "" }));
  app.save_overlay("p");
  await flush();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans/p/overlay" && f.method === "PUT"),
    `expected PUT .../overlay; got ${JSON.stringify(fetches)}`,
  );
});

test("P1.4 BOM shows the optic-standard column (blank until overlay attached)", () => {
  reset();
  app.render_bom("app", JSON.stringify({
    suppressed_cable_assembly_count: 0,
    rows: [{ section: "switch_transceiver", module_type_model: "OSFP-400G-DR4", hedgehog_class: "", manufacturer: "Generic", quantity: "64", standard: "400GBASE-DR4" }],
  }));
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /Optic standard/, "BOM has an optic-standard column header");
  assert.match(html, /400GBASE-DR4/, "the optic standard renders when present");
});

// --- P2.1 (#71): derived facts + BOM CSV download -----------------------------
const PLANS_FACTS = JSON.stringify({
  plans: [{
    id: "m64", name: "Mesh 64", status: "active",
    facts: { topology: "mesh", gpu_count: 64, server_total: 17, switch_total: 4, is_valid: true, computable: true },
  }],
});
const DETAIL_FACTS = JSON.stringify({
  id: "m64", name: "Mesh 64", status: "active",
  yaml: "meta:\n  case_id: m64\n",
  facts: { topology: "Clos", gpu_count: 32, server_total: 32, switch_total: 9, is_valid: true, computable: true },
});

test("P2.1 list row shows derived facts (topology/gpu/servers/switches/validity)", () => {
  reset();
  app.render_plan_list("app", PLANS_FACTS);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /mesh/, "topology");
  assert.match(html, /64 GPU/, "gpu count");
  assert.match(html, /17 servers/, "server total");
  assert.match(html, /4 switches/, "switch total");
  assert.match(html, /Valid/, "validity badge");
});

test("P2.1 uncomputable plan shows fully-unknown facts (no misleading mesh/0/0)", () => {
  reset();
  // A plan that fails structural ingest: server class quantity is still set, but
  // computable:false means NONE of the facts are trustworthy. The row must not
  // present "mesh · 0 GPU · 0 servers" — every field is "—" + a "not computable" badge.
  const plans = JSON.stringify({
    plans: [{
      id: "broke", name: "Broken Plan", status: "active",
      facts: { topology: "mesh", gpu_count: 0, server_total: 0, switch_total: 0, is_valid: false, computable: false },
    }],
  });
  app.render_plan_list("app", plans);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /not computable/, "shows the not-computable badge");
  assert.match(html, /— GPU/, "GPU is unknown, not 0");
  assert.match(html, /— servers/, "servers is unknown, not 0");
  assert.match(html, /— switches/, "switches is unknown");
  assert.doesNotMatch(html, /0 GPU/, "must not render a misleading 0 GPU");
  assert.doesNotMatch(html, /0 servers/, "must not render a misleading 0 servers");
  assert.doesNotMatch(html, /mesh ·/, "must not render a guessed topology for an uncomputable plan");
});

test("P2.1 detail header shows derived facts", () => {
  reset();
  app.render_plan_detail("app", DETAIL_FACTS);
  const html = dom["detail-facts"]?.innerHTML ?? dom["app"]?.innerHTML ?? "";
  const all = dom["app"]?.innerHTML ?? "";
  assert.match(all, /Clos/, "topology in header");
  assert.match(all, /9 switches/, "switch total in header");
  assert.match(all, /Valid/, "validity badge in header");
});

test("P2.1 download_bom_csv: GET bom?format=csv -> saves the CSV (guarded)", async () => {
  reset();
  setResponder(() => "section,quantity\nswitch,4\n");
  app.download_bom_csv("p");
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans/p/bom?format=csv" && f.method === "GET"),
    `expected GET bom?format=csv; got ${JSON.stringify(fetches)}`,
  );
  assert.ok(saved.some((s) => s.content.includes("section,quantity")), "the CSV body was saved to a file");
});

test("P2.1 download_bom_csv guard: an error response is NOT saved", async () => {
  reset();
  setResponder(() => ({ status: 422, body: JSON.stringify({ error: "cannot compute BOM" }) }));
  app.download_bom_csv("p");
  await flush();
  assert.equal(saved.length, 0, "an error body must never be written to a .csv file");
});

// --- P2.3 (#72): accessibility — scope/caption, live regions, non-color cues,
// labelled controls ----------------------------------------------------------

test("P2.3 plan-list table: scope=col headers + a visually-hidden caption", () => {
  reset();
  app.render_plan_list("app", PLANS_FACTS);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /<caption class="visually-hidden">Topology plans/, "list table has a visually-hidden caption");
  assert.match(html, /<th scope="col">Name<\/th>/, "Name header has scope=col");
  assert.match(html, /<th scope="col">Status<\/th>/, "Status header has scope=col");
  assert.match(html, /<th scope="col"[^>]*>Actions<\/th>/, "Actions header has scope=col");
  // the list-error region is an assertive live region so a failed action announces.
  assert.match(html, /id="list-error"[^>]*role="alert"[^>]*aria-live="assertive"/, "list-error is an assertive live region");
});

test("P2.3 BOM table: scope=col headers + a visually-hidden caption", () => {
  reset();
  app.render_bom("app", JSON.stringify({
    rows: [{ section: "switch", module_type_model: "DS5000", hedgehog_class: "leaf", manufacturer: "HH", quantity: "4", standard: "" }],
    suppressed_cable_assembly_count: 0,
  }));
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /<caption class="visually-hidden">Line items:/, "BOM table has a visually-hidden caption");
  assert.match(html, /<th scope="col">Section<\/th>/, "Section header has scope=col");
  assert.match(html, /<th scope="col"[^>]*>Qty<\/th>/, "Qty header has scope=col");
});

test("P2.3 detail: async regions are polite live regions; errors are assertive", () => {
  reset();
  app.render_plan_detail("app", DETAIL);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /id="live-validation"[^>]*role="status"[^>]*aria-live="polite"/, "live-validation is a polite live region");
  assert.match(html, /id="detail-result"[^>]*role="status"[^>]*aria-live="polite"/, "detail-result is a polite live region");
  assert.match(html, /id="edit-error"[^>]*role="alert"[^>]*aria-live="assertive"/, "edit-error is an assertive live region");
});

test("P2.3 loading + validating indicators expose aria-busy", () => {
  assert.match(app.loading_html("Loading plans…"), /aria-busy="true"/, "loading panel is aria-busy");
  assert.match(app.loading_html("x"), /role="status"[^>]*aria-live="polite"/, "loading panel is a polite live region");
  assert.match(app.validating_html(), /aria-busy="true"/, "validating indicator is aria-busy");
  // the spinner glyph is decorative — hidden from assistive tech.
  assert.match(app.loading_html("x"), /spinner-border"[^>]*aria-hidden="true"/, "spinner is aria-hidden (decorative)");
});

test("P2.3 status badges pair an icon with text, never color alone", () => {
  // valid facts -> ✓ + "Valid"; the spoken text is the cue, the glyph is aria-hidden.
  const validRow = JSON.stringify({ plans: [{ id: "v", name: "V", status: "active",
    facts: { topology: "mesh", gpu_count: 8, server_total: 1, switch_total: 1, is_valid: true, computable: true } }] });
  reset();
  app.render_plan_list("app", validRow);
  let html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /<span aria-hidden="true">✓ <\/span><span>Valid<\/span>/, "Valid badge pairs ✓ glyph (hidden) with the word Valid in its own text node");

  // overlay present/none carry text cues too, not just green/grey.
  assert.match(app.overlay_section_html(true, ""), /<span aria-hidden="true">✓ <\/span><span>present<\/span>/, "overlay present cue");
  assert.match(app.overlay_section_html(false, ""), /<span aria-hidden="true">○ <\/span><span>none<\/span>/, "overlay none cue");
});

test("P2.3 structured controls are programmatically labelled", () => {
  reset();
  const html = app.structure_editor_html(CONN_STRUCT);
  // table-embedded selects get an aria-label (a column <th> does not name a control).
  assert.match(html, /id="conn-0-target_zone"[^>]*aria-label="Target zone for connection scale-out-rail-0"/, "target_zone select aria-label");
  assert.match(html, /id="conn-0-nic"[^>]*aria-label="NIC for connection scale-out-rail-0"/, "connection NIC select aria-label");
  assert.match(html, /id="conn-0-speed"[^>]*aria-label="Speed for connection scale-out-rail-0"/, "speed input aria-label");
  // the remove button has an accessible name (the ✕ glyph alone is not one).
  assert.match(html, /id="conn-rm-0"[^>]*aria-label="Remove connection scale-out-rail-0"/, "remove button aria-label");
  // server-class numeric fields use real <label for=…> association.
  assert.match(html, /<label[^>]*for="srv-compute_xpu-qty"/, "quantity label is associated");
  assert.match(html, /<label[^>]*for="srv-compute_xpu-gpus"/, "gpus label is associated");
  // the connection table itself is captioned + scoped.
  assert.match(html, /<caption class="visually-hidden">Server connections<\/caption>/, "connection table caption");
  assert.match(html, /<th scope="col">Target zone<\/th>/, "connection table scope=col");
});

test("P2.3 archived status badge is not low-contrast (bordered on the dark theme)", () => {
  reset();
  app.render_plan_list("app", JSON.stringify({ plans: [{ id: "a", name: "A", status: "archived" }] }));
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /text-bg-dark border border-secondary/, "archived badge gains a border for contrast");
});

// --- P2.4 (#73): cleanup — action toolbar + clickable rows --------------------

test("P2.4 detail actions are grouped in one toolbar with a single primary (Calculate)", () => {
  reset();
  app.render_plan_detail("app", DETAIL);
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /role="toolbar"[^>]*aria-label="Plan actions"/, "actions live in a labelled toolbar");
  // Calculate is the ONE primary button; View BOM / Delete are non-primary.
  const primaries = (html.match(/btn btn-primary/g) || []).length;
  assert.equal(primaries, 1, "exactly one primary action button");
  assert.match(html, /id="calc-btn" class="btn btn-primary"/, "Calculate is the primary");
  assert.match(html, /id="bom-btn"[^>]*btn-outline-secondary/, "View BOM is secondary");
  assert.match(html, /id="detail-del-btn"[^>]*btn-outline-danger/, "Delete is a danger outline");
  assert.match(html, /id="back-btn"/, "the back-to-plans link is preserved");
});

test("P2.4 list rows are clickable: a row-body click opens the plan detail", async () => {
  reset();
  setResponder((url) => (url === "/api/plans" ? PLANS : DETAIL));
  app.load_plans("app");
  await flush();
  // the row carries a stable id + a pointer cursor (mouse affordance).
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /<tr id="row-clos-small"[^>]*cursor:pointer/, "row has an id + pointer cursor");
  // a synthetic row click (no event target -> not a button) opens the detail.
  el("row-clos-small").click();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/plans/clos-small" && f.method === "GET"),
    `row click should GET the plan detail; got ${JSON.stringify(fetches)}`,
  );
  assert.match(dom["app"]?.innerHTML ?? "", /id="calc-btn"/, "the detail rendered after the row click");
});

// --- #81: structured-CREATE mini-forms (add switch class / zone / nic) --------
// RED: structure_editor_html renders no create mini-forms for switch classes,
// zones, or NICs yet, so these render assertions fail. GREEN adds the forms +
// wiring; the exact op bodies they PUT are pinned by the Go structure API tests.
const CREATE_STRUCT = JSON.stringify({
  server_classes: [
    { id: "compute_xpu", quantity: 8, gpus_per_server: 8, server_device_type: "srv_xpu_generic_dt", nics: [] },
  ],
  switch_classes: [
    { id: "soc_storage_scale_out_leaf", topology_mode: "mesh", device_type_extension: "sw_ds5000_ext", override_quantity: 2 },
  ],
  catalog: {
    module_types: ["nic_dual_25g", "osfp_400g_dr4"],
    device_types: ["srv_xpu_generic_dt"],
    device_type_extensions: ["sw_ds5000_ext", "sw_ds2000_inb_ext"],
    breakout_options: ["brk_2x400_osfp"],
    target_zones: ["soc_storage_scale_out_leaf/uplink"],
  },
});

test("#81 add-switch-class mini-form: data-derived fields + button", () => {
  const html = app.structure_editor_html(CREATE_STRUCT);
  assert.match(html, /id="add-swc-id"/, "new switch class id input");
  assert.match(html, /id="add-swc-fabric-name"/, "fabric_name input");
  assert.match(html, /id="add-swc-fabric-class"/, "fabric_class selector");
  assert.match(html, /id="add-swc-role"/, "hedgehog_role selector");
  assert.match(html, /id="add-swc-devext"/, "device_type_extension selector");
  assert.match(html, /<option value="sw_ds2000_inb_ext"/, "devext dropdown is data-derived from the catalog");
  assert.match(html, /id="add-swc-btn"/, "add switch class button");
});

test("#81 add-zone mini-form: data-derived parent/breakout/transceiver + button", () => {
  const html = app.structure_editor_html(CREATE_STRUCT);
  assert.match(html, /id="add-zone-swc"/, "parent switch_class selector");
  assert.match(html, /id="add-zone-name"/, "zone_name input");
  assert.match(html, /id="add-zone-type"/, "zone_type selector");
  assert.match(html, /id="add-zone-portspec"/, "port_spec input");
  assert.match(html, /id="add-zone-breakout"/, "breakout_option selector");
  assert.match(html, /<option value="brk_2x400_osfp"/, "breakout dropdown is data-derived");
  assert.match(html, /id="add-zone-xcvr"/, "transceiver_module_type selector");
  assert.match(html, /id="add-zone-btn"/, "add zone button");
});

test("#81 add-nic mini-form: data-derived server class + module type + button", () => {
  const html = app.structure_editor_html(CREATE_STRUCT);
  assert.match(html, /id="add-nic-server"/, "parent server_class selector");
  assert.match(html, /id="add-nic-id"/, "nic_id input");
  assert.match(html, /id="add-nic-module"/, "module_type selector");
  assert.match(html, /<option value="nic_dual_25g"/, "module dropdown is data-derived");
  assert.match(html, /id="add-nic-btn"/, "add nic button");
});

// --- #81: exact op-body tests — drive the submit actions, assert PUT JSON -----
// The lead RED gate: pin the EXACT PUT /api/plans/{id}/structure op body each
// mini-form submits (not just that the form renders). RED: the submit stubs are
// inert (no request), so "expected a PUT" fails. GREEN reads the form fields,
// builds the op, and PUTs it via patch_structure.
function putOp(fetches) {
  const put = fetches.find((f) => f.url === "/api/plans/p/structure" && f.method === "PUT");
  assert.ok(put, `expected a PUT /api/plans/p/structure; got ${JSON.stringify(fetches)}`);
  const parsed = JSON.parse(put.body);
  assert.ok(Array.isArray(parsed.ops) && parsed.ops.length === 1, "expected exactly one op");
  return parsed.ops[0];
}

test("#81 add_switch_class submit: exact PUT op body", async () => {
  reset();
  el("add-swc-id").value = "extra_leaf";
  el("add-swc-fabric-name").value = "extra-fabric";
  el("add-swc-fabric-class").value = "managed";
  el("add-swc-role").value = "server-leaf";
  el("add-swc-devext").value = "sw_ds2000_inb_ext";
  el("add-swc-topo").value = "mesh";
  el("add-swc-override").value = "2";
  setResponder(() => STRUCTURE); // patch_structure reloads the structure after PUT
  app.add_switch_class_submit("p");
  await flush();
  const op = putOp(fetches);
  assert.equal(op.op, "add_switch_class");
  assert.equal(op.switch_class, "extra_leaf");
  assert.equal(op.fields.fabric_name, "extra-fabric");
  assert.equal(op.fields.fabric_class, "managed");
  assert.equal(op.fields.hedgehog_role, "server-leaf");
  assert.equal(op.fields.device_type_extension, "sw_ds2000_inb_ext");
  assert.equal(op.fields.topology_mode, "mesh");
  assert.equal(op.fields.override_quantity, "2");
});

test("#81 add_zone submit: exact PUT op body", async () => {
  reset();
  el("add-zone-swc").value = "soc_storage_scale_out_leaf";
  el("add-zone-name").value = "extra_zone";
  el("add-zone-type").value = "server";
  el("add-zone-portspec").value = "1-4";
  el("add-zone-breakout").value = "brk_2x400_osfp";
  el("add-zone-xcvr").value = "osfp_400g_dr4";
  el("add-zone-alloc").value = "sequential";
  el("add-zone-priority").value = "99";
  setResponder(() => STRUCTURE);
  app.add_zone_submit("p");
  await flush();
  const op = putOp(fetches);
  assert.equal(op.op, "add_zone");
  assert.equal(op.switch_class, "soc_storage_scale_out_leaf");
  assert.equal(op.zone_name, "extra_zone");
  assert.equal(op.fields.zone_type, "server");
  assert.equal(op.fields.port_spec, "1-4");
  assert.equal(op.fields.breakout_option, "brk_2x400_osfp");
  assert.equal(op.fields.transceiver_module_type, "osfp_400g_dr4");
  assert.equal(op.fields.allocation_strategy, "sequential");
  assert.equal(op.fields.priority, "99");
});

test("#81 add_nic submit: exact PUT op body", async () => {
  reset();
  el("add-nic-server").value = "compute_xpu";
  el("add-nic-id").value = "extra_nic";
  el("add-nic-module").value = "nic_dual_25g";
  setResponder(() => STRUCTURE);
  app.add_nic_submit("p");
  await flush();
  const op = putOp(fetches);
  assert.equal(op.op, "add_nic");
  assert.equal(op.server_class, "compute_xpu");
  assert.equal(op.nic_id, "extra_nic");
  assert.equal(op.value, "nic_dual_25g");
});
