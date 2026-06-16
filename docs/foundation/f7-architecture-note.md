# F7 Surfaces — retarget CLI/REST/GUI onto the rebuilt engine; retire the old adapter path (Issue #64)

**Status:** proposed — awaiting lead + devb sign-off before RED.
**Author:** deva. **Scope:** D25 *Surfaces only* — the **last** rebuild phase.
Route the three user surfaces (CLI, REST, GUI) through the rebuilt engine
(`topology.IngestBundled → catalog.Merge(overlay) → calc.Compute → bom.Render* /
wiring.Render`) so they reproduce committed **oracle** results for **≥1 mesh
(xoc-64) + ≥1 Clos (xoc-256)** composition, then **retire** the old
`internal/orchestrate` + Rust WASM adapter path. The MoonBit proved kernel
(`calc.Compute → components.Kernel() → embed/kernel.wasm`), `wasmhost`, the F2/F3
kernel entries, and the `moon prove` gate are **kept** — only the
old adapter/orchestrate/old-schema path retires.

**Out of scope (deferred, explicitly flagged):** authored-plan **normalization**
(D25 — no HNP transform exists; dropped as a phase); **arbitrary-plan catalog
authoring** (a user supplying their own overlay for a hand-authored plan — F7
makes the *XOC compositions* work through the surfaces and ships their committed
overlays; arbitrary-plan overlay authoring is a named follow-up, §2.4); the
optional "OCP `topology-map` → DIET" import helper (D25, AID-invented, no oracle);
**xoc-512/1024** (oracle table extension, one row each, not required here);
**NetBox** (D22). No invented model or transform (D25).

Everything below is grounded in the live engines and the committed oracle
snapshots under `tests/oracle/{xoc-64-mesh-conv-ro,xoc-256-2xopg128-clos-ro}/`.
The end-to-end call chain in §1 is copied verbatim from the proven oracle harness
(`internal/oracle/oracle_test.go` ingest/mergeOverlay helpers + the F3/F4 tests).

---

## 0. Headline finding — the engine is built; the surfaces never connected to it

The rebuilt engine (F0–F6) is **complete and proven**, but **no surface touches
it**. All three surfaces route through the *old* path:

- **CLI** (`cmd/aid/commands.go`): every subcommand calls
  `orchestrate.Validate/Calculate/ExportBOM/ExportWiring`
  (`commands.go:52,89,128,157`), which load the **old invented plan schema** via
  `plan.YAMLToJSON` (`commands.go:26`) and run the **old kernel entries**
  (`export_calculate`/`export_validate`) + the **Rust WASM adapters**
  (`hhfab-adapter`, `bom-adapter`) via `orchestrate.go`.
- **REST** (`cmd/aid/serve.go`): `calcPlan`/`bomPlan`/`wiringPlan`
  (`serve.go:202,225,241`) reuse the same `orchestrate.*` functions; the file
  header says so explicitly (`serve.go:32-33`).
- **GUI** (`ui/src/render.mbt`): renders the **old wire shapes** — `calc_summary_html`
  reads `{ir:{nodes,edges,fabrics}, validation:{is_valid,errors}}`
  (`render.mbt:167-176`); `bom_html` reads the old hierarchical
  `{boms:[{device_class,plan_quantity,line_items:[{level,quantity_per_unit,fleet_quantity}]}]}`
  (`render.mbt:201-234`).

The rebuilt engine is reachable **only** from the oracle tests. F7 is therefore
**plumbing + deletion, not invention**: build a thin coordinator over the rebuilt
API, point the three surfaces at it, adjust the response shapes the GUI consumes,
and delete the old path. There is **no new engine behavior and no new proof
obligation** — every quantity still comes from the proved kernel cores
(`KernelF2Calculate`, `KernelF3Bom`).

---

## 1. The coordinator — `internal/design` (replaces `internal/orchestrate`)

A single Go package that wraps the rebuilt engine end-to-end. The call chain is
**exactly** what the oracle harness already proves
(`internal/oracle/oracle_test.go:26-49` ingest+mergeOverlay; the F3 chain
`calc.Compute → bom.Resolve → bom.RenderProjection`; the F4 chain
`wiring.Render`):

```go
// internal/design/design.go  (new; name TBD at sign-off — "design" mirrors the tool name)

// Inputs is one self-contained design request.
type Inputs struct {
    TrainingYAML []byte // the DIET/training bundle (HNP's authoring format, D25)
    OverlayYAML  []byte // optional AID optic/identity overlay (§2); nil ⇒ base catalog only
}

// Resolved is the fully-computed model every surface renders from.
type Resolved struct {
    Plan     *topology.Plan
    Catalog  *catalog.Catalog
    Calc     *calc.CalcOutput   // switch/server quantities, endpoints, transceiver verdicts
    BOM      *bom.ResolvedModel  // fleet-scaled lines (projection + full views)
    // wiring is rendered on demand (per-fabric) via Wiring(), not eagerly.
}

func Resolve(in Inputs) (*Resolved, error) {
    plan, cat, err := topology.IngestBundled(in.TrainingYAML)        // F1
    if err != nil { return nil, err }
    calcOut, err := calc.Compute(plan, cat)                          // F2 (base catalog)
    if err != nil { return nil, err }
    if len(in.OverlayYAML) > 0 {                                     // §2 overlay merge
        overlay, err := catalog.LoadBytes(in.OverlayYAML)            // (helper to add; see §2.3)
        if err != nil { return nil, err }
        cat.Merge(overlay)
    }
    model, err := bom.Resolve(plan, cat, calcOut)                    // F3
    if err != nil { return nil, err }
    return &Resolved{plan, cat, calcOut, model}, nil
}

func (r *Resolved) Wiring(fabric string) ([]wiring.Doc, error) {    // F4, on demand
    docs, err := wiring.Render(r.Plan, r.Catalog, r.Calc)
    if err != nil { return nil, err }
    if fabric != "" { /* filter to docs[i].Fabric == fabric */ }
    return docs, nil
}
```

**⚠️ Ordering is load-bearing and must match the oracle tests.** `calc.Compute`
runs on the **base** extracted catalog (`oracle_test.go:32` ingest → `:88`/F3 test
calls `calc.Compute` *before* `mergeOverlay` at `:48`). The overlay enriches
optic/description fields (`bom.csv` cols 7–19) that **BOM/wiring** need but
**calc** does not — `calc.BuildCalcPlan` resolves transceiver attrs from the
*base* catalog's `calc_profile` (`calc.go:252,297`). The coordinator therefore
merges the overlay **after** `calc.Compute` and **before** `bom.Resolve` /
`wiring.Render`. RED will assert this ordering (a fixture that would change calc
output if the overlay merged early is the teeth).

**No surface re-counts.** Surfaces consume `Resolved` only; they never re-derive
quantities from the plan (the same anti-drift posture as `bom.Resolve` →
`RenderProjection`/`RenderFullBOM`, `bom.go:1-19`).

---

## 2. Catalog / overlay input contract (Issue #64 point 2)

**The problem.** The overlay (`optic-overlay.yaml`) is today a **per-composition
test fixture** (`composition.go:38`, `oracle_test.go:42-49`). It is a real,
required input for `bom.csv` cols 7–19 and switch `Item.Model` resolution — not a
test artifact. A surface user must supply it somehow. Issue #64 asks: ship a
default? `--overlay` flag? fold into the plan?

**Decision (F7 scope = make the XOC compositions work through the surfaces):**

1. **The overlay is an explicit, optional second input** to the coordinator
   (`Inputs.OverlayYAML`). It is **not** folded into the training YAML — they are
   distinct planes (DIET training = HNP's authoring format, D25; overlay = AID's
   optic/identity plane, owned by AID per D21/§3.3). Folding would corrupt the
   "training YAML *is* HNP's input contract" invariant.

2. **CLI:** add a `--overlay <file>` flag to every command that renders
   BOM/wiring (and to a new top-level `aid design` command, §5). Absent ⇒ base
   catalog only (BOM optic columns render empty; calc/quantities/wiring-structure
   are unaffected). This makes the XOC path a one-liner:
   `aid topology bom tests/oracle/xoc-64-mesh-conv-ro/training.yaml --overlay tests/fixtures/f3/optic-overlay.yaml`.

3. **REST:** the plan store gains an **optional companion overlay** per plan
   (stored beside the plan YAML, e.g. `<id>.overlay.yaml`), settable via
   `PUT /api/plans/{id}/overlay` (and readable via `GET`). Calc/BOM/wiring use it
   when present. This keeps the existing CRUD shape intact and adds one
   sub-resource. The GUI gets a follow-up affordance to author it (§2.4).

4. **Default-overlay option — rejected for F7.** Shipping a single baked-in
   default overlay would be wrong: the three XOC overlays differ
   (`tests/fixtures/f3/optic-overlay.yaml` vs the two `tests/oracle/.../optic-overlay.yaml`).
   There is no one default; F7 ships them as fixtures the integration tests feed
   explicitly.

### 2.3 Small enabling change
`catalog.Load(path)` exists (`catalog.go:275`); add a sibling
`catalog.LoadBytes([]byte)` so the coordinator/REST can merge an overlay from a
request/store buffer without a temp file. Pure refactor of the existing parser.

### 2.4 Explicit follow-up (NOT F7)
**Arbitrary-plan catalog/overlay authoring** — a GUI/REST flow where a user
hand-authors a plan *and* its overlay from scratch (vs. consuming the committed
XOC overlays). Flagged here, deferred. F7's acceptance is the **XOC compositions
reproduced through the surfaces**, not arbitrary-plan authoring.

---

## 3. Output-shape changes (Issue #64 point 3)

The old path returns **WASM JSON envelopes** (`orchestrate/wire.go`:
`CalcResult{Ok:{IR,Validation}}`, `BomOutput{Content}`, `WiringDocument`). The
rebuilt engine returns **Go structs**: `calc.CalcOutput`
(`SwitchQuantity`/`ServerQuantity` maps, `Endpoints`, `TransceiverVerdicts`,
`Errors` — `calc.go:148-154`), `bom.ResolvedModel`/`RenderProjection`/
`RenderFullBOM` (CSV; `bom.go`), `wiring.Doc{Fabric,YAML}` (`wiring.go:39-42`).
The surfaces must move to these.

### 3.1 CLI (`cmd/aid/commands.go`)
Keep the command tree (`plan validate`, `topology calc`, `topology bom`,
`export wiring`) — only the internals change:
- `topology calc` → print per-class **switch/server quantities** + endpoint/
  verdict summary from `CalcOutput` (replaces the `nodes/edges/fabrics` IR
  summary, which was the old invented IR — gone with the old schema).
- `topology bom` → print `bom.RenderProjection(model)` for `--format csv`
  (the real 19-col HNP CSV); `--format json` → a structured view derived from
  `ResolvedModel.Lines` (§3.4). `--full` flag (new) → `RenderFullBOM`.
- `export wiring` → `Resolved.Wiring(fabric)` → join `Doc.YAML` (unchanged shape).
- `plan validate` → ingest + resolve; surface `topology.Validate` /
  `calc.Compute` errors as the constraint violations (replaces
  `orchestrate.Validate` + old `export_validate`).

### 3.2 REST (`cmd/aid/serve.go`)
Same routes; new response bodies (versioned implicitly by the rebuild — the old
shapes had no external consumers but the GUI, which we update in lockstep):
- `POST /api/plans/{id}/calc` → `{ "switch_quantity": {...}, "server_quantity":
  {...}, "endpoints": [...], "transceiver_verdicts": [...], "errors": [...] }`
  (marshal `CalcOutput` directly). Semantic errors stay **data** (200 with a
  populated `errors`), per the existing `serve.go:194-196` contract; only an
  ingest/structural failure is 422.
- `GET /api/plans/{id}/bom` → `{ "rows": [...], "suppressed_cable_assembly_count":
  N }` derived from `ResolvedModel` (§3.4). Add `?view=projection|full`
  (default `projection`) and `?format=csv` (returns `text/csv` via the renderer).
- `GET /api/plans/{id}/wiring/{fabric}` → unchanged (`text/x-yaml`, multi-doc).
- New: `GET/PUT /api/plans/{id}/overlay` (§2.3).

### 3.3 GUI (`ui/src/render.mbt`, `ui/src/api.mbt`, `ui/test/`)
- `calc_summary_html` (`render.mbt:161`) — retarget from `ir.nodes/edges/fabrics`
  + `validation.is_valid` to the new `CalcOutput` shape: a per-class
  **switch/server quantity** table + an errors list (validation surfaced as data,
  same UX). 
- `bom_html` (`render.mbt:201`) — retarget from the hierarchical
  `boms[].line_items[]` to the flat `rows[]` shape (§3.4): a BOM table with the
  projection columns + fleet quantity. Keep the NetBox/Bootstrap styling.
- `api.mbt` path builders unchanged (routes are stable); add an overlay path if
  the GUI overlay affordance lands (else deferred to §2.4).
- `ui/test/ui.test.mjs` + `harness.mjs` — update the stubbed fixtures to the new
  JSON shapes; `make ui` rebuilds the committed `ui/static/app.js` (the
  `make ui-check` freshness guard forces this to be regenerated, not hand-edited).

### 3.4 The BOM JSON view (shared by CLI `--format json` + REST)
Define one small Go projector `bom.RenderJSON(model, view)` that emits
`ResolvedModel.Lines` as JSON rows (the same fields the CSV renderer uses:
section/class/manufacturer/model/part_number/description/optic cols +
`fleet_quantity` + membership flags). This keeps the JSON and CSV views from
drifting (both read `[]ResolvedLine`, never a second count). Pure renderer, no
proof obligation.

---

## 4. Retirement list + CI changes (Issue #64 point 4; cross-ref #35/#38/#43)

**Retire (delete) — the old path:**
- `internal/orchestrate/` — `orchestrate.go`, `wire.go`, `encoder_guard_test.go`
  (the guard's own comment tracks this as **#35**, `encoder_guard_test.go:10`).
- `hhfab-adapter/` and `bom-adapter/` — the Rust WASM crates.
- `embed/hhfab.wasm`, `embed/bom.wasm` — the built adapter artifacts.
- `internal/components`: drop `Hhfab()`/`Bom()` (`components.go:47-50`) and the
  `KernelCalculate`/`KernelValidate`/`HhfabExport`/`BomExport` entry constants
  (`components.go:15-23`). **Keep** `Kernel()`, `KernelF2Calculate`,
  `KernelF3Bom`.
- `internal/plan` (`YAMLToJSON`, old invented plan schema) — **iff** nothing else
  uses it after the surfaces move to `IngestBundled`. Verify in RED; the
  planstore reads only `meta` (`planstore.go:205`, id/name/status), not the old
  schema, so it is **not** coupled (good — it can store DIET YAML as-is).
- MoonBit kernel: the old `export_calculate`/`export_validate` ABI shells
  (`kernel/wasm/abi.mbt:71-85`) and any now-dead old-schema calc logic they reach.
  **Keep** `export_f2_calculate`/`export_f3_bom` (`abi.mbt:88-108`),
  `kernel/proofs/`, and the prove gate. Confirm with `moon prove` still green.

**Keep (the rebuilt engine + proofs):** `internal/{topology,catalog,calc,bom,
wiring}`, `internal/components.Kernel()`, `internal/wasmhost`, `kernel/src`
(F2/F3 + proved cores), `kernel/proofs`, `embed/kernel.wasm`.

**CI changes (`.github/workflows/ci.yml`, `Makefile`):**
- Remove the **hhfab-adapter** and **bom-adapter** `cargo test` steps and the
  `make hhfab-wasm`/`make bom-wasm` targets; drop them from `make wasm`.
- The **golden-path** test that shells `hhfab validate` through `orchestrate`
  (and its "did-not-skip" assertion, `ci.yml` ~`:123-136`) is **re-pointed** at
  the rebuilt wiring path — the oracle `hhfab validate` gate
  (`oracle_test.go` F4 test) already does this against `wiring.Render`, so the
  golden-path assertion becomes "the oracle wiring test ran and passed," not a
  separate orchestrate shell. (This is the **#38** CI surface referenced in #64 —
  ⚠️ *I cannot read #38 (gh 401); lead to confirm #38 is the adapter-CI retirement
  I'm inferring from context. Flagged.*)
- `make embed-check` (#33 stale-embed guard) drops `hhfab.wasm`/`bom.wasm` from
  its set, keeps `kernel.wasm`.
- `make ui` / `make ui-test` / `make ui-check` unchanged as gates — the GUI must
  still build and pass with the retargeted fixtures.
- **#43** (Go-version pressure from manual `ServeMux` dispatch, `serve.go:64-66`):
  out of scope to *fix* in F7, but the overlay sub-resource (§2.3) adds one more
  manual route — keep the manual dispatcher, note #43 stays open.

---

## 5. Decomposition — four sub-steps, each its own RED → devb → GREEN → devb → lead merge

Per the issue's suggested order. Each pushes and **pauses at its gate** (never
self-merge).

- **F7a — Coordinator + CLI.** Add `internal/design` (§1) + `catalog.LoadBytes`
  (§2.3); retarget the four CLI commands + add `--overlay`/`--full`/`aid design`.
  **RED:** integration tests that run the CLI against the committed
  `tests/oracle/xoc-64` + `xoc-256` artifacts and assert BOM CSV == committed
  `bom.csv` and quantities == derived counts (real oracle reproduction, not
  "runs"). **GREEN:** wire to the coordinator.
- **F7b — REST.** Retarget `calcPlan`/`bomPlan`/`wiringPlan` to the coordinator;
  add the overlay sub-resource (§2.3); new response shapes (§3.2). **RED:**
  `httptest` integration asserting the calc/bom responses reproduce the oracle
  for xoc-64 (mesh) + xoc-256 (Clos).
- **F7c — GUI.** Retarget `render.mbt` (§3.3) + `ui/test` fixtures; `make ui`
  regenerates `app.js`. **RED/GREEN:** `make ui-test` green against new shapes;
  `make ui` builds; `make ui-check` clean.
- **F7d — Retirement.** Delete the old path (§4) + CI/Makefile changes. **RED:**
  the suite + `moon prove` + `make ui-*` all green *after* deletion; oracle
  xoc-64/128/256 still reproduced (the regression guard). This step is mostly red
  build/test until the deletions are consistent.

Ordering rationale: build the new path and prove it reproduces the oracle through
**two** live surfaces (F7a CLI, F7b REST) **before** deleting the old one (F7d), so
retirement is a safe subtraction, not a leap. GUI (F7c) slots before retirement
because it depends on F7b's response shapes.

---

## 6. Acceptance (Issue #64)

- **CLI + REST reproduce oracle results through the rebuilt engine** for **≥1
  mesh (xoc-64) + ≥1 Clos (xoc-256)** plan — integration-tested against the
  committed `bom.csv` / derived counts / `wiring/*.yaml`, not "it runs."
- **GUI:** `make ui` builds, `make ui-test` green, renders the retargeted
  responses (`make ui-check` clean).
- **Old path retired** (§4): `orchestrate`, both Rust adapters, their wasm +
  entry points, dead old-schema kernel shells — deleted; CI no longer builds/tests
  them.
- **`moon prove` still green**; full suite + CI green.
- **No oracle regression:** xoc-64/128/256 still reproduced (the parametric
  harness, `internal/oracle`, untouched in behavior).
- **No NetBox** (D22); **no invented model/transform** (D25).

---

## 7. Open questions for lead / devb (resolve before RED)

1. **Coordinator package name** — `internal/design` proposed (mirrors the tool's
   purpose); alternatives `internal/pipeline` / `internal/engine`. Lead pick.
2. **#38 confirmation** — I infer #38 is the adapter-CI-retirement surface; gh is
   401 so I cannot read it. Please confirm or correct (§4).
3. **Overlay transport for REST** — companion sub-resource (§2.3, proposed) vs.
   a multipart create. Sub-resource keeps CRUD intact; confirm acceptable.
4. **CLI `topology calc` output** — drop the old `nodes/edges/fabrics` IR line
   entirely (it was the invented IR) and print switch/server quantities + verdict
   summary. Confirm no external consumer depends on the old line.
5. **Old-schema kernel deletion depth** — delete `export_calculate`/
   `export_validate` shells now (proposed), or leave the dead ABI exports in
   `abi.mbt` and only stop calling them? Proposed: delete, to make retirement
   real; confirm the prove gate is unaffected (it gates `kernel/proofs`, not the
   ABI shells).

---

## 8. Evidence (verified before this note)

- Surfaces all route through `orchestrate`: `commands.go:52,89,128,157`;
  `serve.go:32-33,202,225,241`; GUI old shapes `render.mbt:167-176,201-234`.
- Rebuilt engine end-to-end chain (the coordinator target) is the proven oracle
  path: `oracle_test.go:26-49` (ingest+mergeOverlay) + the F3 chain
  (`calc.Compute → bom.Resolve → RenderProjection`) + F4 (`wiring.Render` +
  `hhfab validate`). Ordering (calc on base catalog, overlay merged after) read
  from `oracle_test.go:32,48`.
- Signatures: `topology.IngestBundled(topology.go:288)`,
  `catalog.Merge(catalog.go:215)` / `Load(catalog.go:275)`,
  `calc.Compute(calc.go:164)` → `CalcOutput(calc.go:148-154)`,
  `bom.Resolve(bom.go:148)` / `ResolvedModel`,
  `wiring.Render(wiring.go:48)` → `Doc(wiring.go:39-42)`.
- Retirement targets: `orchestrate/{orchestrate,wire}.go`,
  `components.go:15-23,47-50`, `kernel/wasm/abi.mbt:71-85`, `hhfab-adapter/`,
  `bom-adapter/`, `embed/{hhfab,bom}.wasm`. `#35` per
  `encoder_guard_test.go:10`; `#43` per `serve.go:66`.
- Composition table (oracle targets): `composition.go:42-71` — xoc-64 mesh
  (5/3/21, overlay `tests/fixtures/f3/optic-overlay.yaml`) + xoc-256 clos
  (1/4/9, derived counts, overlay `tests/oracle/xoc-256-.../optic-overlay.yaml`).
- planstore is NOT coupled to the old schema (`planstore.go:205` reads only
  `meta` id/name/status) — it can store DIET training YAML as-is.
