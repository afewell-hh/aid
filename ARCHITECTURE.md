# AID Architecture

## Design Principles

1. **Calculation is independent of persistence.** The topology kernel must produce a correct
   result given only plan data as input — no database, no running service, no file I/O.

2. **NetBox is an optional adapter, not a dependency.** AID can be used entirely offline.
   Publishing to NetBox is a one-way push via REST API, not a coupling point.

3. **System-of-systems via WASM Component Model.** Every major capability boundary is
   expressed as a WIT interface. Components are language-independent and composable.

4. **Formal verification for hard invariants.** The topology kernel carries machine-checked
   proofs for its correctness properties. If the proof fails, the build fails.

5. **Server class is the atomic BOM unit.** All hardware quantities are derived from
   per-server-class specifications, not from post-generation inventory reads.

---

## Four-Layer Architecture

### Layer 1 — Topology Calculation Kernel (MoonBit WASM component)

**Responsibility:** Given a `TopologyPlan` input, produce a `TopologyIR` output.

Inputs (via WIT):
- `TopologyPlan` — server classes, NICs, connections, switch classes, port zones

Outputs (via WIT):
- `TopologyIR` — the complete topology as a typed graph
- `ServerClassBOM[]` — bill of materials per server class
- `ValidationResult` — constraint violations, warnings (oversubscription ratio per fabric)

Contains:
- Switch quantity calculation (leaf and spine counts)
- Port allocation (zone-aware, priority-ordered)
- Clos wiring distribution (alternating, rail-optimized, same-switch)
- Mesh pair enumeration and inter-switch link assignment
- Breakout option selection
- BOM derivation (per-server-class, scalable to fleet quantities)
- Constraint validation (topology mode rules, MCLAG/ESLAG counts, oversubscription)

Formally verified properties (`moon prove`):
- Port non-overlap: no logical port is allocated to more than one connection
- Allocation completeness: total allocated ports == total demanded ports
- Switch count lower bound: effective_quantity >= ceil(demand / capacity) for each zone
- BOM scaling: fleet_count(component) == per_server_count × server_class.quantity
- Mesh constraint: mesh switch count ∈ {2, 3}
- MCLAG even-count: MCLAG switch count is even and >= 2

Zero imports from NetBox, Django, filesystem, or HTTP.

### Layer 2 — Export Adapters (WASM components, Rust or MoonBit)

**Responsibility:** Transform `TopologyIR` into output artifacts.

#### hhfab-adapter (Rust WASM component)
- Input: `TopologyIR` + export options (fabric scope, split-by-fabric)
- Output: hhfab wiring YAML (Kubernetes CRD format, `wiring.githedgehog.com/v1beta1`)
- CRD types: VLANNamespace, IPv4Namespace, SwitchGroup, Switch, Server, Connection
- Validates output structure before returning

#### bom-adapter (MoonBit or Rust WASM component)
- Input: `ServerClassBOM[]` + plan metadata
- Output: BOM CSV and/or JSON
- Format: per-server-class sections with per-unit and fleet-total quantities
- No NetBox reads — BOM is derived entirely from the plan model

### Layer 3 — I/O Adapters (Rust or Go)

**Responsibility:** Side-effecting integrations with external systems.

#### netbox-adapter (Rust or Go)
- Input: `TopologyIR`
- Action: POST Devices, Interfaces, Cables, Modules to NetBox via REST API
- Idempotent: uses `name` as idempotency key per object type
- Does not use Django ORM — NetBox REST API only
- This layer is optional. AID functions fully without it.

#### plan-storage (Go)
- Reads and writes topology plan YAML files
- Persists generated state (TopologyIR hash, last generation timestamp, BOM cache)
  in SQLite (local, single-user) or a configurable backend
- No schema migrations required for plan YAML — it is a versioned document format

### Layer 4 — CLI and Orchestration (Go)

**Responsibility:** User-facing command surface and component orchestration.

```
aid plan create   --output plan.yaml
aid plan validate plan.yaml
aid topology calc plan.yaml --output topology.json
aid topology bom  plan.yaml --format csv --output bom.csv
aid export wiring plan.yaml --fabric backend --output wiring-backend.yaml
aid publish netbox plan.yaml --netbox-url https://... --token ...
```

- Hosts WASM components via `wasmtime-go`
- Reads plan YAML, passes to topology-calculator.wasm
- Routes TopologyIR to appropriate adapters based on subcommand
- Returns human-readable output for terminal use

---

## WASM Component Model Boundaries

```
┌────────────────────────────────────────────────────────────────┐
│ aid CLI (Go)                                                    │
│                                                                 │
│  plan.yaml ──► [topology-calculator.wasm] ──► TopologyIR       │
│                        │                           │           │
│                        ▼                           ▼           │
│               ValidationResult         [hhfab-adapter.wasm]    │
│                        │               [bom-adapter.wasm]      │
│                        │               [netbox-adapter]        │
│                        ▼                           ▼           │
│               (stdout / exit code)        (files / REST API)   │
└────────────────────────────────────────────────────────────────┘
```

All component boundaries are defined by WIT interfaces in `wit/`. The CLI is the sole
orchestrator — components do not call each other directly.

---

## TopologyIR — Intermediate Representation

The `TopologyIR` is the pure output of the calculation kernel and the shared input
to all export adapters. It is a typed, labeled graph:

```
TopologyIR {
  nodes: Node[]       // Switch, Server, Spine, SpineGroup
  edges: Edge[]       // PlannedCable with speed, zone, fabric, breakout, conn_type
  fabrics: FabricSummary[]  // per-fabric: switch counts, oversubscription ratio
  metadata: PlanMetadata
}
```

The `TopologyIR` is serializable to JSON and is the stable handoff point between
all AID components. An `aid topology calc` command can write it to disk for inspection
or pipe it to any adapter.

---

## Plan Persistence Strategy

- **Source of truth:** topology plan YAML files (human-authored, version-controlled)
- **Generated state:** SQLite database (`~/.aid/state.db` or project-local `.aid/state.db`)
  - last topology IR hash per plan file
  - last generation timestamp
  - cached BOM outputs
- **No server required for local use.** SQLite is embedded.
- Multi-user / team use: swap the storage adapter for a shared backend (future).

---

## NetBox Integration Model

AID treats NetBox as a publish target, not a dependency.

The netbox-adapter maps `TopologyIR` to NetBox REST API objects:

| TopologyIR | NetBox object | API endpoint |
|-----------|---------------|-------------|
| Node (Switch) | dcim.Device | POST /api/dcim/devices/ |
| Node (Server) | dcim.Device | POST /api/dcim/devices/ |
| Edge (cable) | dcim.Cable | POST /api/dcim/cables/ |
| ServerNIC | dcim.Module | POST /api/dcim/modules/ |

Custom fields (`aid_plan_id`, `aid_fabric`, `aid_zone`) are stamped on generated objects
for plan-scoped cleanup and re-export. The adapter creates these fields on first use.

Cleanup: `aid publish netbox --clean plan.yaml` deletes all objects tagged with `aid_plan_id`.
