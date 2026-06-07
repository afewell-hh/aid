# AID — AI Infrastructure Designer

AID is a standalone tool for designing AI/ML cluster network topologies.

Given a description of server hardware (counts, NIC types, fabric intent), AID calculates
the switch infrastructure required, validates topology constraints, derives a full bill of
materials, and exports wiring artifacts that can be applied directly to supported network
fabrics or published to an inventory management system.

## What AID Does

- **Topology calculation**: derive switch counts, port allocations, and fabric wiring from
  server inventory inputs — no manual switch sizing required
- **BOM derivation**: produce a complete, per-server-class bill of materials including
  chassis, CPUs, memory, drives, NICs, and transceivers, scalable to fleet quantities
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
aid CLI (Go)
  ├── plan YAML → topology-calculator.wasm (MoonBit)  ← formal verification
  │                       ↓ TopologyIR
  ├── hhfab-adapter.wasm (Rust)   → wiring YAML per fabric
  ├── bom-adapter.wasm (MoonBit or Rust) → BOM CSV/JSON
  └── netbox-adapter (Rust/Go)    → NetBox REST API [optional]
```

Components communicate through WIT-defined interfaces using the WASM Component Model.
The topology calculation kernel is implemented in MoonBit with formally verified invariants.

## Technology Stack

| Layer | Language | Reason |
|-------|----------|--------|
| Topology calculation kernel | MoonBit | Formal verification, WASM-native, agentic-first |
| Wiring YAML adapter | Rust | serde_yaml, cargo-component, mature WASM ecosystem |
| NetBox REST adapter | Rust or Go | reqwest / net/http, no ORM required |
| CLI, plan storage, orchestration | Go | cobra/viper, wasmtime-go, fast binary distribution |

## Supported Topology Families

- **Clos rail-optimized (RO)**: GPU rails wired to dedicated leaf pairs, non-blocking backend
- **Clos single-homed (SH)**: servers single-homed to leaf switches
- **Dual-plane**: two independent Clos planes for redundancy
- **Mesh-converged**: 2- or 3-switch mesh fabric for smaller building blocks
- **XOC compositions**: multiple OPG building blocks composed into larger cluster units

## Key Inputs

A topology plan YAML describes:
- Server classes (quantity, category, GPU count)
- Per-server NIC inventory (NIC id, module type, port count, speed)
- Per-NIC connection intent (target fabric, distribution mode, breakout)
- Switch classes (fabric, role, topology mode, port zones)

AID derives everything else: switch counts, port assignments, wiring, BOM.

## Status

Pre-release. See `ROADMAP.md` for the implementation phases.
