// library.test.mjs — RED (#80) Node ESM mock tests for the read-only Library
// browse + Reference-topology gallery surfaces.
//
// These drive the REAL compiled MoonBit->JS exports (../static/app.js) against
// the stubbed document/fetch (harness.mjs). RED: load_library/load_reference_
// gallery are stubs that issue no request and render a placeholder, and
// library_html/reference_gallery_html return placeholders, so every assertion
// below fails at the intended seam. #80 GREEN implements the loaders + renderers
// and turns these green.

import { test } from "node:test";
import assert from "node:assert/strict";
import { dom, el, fetches, setResponder, reset, flush } from "./harness.mjs";
import * as app from "../static/app.js";

// GET /api/catalog shape: the built-in Library union (deduped catalog items).
const CATALOG = JSON.stringify({
  items: [
    { id: { name: "fe-leaf", version: "1" }, kind: "switch", layer: "class", model: "DS5000" },
    { id: { name: "soc_storage_scale_out_leaf", version: "1" }, kind: "switch", layer: "class" },
  ],
});

// GET /api/templates shape: the shipped reference topologies (gallery source).
const TEMPLATES = JSON.stringify({
  templates: [
    { id: "xoc-64-mesh", name: "XOC-64 Mesh", topology: "mesh", description: "Smallest mesh." },
    { id: "xoc-256-clos", name: "XOC-256 Clos", topology: "clos", description: "Clos." },
  ],
});

test("load_library: GET /api/catalog and render the Library table", async () => {
  reset();
  setResponder(() => CATALOG);
  app.load_library();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/catalog" && f.method === "GET"),
    `expected GET /api/catalog; got ${JSON.stringify(fetches)}`,
  );
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /Library/, "expected a Library heading");
  assert.match(html, /fe-leaf/, "expected the catalog item rows rendered");
});

test("library_html renders item rows from catalog JSON", () => {
  const html = app.library_html(CATALOG);
  assert.match(html, /fe-leaf/, "expected fe-leaf row");
  assert.match(html, /soc_storage_scale_out_leaf/, "expected the mesh class row");
});

test("load_reference_gallery: GET /api/templates and render the gallery", async () => {
  reset();
  setResponder(() => TEMPLATES);
  app.load_reference_gallery();
  await flush();
  assert.ok(
    fetches.some((f) => f.url === "/api/templates" && f.method === "GET"),
    `expected GET /api/templates; got ${JSON.stringify(fetches)}`,
  );
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /Reference topologies/i, "expected the gallery heading");
  assert.match(html, /XOC-64 Mesh/, "expected a reference card");
  assert.match(
    html,
    /use-template-xoc-64-mesh/,
    "expected a 'Use as starting point' control id per reference",
  );
});

test("reference_gallery_html renders a card + start control per template", () => {
  const html = app.reference_gallery_html(TEMPLATES);
  assert.match(html, /XOC-256 Clos/, "expected the clos reference card");
  assert.match(html, /use-template-xoc-256-clos/, "expected the start-control id");
});

// Pins the clone chain at the UNIT seam (#79 spec requirement): clicking
// "Use as starting point" on a reference must reuse the existing template clone
// path — GET /api/templates/{id} then POST /api/plans (the detached copy) — not
// merely render the gallery. RED: the gallery stub renders no control and wires
// no handler, so the click fires nothing and neither request is issued.
test("gallery 'Use as starting point' clones: click -> GET /api/templates/{id} -> POST /api/plans", async () => {
  reset();
  const TRAINING = "meta:\n  case_id: training_xoc64_1xopg64_mesh_conv_ro\n  name: XOC-64\n";
  setResponder((url, opts) => {
    const method = opts.method || "GET";
    if (url === "/api/templates" && method === "GET") return TEMPLATES;
    if (url === "/api/templates/xoc-64-mesh") {
      return JSON.stringify({
        id: "xoc-64-mesh",
        name: "XOC-64 Mesh",
        topology: "mesh",
        training: TRAINING,
        overlay: "",
      });
    }
    if (url === "/api/plans" && method === "POST") {
      return JSON.stringify({ id: "training_xoc64_1xopg64_mesh_conv_ro", name: "XOC-64 Mesh" });
    }
    if (url.startsWith("/api/plans/")) {
      return JSON.stringify({
        id: "training_xoc64_1xopg64_mesh_conv_ro",
        name: "XOC-64 Mesh",
        status: "draft",
        yaml: TRAINING,
      });
    }
    return "{}";
  });

  app.load_reference_gallery();
  await flush();
  // Click the per-reference "Use as starting point" control (#80 GREEN contract).
  el("use-template-xoc-64-mesh").click();
  // Drain the GET-template -> POST-plans -> (navigate) chain.
  await flush();
  await flush();
  await flush();

  const seen = fetches.map((f) => `${f.method} ${f.url}`);
  assert.ok(
    fetches.some((f) => f.url === "/api/templates/xoc-64-mesh" && f.method === "GET"),
    `expected GET /api/templates/xoc-64-mesh; got ${JSON.stringify(seen)}`,
  );
  const post = fetches.find((f) => f.url === "/api/plans" && f.method === "POST");
  assert.ok(post, `expected POST /api/plans (detached clone); got ${JSON.stringify(seen)}`);
  assert.match(post.body ?? "", /case_id:\s*training_xoc64/, "expected the template training POSTed as the new plan body");
});
