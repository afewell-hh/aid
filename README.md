# AID — AI Infrastructure Designer

AID is a standalone tool for designing AI/ML cluster network topologies.

Given a description of server hardware (counts, NIC types, fabric intent), AID calculates
the switch infrastructure required, validates topology constraints, derives a full bill of
materials, and exports wiring artifacts that can be applied directly to supported network
fabrics or published to an inventory management system.

## What AID Does

- **Topology calculation**: derive switch counts, port allocations, and fabric wiring from
  server inventory inputs — no manual switch sizing required
- **BOM derivation**: produce a complete, hierarchical bill of materials for any device
  class in the plan — servers, switches, NICs, transceivers, or any nested sub-component
  — scalable to fleet quantities while preserving per-unit breakdown at every level
- **Wiring export**: generate per-fabric wiring YAML validated against the hhfab CLI
- **Topology validation**: enforce mesh, Clos, and dual-plane constraints; report
  oversubscription ratios per fabric tier
- **Inventory publish**: optionally populate a NetBox instance with the generated topology
  (Device, Interface, Cable objects) via the NetBox REST API

## What AID Does Not Do

- AID does not manage or operate a live network fabric
- AID does not require a running NetBox instance to calculate topology
- AID is not a general datacenter design tool — it is scoped to AI/ML cluster topologies
  using the Hedgehog Open Network Fabric wiring model

## Architecture Overview

```
Browser / Desktop
  └── aid-ui (MoonBit → JS + Bootstrap 5)  ← NetBox-style GUI
           ↕ REST API
aid CLI (Go) / aid-server (Go)
  ├── plan YAML → topology-calculator.wasm (MoonBit)  ← formal verification
  │                       ↓ TopologyIR
  ├── hhfab-adapter.wasm (Rust)   → wiring YAML per fabric
  ├── bom-adapter.wasm (MoonBit or Rust) → BOM CSV/JSON
  └── netbox-adapter (Rust/Go)    → NetBox REST API [optional]
```

Components communicate through WIT-defined interfaces using the WASM Component Model.
The topology calculation kernel is implemented in MoonBit with formally verified invariants.

The six kernel invariants (port non-overlap, allocation completeness, switch-count
lower bound, BOM scaling, mesh switch count ∈ {2,3}, MCLAG even-count) are
machine-proved by `moon prove` at their **pure-arithmetic cores** (the
`aid/kernel/proofs` package, which the kernel routes its real computation
through); the surrounding `Array`/whole-plan wiring is covered by fixture tests.
A CI proof gate blocks the build on any unproved core. See `ARCHITECTURE.md`
Layer 1 ("Verification scope") and Issue #7 for the per-invariant detail.

## Technology Stack

| Layer | Language | Reason |
|-------|----------|--------|
| Topology calculation kernel | MoonBit | Formal verification, WASM-native, agentic-first |
| Frontend UI | MoonBit → JavaScript + Bootstrap 5 | NetBox-style appearance, no Python/Django |
| Wiring YAML adapter | Rust | serde_yaml, cargo-component, mature WASM ecosystem |
| NetBox REST adapter | Rust or Go | reqwest / net/http, no ORM required |
| CLI, API server, orchestration | Go | cobra/viper, wasmtime-go, fast binary distribution |

## Supported Topology Families

- **Clos rail-optimized (RO)**: GPU rails wired to dedicated leaf pairs, non-blocking backend
- **Clos single-homed (SH)**: servers single-homed to leaf switches
- **Dual-plane**: two independent Clos planes for redundancy
- **Mesh-converged**: 2- or 3-switch mesh fabric for smaller building blocks
- **XOC compositions**: multiple OPG building blocks composed into larger cluster units

## Key Inputs

A topology plan YAML describes:
- Device classes (any hardware: servers, switches, NICs, transceivers — each with arbitrary
  attributes and nested sub-components at defined per-parent quantities)
- Per-device connection intent (target fabric, distribution mode, breakout)
- Fabric domains (fabric name, topology mode, switch roles, port zones)

AID derives everything else: switch counts, port assignments, wiring, hierarchical BOM.

## Status

Pre-release. See `ROADMAP.md` for the implementation phases.
