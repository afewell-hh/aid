# AID Domain Model

## Design Principles

- **DeviceClass is the universal hardware building block.** Any hardware component —
  server, switch, NIC, GPU board, memory DIMM, storage drive, transceiver, PDU, rack
  unit — is a `DeviceClass`. There is no server-specific or switch-specific root type.
- **Composition via recursive sub-components.** A `DeviceClass` may contain other
  `DeviceClass` instances as named sub-components with per-parent quantities. BOM
  derivation is a recursive traversal at plan time with no database.
- **Topology is an explicit graph.** The design output is a labeled bipartite graph —
  it is not reconstructed from database tags at export time.
- **Fabric is a first-class aggregate.** A fabric owns its switch plan entries and
  enforces its topology-mode invariants internally.
- **Plan-specific concerns live in PlanEntry, not DeviceClass.** Quantity, role,
  connections, and port zones are plan-level data. `DeviceClass` is a reusable hardware
  template with no plan-specific state.

---

## Core Domain Classes

### TopologyPlan (root aggregate)

The root object. Owns everything in the design.

```
TopologyPlan {
  id: PlanId               // stable identifier
  name: string
  customer_name: string
  status: Draft | Review | Approved | Exported
  entries: PlanEntry[]     // every device in the plan (servers, switches)
  fabric_domains: FabricDomain[]
  device_catalog: DeviceClass[]  // registered hardware templates
  naming_templates: NamingTemplate[]
}
```

---

### DeviceClass (universal hardware template)

Represents any class of hardware component. Reusable across plans and contexts.

```
DeviceClass {
  id: DeviceClassId
  name: string             // e.g. "AS-4126GS GPU Server", "ConnectX-7 OSFP NIC"
  slug: string             // kebab-case unique identifier
  category: string         // user-defined: "compute", "network", "nic", "transceiver", etc.
  manufacturer: string?
  part_number: string?
  description: string?
  attributes: Attribute[]  // arbitrary key-value pairs (vendor-specific specs, etc.)
  ports: PortSpec[]        // network ports on this device (for NICs, switches)
  sub_components: SubComponent[]  // child DeviceClass instances
}

Attribute {
  key: string
  value: string            // always a string; interpret by key convention
}

PortSpec {
  port_id: string          // e.g. "p0", "p1"
  speed_gbps: int
  cage_type: OSFP | QSFP112 | QSFP28 | SFP28 | RJ45
  medium: Optical | DAC | AOC | Copper
}

SubComponent {
  slot_id: string          // unique within parent (e.g. "nic-fe", "gpu-board", "dimm-0")
  device_class: DeviceClass
  quantity_per_parent: int
}
```

**Example — GPU server with nested sub-components:**

```
DeviceClass: AS-4126GS ComputeGPU Server
  sub_components:
    slot_id: "chassis"      → DeviceClass: AS-4126GS Barebone        qty: 1
    slot_id: "nic-fe"       → DeviceClass: ConnectX-7 OSFP NIC       qty: 2
      sub_components:
        slot_id: "xcvr"     → DeviceClass: OSFP 400G Transceiver     qty: 1 (per NIC)
    slot_id: "nic-be"       → DeviceClass: BlueField-3 DPU           qty: 1
    slot_id: "gpu"          → DeviceClass: H100 SXM GPU Board        qty: 8
    slot_id: "dimm"         → DeviceClass: 64GB DDR5 DIMM            qty: 24

DeviceClass: DS5000 Leaf Switch
  sub_components:
    slot_id: "xcvr"         → DeviceClass: OSFP 800G Transceiver     qty: 64
```

**BOM derivation (recursive, plan-time, no database):**

```
DeviceClassBOM {
  device_class: DeviceClass
  plan_quantity: int           // from PlanEntry.quantity
  line_items: BOMLineItem[]    // one per sub_component, depth-first
}

BOMLineItem {
  path: string[]               // e.g. ["nic-fe", "xcvr"]
  device_class: DeviceClass
  quantity_per_parent: int
  quantity_per_unit: int       // product of all qty_per_parent up the tree
  fleet_quantity: int          // quantity_per_unit × plan_quantity
}
```

---

### FabricDomain

A named, independently-wired switching fabric. Enforces topology-mode invariants.

```
FabricDomain {
  fabric_id: FabricId
  fabric_name: string           // e.g. "frontend", "backend", "scale-out"
  fabric_class: Managed | Unmanaged
  topology_mode: Clos | Mesh
  switch_entries: PlanEntry[]   // must be switch-role entries
}

Invariants:
- Mesh fabric: switch count ∈ {2, 3}
- All non-spine switch entries in a Clos fabric share topology_mode
- MCLAG switch count is even and >= 2
- ESLAG switch count is 2–4
```

---

### PlanEntry (plan-specific use of a DeviceClass)

Represents a group of identical devices within the topology plan.
Quantity, role, and wiring intent all live here, not in `DeviceClass`.

```
PlanEntry {
  entry_id: EntryId
  device_class: DeviceClass    // what kind of device
  quantity: int                // how many instances in this plan
  role: PlanRole               // Server | Spine | ServerLeaf | BorderLeaf | OOBLeaf | HHG
  label: string?               // override for naming templates (e.g. "gpu-servers")
  // Switch-specific (only when role ∈ {Spine, ServerLeaf, BorderLeaf, OOBLeaf}):
  fabric_domain: FabricDomain?
  override_quantity: int?      // explicit override; else quantity is calculated
  topology_mode: Clos | Mesh?
  redundancy: MCLAGConfig? | ESLAGConfig?
  port_zones: SwitchPortZone[]
  // Server-specific (when role == Server):
  connections: PlanConnection[]
}

PlanRole = Server | Spine | ServerLeaf | BorderLeaf | OOBLeaf | HHG

effective_quantity = override_quantity ?? calculated_quantity ?? quantity
```

---

### SwitchPortZone

A named allocation region on a switch plan entry.

```
SwitchPortZone {
  zone_id: ZoneId
  zone_name: string
  zone_type: Server | Uplink | Mesh | Peer | Session | OOB
  port_spec: PortRange         // e.g. "1-48" or "1-32:2"
  breakout_option: BreakoutOption?
  priority: int                // lower = allocate first
  allocation_strategy: Sequential | Interleaved | Spaced
  transceiver_intent: DeviceClass?  // a transceiver DeviceClass expected in this zone
  peer_zone: SwitchPortZone?        // target zone for uplink-type zones
}
```

---

### PlanConnection

A named connection from a specific NIC sub-component port to a switch port zone.

```
PlanConnection {
  connection_id: ConnectionId
  nic_slot_id: string          // references SubComponent.slot_id on the device_class
  port_index: int              // zero-based port index on the NIC's PortSpec list
  ports_per_connection: int
  connection_type: Unbundled | Bundled | MCLAG | ESLAG
  distribution: SameSwitch | Alternating | RailOptimized
  target_zone: SwitchPortZone
  speed_gbps: int
  rail: int?                   // for RailOptimized distribution
  port_type: Data | IPMI | PXE
  transceiver_intent: DeviceClass?  // expected transceiver DeviceClass on this port
}
```

The `nic_slot_id` references a `SubComponent.slot_id` on the parent `DeviceClass`. For
example, if the server DeviceClass has `slot_id: "nic-fe"`, a connection with
`nic_slot_id: "nic-fe"` refers to the ConnectX-7 NIC in that slot.

---

## TopologyIR (Output Graph)

The pure output of the calculation kernel. Input to all export adapters.

```
TopologyIR {
  plan_id: PlanId
  nodes: TopologyNode[]
  edges: TopologyEdge[]
  fabrics: FabricSummary[]
  boms: DeviceClassBOM[]       // one per PlanEntry
  validation: ValidationResult
}

TopologyNode {
  name: string
  node_type: Server | Switch | Spine
  device_class_id: DeviceClassId
  fabric: string?
  hedgehog_role: string?
  instance_index: int
}

TopologyEdge {
  edge_id: EdgeId
  node_a: TopologyNode
  node_b: TopologyNode
  speed_gbps: int
  fabric: string
  zone: string
  breakout_index: int?
  connection_type: string
  port_a: string               // physical port name on node_a
  port_b: string               // physical port name on node_b
}

FabricSummary {
  fabric_name: string
  switch_count: int
  total_server_bandwidth_gbps: int
  total_spine_bandwidth_gbps: int
  oversubscription_ratio: float    // > 1.0 triggers WARNING
}

ValidationResult {
  is_valid: bool
  errors: ValidationIssue[]    // block generation
  warnings: ValidationIssue[]  // surface in report, do not block
}
```

---

## Ownership Hierarchy Summary

```
TopologyPlan
  ├── DeviceClass catalog (0..N)       ← hardware templates; reused across entries
  │     └── SubComponent (0..N)        ← nested DeviceClass with qty_per_parent
  │           └── SubComponent ...     ← recursive; arbitrarily deep
  ├── FabricDomain (1..N)
  │     └── (references PlanEntry)
  └── PlanEntry (1..N)                 ← plan-specific: qty, role, connections
        ├── device_class → DeviceClass  (reference, not ownership)
        ├── SwitchPortZone (0..N)      ← switch entries only
        └── PlanConnection (0..N)      ← server entries only
              └── nic_slot_id → SubComponent.slot_id (reference)
```

The key distinction: `DeviceClass` is a reusable hardware template (owned by the catalog).
`PlanEntry` is a plan-specific instantiation (owned by `TopologyPlan`). BOM derivation
walks `DeviceClass.sub_components` recursively, scaled by `PlanEntry.quantity`.
