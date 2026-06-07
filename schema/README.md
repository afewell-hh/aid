# AID Topology Plan Schema

This directory holds the canonical, versioned JSON Schema for the **AID topology
plan** — the user-authored input that the AI Infrastructure Designer consumes to
calculate switch counts, derive bills of materials, validate topology
constraints, and export wiring artifacts.

| File | Contents |
|------|----------|
| `topology-plan-v1.json` | JSON Schema (Draft 2020-12) for topology plan files |
| `README.md` | this field reference and validation guide |

A topology plan is normally authored as **YAML** and validated as JSON against
this schema. The plan is the source-of-truth design document: version it, review
it in pull requests, and share it between teams. It does not require a GUI, a
database, or a running service to create or edit.

## Versioning

The schema is published as a versioned artifact (`topology-plan-v1.json`). A new
major version is created for breaking changes; additive, backward-compatible
changes are made in place within a major version. The schema is intended to be
publishable as a standalone community artifact under Apache 2.0.

## Validating a plan

Any JSON Schema Draft 2020-12 validator works. With Node.js and `ajv-cli`:

```bash
# Validate one or more plan YAML files against the schema
npx --yes ajv-cli@5.0.0 validate --spec=draft2020 \
  -s schema/topology-plan-v1.json \
  -d 'path/to/plan.yaml'
```

To check that the schema document itself is syntactically valid JSON:

```bash
jq empty schema/topology-plan-v1.json
```

## Document structure

A plan is a single object with these top-level fields:

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `id` | string (kebab-case) | yes | Stable plan identifier |
| `name` | string | yes | Human-readable plan name |
| `customer_name` | string | yes | Customer / project the plan is for |
| `status` | enum | yes | `draft` \| `review` \| `approved` \| `exported` |
| `device_catalog` | array | yes | Reusable hardware templates (DeviceClass) |
| `fabric_domains` | array | yes | Independently-wired switching fabrics |
| `entries` | array | yes | Device groups with quantity, role, wiring intent |

### Hardware model: `device_catalog[]`

Any hardware component — server, switch, NIC, transceiver, GPU board, PDU — is a
single universal **DeviceClass**. There is no server-specific or switch-specific
root type. Composition is recursive: a DeviceClass lists `sub_components`, each
referencing another catalog entry by `device_class_id` with a
`quantity_per_parent`. Bills of materials are derived by walking this tree at
plan time, multiplying quantities down each path — no database required.

The catalog is **normalized**: every DeviceClass referenced anywhere in the plan
must appear in `device_catalog` exactly once, and is referenced elsewhere by id.

Each DeviceClass field:

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `id` | string (kebab-case) | yes | Unique catalog id |
| `name` | string | yes | Human-readable name |
| `slug` | string (kebab-case) | yes | Unique slug |
| `category` | string | yes | Free-form, e.g. `compute`, `network`, `nic`, `transceiver` |
| `manufacturer` | string | no | Vendor name |
| `part_number` | string | no | Vendor part number |
| `description` | string | no | Free text |
| `attributes[]` | `{key, value}` | no | Arbitrary vendor-specific specs |
| `ports[]` | port spec | no | Network ports (`port_id`, `speed_gbps`, `cage_type`, `medium`) |
| `sub_components[]` | sub-component | no | Child DeviceClasses (`slot_id`, `device_class_id`, `quantity_per_parent`) |

`cage_type` ∈ `osfp` \| `qsfp112` \| `qsfp28` \| `sfp28` \| `rj45`.
`medium` ∈ `optical` \| `dac` \| `aoc` \| `copper`.

### Fabrics: `fabric_domains[]`

A fabric is a named, independently-wired switching domain. It references its
switch-role plan entries by id.

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `fabric_id` | string (kebab-case) | yes | Unique fabric id |
| `fabric_name` | string | yes | Human-readable name |
| `fabric_class` | enum | yes | `managed` \| `unmanaged` |
| `topology_mode` | enum | yes | `clos` \| `mesh` |
| `switch_entry_ids[]` | array of ids | yes | Switch entries that belong to this fabric |
| `allow_oversubscription` | boolean | no | Suppress the oversubscription warning (default `false`) |

A `mesh` fabric must list exactly **2 or 3** switch entries (each switch is its
own entry). Oversubscription is always a warning, never a blocking error;
`allow_oversubscription: true` suppresses the warning for that fabric.

### Device groups: `entries[]`

A plan entry is a group of identical devices. Plan-specific concerns live here;
the hardware template lives in the catalog.

| Field | Type | Required | Applies to |
|-------|------|----------|------------|
| `entry_id` | string (kebab-case) | yes | all |
| `device_class_id` | string (id) | yes | all |
| `quantity` | integer ≥ 1 | yes | all |
| `role` | enum | yes | all |
| `label` | string | no | all |
| `fabric_id` | string (id) | no | switch roles |
| `override_quantity` | integer ≥ 1 | no | switch roles |
| `topology_mode` | enum | no | switch roles |
| `redundancy` | object | no | switch roles |
| `port_zones[]` | switch port zone | no | switch roles |
| `connections[]` | connection | no | server role |

`role` ∈ `server` \| `spine` \| `server-leaf` \| `border-leaf` \| `oob-leaf` \|
`hhg`.

**Redundancy** is `{ type, switch_count }` where `type` ∈ `none` \| `mclag` \|
`eslag`. `mclag` requires an even `switch_count` ≥ 2; `eslag` requires a
`switch_count` from 2 to 4; `none` needs no count.

**Switch port zones** (`port_zones[]`) name allocation regions on a switch:

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `zone_id` | string (kebab-case) | yes | Unique zone id |
| `zone_name` | string | yes | Human-readable name |
| `zone_type` | enum | yes | `server` \| `uplink` \| `mesh` \| `peer` \| `session` \| `oob` |
| `port_range` | string | yes | e.g. `"1-48"` or stepped `"1-32:2"` |
| `breakout` | object | no | `{logical_ports, logical_speed_gbps}` |
| `priority` | integer ≥ 0 | yes | Lower allocates first |
| `allocation` | enum | yes | `sequential` \| `interleaved` \| `spaced` |
| `transceiver_intent_id` | string (id) | no | Expected transceiver DeviceClass |
| `peer_zone_id` | string (id) | no | Target/peer zone (for uplink zones) |

**Connections** (`connections[]`) declare a server's wiring intent from a NIC
sub-component port to a switch port zone:

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `connection_id` | string (kebab-case) | yes | Unique connection id |
| `nic_slot_id` | string | yes | `sub_component.slot_id` of the NIC on the device class |
| `port_index` | integer ≥ 0 | yes | Zero-based port index on the NIC |
| `ports_per_connection` | integer ≥ 1 | yes | Physical ports per connection (1 = unbundled, 2 = LAG) |
| `connection_type` | enum | yes | `unbundled` \| `bundled` \| `mclag` \| `eslag` |
| `distribution` | enum | yes | `same-switch` \| `alternating` \| `rail-optimized` |
| `target_zone_id` | string (id) | yes | Switch zone this connection targets |
| `speed_gbps` | integer ≥ 1 | yes | Connection speed |
| `rail` | integer ≥ 0 | no | GPU rail index (rail-optimized only) |
| `port_type` | enum | yes | `data` \| `ipmi` \| `pxe` |
| `transceiver_intent_id` | string (id) | no | Expected transceiver DeviceClass |

## Concise example

```yaml
id: example-plan
name: Example Plan
customer_name: Example Co
status: draft

device_catalog:
  - id: gpu-server
    name: Example GPU Server
    slug: gpu-server
    category: compute
    sub_components:
      - slot_id: nic-fe
        device_class_id: cx7-nic
        quantity_per_parent: 2
  - id: cx7-nic
    name: ConnectX-7 NIC
    slug: cx7-nic
    category: nic
    ports:
      - { port_id: p0, speed_gbps: 400, cage_type: osfp, medium: optical }
    sub_components:
      - slot_id: xcvr
        device_class_id: osfp-400g
        quantity_per_parent: 1
  - id: osfp-400g
    name: OSFP 400G Transceiver
    slug: osfp-400g
    category: transceiver
  - id: leaf
    name: Example Leaf
    slug: leaf
    category: network

fabric_domains:
  - fabric_id: frontend
    fabric_name: frontend
    fabric_class: managed
    topology_mode: clos
    switch_entry_ids: [leaves]

entries:
  - entry_id: gpu-servers
    device_class_id: gpu-server
    quantity: 4
    role: server
    connections:
      - connection_id: fe-p0
        nic_slot_id: nic-fe
        port_index: 0
        ports_per_connection: 1
        connection_type: unbundled
        distribution: alternating
        target_zone_id: leaf-server
        speed_gbps: 400
        port_type: data
  - entry_id: leaves
    device_class_id: leaf
    quantity: 2
    role: server-leaf
    fabric_id: frontend
    topology_mode: clos
    port_zones:
      - zone_id: leaf-server
        zone_name: server-access
        zone_type: server
        port_range: "1-48"
        priority: 0
        allocation: sequential
```

## Property naming and the WIT contract

Plan files use **snake_case** property names for YAML ergonomics. The equivalent
component contract (WIT package `aid:core@0.1.0` in `wit/`) uses **kebab-case**
field names for the same data — for example, `device_class_id` here maps to
`device-class-id` in WIT, and `customer_name` maps to `customer-name`. Enum
string values (e.g. `server-leaf`, `same-switch`, `rail-optimized`) are spelled
identically in both. Redundancy is modeled here as an object with a `type`
discriminator (`none` / `mclag` / `eslag`) to match the WIT `redundancy`
variant.

## Semantic validation deferred to AID

JSON Schema enforces structure, required fields, enums, numeric bounds, ID/slug
patterns, the mesh 2–3 switch-count rule, and the MCLAG/ESLAG `switch_count`
rules. The following checks require cross-referencing data across the document
or domain logic that JSON Schema cannot robustly express. They are **deferred to
AID's semantic validation** and are intentionally not enforced here:

- **Referential integrity** — that every `device_class_id`, `fabric_id`,
  `target_zone_id`, `peer_zone_id`, `transceiver_intent_id`, and
  `switch_entry_ids[]` id actually resolves to an object defined elsewhere in
  the plan.
- **Catalog uniqueness and single-definition** — that each DeviceClass appears
  in `device_catalog` exactly once and that ids/slugs are unique.
- **`nic_slot_id` resolution** — that a connection's `nic_slot_id` names a real
  `sub_component.slot_id` on the entry's DeviceClass, and that `port_index` is
  within that NIC's port count.
- **Role-based field applicability** — that switch-only fields (`fabric_id`,
  `override_quantity`, `topology_mode`, `redundancy`, `port_zones`) appear only
  on switch-role entries and `connections` only on server-role entries.
- **`switch_entry_ids` membership** — that every id in a fabric's
  `switch_entry_ids` refers to a switch-role entry.
- **Mixed-distribution conflict** — that rail-optimized and alternating
  connections do not target the same switch class/zone (ALGORITHMS.md
  Algorithm 3).
- **Capacity feasibility** — zone-capacity pre-flight, switch-count lower
  bounds, breakout math, and oversubscription ratio (these are computed
  outputs, not input constraints).
- **`port_range` semantics** — the schema validates the lexical form
  (`"1-48"`, `"1-32:2"`); it does not check that the start ≤ end or that ranges
  within a switch do not overlap.

These deferrals exist because they depend on the whole-document graph or on the
topology calculation itself, not on the shape of any single value.
