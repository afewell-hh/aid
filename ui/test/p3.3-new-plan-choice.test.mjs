// p3.3-new-plan-choice.test.mjs — RED (#87) Node ESM mock tests for the guided
// new-plan CHOICE surface and the shared COLLISION-AWARE identity helper.
//
// These drive the REAL compiled MoonBit->JS exports (../static/app.js) against
// the stubbed document/fetch (harness.mjs), like ui.test.mjs / library.test.mjs.
//
// RED intent (no production code yet):
//   1. `+ New plan` (list header AND empty-state CTA) must open an intentional
//      CHOICE surface (primary "reference topology" + expert "import/paste"),
//      NOT the raw-YAML form directly. Today `open_new_plan_form` renders the
//      raw form (#new-yaml, heading "New plan"), so the choice-marker assertions
//      fail at the intended seam.
//   2. Reference-clone and Duplicate must derive a COLLISION-FREE id from the
//      current /api/plans list via ONE shared helper. Today `use_template` POSTs
//      the reference training YAML VERBATIM (id == seed) and `duplicate_plan`
//      uses a FIXED `-copy` suffix, so both silently overwrite on repeat use —
//      the `-copy-2` / first-free assertions below fail at the intended seam.
//
// devb's pinned #86 note is mandatory: the helper is tested against a plan list
// that ALREADY contains the seed id AND a first clone id, proving it selects the
// NEXT free suffix (`-copy-2`), not merely "differs from source".
//
// #87 GREEN implements the choice renderer + the shared `next_free_id` helper and
// routes both clone paths through it, turning these green.

import { test } from "node:test";
import assert from "node:assert/strict";
import { dom, el, fetches, setResponder, setConfirm, reset, flush } from "./harness.mjs";
import * as app from "../static/app.js";

// The xoc-64 reference's training case_id IS the seeded oracle plan id — which is
// exactly why a verbatim clone overwrites the seed. GREEN must mutate identity.
const SEED_ID = "training_xoc64_1xopg64_mesh_conv_ro";

const TEMPLATES = JSON.stringify({
  templates: [
    { id: "xoc-64-mesh", name: "XOC-64 Mesh", topology: "mesh", description: "Smallest mesh." },
    { id: "xoc-256-clos", name: "XOC-256 Clos", topology: "clos", description: "Clos." },
  ],
});
const TEMPLATE_64 = JSON.stringify({
  id: "xoc-64-mesh",
  name: "XOC-64 Mesh",
  topology: "mesh",
  training: `meta:\n  case_id: ${SEED_ID}\n  name: XOC-64\n`,
  overlay: "",
});

// A detail body the post-create navigation (load_detail) can render.
function detailFor(id) {
  return JSON.stringify({ id, name: id, status: "draft", yaml: `meta:\n  case_id: ${id}\n` });
}

// ---------------------------------------------------------------------------
// 1. Choice surface renderer + routing
// ---------------------------------------------------------------------------

// The new choice renderer offers BOTH the primary reference path and the expert
// import path, with stable control ids the wiring / E2E key on.
test("new_plan_choice_html: offers primary reference + expert import paths", () => {
  reset();
  assert.equal(
    typeof app.new_plan_choice_html,
    "function",
    "expected an exported choice-surface renderer new_plan_choice_html (#87 GREEN contract)",
  );
  const html = app.new_plan_choice_html(TEMPLATES);
  assert.match(html, /id="choice-reference"/, "expected the primary 'reference topology' control");
  assert.match(html, /id="choice-import"/, "expected the expert 'import / paste YAML' control");
  assert.match(html, /reference/i, "expected reference-topology wording on the primary path");
  assert.match(html, /import|paste/i, "expected import/paste wording on the expert path");
  // The reference path is the primary (visually emphasized) action.
  assert.match(html, /id="choice-reference"[^>]*btn-primary|btn-primary[^>]*id="choice-reference"/,
    "expected the reference path to be the primary action");
});

// Header `+ New plan` opens the CHOICE surface, not the raw YAML textarea.
test("header + New plan opens the choice surface, not #new-yaml directly", async () => {
  reset();
  setResponder((url, opts) => {
    if (url === "/api/plans" && (!opts.method || opts.method === "GET"))
      return JSON.stringify({ plans: [{ id: SEED_ID, name: "XOC-64", status: "" }] });
    if (url === "/api/templates") return TEMPLATES;
    return "{}";
  });
  app.main_entry();
  await flush();
  dom["new-plan-btn"].click();
  await flush();
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /id="choice-reference"/, "expected the choice surface (reference path)");
  assert.match(html, /id="choice-import"/, "expected the choice surface (import path)");
  assert.doesNotMatch(html, /id="new-yaml"/, "the raw YAML textarea must NOT be the default landing");
});

// Empty-state `+ New plan` opens the SAME choice surface (same handler).
test("empty-state + New plan opens the same choice surface", async () => {
  reset();
  setResponder((url, opts) => {
    if (url === "/api/plans" && (!opts.method || opts.method === "GET"))
      return JSON.stringify({ plans: [] });
    if (url === "/api/templates") return TEMPLATES;
    return "{}";
  });
  app.main_entry();
  await flush();
  assert.match(dom["app"]?.innerHTML ?? "", /id="empty-state"/, "expected the empty-state panel");
  dom["new-plan-btn"].click();
  await flush();
  const html = dom["app"]?.innerHTML ?? "";
  assert.match(html, /id="choice-reference"/, "empty-state CTA must reach the choice surface");
  assert.match(html, /id="choice-import"/, "empty-state CTA must reach the choice surface");
});

// The expert import path opens the paste textarea and preserves the existing
// malformed-create behavior: a 400 renders the in-form error and creates NO plan.
test("import path: choose expert -> paste form -> malformed body shows error, no ghost plan", async () => {
  reset();
  let posts = 0;
  setResponder((url, opts) => {
    const method = opts.method || "GET";
    if (url === "/api/plans" && method === "GET")
      return JSON.stringify({ plans: [{ id: SEED_ID, name: "XOC-64", status: "" }] });
    if (url === "/api/templates") return TEMPLATES;
    if (url === "/api/plans" && method === "POST") {
      posts += 1;
      return { ok: false, status: 400, body: JSON.stringify({ error: "planstore: invalid plan: bad yaml" }) };
    }
    return "{}";
  });
  app.main_entry();
  await flush();
  dom["new-plan-btn"].click();
  await flush();
  // The choice surface comes FIRST — the paste textarea is NOT the default
  // landing; it appears only after choosing the expert import path.
  assert.doesNotMatch(dom["app"]?.innerHTML ?? "", /id="new-yaml"/, "paste textarea must not be the default landing (choice surface first)");
  assert.match(dom["app"]?.innerHTML ?? "", /id="choice-import"/, "expected the expert import control on the choice surface");
  // Enter the expert import path from the choice surface.
  el("choice-import").click();
  await flush();
  assert.match(dom["app"]?.innerHTML ?? "", /id="new-yaml"/, "expert path must expose the paste textarea");
  el("new-yaml").value = "this: : not: valid";
  dom["new-submit-btn"].click();
  await flush();
  assert.match(dom["new-error"]?.innerHTML ?? "", /alert-danger/, "expected the in-form error alert");
  assert.match(dom["new-error"]?.innerHTML ?? "", /invalid plan|400/i, "expected the server error surfaced");
});

// ---------------------------------------------------------------------------
// 2. Shared collision-aware identity helper (devb's pinned #86 requirement)
// ---------------------------------------------------------------------------

// The shared seam: given the current /api/plans list and a desired id, return the
// first FREE id — desired if unused, else desired-2, desired-3, ... This is the
// one helper both clone paths must route through (no duplicated suffix logic).
test("next_free_id: first-free selection against an occupied list", () => {
  assert.equal(
    typeof app.next_free_id,
    "function",
    "expected an exported shared collision helper next_free_id(plans_json, desired) (#87 GREEN contract)",
  );
  const free = JSON.stringify({ plans: [{ id: "a" }, { id: "b" }] });
  const took1 = JSON.stringify({ plans: [{ id: "z" }] });
  const took2 = JSON.stringify({ plans: [{ id: "z" }, { id: "z-2" }] });
  assert.equal(app.next_free_id(free, "z"), "z", "unused id returned unchanged");
  assert.equal(app.next_free_id(took1, "z"), "z-2", "occupied desired -> first free suffix");
  assert.equal(app.next_free_id(took2, "z"), "z-3", "occupied desired + -2 -> advance to -3");
});

// Repeat reference-clone: with the seed AND its first clone (`<seed>-copy`) already
// present, "Use as starting point" must POST the NEXT free id (`<seed>-copy-2`) —
// never the seed verbatim (today's overwrite bug) and never the occupied `-copy`.
test("reference clone is collision-aware: occupied seed + first clone -> posts <seed>-copy-2", async () => {
  reset();
  let postedBody = null;
  setResponder((url, opts) => {
    const method = opts.method || "GET";
    if (url === "/api/plans" && method === "GET")
      return JSON.stringify({ plans: [{ id: SEED_ID }, { id: SEED_ID + "-copy" }] });
    if (url === "/api/templates" && method === "GET") return TEMPLATES;
    if (url === "/api/templates/xoc-64-mesh") return TEMPLATE_64;
    if (url === "/api/plans" && method === "POST") {
      postedBody = opts.body;
      return JSON.stringify({ id: SEED_ID + "-copy-2", name: "XOC-64 (copy)" });
    }
    return detailFor(SEED_ID + "-copy-2");
  });
  app.load_reference_gallery();
  await flush();
  el("use-template-xoc-64-mesh").click();
  await flush();
  await flush();
  await flush();
  const seen = fetches.map((f) => `${f.method} ${f.url}`);
  assert.ok(postedBody, `expected a POST /api/plans for the clone; got ${JSON.stringify(seen)}`);
  // The helper must consult the live list to pick the first free id.
  assert.ok(
    fetches.some((f) => f.url === "/api/plans" && f.method === "GET"),
    "expected GET /api/plans so the helper sees current occupancy",
  );
  assert.match(
    postedBody,
    new RegExp("case_id:\\s*" + SEED_ID + "-copy-2\\b"),
    "expected the FIRST-FREE clone id <seed>-copy-2 (not the seed, not the occupied -copy)",
  );
  assert.doesNotMatch(
    postedBody,
    new RegExp("case_id:\\s*" + SEED_ID + "\\s*$", "m"),
    "must NOT post the seed id verbatim (that overwrites the seeded plan)",
  );
});

// Repeat Duplicate: with the source AND `<source>-copy` already present, a second
// Duplicate must POST `<source>-copy-2` — proving first-free, not the fixed
// `-copy` suffix that overwrites the first duplicate today.
test("duplicate is collision-aware: occupied source + first copy -> posts <source>-copy-2", async () => {
  reset();
  setConfirm(true);
  const SRC = "clos-small";
  let postedBody = null;
  setResponder((url, opts) => {
    const method = opts.method || "GET";
    if (url === "/api/plans" && method === "GET")
      return JSON.stringify({ plans: [{ id: SRC, name: "Small Clos", status: "draft" }, { id: SRC + "-copy", name: "Small Clos (copy)", status: "draft" }] });
    if (url === `/api/plans/${SRC}` && method === "GET")
      return JSON.stringify({ id: SRC, name: "Small Clos", status: "draft", yaml: `meta:\n  case_id: ${SRC}\n  name: Small Clos\n` });
    if (url === `/api/plans/${SRC}/overlay`) return { ok: false, status: 404, body: JSON.stringify({ error: "no overlay" }) };
    if (url === "/api/plans" && method === "POST") {
      postedBody = opts.body;
      return JSON.stringify({ id: SRC + "-copy-2", name: "Small Clos (copy)", status: "draft" });
    }
    return "{}";
  });
  app.load_plans("app");
  await flush();
  dom[`dup-${SRC}`].click();
  await flush();
  await flush();
  assert.ok(postedBody, "expected a POST /api/plans for the duplicate");
  assert.match(
    postedBody,
    new RegExp("case_id:\\s*" + SRC + "-copy-2\\b"),
    "expected first-free <source>-copy-2 when <source>-copy already exists",
  );
});

// The two paths must SHARE the collision seam (no copy/pasted suffix logic):
// both reduce to the same `next_free_id` contract proven above. This guards
// AC9 — a regression that forks the logic would break one path's first-free
// behavior while the other still passed.
test("reference clone and Duplicate share the first-free contract (next_free_id)", () => {
  assert.equal(typeof app.next_free_id, "function", "shared helper must exist");
  const occupied = JSON.stringify({ plans: [{ id: "p" }, { id: "p-copy" }] });
  // Duplicate's desired base is `<source>-copy`; first-free advances to -copy-2.
  assert.equal(app.next_free_id(occupied, "p-copy"), "p-copy-2", "duplicate first-free");
  // A reference clone whose desired base collides advances identically.
  assert.equal(app.next_free_id(occupied, "p"), "p-2", "reference clone first-free");
});

// ---------------------------------------------------------------------------
// 3. Fail CLOSED when the /api/plans occupancy probe fails (devb #87 review)
// ---------------------------------------------------------------------------
//
// The collision guard reads the live /api/plans list to pick a free id. If that
// probe fails (HTTP error / network), the code must NOT fall back to "assume no
// plans" and POST a deterministic clone id — planstore.Create has no server-side
// existence guard, so that would silently overwrite an existing clone. Both clone
// paths must fail closed: surface the error and issue NO POST.

test("reference clone fails CLOSED: occupancy probe 500 -> no POST, error shown", async () => {
  reset();
  setResponder((url, opts) => {
    const method = opts.method || "GET";
    if (url === "/api/templates" && method === "GET") return TEMPLATES;
    if (url === "/api/templates/xoc-64-mesh") return TEMPLATE_64;
    // The occupancy probe fails.
    if (url === "/api/plans" && method === "GET")
      return { ok: false, status: 500, body: JSON.stringify({ error: "list unavailable" }) };
    return "{}";
  });
  app.load_reference_gallery();
  await flush();
  el("use-template-xoc-64-mesh").click();
  await flush();
  await flush();
  await flush();
  assert.ok(
    !fetches.some((f) => f.url === "/api/plans" && f.method === "POST"),
    `a failed occupancy probe must NOT POST a clone; got ${JSON.stringify(fetches.map((f) => f.method + " " + f.url))}`,
  );
  assert.match(dom["reference-error"]?.innerHTML ?? "", /alert-danger/, "expected the error surfaced on the gallery");
});

test("duplicate fails CLOSED: occupancy probe network error -> no POST, error shown", async () => {
  reset();
  const SRC = "clos-small";
  // First render the list (probe succeeds) so the Duplicate control exists.
  setResponder(() => JSON.stringify({ plans: [{ id: SRC, name: "Small Clos", status: "draft" }] }));
  app.load_plans("app");
  await flush();
  // Now the source detail loads, but the occupancy probe throws (network error).
  setResponder((url, opts) => {
    const method = opts.method || "GET";
    if (url === `/api/plans/${SRC}` && method === "GET")
      return JSON.stringify({ id: SRC, name: "Small Clos", status: "draft", yaml: `meta:\n  case_id: ${SRC}\n  name: Small Clos\n` });
    if (url === "/api/plans" && method === "GET") throw new Error("network down");
    return "{}";
  });
  dom[`dup-${SRC}`].click();
  await flush();
  await flush();
  assert.ok(
    !fetches.some((f) => f.url === "/api/plans" && f.method === "POST"),
    `a failed occupancy probe must NOT POST a clone; got ${JSON.stringify(fetches.map((f) => f.method + " " + f.url))}`,
  );
  assert.match(dom["list-error"]?.innerHTML ?? "", /alert-danger/, "expected the error surfaced on the list");
});
