# AID Domain Model

## Design Principles

- **Server class is the atomic BOM unit.** A fleet of N identical servers has exactly
  N times the per-server BOM. The per-server BOM is always derivable without any
  database queries.
- **NIC is a first-class owned component.** A NIC card lives on a server class (owned,
  composition). A NIC port makes a connection to a switch zone (association).
- **Topology is an explicit graph.** The design output is a labeled bipartite graph —
  it is not reconstructed from database tags at export time.
- **Fabric is a first-class aggregate.** A fabric owns its switch classes and enforces
  its topology-mode invariants internally.

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
  server_classes: ServerClass[]
  fabric_domains: FabricDomain[]
  naming_templates: NamingTemplate[]
}
```

### FabricDomain

A named, independently-wired switching fabric. Enforces its own topology-mode invariants.

```
FabricDomain {
  fabric_id: FabricId
  fabric_name: string           // e.g. "frontend", "backend", "scale-out"
  fabric_class: Managed | Unmanaged
  topology_mode: Clos | Mesh
  switch_classes: SwitchClass[]
}

Invariants:
- Mesh fabric: switch count ∈ {2, 3}
- All non-spine classes in a Clos fabric share topology_mode
- MCLAG switch count is even and >= 2
- ESLAG switch count is 2–4
```

### SwitchClass

A group of identical switches within a fabric.

```
SwitchClass {
  switch_class_id: SwitchClassId
  fabric_domain: FabricDomain   // owned by
  hedgehog_role: Spine | ServerLeaf | BorderLeaf
  switch_profile: SwitchProfile  // replaces NetBox DeviceType + DeviceTypeExtension
  override_quantity: int?
  calculated_quantity: int?       // written by kernel
  topology_mode: Clos | Mesh
  redundancy: MCLAGConfig? | ESLAGConfig?
  port_zones: SwitchPortZone[]
}

effective_quantity = override_quantity ?? calculated_quantity ?? 0
```

### SwitchProfile (replaces DeviceType + DeviceTypeExtension)

The switch hardware specification. Not a live database object — a plain data record.

```
SwitchProfile {
  profile_id: ProfileId
  slug: string              // e.g. "celestica-ds5000"
  model: string
  manufacturer: string
  native_port_speed_gbps: int
  total_ports: int
  supported_breakouts: BreakoutOption[]
  mclag_capable: bool
  hedgehog_profile_name: string
  hhfab_role_tags: string[]
}
```

### SwitchPortZone

A named allocation region on a switch class.

```
SwitchPortZone {
  zone_id: ZoneId
  zone_name: string
  zone_type: Server | Uplink | Mesh | Peer | Session | OOB
  port_spec: PortRange         // e.g. "1-48" or "1-32:2"
  breakout_option: BreakoutOption?
  priority: int                // lower = allocate first
  allocation_strategy: Sequential | Interleaved | Spaced
  transceiver_intent: TransceiverSpec?
}
```

### ServerClass (aggregate root for server hardware)

Owns the complete hardware specification for one server type.

```
ServerClass {
  server_class_id: ServerClassId
  server_class_name: string
  category: GPU | Storage | Infrastructure
  quantity: int               // PRIMARY INPUT — all switch math derives from this
  components: ServerComponent[]  // barebone, GPU board, CPU, memory, drive, accessory
  nics: ServerNIC[]           // NIC cards installed in this server class
}

bom() → ServerClassBOM       // derived, no DB required
fleet_bom() → ServerClassBOM // bom() scaled by quantity
```

### ServerComponent

A hardware component in the server BOM. Not a NIC (NICs are separate).

```
ServerComponent {
  component_type: Barebone | GPUBoard | CPU | MemoryDIMM | StorageDrive | Accessory
  quantity_per_server: int
  part_number: string?
  manufacturer: string?
  model_description: string
}
```

### ServerNIC (owns its connections)

A NIC card installed in a server class. Owns the port connections that use it.

```
ServerNIC {
  nic_id: NICId              // e.g. "nic-fe", "nic-be0"
  server_class: ServerClass  // owned by
  module_spec: ModuleSpec    // port count, speed, cage type, etc.
  connections: ServerConnection[]  // owned by this NIC
}
```

### ServerConnection

A named connection from a NIC port to a switch port zone.

```
ServerConnection {
  connection_id: ConnectionId
  nic: ServerNIC              // owned by
  port_index: int             // zero-based port index on NIC
  ports_per_connection: int
  connection_type: Unbundled | Bundled | MCLAG | ESLAG
  distribution: SameSwitch | Alternating | RailOptimized
  target_zone: SwitchPortZone
  speed_gbps: int
  rail: int?                  // for rail-optimized distribution
  port_type: Data | IPMI | PXE
  transceiver_intent: TransceiverSpec?
}
```

### ModuleSpec (replaces dcim.ModuleType)

Hardware specification for a NIC or transceiver. Not a live database object.

```
ModuleSpec {
  spec_id: SpecId
  model: string
  manufacturer: string
  port_count: int
  port_speed_gbps: int
  cage_type: OSFP | QSFP112 | QSFP28 | SFP28 | RJ45
  medium: Optical | DAC | AOC | Copper
  reach_class: string?
  attribute_data: map[string, any]  // vendor-specific transceiver attributes
}
```

### ServerClassBOM

Derived from a ServerClass. Derivable at plan time with no database.

```
ServerClassBOM {
  server_class_id: ServerClassId
  per_server: BOMLineItem[]
  fleet_quantity: int          // server_class.quantity
  fleet_total: BOMLineItem[]   // per_server scaled by fleet_quantity
}

BOMLineItem {
  section: Barebone | GPUBoard | CPU | Memory | Drive | NIC | Transceiver | Accessory
  part_number: string?
  description: string
  quantity_per_unit: int
}
```

---

## TopologyIR (Output Graph)

The pure output of the calculation kernel. Input to all export adapters.

```
TopologyIR {
  plan_id: PlanId
  nodes: TopologyNode[]
  edges: TopologyEdge[]
  fabrics: FabricSummary[]
  boms: ServerClassBOM[]
  validation: ValidationResult
}

TopologyNode {
  name: string
  node_type: Server | Switch | Spine
  device_class_id: string   // ServerClass.server_class_id or SwitchClass.switch_class_id
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
  port_a: string           // physical port name on node_a
  port_b: string           // physical port name on node_b
}

FabricSummary {
  fabric_name: string
  switch_count: int
  total_server_bandwidth_gbps: int
  total_spine_bandwidth_gbps: int
  oversubscription_ratio: float   // > 1.0 triggers WARNING
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
  ├── FabricDomain (1..N)
  │     └── SwitchClass (1..N)
  │           └── SwitchPortZone (1..N)
  └── ServerClass (1..N)
        ├── ServerComponent (0..N)   ← new: barebone, GPU, CPU, DIMM, drive
        └── ServerNIC (1..N)
              └── ServerConnection (1..N) → SwitchPortZone
```

The key structural difference from earlier designs: `ServerConnection` is owned by
`ServerNIC`, not by `ServerClass`. This matches physical reality (a cable plugs into
a NIC port) and makes `ServerNIC.bom()` and `ServerNIC.connections` natural queries.
