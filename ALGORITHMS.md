# AID Core Topology Algorithms

These algorithms are the quantitative core of AID. They are vendor-neutral and intended
to be published as an OCP community resource.

All variables are defined per-algorithm. Units: ports are logical ports after breakout.

---

## Algorithm 1: Leaf Switch Quantity

**Problem:** Given N servers each with M connection ports at speed S targeting a switch
zone, how many leaf switches are required?

**Formula:**
```
zone_logical_capacity = zone_physical_ports × breakout_factor(S)
breakout_factor(S)    = breakout.logical_ports  (if zone has a breakout option for S)
                      = 1                        (if native speed, no breakout)

leaf_count = ceil(
    (server_count × ports_per_connection)
    / (zone_logical_capacity - uplink_reservation)
)
```

**Variables:**
- `server_count`: number of servers in the class
- `ports_per_connection`: physical ports per server connection (e.g., 1 for unbundled, 2 for LAG)
- `zone_physical_ports`: count of physical ports in the switch zone (parsed from port_spec)
- `uplink_reservation`: logical ports reserved for spine uplinks in this zone (0 for pure server zones)

**Constraints:**
- `leaf_count >= 2` when distribution is alternating (minimum for fault domain separation)
- `leaf_count` must be even when MCLAG redundancy is required (round up to nearest even)
- `leaf_count` is clamped to [2, 4] when ESLAG redundancy is required

**Multiple connection types targeting the same zone:**
When multiple groups of servers target the same zone at different speeds, compute
`leaf_count` for each (zone, speed) pair independently and take the maximum:
```
leaf_count = max over all (zone, speed) pairs of:
    ceil((demand_at_speed) / (zone_logical_capacity_at_speed))
```

---

## Algorithm 2: Spine Switch Quantity

**Problem:** Given L leaf switches each contributing U uplinks, and a spine zone of
capacity C, how many spine switches are needed?

**Formula:**
```
total_leaf_uplink_demand = sum over all leaf classes of (effective_quantity × uplinks_per_leaf)
spine_count = ceil(total_leaf_uplink_demand / spine_fabric_port_capacity)
```

**Variables:**
- `effective_quantity`: resolved leaf count (override or calculated)
- `uplinks_per_leaf`: logical uplink ports per leaf switch (from UPLINK zone capacity)
- `spine_fabric_port_capacity`: logical ports in the spine's FABRIC zone

---

## Algorithm 3: Rail-Optimized Switch Distribution

Rail-optimized distribution assigns each server's GPU NIC ports to a specific leaf switch
based on the server's GPU rail index. Used for backend fabrics in AI training clusters.

**Two sub-cases based on switch count vs rail count:**

**Sub-case A — Capacity-sharing (effective_quantity < total_rails):**
Multiple rails share a single switch pair. Used when there are fewer switch pairs than rails.
```
switch_index = floor((server_index × ports_per_connection + port_index) / ports_per_switch_zone)
```

**Sub-case B — Domain-based (effective_quantity >= total_rails):**
Each rail maps to a dedicated domain of switches.
```
servers_per_domain = floor(effective_quantity / total_rails)
switch_index = (rail × servers_per_domain) + floor(server_index / servers_per_domain)
```

**Variables:**
- `server_index`: 0-based index of the server instance within its plan entry
- `port_index`: 0-based port index on the NIC
- `total_rails`: total number of GPU rails in the plan (from server connection `rail` fields)
- `ports_per_switch_zone`: logical ports per zone per switch instance

**Constraint:** A zone cannot mix rail-optimized and alternating connections targeting the
same switch class. This must be validated at plan-edit time.

---

## Algorithm 4: Alternating Distribution

Distributes connections round-robin across switch instances. Used for frontend fabrics.

```
switch_index = port_index % effective_quantity
```

For single-port connections (`port_index` is always 0), this always selects switch 0.
Use `same-switch` distribution for single-port connections unless explicit alternation
is needed and the connection target zone has a redundancy pair (MCLAG/ESLAG).

---

## Algorithm 5: Mesh Inter-Switch Link Count

**2-switch mesh:**
```
cables_per_pair = total_logical_mesh_ports_per_switch
```
All mesh-zone ports are used for the single inter-switch link bundle.

**3-switch mesh (full mesh, 3 pairs):**
```
cables_per_pair = floor(total_logical_mesh_ports_per_switch / 2)
```
Must be even — validated as a hard constraint. Each of the 3 pairs gets equal link budget.

**Constraint:** Mesh switch count must be exactly 2 or 3. This is not an arbitrary limit:
- 2-switch: all mesh ports go to one pair → integer allocation always works
- 3-switch: mesh ports split evenly across 2 pairs per switch (3 pairs total)
- 4+ switches: ports per pair drops below 1 physical cable → non-integer, infeasible

---

## Algorithm 6: BOM Scaling (Recursive DeviceClass Traversal)

`DeviceClass` is the atomic BOM unit. Any hardware component — server, switch, NIC,
transceiver, GPU board, PDU — is a `DeviceClass` that may contain other `DeviceClass`
instances as `SubComponent { slot_id, device_class, quantity_per_parent }` entries. BOM
derivation is a depth-first traversal of the sub-component tree at plan time, with no
database access (see `DECISIONS.md` D6 and D13). There is no server-specific root type.

For each node in the tree, quantities multiply down the path from the root device class:

```
quantity_per_unit(node) = product of quantity_per_parent for every edge from the root to node
fleet_quantity(node)    = quantity_per_unit(node) × plan_entry.quantity
```

`quantity_per_unit` of the root device class is 1. For every BOM line item to be valid,
each `quantity_per_parent` must be a positive integer. Fractional sub-component counts
(e.g. "0.5 CPUs per parent") are a modeling error.

**Root inclusion is role-dependent.** Whether the root `DeviceClass` itself appears as a
BOM line item depends on the owning `PlanEntry.role`:
- **Server-role entries omit the root.** An assembled server is not itself a procurement
  line item — you buy its chassis, GPUs, NICs, and transceivers, not the assembly. The BOM
  lists the recursive sub-components only. (This is why the GPU-server example below lists
  the chassis/GPU/NIC/transceiver but not "ComputeGPU Server".)
- **Switch-role entries include the root.** A switch is itself a purchasable SKU, so the
  switch `DeviceClass` appears as a line item (`quantity_per_unit = 1`) alongside its
  sub-components (e.g. transceivers).

This rule is fixed by the behavioral contract in `tests/fixtures/` (e.g. `clos-small`
omits the `gpu-server` root but includes `leaf-switch`/`spine-switch`; `switch-bom`
includes the `leaf-switch-800g` root). Decided as the Phase 3 kernel architecture sign-off
(issue #6).

**Example — GPU server device class, plan quantity = 16:**
```
DeviceClass: ComputeGPU Server        plan_quantity = 16
  sub_components (quantity_per_parent):
    chassis  → AS-4126GS Barebone     1
    gpu      → H100 SXM GPU Board      8
    cpu      → 2-socket CPU            2
    dimm     → 64GB DDR5 DIMM         24
    nvme     → NVMe drive              2
    nic-fe   → ConnectX-7 NIC          8
      sub_components:
        xcvr → OSFP 400G Transceiver   1   (per NIC)
    nic-be   → BlueField-3 DPU         1

Per-unit (quantity_per_unit) and fleet (× 16) totals:
    Barebone chassis:        1   →    16
    H100 SXM GPU Board:      8   →   128
    CPU:                     2   →    32
    64GB DDR5 DIMM:         24   →   384
    NVMe drive:              2   →    32
    ConnectX-7 NIC:          8   →   128
    OSFP 400G Transceiver:   8   →   128   (8 NICs × 1 transceiver each, recursed)
    BlueField-3 DPU:         1   →    16
```

A switch `DeviceClass` with transceiver sub-components derives its BOM by the same
recursion — there is no server-specific special case. See `DOMAIN_MODEL.md`
(`DeviceClassBOM`, `BOMLineItem`) for the output structure.

---

## Algorithm 7: Oversubscription Ratio

**Formula (per fabric tier):**
```
total_server_bandwidth_gbps = sum over all server connections targeting this fabric of:
    (server_count × ports_per_connection × speed_gbps)

total_spine_bandwidth_gbps = spine_count × spine_fabric_port_capacity_gbps

oversubscription_ratio = total_server_bandwidth_gbps / total_spine_bandwidth_gbps
```

**Interpretation:**
- `ratio = 1.0`: non-blocking (every server port has a guaranteed path to any other)
- `ratio > 1.0`: oversubscribed; multiple servers competing for the same spine bandwidth
- `ratio < 1.0`: over-provisioned (unusual but valid)

AI/ML training workloads using RDMA collective operations (all-reduce, all-to-all) are
highly sensitive to oversubscription. A ratio > 1.0 should always be surfaced as a
plan warning.

---

## Algorithm 8: Port Allocation (Zone Cursor)

AID uses a zone-aware sequential allocator. For each (switch_instance, zone) pair:

1. Build the ordered logical port sequence by parsing port_spec into physical ports,
   then expanding each physical port into logical breakout slots.
2. Apply allocation strategy (Sequential, Interleaved, or Spaced) to determine ordering.
3. Maintain a cursor per (switch_instance, zone). Allocate by advancing the cursor.
4. Pre-flight check: before any allocation, verify that total_demanded <= total_capacity
   for each (switch_class, zone) pair across all connections. Fail fast with a per-zone
   deficit report before any topology is built.

**Formally verified invariants:**
- Cursor advances monotonically — no port is revisited
- Total allocated ports per zone ≤ zone capacity
- Total allocated across all zones == total demanded (completeness)
