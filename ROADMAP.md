# AID Implementation Roadmap

Each phase produces a reviewable deliverable. No phase begins until the previous phase
is approved. Phases 1–5 can proceed in parallel where dependencies allow.

---

## Phase 1 — WIT Interface Design

**Goal:** Define the WASM component boundaries before any implementation begins.

**Deliverable:**
- `wit/topology-calculator.wit`: full input/output type definitions for the kernel
- `wit/hhfab-adapter.wit`: `TopologyIR` → wiring YAML interface
- `wit/bom-adapter.wit`: `DeviceClassBOM[]` → CSV/JSON interface
- `wit/netbox-adapter.wit`: `TopologyIR` + config → publish result interface

**Exit gate:**
- WIT files are valid (`wasm-tools validate`)
- All domain types from `DOMAIN_MODEL.md` are represented in WIT
- Review: are the type boundaries clean? No "God types" that mix concerns?
- MoonBit bindgen (`wit-bindgen moonbit`) generates valid scaffolding from the WIT files

**Read first:** `ARCHITECTURE.md`, `DOMAIN_MODEL.md`

---

## Phase 2 — Topology Plan Schema

**Goal:** Define the canonical topology plan YAML schema as a versioned JSON Schema.

**Deliverable:**
- `schema/topology-plan-v1.json`: JSON Schema for plan YAML files
- `schema/README.md`: field reference and example
- `tests/fixtures/`: 3–5 reference plan YAML files derived from the reference architecture
  (see `HNP_REFERENCE.md` for source)
- Expected output counts for each fixture (device count, per-fabric switch counts, BOM totals)

**Exit gate:**
- All fixture files validate against the schema
- Schema is self-describing (field descriptions present for all fields)
- A plan that the kernel will reject (e.g., mesh with 4 switches) fails schema validation
  with a clear error message

**Read first:** `DOMAIN_MODEL.md`, `ALGORITHMS.md`

---

## Phase 3 — MoonBit Topology Kernel (Pure Logic, No Proofs)

**Goal:** Implement the topology calculation engine in MoonBit. Tests pass. Proofs not yet written.

**Deliverable:**
- `kernel/` MoonBit package implementing all 8 algorithms in `ALGORITHMS.md`
- `moon test` passes all unit tests
- Compiles to a WASM component that the Go CLI can invoke via `wasmtime-go`
- Behavioral acceptance: for each reference fixture, kernel produces correct device/cable/interface counts

**Exit gate:**
- All fixture acceptance tests pass
- All constraint violations (mesh=4, MCLAG odd count, mixed rail/alternating) are surfaced
  as `ValidationResult.errors`, not panics
- Oversubscription ratio computed and present in `FabricSummary` for every fabric

**Read first:** `ALGORITHMS.md`, `DOMAIN_MODEL.md`, Phase 1 WIT definitions

---

## Phase 4 — BOM Adapter

**Goal:** Implement plan-time BOM derivation. No NetBox, no database.

**Deliverable:**
- `bom-adapter/` package (MoonBit or Rust)
- `DeviceClass.bom(quantity)` → `DeviceClassBOM` recursive traversal (per-unit and fleet totals)
- CSV output: per-device-class section with sub-component tree, per-unit column, fleet-total column
- JSON output matching `wit/bom-adapter.wit`
- Test: given the reference GPU server (from fixture), verify fleet totals are quantity × per-unit
  at every level (server, NIC, transceiver)

**Exit gate:**
- Per-unit BOM is identical regardless of fleet quantity
- Fleet total = per-unit × quantity at each tree level (formally verified or test-proven)
- A switch device class with transceiver sub-components produces a correct BOM as a non-server example
- CSV output is human-reviewable and matches manual calculation from fixture input

**Read first:** `DOMAIN_MODEL.md` (DeviceClass and DeviceClassBOM sections), `ALGORITHMS.md` (Algorithm 6)

---

## Phase 5 — hhfab Wiring Adapter

**Goal:** Produce hhfab-compatible wiring YAML from `TopologyIR`. No NetBox.

**Deliverable:**
- `hhfab-adapter/` Rust crate implementing the WIT interface
- For each reference fixture: `hhfab validate` passes on the generated wiring YAML
- Per-fabric split supported (`--fabric backend` flag)

**CRD types to emit:** VLANNamespace, IPv4Namespace, SwitchGroup, Switch, Server,
Connection (unbundled / bundled / mclag / eslag / fabric / mesh variants)

**Exit gate:**
- `hhfab validate` passes for all reference fixtures
- Per-fabric export produces one valid YAML per managed fabric
- No empty `ecmp: {}` sections (known hhfab validation failure)

**Read first:** `ARCHITECTURE.md` (Layer 2 section), hhfab wiring CRD schema reference

---

## Phase 6 — Go CLI and Plan Storage

**Goal:** Ship the `aid` CLI with all core subcommands wired to the WASM components.

**Deliverable:**
- `cmd/aid/` Go CLI with subcommands:
  - `aid plan validate <file>`
  - `aid topology calc <file> [--output topology.json]`
  - `aid topology bom <file> [--format csv|json]`
  - `aid export wiring <file> [--fabric <name>]`
- Local SQLite state: tracks last IR hash per plan file, flags plan-changed since last calc
- `~/.aid/config.yaml` for NetBox URL, token, default site
- `aid serve [--port 8080]` — REST API server stub (endpoints stubbed, fully implemented in Phase 6b)

**Exit gate:**
- Golden path: `aid topology calc fixture.yaml && aid export wiring fixture.yaml --fabric backend`
  produces a wiring YAML that passes `hhfab validate`
- `aid plan validate` gives a human-readable error for every constraint violation
- Single-binary distribution: `go build -o aid ./cmd/aid`

---

## Phase 6b — Web Frontend (MoonBit → JavaScript + Bootstrap 5)

**Goal:** Ship a browser-based GUI that emulates NetBox's visual style. Depends on Phase 6 API stub.

**Deliverable:**
- `ui/` MoonBit module compiled to JavaScript (`moon build --target js`)
- `aid serve` fully implements REST endpoints consumed by the frontend:
  - `GET/POST /api/plans` — plan list and create
  - `GET/PUT/DELETE /api/plans/:id` — plan detail and edit
  - `POST /api/plans/:id/calc` — trigger topology calculation
  - `GET /api/plans/:id/bom` — BOM as JSON
  - `GET /api/plans/:id/wiring/:fabric` — wiring YAML download
- Bootstrap 5 bundled in `ui/static/` (no CDN dependency)
- UI surfaces: plan list, plan detail (fabrics + device classes + BOM summary + validation),
  device class editor, connection intent editor, wiring export trigger

**Visual standard:** Match NetBox's Bootstrap 5 appearance — dark navbar, card layout,
table views with row actions, badge-based status indicators.

**Exit gate:**
- Plan list, plan detail, and topology calc trigger work end-to-end in a browser
- BOM view shows per-unit and fleet-total for a multi-level device class (server → NIC → transceiver)
- `aid serve` binary passes `go test` for all REST endpoint handlers
- Air-gapped test: run with no internet access; all assets load from bundled static files

**Read first:** `ARCHITECTURE.md` (Layer 5), `DECISIONS.md` (D14, D15), `TECH_STACK.md` (MoonBit → JS section)

---

## Phase 7 — MoonBit Formal Verification Spike (Go/No-Go Gate)

**Goal:** Validate that MoonBit's formal verification is practical for AID's invariants.

**Time-box:** 2 weeks maximum.

**Target invariant for the spike:** Port allocation non-overlap
(no logical port is allocated to more than one connection in any zone).

**Deliverable:**
- `.mbtp` predicate file and `proof_requires`/`proof_ensures` annotations for port allocator
- `moon prove` verifies the invariant
- Go test calls the MoonBit component via `wasmtime-go`, measuring latency

**Success criteria (all required to proceed to Phase 8):**
1. `moon prove` verifies the non-overlap invariant within 30 seconds (Z3 does not time out)
2. MoonBit component callable from Go test via `wasmtime-go`, cold-start < 500ms, per-call < 10ms
3. MoonBit stdlib used in spike is compatible with MoonBit 1.0 (no breaking renames post-spike)
4. Formal verification syntax is documentable without AI assistance for new invariants

**If spike fails:** The kernel stays in pure Go or Rust. MoonBit is revisited at MoonBit 1.1.
No other phases are blocked by this outcome.

---

## Phase 8 — Formal Verification (if Phase 7 passes)

**Goal:** Add `moon prove` proofs for all kernel invariants identified in `DECISIONS.md`.

**Invariants to verify:**
- Port non-overlap
- Allocation completeness
- Switch count lower bound
- BOM scaling correctness
- Mesh switch count ∈ {2, 3}
- MCLAG even-count

**Exit gate:**
- `moon prove` passes for all 6 invariants in CI
- Proof failure blocks the build

---

## Phase 9 — NetBox Publish Adapter

**Goal:** Optional NetBox publish path via REST API.

**Deliverable:**
- `netbox-adapter/` (Rust or Go) implementing `wit/netbox-adapter.wit`
- `aid publish netbox <plan.yaml> --netbox-url ... --token ...`
- Idempotent: re-running publish on a previously-published plan updates existing objects
- `aid publish netbox --clean <plan.yaml>` deletes all aid-tagged objects for the plan

**Exit gate:**
- Integration test against a local NetBox instance: publish → validate objects exist → clean → validate gone
- Does not use NetBox ORM — REST API only

---

## Phase 10 — OCP Contribution Package

**Goal:** Publish community-facing artifacts to OCP.

**Deliverable:**
- `schema/topology-plan-v1.json` published to a separate `aid-schemas` repository (Apache 2.0)
- `algorithms/` folder: written documentation of the 8 algorithms in `ALGORITHMS.md`
  with formal mathematical notation and proofs of key properties
- OCP technical note: server-driven switch quantity formula and DeviceClass composite BOM model

**Read first:** `DECISIONS.md` (D10, D12)

---

## Dependency Graph

```
Phase 1 (WIT)
    ├── Phase 2 (Schema)    ─── Phase 3 (Kernel) ─── Phase 7 (Spike)
    │                                │                     └── Phase 8 (Proofs)
    │                                ├── Phase 4 (BOM)
    │                                └── Phase 5 (hhfab)
    │                                        │
    └───────────────────────────── Phase 6 (CLI) ── Phase 6b (Frontend)
                                        │                │
                                        └── Phase 9 (NetBox)
                                                    └── Phase 10 (OCP)
```

Phases 2–5 can proceed in parallel after Phase 1 completes.
Phase 6 depends on 3, 4, 5.
Phase 6b depends on Phase 6 (REST API stub).
Phase 9 depends on Phase 6.
Phase 10 depends on all previous phases.
