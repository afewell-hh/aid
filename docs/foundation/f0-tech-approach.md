# F0 tech-approach note (RED)

Phase F0 of the foundation rebuild (#48): the model of record + the failing oracle
harness. **No calculation** (calc is F2+). This note accompanies the F0 **RED** for devb
review; it explains the representation, language choices, harness design, and how the
validation contracts are enforced.

## 1. Schema / substrate representation

- **Canonical wire contracts = JSON Schema** under `schema/` (D18): `topology-plan-v2.json`
  and `catalog-v1.json`. These are language-neutral so the MoonBit kernel (F2) and Rust/Go
  reducers (F3) consume the same contracts. The invented `topology-plan-v1.json` is
  **retired** to `schema/superseded/`.
- `topology-plan-v2.json` is a real **`oneOf` contract**, not a placeholder: a document is
  EITHER the external diet/XOC 9-section bundled shape (no `spec`/`status`) OR the
  AID-canonical pure-reference shape (`meta` + constrained `spec` with **pinned**
  `class_ref{name,version}` + optional `status`/`expected`). The branches are mutually
  exclusive, so a malformed `spec`/`status` matches neither and is **rejected**. Verified
  against all **seven** fixtures with a JSON-Schema validator: the two external files and the
  two canonical files validate; the malformed canonical spec (`class_ref` missing its pinned
  `version`) and two external-negatives (`meta`-only, and `spec` mistyped as `specc`) are
  rejected. The external branch requires the load-bearing diet sections (`meta`, `plan`,
  `switch_classes`, `server_classes`, `server_connections`), so a non-plan cannot slip through.
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

`go test ./internal/... ./cmd/... ./embed/...` on this branch — **6 PASS / 21 FAIL / 2 SKIP**:
- **PASS (6, model-of-record + wiring):** substrate dedup (IDs, kinds); `catalog`
  `TestModel_ExpressesRealServer` (the model represents the owner's real B200 server — 8× CX-7
  as one quantity-8 slot, BF3 = 1 fixed BMC + 2 cages, 4 non-physical slots); `planschema`
  schema files are valid JSON; `oracle` Layer A/B oracles wired (counts pinned).
- **FAIL (21, the F0 GREEN targets — failing for the right reason, `ErrNotImplemented`):**
  - `planschema` (7): the external training.yaml + topology-plan.yaml validate; the canonical
    input-only + input+expected validate; the malformed canonical spec and two external-negatives
    (meta-only, typoed `specc`) are **rejected**.
  - `topology` (8) — guardrail-locked: ingest yields **pinned** class refs that resolve (G1);
    an unpinned ref is rejected → `ErrUnpinnedRef` (G1); round-trip preserves
    reference_data object ids by subsection + full server_nics/server_connections identity tuples
    + expected.counts (G2); Validate resolves refs,
    rejects an unresolved ref → `ErrUnresolvedRef`, and **ignores** conflicting status (G3);
    ExpandPorts yields the exact `(server_class,nic_slot,port_index)→zone` sequence and rejects
    insufficient cages → `ErrInsufficientPorts` (G4).
  - `objectmodel` (3): required-fields-per-projection, `component_slot` acyclicity, quantity
    composition.
  - `catalog` (3): Contracts (acyclic+quantity-bearing component_slot), ToObjects, Load.
- **SKIP (2, pending calc):** `oracle` Layer A counts comparison, Layer B 1×/2× scaling.

The guardrail tests assert exact promised behavior (pinned refs, lossless preservation,
status-ignored, deterministic expansion sequence, typed rejection errors) so a trivial or
partially-fake GREEN cannot pass them (devb RED review, #48).

At **F0 GREEN** the 21 FAILs become PASS (schema validates the real files; ingest round-trips
losslessly; the substrate contracts hold) and only the 2 oracle comparisons remain skipped, so
`main` stays green. No calc lands in F0.

> Note: `gitignored/refs/{hnp,xoc}` (the developer-side authoritative clones) contain non-AID
> source and are `.gitignore`d — they are absent from a clean checkout, so CI's `go test ./...`
> is unaffected by them. Verify locally with `go test ./internal/... ./cmd/... ./embed/...`.

## 6. F0 GREEN plan (after RED review)

Implement `planschema.Validate` (wire the validator + YAML→JSON); `topology.IngestBundled`/
`Rebundle` (deterministic lossless split of `reference_data` → catalog, IDs preserved);
`catalog.Contracts`/`ToObjects`/`Load`; `objectmodel.Validate`/`CheckAcyclic`/`ComposeQuantity`;
`topology.Validate`/`ExpandPorts`. Author the AID catalog fixture. Turn the 21 RED tests green;
keep the 2 oracle comparisons pending. Still no calc.

## 7. F0 GREEN result (implemented)

All 21 RED targets are green; the 2 oracle comparisons stay skipped (pending calc, F2+), so
the suite is green for what F0 lands. **No calc** was added.

- **`planschema.Validate`** loads the JSON Schema, normalizes YAML→JSON-compatible values, and
  validates via `github.com/santhosh-tekuri/jsonschema/v5` (the only new runtime dep; pure-Go,
  draft 2020-12). The real **`training.yaml` validates**, the authored `topology-plan.yaml`
  validates, the two canonical fixtures (input-only / input+expected) validate, and the three
  negatives (malformed canonical spec, meta-only, typoed `specc`) are rejected with real
  validation errors.
- **`topology.IngestBundled`/`Rebundle`** split the bundled plan into a pure-reference `Plan` +
  the extracted `Catalog`: every server/switch class becomes a **pinned** (`id@version`)
  catalog ref that resolves into the extracted catalog (G1), and the `reference_data`/
  `server_nics` are retained on the catalog verbatim while connections (with `target_zone` split
  into `class`+`zone`) and `expected.counts` are modeled on the plan — so the bundle round-trips
  losslessly by canonical identity (G2). `IngestPureReference` rejects an unpinned ref
  (`ErrUnpinnedRef`).
- **`topology.Validate`** resolves refs to catalog classes and **never reads `Status`** (G3): a
  conflicting `status.expected` does not affect validation. **`topology.ExpandPorts`** yields the
  exact ascending `(server_class, nic_slot, port_index) → zone` sequence and rejects a
  connection that overflows the NIC's cages (`ErrInsufficientPorts`) (G4).
- **`objectmodel`** enforces required-fields-per-projection (`ErrContract`), `component_slot`
  acyclicity (`ErrCycle`, DFS three-color), and the quantity multiply (`ComposeQuantity`).
- **`catalog`** declares the contracts (acyclic, quantity-bearing `component_slot`; `bom`
  projection requires the SKU), projects items onto the substrate (`ToObjects`), and parses the
  authored `tests/oracle/xoc-64-mesh-conv-ro/catalog.yaml` (`Load`, via the YAML→JSON tag bridge).

**Result:** `go test ./internal/... ./cmd/... ./embed/...` → **27 PASS / 0 FAIL / 2 SKIP**
(the 6 model-of-record/wired + 21 former-RED targets pass; the 2 oracle comparisons skip,
pending calc). `go build ./cmd/aid` and `go vet` clean.
