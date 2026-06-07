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

## D6: Server class is the atomic BOM unit

**Decision:** The BOM is derived from the plan model at plan time, not from generated
inventory after a database write. `ServerClass.bom()` returns the per-server BOM with
no database queries. Fleet totals are `per_server_count × server_class.quantity`.

**Rationale:** A pre-sales topology design tool must be able to produce a procurement
BOM before any hardware is ordered or any database is populated. The predecessor tool's
inventory-based BOM (reading from generated NetBox Module records) could not produce a
BOM until after full generation into a running NetBox. That was a fundamental mismatch
with the tool's use case.

---

## D7: ServerConnection is owned by ServerNIC, not ServerClass

**Decision:** `ServerConnection` records have `ServerNIC` as their owning parent
(cascade-owned). `ServerClass` owns `ServerNIC` instances, which in turn own their
connections. The connection ownership hierarchy matches physical reality.

**Rationale:** A cable plugs into a port on a NIC card, not directly into a "server class."
The correct aggregate boundary is: `ServerClass → ServerNIC → ServerConnection`. This
makes `ServerNIC.connections` a natural query and enables `ServerNIC.bom()` to return
the transceiver BOM for its own ports. The previous model had connections owned by
ServerClass and only referencing the NIC, which blurred ownership and made BOM
derivation awkward.

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
