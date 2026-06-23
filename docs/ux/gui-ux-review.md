# AID GUI — UX Review and Design Roadmap

*Synthesis of five user-perspective review lenses (golden-path walkthrough, authoring/design experience, polish & visual affordances, errors/validation/feedback, testing-infrastructure feasibility). Reviewer perspective: the END USER trying to design a cluster network topology with AID.*

Repo: `/home/ubuntu/afewell-hh/aid` · GUI: `ui/src/*.mbt` (MoonBit → `ui/static/app.js`) · Server: `cmd/aid/serve.go`

---

## 1. Honest current-state assessment

**What the GUI is today: a read-only viewer, not a designer.** AID is branded an "AI Infrastructure Designer," and the compute engine genuinely lives up to that — it ingests a DIET/training plan, computes switch/server quantities, bills a BOM, and emits hhfab-validated wiring CRDs, reproducing real mesh and Clos topologies. But the browser experience exposes only five flows: **list plans → view a plan's detail → Calculate → View BOM →** (a compiled-but-unwired) download-wiring. There is not a single form, input, textarea, file-upload, or Create/Edit/Delete button anywhere in `ui/src/render.mbt` or `ui/src/app.mbt`. A user **cannot bring a plan into existence from the GUI**; plans are born only via `curl`/CLI by someone who already knows the ~900-line interlocking YAML schema.

### What genuinely works
- **The engine is excellent and reachable.** Once a plan with a top-level `id` exists, `POST /calc` returns `is_valid:true` with correct xoc-64 quantities (switches `{soc_storage_scale_out_leaf:2, inb_mgmt_leaf:1, oob_leaf:1}`, servers `{compute_xpu:8, hh_controller:1, hh_gateway:2, metadata_srv:3, storage_srv:3}`); `GET /bom` returns 21 projected rows; `GET /wiring/` returns ~47 KB of valid `wiring.githedgehog.com` CRDs.
- **Air-gapped.** `go build -o aid ./cmd/aid` succeeds; `aid serve` boots cleanly and serves the embedded Bootstrap UI + REST API with no CDN/external requests.
- **Two-plane validation is well-modeled — on one surface.** `calc_summary_html` (render.mbt:163-201) correctly distinguishes a structural 4xx `{"error":...}` body (red "Cannot resolve plan" alert, no Valid badge) from calc-as-data (200 + `is_valid` badge + per-rule `errors[]`). This is exactly the feedback an author needs.
- **Defensive rendering.** All dynamic text is HTML-escaped via `esc()`; missing JSON keys degrade to `""`/`0`/`[]` instead of throwing.
- **Coherent visual system.** Bootstrap 5 dark theme, NetBox-style navbar, hover tables, card-based detail, status badges, right-aligned numeric columns.
- **The backend is already authoring-complete.** `POST/PUT/DELETE /api/plans` and `GET/PUT /api/plans/{id}/overlay` all exist (serve.go:23-30, 154-335). The gap is entirely frontend.
- **A fast, air-gapped unit harness exists.** `make ui-test` runs 10 Node smoke tests against the *real* compiled `app.js`, asserting render output and request URLs/methods.

### The headline gaps (no hype — these are blocking)
1. **No authoring/design capability at all.** No create, no edit, no delete, no overlay editor, no topology selector. The product's primary purpose is impossible in the GUI.
2. **The single documented on-ramp is broken.** `ui/docs/walkthrough.md` tells a new user to `POST` a vendored `tests/oracle/*/training.yaml`. Every vendored DIET plan carries identity under `meta.case_id`/`meta.name`, but `planstore` reads top-level `id`/`name` (planstore.go:48-52) → **HTTP 400 "plan has no id or name."** Verified live. A new user cannot get *one* plan into the store. The app shows an empty table forever.
3. **The final deliverable is unreachable.** `download_wiring` is compiled and exported (app.mbt:68) but no button calls it. The hhfab wiring — the payoff of the whole pipeline — cannot be obtained from the browser.
4. **No HTTP/network error handling.** `fetch` ignores `r.ok`/`r.status` and has no `.catch` (ffi.mbt:20-25). A 500 on the list renders "0 plan(s)" (outage ≡ empty account); a 404 renders a ghost detail card with live buttons; a 422 on BOM renders an empty BOM with "Suppressed cable assemblies: 0" (a dangerously misleading "this design needs zero hardware" for a procurement tool); a dropped connection freezes the UI silently.
5. **No real-browser testing.** The harness uses a mock DOM + mock fetch and re-implements the FFI rather than executing it — structurally blind to render, CSS, event-wiring, and async-failure bugs. (Good news: real Playwright E2E is fully feasible offline here — see §5.)

---

## 2. The Golden Path — the ideal end-to-end design journey

This is the **target experience**: a network architect designs an AI cluster topology end-to-end in the browser and walks away with a validated, hhfab-ready wiring bundle. Opinionated and concrete.

**Step 0 — First run / orientation.** User opens `/`. If no plans exist, an **empty state** explains what AID does and offers two primary actions: **"New plan from template"** (pick mesh-64 / mesh-128 / Clos-256 starters) and **"Import YAML"**. If plans exist, the list shows each with **derived facts at a glance**: topology mode (mesh/Clos), GPU count, switch & server totals, validity status — not just name/id/status.

**Step 1 — Create / choose a plan.** "New plan" opens an editor pre-seeded from a starter template (a vendored oracle composition). User names the plan and lands in the design surface. "Duplicate" on any existing row clones a known-good plan as a safe branch-and-tweak starting point.

**Step 2 — Design the topology (structured, not raw YAML).**
- **Server classes:** add/edit classes with quantity, GPUs-per-server, NICs — NIC `module_type` chosen from a **dropdown populated from the plan's own catalog ids** (no free-text typos).
- **Switch classes:** set per-class **topology mode via an explicit `mesh | clos` selector** (the headline capability must be discoverable, not buried in YAML), device type, and optional `override_quantity`.
- **Connections:** wire `target_zone` via dropdowns sourced from the plan's switch-classes/zones (the brittle `"switch_class/zone_name"` join becomes pickable, not typed).
- **Catalog / reference_data:** managed as a **reusable, pickable library** (breakouts, transceiver module types, device types) rather than re-pasted per plan.
- **Advanced escape hatch:** a raw-YAML editor for power users, round-tripping to the same store.

**Step 3 — Attach the optic/identity overlay.** A first-class **Overlay tab** (GET to view, PUT to save), surfaced prominently because BOM identity depends on it. Presence/absence is always visible so the user knows whether the BOM will carry real SKUs. Ideally the optic catalog is a shared pickable library.

**Step 4 — Live validation as you design.** On every meaningful edit, the surface **debounce-validates** (save + dry-run calc) and renders `calc_summary_html` inline: a green **Valid** badge or red per-rule violations with code+message located near the offending construct. Structural failures surface as a distinct "plan cannot be computed" alert. No blind trial-and-error against an opaque 422.

**Step 5 — Review derived outputs.** Once valid, the user sees **computed switch/server quantities**, the **BOM table**, and a **per-fabric wiring preview**. Each fabric (frontend/backend for Clos, single for mesh) is named and selectable — derived from the plan's `fabric_name` fields, never guessed.

**Step 6 — Iterate.** Flip a topology mode, bump a quantity, change a NIC; recalc and **compare** quantities/BOM/wiring interactively. The read view *is* the design/iterate loop.

**Step 7 — Export / handoff.** **Download wiring** per fabric (real browser file download), download BOM (CSV), and copy/share the plan. Exports never write a poisoned file: a non-OK response shows an error alert instead of saving a `.yaml` containing `{"error":...}`.

---

## 3. Gap analysis — golden path vs current GUI

| # | Golden-path step | Current GUI support | Severity of gap |
|---|---|---|---|
| 0a | First-run empty state with guidance | **Missing** — blank table, "0 plan(s)", no CTA (render.mbt:81-108) | Major |
| 0b | List shows derived facts (topology, GPUs, totals, validity) | **Missing** — only Name/ID/Status | Polish |
| 1a | Create plan (new / from template) | **Missing** — no UI; documented seed `POST` returns **HTTP 400** for every vendored plan | **Blocker** |
| 1b | Duplicate / save-as | **Missing** | Minor |
| 1c | Delete plan | **Missing** (DELETE route exists, serve.go:196) | Minor |
| 2a | Structured server-class editing | **Missing** — raw read-only YAML only | **Blocker** |
| 2b | Structured switch-class editing | **Missing** | **Blocker** |
| 2c | mesh ↔ Clos selector | **Missing** — free-text `topology_mode` in YAML, undiscoverable | Major |
| 2d | Connection wiring via dropdowns (no string joins) | **Missing** | Major |
| 2e | Catalog/reference_data as pickable library | **Missing** — re-pasted per plan (~900 lines) | Major |
| 2f | Raw-YAML edit escape hatch | **Missing** (PUT route exists, serve.go:181) | **Blocker** |
| 3 | Overlay view/edit + visibility | **Missing** — route exists (serve.go:311), UI calls neither; detail never shows overlay | Major |
| 4 | Live validation during authoring | **Partial** — `calc_summary_html` is excellent but only reachable via static Calculate button on a saved plan | Major |
| 5a | Review derived quantities | **Have** — Calculate → quantity table | Have |
| 5b | Review BOM | **Have** — View BOM → 21-row table | Have |
| 5c | Per-fabric wiring preview / selection | **Missing** — fabric names undiscoverable; wrong fabric returns 200 + empty body | Major |
| 6 | Iterate (edit → recalc → compare) | **Missing** — no edit; navigation is one-way (no Back) | Major |
| 7a | Download wiring | **Missing in UI** — `download_wiring` compiled but wired to no button | **Blocker** |
| 7b | Download BOM (CSV) | **Missing** | Minor |
| — | HTTP/network error feedback | **Missing** — no `r.ok`/`.catch`; outage looks like empty, 404→ghost card, 422→empty BOM | **Blocker** |
| — | Loading/spinner/disabled states | **Missing** — silent latency, double-click risk | Major |
| — | Back navigation / breadcrumb | **Missing** — View overwrites `#app`; reload is the only recovery | Major |
| — | Accessibility (scope, caption, aria-busy, non-color status) | **Missing/thin** | Minor |
| — | Real-browser test coverage | **Missing** — mock DOM/fetch only | Major |

---

## 4. Prioritized roadmap

Each phase is scoped for an **arch-note-first implementation ticket**. Phases are ordered so that "design a topology at all" is unblocked before UX refinement, before polish.

### P0 — Unblock "design a topology at all" (blockers)
*Goal: a new user can create, edit, validate, and export a topology entirely in the browser.*

- **P0.1 — Fix the plan-identity contract (server).** Make `planstore.parseMeta` read `meta.case_id` / `meta.name` / `meta.tags`(status) as canonical identity, with top-level as fallback, so vendored DIET plans `POST` successfully. *Dependency: none.* Add a Go test that `POST`s each `tests/oracle/*/training.yaml` through the real store. **This single change turns the documented walkthrough from broken to working.**
- **P0.2 — Create / Edit / Delete UI (raw-YAML v1).** "New plan" modal: name field + YAML `<textarea>` prefilled from a starter template → `POST /api/plans` → route to detail. Make the detail YAML block editable with **Save** → `PUT /api/plans/{id}`. Add **Delete** (confirm) per row. *Dependency: P0.1 (so created plans are addressable); needs `api_put`/`api_delete` client wrappers in `api.mbt`.*
- **P0.3 — Wire up wiring download + fabric discovery.** Add per-fabric **Download wiring** buttons to detail/calc view; bind to `download_wiring`. Expose managed fabric names on the plan-detail or calc response (derive from `switch_classes.fabric_name`) so buttons are populated, not guessed. Return **404/422 + valid-fabric list** for a non-matching fabric instead of 200 + empty body. *Dependency: none.*
- **P0.4 — HTTP/network error handling (FFI).** Change the fetch FFI to deliver `{ok, status, body}` (or reject) and add `.catch`. Add a shared error renderer; have list/detail/BOM check for an `error` key (and non-2xx) **before** rendering, mirroring `calc_summary_html`. Guard `download_wiring` so a non-OK body never gets saved to a `.yaml`. *Dependency: none — should land alongside P0.2/P0.3.*

### P1 — Make authoring first-class (major UX)
*Goal: design without hand-writing interlocking YAML; iterate fast.*

- **P1.1 — Structured editor for server/switch classes & connections.** Forms/wizard for high-level intent; every cross-reference (`target_zone`, NIC `module_type`, breakout, device-type-extension) is a **dropdown sourced from the plan's own ids**, not a string field. Keep the raw-YAML escape hatch. *Dependency: P0.2.*
- **P1.2 — mesh ↔ Clos selector** per fabric/switch-class, with quantities/BOM/wiring recomputing on change for interactive comparison.
- **P1.3 — Live validation loop.** Debounce save+calc on edit; render `calc_summary_html` inline; locate resolve errors near the offending construct. (Consider a dedicated dry-run/validate endpoint to avoid persisting on every keystroke.) *Dependency: P0.4 for clean error surfacing.*
- **P1.4 — Overlay tab.** GET/PUT overlay in the detail/edit surface; always show overlay presence/absence. Treat optic catalog as a shared pickable library where feasible. *Dependency: P0.2.*
- **P1.5 — Navigation & loading states.** "Back to plans" + breadcrumb; SPA navbar brand; spinner/disabled-button on every async action (`load_plans`/`load_detail`/`trigger_calc`/`load_bom`); empty-state panel when `plans.length()==0`.
- **P1.6 — Catalog/reference_data as a reusable library** (breakouts, module types, device types) rather than re-pasted per plan.

### P2 — Polish & confidence
- **P2.1 — Derived-fact summaries** on list rows and detail header (topology mode, GPU count, switch/server totals).
- **P2.2 — Duplicate / save-as**; **Download BOM (CSV)**.
- **P2.3 — Accessibility:** `scope="col"` + visually-hidden `<caption>` on tables; `role="status"`/`aria-live` on dynamic regions; non-color status cues; verify dark-theme badge contrast.
- **P2.4 — Cleanup:** remove the dead `#result` div (index.html:19); group detail actions in a wrapping `btn-toolbar` with one clear primary (Calculate); consider clickable rows.
- **P2.5 — Regenerate air-gapped evidence by driving the real REST API** (POST a vendored plan, PUT overlay, calc/bom/wiring) so docs become a true end-to-end smoke test instead of fabricated happy-path JSON.

---

## 5. GUI testing plan

### Recommended approach
**Two layers, additive — keep the fast mock harness, add real-browser E2E on top.**

- **Layer 1 (keep): Node mock-harness unit tests** (`make ui-test`, `ui/test/ui.test.mjs` + `harness.mjs`). Fast, air-gapped, zero npm deps; a legitimately good *contract* test of `render.mbt` HTML-string output and request URLs/methods, including the two-plane validation distinction. Honest limitation: it uses a **mock DOM + mock fetch** that model only `{id, innerHTML, value, click}`, and it **re-implements the FFI** rather than executing `ffi.mbt`'s real extern JS — so it cannot catch render/CSS/event-wiring bugs or real async/network failures. Do **not** rely on it for "does the GUI work" claims. *Improvement:* extend `harness.mjs` to model `{ok, status, text}` so future tests can assert status-based error rendering.
- **Layer 2 (add): Playwright E2E in real headless chromium.** Preferred over Puppeteer (better selectors/auto-wait/download API) and over jsdom+Testing-Library (jsdom still has no layout/CSS — only a marginal upgrade over the current mock). Asserts on the rendered/visible DOM, real clicks, and real file downloads (`page.on('download')`).

### Feasibility in THIS environment — verified
Real-browser E2E is **fully feasible offline here.** Playwright 1.48.2 + chromium-1140 are already installed (in the sibling `hh-learn` `node_modules` + `~/.cache/ms-playwright`); Node 24 and a prebuilt `aid` binary are present. A reviewer launched headless chromium against a live `aid serve`, seeded a real oracle plan via `POST /api/plans`, and confirmed the real DOM/Bootstrap table rendered and that a real View-button click navigated list→detail with **zero console errors**.

**The only gap:** the `aid` repo has **no `package.json`/`node_modules`** — the working setup currently relies on an accidental absolute path into the `hh-learn` repo. To make E2E reproducible/CI-safe:
- Add a minimal `ui/test/package.json` declaring `playwright` as a devDependency.
- Pin the browser via `PLAYWRIGHT_BROWSERS_PATH` to the vendored `~/.cache/ms-playwright` (offline `npx playwright install` is blocked — point CI at the existing cache).
- Add a `make ui-e2e` target that builds `aid`, starts `aid serve` on an ephemeral port + temp plans dir, seeds the three oracle compositions via the API, runs the spec, and tears down.

### Workflows to cover (enumerated)

**Testable now (read-only flow, current build):**
1. Initial load renders the plan list from `GET /api/plans`.
2. **Empty state** when no plans exist.
3. Click **View** → detail renders fabric/validation.
4. **Calculate (valid)** → Valid badge + correct switch/server quantities, for each oracle composition (xoc-64 mesh, xoc-128 mesh, xoc-256 Clos).
5. **Calculate (invalid as data)** → Invalid badge + error code/message (e.g. `ZONE_OVERFLOW`/`INVALID_PLAN`).
6. **Calculate (structural 4xx)** → "Cannot resolve plan" alert, no Valid badge. *(Note: confirm with the engine team — currently many unrunnable plans return 200 `is_valid:false` rather than 4xx, so this branch is rarely exercised; the split may need engine work.)*
7. **View BOM** → rows with section/model/class/qty + suppressed-cable footer.
8. **Error states:** 500 on list (must NOT look like empty), 404 on detail (must NOT render a ghost card), 422 on BOM (must NOT render an empty "zero hardware" BOM), network drop / slow / empty body.
9. **Loading states:** spinner/disabled while a request is in flight; no duplicate POST on double-click.
10. Responsive/visual smoke at mobile + desktop widths.

**Needs P0.3 wiring before it can be tested:**
11. **Download wiring** per fabric → real file download captured via `page.on('download')`, with expected YAML; wrong-fabric → error, not a poisoned file.

**Needs P0/P1 authoring UI before it exists:**
12. **Create** (new / from template / import) → POST round-trip → routes to detail.
13. **Edit** YAML/structured fields → PUT round-trip → recalc reflects change.
14. **Delete** (with confirm) and **Duplicate**.
15. **Overlay** view/edit → PUT round-trip; BOM identity reflects it.
16. **Live validation** during edit; **unsaved-change** handling; form/field-level validation feedback.
17. **mesh ↔ Clos** toggle → quantities/BOM/wiring recompute.

---

## 6. Top 10 prioritized recommendations

1. **Fix the plan-identity mismatch** so vendored DIET plans `POST` successfully (`planstore.parseMeta` reads `meta.case_id`/`meta.name`). The one change that makes the documented walkthrough work. *(P0.1 — blocker)*
2. **Add a Create-plan flow** (New + Import, raw-YAML `<textarea>` v1) → `POST /api/plans`. Turns the product from viewer into authoring tool. *(P0.2 — blocker)*
3. **Add Edit (PUT) and Delete (DELETE) from the UI** so designs are iterable and prunable. *(P0.2 — blocker)*
4. **Wire up Download-wiring with discoverable per-fabric buttons**, and return 404/list instead of 200+empty for a bad fabric. Makes the final deliverable reachable. *(P0.3 — blocker)*
5. **Add HTTP/network error handling** (FFI surfaces `{ok,status}` + `.catch`; list/detail/BOM check `error` first; guard the wiring download against saving error bodies). *(P0.4 — blocker)*
6. **Add loading/spinner/disabled states + Back navigation + an empty state.** The cheapest, highest-felt polish. *(P1.5 — major)*
7. **Build the structured editor with dropdown-driven cross-references** (no free-text id joins) so authors don't hand-write ~900 lines of brittle YAML. *(P1.1 — major)*
8. **Expose mesh ↔ Clos as an explicit selector** with live recompute. The headline capability must be discoverable. *(P1.2 — major)*
9. **Add live validation during authoring** (debounced save+calc, inline `calc_summary_html`, errors located near the offending construct). *(P1.3 — major)*
10. **Stand up Playwright E2E** (`make ui-e2e`, declared deps, vendored chromium cache) covering the golden path + the error/edge cases enumerated in §5; keep the mock harness as fast unit tests. *(testing — major)*
