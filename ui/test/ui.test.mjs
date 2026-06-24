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
