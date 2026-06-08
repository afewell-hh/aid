# AID Key Decisions

This document records the key architectural and product decisions for AID and their rationale.
These are settled decisions — they are not open for re-debate without a new decision record.

---

## D1: No Python in the AID codebase

**Decision:** AID is implemented in MoonBit, Rust, and Go only. Python is not used.

**Rationale:** Python's presence in predecessor tools was entirely due to the Django/NetBox
ORM coupling. AID does not use the NetBox ORM — it calls NetBox via REST API, which any
language can do. Python adds nothing that Rust or Go cannot do more efficiently for a
standalone CLI tool. Engineering team preference explicitly excludes Python.

**Consequences:** The existing test suite for the predecessor tool (~870 Django tests)
cannot be run against AID. The behavioral contract is extracted from fixture YAML files
and expressed as new tests in Go/Rust.

---

## D2: MoonBit for the topology calculation kernel

**Decision:** The topology calculation kernel (switch quantity math, port allocation,
BOM derivation, constraint validation) is implemented in MoonBit and compiled to a
WASM component.

**Rationale:** MoonBit v0.9+ provides first-class formal verification via `moon prove`
(Z3 backend). The topology kernel has hard correctness invariants (port non-overlap,
switch count lower bounds, BOM scaling) that benefit from machine-checked proofs. MoonBit
is WASM-native and supports the Component Model. The formal verification investment is
highest-value on pure-function, zero-I/O code — exactly what the topology kernel is.

**Go/No-Go gate (Phase 7 spike required before committing to MoonBit production use):**
- `moon prove` verifies port allocation non-overlap invariant within 30 seconds
- MoonBit component callable from Go via `wasmtime-go` with cold-start < 500ms
- MoonBit stdlib compatible with 1.0 (no breaking renames post-spike)
- Proof syntax documentable without AI assistance for new invariants

If the spike fails any criterion: fall back to pure Go or Rust for the kernel.

**Spike outcome (resolved 2026-06-07 — issue #5 / PR #19): PASS. MoonBit is confirmed
for the kernel.** Evidence in `spikes/moonbit-port-proof/README.md`, independently
reviewed (issue #20). Gate-by-gate:
- ✅ `moon prove` verified the port-non-overlap invariant in **~0.6s** (< 30s). A
  negative control confirmed the solver genuinely rejects a false claim (not a no-op).
- ✅ MoonBit-produced WASM callable from Go via `wasmtime-go`: cold-start **~2ms**
  (< 500ms), per-call **~7.6µs** (< 10ms). Validated via the **core-wasm** path; the
  full Component-Model boundary (canonical-ABI marshalling of the richer `aid:core`
  types) is deferred to Phase 3.
- ⚠️ MoonBit 1.0 stdlib compatibility is **unverifiable until 1.0 ships**; the spike
  used only `Int` arithmetic + the proof DSL (low churn risk). Re-verify at 1.0.
- ✅ Proof syntax documented for another agent (maintenance guide in the spike README).

**Material consequence:** `moon prove` requires **Why3 1.7.2 + an SMT solver (Z3)**,
which are not bundled with the MoonBit toolchain. They are now provisioned and
documented in `DEVELOPMENT.md`; folding them into CI/setup and adding a `moon prove`
stdout-parsing gate (its exit code is unreliable) is tracked in **#21**, a
**prerequisite for Phase 8** (#7).

---

## D3: Rust for wiring YAML export and NetBox REST adapter

**Decision:** The hhfab wiring YAML adapter and NetBox REST adapter are implemented in Rust.

**Rationale:** Rust has `serde_yaml` (production-proven YAML serialization), `reqwest`
(async HTTP), and `cargo-component` (first-class WASM Component Model tooling). These
libraries have no equivalents in MoonBit's current ecosystem. Rust and MoonBit have a
compatible memory safety story (no GC, ownership) and are natural partners in a
WASM component architecture.

---

## D4: Go for the CLI, plan storage, and orchestration

**Decision:** The `aid` CLI, plan YAML persistence, SQLite state storage, and WASM
component orchestration are implemented in Go.

**Rationale:** Go's `cobra`/`viper` ecosystem is the strongest for CLI tooling. Go
compiles to a single static binary, simplifying distribution. `wasmtime-go` provides
the WASM component host. `mattn/go-sqlite3` covers local state storage. Go is faster
to iterate on than Rust for I/O-heavy orchestration code.

---

## D5: NetBox is an optional publish adapter, not a dependency

**Decision:** AID topology calculation, BOM derivation, and wiring export all work
without a running NetBox instance. NetBox is an optional downstream publish target.

**Rationale:** The predecessor tool was tightly coupled to NetBox's Django ORM.
This coupling required a full NetBox stack for any topology calculation, prevented
offline use, and made testing expensive. AID eliminates this by treating NetBox as one
possible output target among several. The NetBox adapter calls the REST API only.

---

## D6: BOM derivation is plan-time, not inventory-time

**Decision:** The BOM is derived from the plan model at plan time, not from generated
inventory after a database write. `DeviceClass.bom()` returns the per-unit BOM with
no database queries. Fleet totals are `per_unit_count × plan_entry.quantity`.

**Rationale:** A pre-sales topology design tool must be able to produce a procurement
BOM before any hardware is ordered or any database is populated. The predecessor tool's
inventory-based BOM (reading from generated NetBox Module records) could not produce a
BOM until after full generation into a running NetBox. That was a fundamental mismatch
with the tool's use case.

---

## D7: PlanConnection is owned by PlanEntry via NIC slot reference

**Decision:** `PlanConnection` records are owned by their `PlanEntry` and reference a
specific NIC sub-component by its `slot_id` within the parent `DeviceClass`. The
connection ownership hierarchy matches physical reality.

**Rationale:** A cable plugs into a port on a NIC card. The NIC is a sub-component of
the device class (a `SubComponent` with a `slot_id`). `PlanConnection` references the
slot to identify which NIC the port belongs to. This keeps connection ownership at the
plan-entry level (where quantity lives) while preserving the NIC as a named structural
element. See D13 for the full DeviceClass model.

---

## D8: WASM Component Model for all capability boundaries

**Decision:** Every major AID capability boundary is expressed as a WIT interface in
`wit/`. Components communicate only through these interfaces.

**Rationale:** The user's stated goal is a "system of systems" architecture with
well-defined boundaries. The WASM Component Model provides language-agnostic,
formally-specified interfaces. The topology calculator, wiring adapter, BOM adapter,
and NetBox adapter are each independent components that can be versioned, tested,
and swapped independently. This also makes AID extensible: a third party can implement
a new export adapter (e.g., for a different network fabric) by implementing the
`topology-ir → wiring-yaml` WIT interface.

---

## D9: Plan YAML as canonical input format

**Decision:** The topology plan YAML file is the canonical user-authored input to AID.
AID does not require a GUI or a database to create or edit plans.

**Rationale:** Plans are source-of-truth documents that should be version-controlled,
reviewed in pull requests, and shared between teams. A YAML file (or JSON) is the right
format. The schema is published as a JSON Schema specification (see D10).

---

## D10: Topology plan schema published as OCP community artifact

**Decision:** The topology plan YAML schema is published as a versioned JSON Schema
specification under an Apache 2.0 license, separate from the AID implementation.

**Rationale:** The schema describes AI cluster design intent in a vendor-neutral,
human-readable format. It is more expressive than post-design CRDs (which are deployment
artifacts) and more portable than vendor-specific design tools. OCP members should be
able to validate topology plans without running AID. Publishing the schema separately
also enables other tools to consume AID-authored plans.

---

## D11: Oversubscription ratio is a plan warning, not a blocking error

**Decision:** AID computes `oversubscription_ratio` per fabric tier and surfaces it as a
WARNING in the plan report. A ratio > 1.0 does not block generation. An explicit
`allow_oversubscription: true` field on `FabricDomain` suppresses the warning.

**Rationale:** Oversubscription is a valid design trade-off (cost vs. bandwidth). Some
AI workloads tolerate it; others do not. The tool should inform the designer, not
unilaterally block the design. Making it a warning with an explicit override forces a
conscious decision rather than a silent acceptance.

---

## D12: AID is positioned as an independent project, not a migration from HNP

**Decision:** AID is released as a new, standalone tool. Its documentation, user guide,
and public positioning do not reference HNP. HNP was an internal development tool that
was never released. AID's users have no prior relationship with HNP.

**Rationale:** HNP provided design validation, test coverage, and algorithmic heritage
that shaped AID's correctness. This is a development-side relationship. From a user
perspective, AID is the original and authoritative tool. Referencing an unreleased
internal predecessor adds confusion with no benefit to users.

---

## D13: Generic recursive DeviceClass composite — no server-first special casing

**Decision:** AID's hardware model uses a single `DeviceClass` type as the universal
building block. Any hardware component — server, switch, NIC, GPU, PDU, rack unit,
transceiver, cable — is a `DeviceClass`. Sub-components are expressed as
`SubComponent { slot_id, device_class, quantity_per_parent }` entries on a parent
`DeviceClass`. There is no `ServerClass`, `SwitchProfile`, `ServerNIC`, or
`ServerComponent` as distinct top-level types.

**BOM derivation** is a recursive traversal: `DeviceClass.bom(quantity)` walks
sub-components depth-first, multiplying quantities at each level. Per-unit and
fleet totals are both produced without any database access.

**Rationale:** The original model treated "server class" as the root of the BOM
hierarchy, with `ServerComponent`, `ServerNIC`, and `ServerNIC.connections` as
server-specific nested types. This was an antipattern: a switch also has transceivers,
a rack has PDUs, a NIC is simultaneously an independent object and a child of a server.
Forcing servers as the root excluded other hardware categories from proper BOM modeling
and made the model structurally incorrect. A generic recursive composite cleanly handles
all cases: a server is a `DeviceClass` that has NIC sub-components; a NIC is a
`DeviceClass` that may have transceiver sub-components; BOM is just recursion.

**Topology-specific concerns** (quantity, role, connections, port zones) live in
`PlanEntry` and `PlanConnection`, which reference `DeviceClass` instances but are
separate from the hardware model itself.

---

## D14: MoonBit compiled to JavaScript for the frontend GUI

**Decision:** AID's web frontend is implemented in MoonBit compiled to JavaScript.
MoonBit's `moon build --target js` produces a JavaScript bundle. The Go API server
serves the HTML shell, static assets, and REST endpoints. Bootstrap 5 provides the CSS
framework for visual styling.

**Rationale:** MoonBit can compile to both WASM and JavaScript — the same language
covers the calculation kernel (WASM) and the frontend UI (JS). This eliminates the
Python/Django requirement while keeping the team in a single strongly-typed language
for all non-adapter code. The WASM component model boundary means the calculation
kernel could optionally run client-side in a future release (the browser can host
WASM). Bootstrap 5 is the CSS framework used by NetBox, which provides the desired
visual appearance without reimplementing a design system.

**Implementation:** The Go server exposes a REST API. MoonBit-compiled JS calls the
API for all data and renders the UI. No server-side HTML rendering is required — the
Go server is a pure API backend. Bootstrap 5 is loaded from a bundled static asset,
not a CDN, to support air-gapped deployments.

---

## D15: Bootstrap 5 for NetBox-style visual appearance

**Decision:** AID's GUI uses Bootstrap 5 as its CSS framework, configured with the
same color palette and component choices as NetBox (dark nav, card-based layout, table
views with row actions, form-based create/edit flows).

**Rationale:** The design goal is "NetBox-like appearance without Django/Python."
Bootstrap 5 is what NetBox itself uses, so replicating its look is straightforward
with the same framework. Users familiar with NetBox will find AID's UI immediately
recognizable. Bootstrap 5 is mature, well-documented, and requires zero build tooling
for basic use (single CSS + JS bundle).

---

## D16: Kernel WASM boundary uses JSON-over-linear-memory for the MVP; WIT/Component-Model is the end-state

**Decision:** Kernel-side WASM components realize their WIT interface across the wasm
boundary as **UTF-8 JSON over linear memory** for the MVP (Phases 3–6). The
`topology-calculator` component exports `alloc`/`dealloc` plus
`calculate(ptr,len) -> (ptr,len)` and `validate(ptr,len) -> (ptr,len)`, where the payload
is JSON of `topology-plan` in and JSON of `result<calc-output, calc-error>` out. The WIT
interface in `wit/` remains the **contract of record and the type source of truth**; the
full WASM Component Model **canonical ABI is the documented end-state**, to migrate to once
tooling matures (targeted during/after Phase 6).

**Rationale:**
- D8 mandates that every boundary is *expressed as a WIT interface*; it does not mandate a
  specific wire encoding. JSON-over-memory and the canonical ABI are two realizations of the
  same WIT contract.
- The Phase 7 spike (D2 outcome, issue #5) documented that the full Component-Model path was
  never exercised and hit real tooling friction: MoonBit emits core wasm;
  `wasm-tools component embed` rejects the `wasm-gc` target; `wasmtime-go`'s component API is
  newer/unproven. Front-loading canonical-ABI marshalling of deeply nested
  records/lists/variants/options onto the critical-path kernel phase is the wrong risk.
- JSON-over-memory is simple, debuggable, language-agnostic, reversible, and unblocks Phase 6
  (Go hosting) without waiting on Component-Model maturity.

**Consequences:**
- This is an explicit, recorded **deviation from a strict (canonical-ABI) reading of D8**.
  D8 still holds as the boundary-contract principle; D16 records the MVP wire encoding.
- The wasm export signatures (`calculate(ptr,len) -> (ptr,len)`) are **not** the WIT-canonical
  signatures: the component implements a JSON proxy of the WIT interface, not its canonical
  ABI. The WIT remains the logical contract and the type source.
- Boundary compile-time type-safety is traded for simplicity — mitigated by validating input
  JSON against `schema/topology-plan-v1.json` at the ingress/test layer and returning
  `calc-error::invalid-plan` on malformed input. The **pure kernel takes typed input only**;
  wire (de)serialization lives at the boundary edge.
- Field naming: input JSON follows the **user-facing plan schema** (`schema/topology-plan-v1.json`,
  `snake_case`), which is what plan YAML/JSON uses; the WIT (`kebab-case`) defines the logical
  type shapes that `kernel/src/types.mbt` mirrors (see the Phase 3 type-sourcing decision).
- Migration path: when Component-Model tooling matures (post-Phase-6), replace only the thin
  (de)serialization edge with `wit-bindgen`-generated canonical ABI. The WIT contract and the
  kernel's internal typed functions are unchanged.
- Approved as the Phase 3 architecture sign-off (issue #6, kernel architecture note).
- **Scope extended to Layer-2 export adapters (Phase 5 sign-off, issue #9).** The same
  JSON-over-linear-memory convention applies to *all* MVP WASM components the Go CLI hosts,
  not just the kernel: each exports `alloc`/`dealloc` + an entry point taking `(ptr,len)` and
  returning a packed `(ptr,len)`, with WIT-shaped JSON in/out. This gives Phase 6 one uniform
  hosting path. The `hhfab-adapter` (Rust) is the first implementation of the pattern; the
  Phase-6 kernel wasm wrapper follows it. Consequence: the **IR JSON shape** (snake_case,
  mirroring `wit/types.wit`) is the Layer-1→Layer-2 wire contract — the Phase-6 kernel encoder
  must emit the same bytes the adapter consumes, so the IR→JSON encoder should consolidate into
  the kernel in Phase 6 (the Phase-5 `ir-gen` tool is interim test-data tooling, not a second
  contract).

---

## D17: Oversubscription is computed from explicitly-declared leaf UPLINK zones, not inferred

**Decision:** The oversubscription ratio (`ALGORITHMS.md` Algorithm 7) is
`total_server_bandwidth / total_uplink_bandwidth`, where the denominator is the bandwidth
of each leaf switch class's **explicitly declared UPLINK port zone** (`zone_type = uplink`):
`sum over leaf classes of (leaf_count × uplink_zone_logical_ports × uplink_speed)`. AID
never infers which ports are uplinks, and does **not** use the spine's total fabric-port
capacity as the denominator.

**Rationale:**
- **There is no universal rule for which ports a switch uses as uplinks.** It varies
  switch-to-switch and is a human design choice. Example: a Celestica DS5000 has
  64×800G + 2×25G ports, but operators typically designate ~32×800G as uplinks and call
  that 1:1, ignoring the 25G ports — a convention, not a derivable fact. The uplink set
  must therefore come from an explicit per-leaf-class declaration, not a heuristic.
- This yields the **leaf-tier downlink:uplink** ratio, which is the contention metric that
  matters operationally and that AI/ML RDMA collectives are sensitive to (D11).
- It makes Algorithm 7's own `ratio = 1.0 ⇔ non-blocking` semantics exact (server access
  bandwidth == leaf uplink bandwidth), which the literal
  `spine_count × spine_fabric_port_capacity` denominator broke whenever the spine is
  over-provisioned (e.g. clos-small: 1 spine of 32 fabric ports terminating only 16 leaf
  uplinks, because you cannot buy half a spine — literal form gives 0.25, the declared
  leaf-uplink form gives the correct 0.5).

**Consequences:**
- Algorithm 7's denominator wording is amended from `spine_count × spine_fabric_port_capacity`
  to the declared leaf-uplink-zone bandwidth (already available from Algorithm 2's
  leaf-uplink computation). The kernel computes it from leaf entries' UPLINK zones, not
  spine entries.
- **Each leaf switch class is expected to declare an UPLINK port zone** identifying its
  uplink-to-spine ports. A leaf class with no UPLINK zone has no computable oversubscription
  for that fabric → reported as **N/A** (a future validation may warn on a leaf class that
  lacks one).
- **Mesh** fabrics have no spine; the mesh/peer zone is the uplink analog. Mesh
  oversubscription is a documented **follow-up** — for now mesh fabrics report N/A.
- Fixture baselines (Phase 3, issue #6): `clos-small` frontend = **0.5** (3200 / 6400);
  `mesh-two-switch` and `switch-bom` = **N/A** (no spine/uplink tier). These are pinned into
  the fixtures' `expected.json` as the lead-approved baseline addition.
- Decided with the project owner during the Phase 3 GREEN review (issue #6).
