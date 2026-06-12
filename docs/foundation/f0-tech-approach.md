# F0 tech-approach note (RED)

Phase F0 of the foundation rebuild (#48): the model of record + the failing oracle
harness. **No calculation** (calc is F2+). This note accompanies the F0 **RED** for devb
review; it explains the representation, language choices, harness design, and how the
validation contracts are enforced.

## 1. Schema / substrate representation

- **Canonical wire contracts = JSON Schema** under `schema/` (D18): `topology-plan-v2.json`
  (the real diet/XOC 9-section shape + a Kubernetes-style `spec`/`status` plane) and
  `catalog-v1.json` (the two-layer catalog). These are language-neutral so the MoonBit
  kernel (F2) and Rust/Go reducers (F3) consume the same contracts. The invented
  `topology-plan-v1.json` is **retired** to `schema/superseded/`.
- **General extensible object substrate** (`internal/objectmodel`, §4.2/D19): every modelled
  thing is a typed `Object{Kind, ID, Attributes: map[namespace]map[field]any, Relations[]}`
  with **open namespaced attributes** (`calc_profile`/`purchase_profile`; future
  power/lifecycle/cost planes attach the same way) and **arbitrary typed nested relations**
  (`component_slot`, `port_template`). New features extend by adding namespaces/relations/
  projections — never by re-foundationing.
- **Concrete typed views** mirror the schemas for the F0 consumers: `internal/catalog`
  (two layers: bare hardware *types* vs configured server/switch *classes*; `component_slots`
  incl. the quantity-bearing 8× CX-7 slot; `port_templates` fixed-vs-cage; `bom_line_templates`
  physical+non-physical; per-NIC-port `cage_bindings`) and `internal/topology` (relational
  `Plan{Meta, Spec, Status}`; `ServerClassUse`/`SwitchClassUse` reference catalog classes by
  **pinned** `Ref{ID, Version, Digest}`; `ServerConnection` keyed on `(nic, port_index) → zone`).

## 2. Language choice — Go for F0

F0 is **I/O + data-modeling + test orchestration** (parse YAML, split `reference_data`,
validate against JSON Schema, read committed CSV/JSON oracles, compare). **Go** owns exactly
this surface already (`internal/plan`, `internal/planstore`, `internal/orchestrate`, the
`hhfab` harness, `go test` + CI). So F0's model of record, ingest, and harness are Go; the
JSON Schemas are the cross-language contract. The **MoonBit** calc kernel (D2) arrives in F2
and the **Rust/Go** BOM reducer in F3, each consuming the same schemas. No new runtime deps in
RED; F0 GREEN adds a JSON-Schema validator (`santhosh-tekuri/jsonschema/v5`, pure-Go) +
a YAML→JSON normalizer for `planschema.Validate`.

## 3. Harness design (`internal/oracle`, §4.5/D20)

Two layers, both **wired to the committed oracles** vendored under `tests/oracle/`:
- **Layer A** — xoc-64 *training* form (the 1:1 oracle; provenance hard-gate). Loaders read
  the committed `bom.csv`, `connectivity-map.csv`, `netbox_inventory.json .metadata.counts`
  (21 devices / 259 modules / 481 interfaces / 128 cables), and `wiring/*.yaml`. Comparison
  functions (`CompareBOMProjection`, `CompareConnectivityMap`, `CompareCounts`,
  `CompareWiringHhfab`) need calc → **pending** (F2+).
- **Layer B** — the owner full BOM (`docs/requirements/real-server-bom.csv`), incl. 1×/2×
  scaling (`CompareFullBOM`) → **pending** (F3).

The loaders are real (they prove the harness is genuinely wired and the counts are pinned);
the *comparisons* are reported unimplemented so their tests **skip (pending), not red**.

## 4. How the validation contracts are enforced (the F0 gate)

`internal/objectmodel` is the contract surface. Each object Kind declares a `Contract`:
`RequiredByProjection` (which `namespace.field`s a consumer like the BOM/HNP-projection/wiring
demands), and per-relation `RelationContract{Acyclic, QuantityField}`. The checks:
- **stable IDs** — `NewGraph`/`catalog.New` reject duplicate IDs (enforced now).
- **required-fields-per-projection** — `Registry.Validate(graph, projection)` (F0 GREEN).
- **`component_slots` acyclicity** — `Registry.CheckAcyclic(graph, "component_slot")` → `ErrCycle`.
- **quantity composition** — `Registry.ComposeQuantity(graph, root, path)` (the 8× multiply).
- **clear errors** — distinct sentinels `ErrInvalidGraph`/`ErrContract`/`ErrCycle`.

The **4 guardrails**: (1) pinning — refs are `ID{Name,Version}`+optional `Digest`; (2)
deterministic lossless ingest — `topology.IngestBundled`/`Rebundle` round-trip (tested); (3)
status-never-drives-calc — `topology.Validate` ignores `Status`, read only in self-check; (4)
deterministic `ports_per_connection>1` — `topology.ExpandPorts` (defined now, exercised by calc later).

## 5. RED state (what devb is reviewing)

`go test ./...` on this branch:
- **PASS (6, model-of-record + wiring):** substrate dedup (IDs, kinds); `catalog`
  `TestModel_ExpressesRealServer` (the model represents the owner's real B200 server — 8× CX-7
  as one quantity-8 slot, BF3 = 1 fixed BMC + 2 cages, 4 non-physical slots); `planschema`
  schema files are valid JSON; `oracle` Layer A/B oracles wired (counts pinned).
- **FAIL (12, the F0 GREEN targets — failing for the right reason, ErrNotImplemented):**
  `planschema` validates training.yaml + topology-plan.yaml (×2); `topology` IngestBundled,
  round-trip, Validate, ExpandPorts (×4); `objectmodel` required-fields, acyclicity, quantity
  composition (×3); `catalog` Contracts, ToObjects, Load (×3).
- **SKIP (2, pending calc):** `oracle` Layer A counts comparison, Layer B 1×/2× scaling.

At **F0 GREEN** the 12 FAILs become PASS (schema validates the real files; ingest round-trips
losslessly; the substrate contracts hold) and only the 2 oracle comparisons remain skipped, so
`main` stays green. No calc lands in F0.

> Note: `gitignored/refs/{hnp,xoc}` (the developer-side authoritative clones) contain non-AID
> source and are `.gitignore`d — they are absent from a clean checkout, so CI's `go test ./...`
> is unaffected by them. Verify locally with `go test ./internal/... ./cmd/... ./embed/...`.

## 6. F0 GREEN plan (after RED review)

Implement `planschema.Validate` (wire the validator + YAML→JSON); `topology.IngestBundled`/
`Rebundle` (deterministic lossless split of `reference_data` → catalog, IDs preserved);
`catalog.Contracts`/`ToObjects`/`Load`; `objectmodel.Validate`/`CheckAcyclic`/`ComposeQuantity`;
`topology.Validate`/`ExpandPorts`. Author the AID catalog fixture. Turn the 12 RED tests green;
keep the 2 oracle comparisons pending. Still no calc.
