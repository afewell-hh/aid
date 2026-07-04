# AID Architecture

> **Status (2026-07): current.** This document was rewritten to match the
> **foundation-rebuild architecture (F0–F7)** that AID actually runs today. It
> **supersedes** the earlier "five-layer WASM Component Model / universal
> `DeviceClass` / Rust export adapters / SQLite" description, which described an
> **invented** design that was replaced during the rebuild. Authoritative
> sources, in order: **`DECISIONS.md`** (esp. D16, D18, D19, D21, D22, D23,
> D25, D26), **`docs/foundation-redesign.md`** (the rebuild design), and the
> code itself. If this file and `DECISIONS.md` ever disagree, `DECISIONS.md`
> wins.

## What AID is

AID (AI Infrastructure Designer) turns a declarative topology **plan** for an
AI/ML cluster into: **switch/server quantities**, a **bill of materials**, and
**hhfab-validatable wiring**. Its correctness anchor is that it **reproduces the
real OCP/XOC reference outputs** (quantities, `bom.csv`, wiring) for the real
diet/XOC plans — validated by an oracle harness against vendored reference
compositions (mesh xoc-64/128, Clos xoc-256). See `DECISIONS.md` D18/D20.

## Design principles (as built)

1. **Reproduce reality, don't invent it.** The plan schema, the domain model,
   and the catalog are the *real* diet/XOC shapes (D18/D19). AID does not model
   an invented topology language. Where AID intentionally diverges from a real
   reference artifact, it is recorded (e.g. D27).
2. **A small proved core, everything else ordinary Go.** The one place with a
   formal-methods boundary is the **calculation kernel** (MoonBit, `moon
   prove` via Why3+Z3). Everything else — ingest, catalog resolution, BOM,
   wiring, surfaces — is plain Go (D2, D16, D23).
3. **One WASM boundary, not a system-of-systems.** Only the kernel crosses a
   WASM boundary (JSON-over-linear-memory, D16). The BOM and wiring exporters
   are **Go packages over the resolved model, not Rust/WASM adapters** (D23).
   The old Rust `hhfab-adapter`/`bom-adapter` were retired (F7d).
4. **Plan is the source of truth; state is a flat file.** Plans are bundled DIET
   YAML documents in a directory of `<id>.yaml` files (D21). No database.
5. **NetBox is deferred** (D22) — not part of the core pipeline.

## The pipeline (data flow)

A plan flows through pure Go packages, with the proved kernel called in the
middle for the arithmetic:

```
bundled DIET plan YAML
   │  internal/topology.IngestBundled          (parse → relational model + catalog)
   ▼
(*topology.Plan, *catalog.Catalog)
   │  internal/calc.Compute / calc.Evaluate     (build calc-plan → run KERNEL → decode)
   ▼        └─ internal/components.Kernel → embed/kernel.wasm  (via internal/wasmhost, D16)
calc.CalcOutput  { switch/server quantities · per-endpoint allocation IR · transceiver verdicts · errors }
   │                                   ┌── internal/bom.Resolve → RenderProjection / RenderFullBOM  (bom.csv + full BOM)
   ├───────────────────────────────────┤
   │  (catalog + overlay merged AFTER calc) └── internal/wiring.Render → hhfab CRDs per managed fabric
   ▼
internal/design  = the coordinator facade over all of the above (one Resolve → validation + quantities + BOM + wiring)
```

`internal/design.Resolve/Evaluate/Wiring` is the single facade the surfaces call.
`calc.Compute` fails on kernel-reported errors; `calc.Evaluate` is the non-failing
variant so surfaces can *show* validation (two-plane validation): structural/parse
failure → HTTP 4xx; calc errors → `is_valid:false` data. The optic/identity
**overlay** is merged into the catalog *after* calc (it only affects BOM optic
columns + wiring `profile`, not the arithmetic).

## The calculation kernel (MoonBit + `moon prove`)

- `kernel/src/*.mbt` — the calc: `decode`/`encode` (the D16 JSON boundary),
  `alloc`/`distribution`/`switch_count`/`f2_calc` (port allocation, distribution,
  switch-count derivation incl. Clos spine counts), `bom` (BOM scaling).
- `kernel/proofs/*.mbt` (`cores.mbt`) — **proved pure cores** (Int/Bool only;
  no Array/struct translation). `kernel/src` routes its arithmetic through these
  cores; `moon prove` (Why3 1.7.2 + Z3 4.8.12, opam switch `why3env`) proves the
  invariants (port non-overlap, ceil-div lower bounds, redundancy floors,
  BOM/fleet scaling). A CI gate parses `moon prove` stdout (exit code is always
  0) and a negative control must stay red. See `scripts/moon-prove-gate.sh`.
- Built to **`embed/kernel.wasm`**, embedded in the Go binary and called over the
  **D16 boundary** (`alloc`/`dealloc` + `export_*(ptr,len)->packed`, JSON in/out)
  by `internal/wasmhost` via `internal/components.Kernel`. This is the **only**
  WASM component (the `abi.mbt` old export shells are dead and tracked for
  removal; `docs/followups/retire-old-kernel-abi-shells.md`).

## The engine packages (Go)

| Package | Role |
|---|---|
| `internal/objectmodel` | typed-but-open object substrate + pinned `ID{Name,Version}` |
| `internal/topology` | ingest bundled DIET plan → relational `Plan` (spec + status/expected) + resolve into the catalog; `Validate`, `Rebundle` |
| `internal/catalog` | the AID-owned two-layer catalog (bare hardware types + configured classes) + overlay merge |
| `internal/planschema` | the plan/catalog JSON schemas |
| `internal/calc` | build the calc-plan, run the kernel, decode `CalcOutput`; `Compute` (fail-fast) + `Evaluate` (non-failing) + `DeriveQuantities` |
| `internal/bom` | one `Resolve` → `RenderProjection` (`bom.csv`) / `RenderFullBOM` / `RenderJSON` (D23) |
| `internal/wiring` | `Render` → per-fabric hhfab CRDs incl. mesh + Clos fabric links (D23) |
| `internal/oracle` | parametric oracle harness (`Composition` table) — reproduces each vendored reference end-to-end |
| `internal/design` | the coordinator facade the CLI/REST call |
| `internal/planedit` | structured projection + `yaml.Node` surgical field-patch / create-ops (re-validate-before-persist, D26) |
| `internal/library` | strict dedup-by-pinned-id **union** of the built-in reference catalogs (Epic #75) |
| `internal/templates` | embedded starter plans + overlays (`go:embed`), served as templates |
| `internal/planstore` | flat-file `<id>.yaml` (+ `<id>.overlay.yaml`) plan store (D21) |
| `internal/wasmhost`, `internal/components` | WASM host + kernel loader |

## Surfaces (all on the rebuilt engine — F7)

- **CLI** (`cmd/aid`): `plan validate`, `topology calc`, `topology bom`,
  `export wiring`, `design`, `serve` — all route through `internal/design`.
- **REST** (`aid serve`, `cmd/aid/serve.go`, Go `net/http`): `/api/plans` CRUD,
  `/api/plans/{id}/{calc,bom,wiring/{fabric},overlay,validate}`, `/api/catalog`,
  `/api/templates`, plus the structured projection/patch/create ops for the GUI
  editor. Plan store is `internal/planstore`.
- **GUI** (`ui/`, MoonBit→JS, D14; Bootstrap 5, D15): a client-only SPA over the
  REST API — plan list, structured designer (server/switch classes, zones, NICs,
  connections, mesh↔Clos), live validation, overlay, BOM/wiring/CSV export, the
  built-in Library browse + reference gallery. Real-browser Playwright E2E
  (`make ui-e2e`) + a Node mock-harness (`make ui-test`). See
  `docs/ux/gui-ux-review.md`.

## What was retired (so old references don't mislead)

- The universal **`DeviceClass`** model → the real diet/XOC **relational** model
  + component-graph catalog (D18/D19). See `DOMAIN_MODEL.md`.
- **Rust WASM export adapters** (`hhfab-adapter`, `bom-adapter`, `ir-gen`,
  `bom-gen`) + `internal/orchestrate` → deleted (F7d); replaced by the Go
  renderers `internal/bom` + `internal/wiring` (D23).
- **SQLite state** → never built; flat-file YAML plan store (D21).
- **`TopologyIR`** (the old export envelope) → the F2 `calc.CalcOutput` IR.
- **NetBox integration as a core layer** → deferred (D22).
- The design-time "authored-map → training normalization" phase → dropped; the
  DIET/training YAML *is* the authoring format (D25).
