# Layer-1 → Layer-2 IR wire contract

Per the D16 extension to Layer 2 (approved on issue #9), the **snake_case JSON
shape of `topology-ir`** is the single-sourced wire contract between the kernel
(Layer 1) and this adapter (Layer 2). It is the exact byte shape the Phase-6 Go
host will hand the adapter, and the shape `src/ir.rs` deserializes.

There is **one** source of truth for the type shapes: **`wit/types.wit`
`topology-ir`**. This document records how the JSON realizes those WIT records.
The interim encoder lives in `tools/ir-gen/` (runs the merged Phase-3 kernel
`calculate()` — no `kernel/` edits); the Phase-6 kernel boundary encoder must
emit these same bytes (flagged there for consolidation — `ir-gen` is interim
test tooling, not the production boundary).

## Conventions

- **Field names:** `snake_case` (WIT records are `kebab-case`; the JSON wire form
  mirrors the user-facing plan-schema convention used throughout AID — see D16).
- **Enums:** lowercase string (`node-type` → `"server"` / `"switch"` / `"spine"`).
- **`option<T>`:** JSON `null` when absent, else the bare value.
- **Integers vs floats:** integer-typed fields emit integer tokens; only
  `oversubscription-ratio` is a float. Whitespace is insignificant (vendored
  files are pretty-printed for review; the contract is the field shape).

## Mapping (`wit/types.wit` → JSON)

`topology-ir`:

| WIT field  | JSON key   | Type                      |
|------------|------------|---------------------------|
| `metadata` | `metadata` | `plan-metadata` (object)  |
| `nodes`    | `nodes`    | array of `topology-node`  |
| `edges`    | `edges`    | array of `topology-edge`  |
| `fabrics`  | `fabrics`  | array of `fabric-summary` |

`plan-metadata`: `plan_id`, `plan_name`, `customer_name` (all string).

`topology-node`:

| WIT field         | JSON key          | Type                                  |
|-------------------|-------------------|---------------------------------------|
| `node-id`         | `node_id`         | string                                |
| `name`            | `name`            | string                                |
| `node-type`       | `node_type`       | `"server"` \| `"switch"` \| `"spine"` |
| `device-class-id` | `device_class_id` | string                                |
| `fabric`          | `fabric`          | string \| null                        |
| `hedgehog-role`   | `hedgehog_role`   | string \| null                        |
| `instance-index`  | `instance_index`  | u32                                   |

`topology-edge`:

| WIT field         | JSON key          | Type           | Notes                                                        |
|-------------------|-------------------|----------------|--------------------------------------------------------------|
| `edge-id`         | `edge_id`         | string         |                                                              |
| `node-a-id`       | `node_a_id`       | string         | endpoint node id                                             |
| `node-b-id`       | `node_b_id`       | string         | endpoint node id                                             |
| `speed-gbps`      | `speed_gbps`      | u32            | 0 on kernel uplink/mesh edges                                |
| `fabric`          | `fabric`          | string         |                                                              |
| `zone`            | `zone`            | string         | IR-internal zone id                                          |
| `breakout-index`  | `breakout_index`  | u32 \| null    |                                                              |
| `connection-type` | `connection_type` | string         | `"unbundled"` / `"uplink"` / `"mesh"` (kernel-emitted)       |
| `port-a`          | `port_a`          | string         | IR-internal port ref (e.g. `nic-fe:0`) — NOT an hhfab port   |
| `port-b`          | `port_b`          | string         | IR-internal port ref — NOT an hhfab port                     |

> **Adapter note:** the kernel emits `connection_type: "uplink"` for leaf→spine
> edges; the adapter maps that to hhfab's **`fabric`** Connection variant. The
> kernel emits one edge per physical port; the adapter **aggregates** per
> leaf↔spine (and per switch↔switch for mesh) into a single Connection with a
> `links[]` list. `port_a`/`port_b` are IR-internal zone/slot refs; the adapter
> **synthesizes** hhfab port names (`<switch>/E1/N`, `<server>/enpNsM`).

`fabric-summary`:

| WIT field                     | JSON key                        | Type |
|-------------------------------|---------------------------------|------|
| `fabric-name`                 | `fabric_name`                   | string |
| `switch-count`                | `switch_count`                  | u32  |
| `total-server-bandwidth-gbps` | `total_server_bandwidth_gbps`   | u64  |
| `total-spine-bandwidth-gbps`  | `total_spine_bandwidth_gbps`    | u64  |
| `oversubscription-ratio`      | `oversubscription_ratio`        | f64  |

## Regenerating the vendored IR

```sh
hhfab-adapter/tools/gen-ir.sh
```

Runs `aid/kernel` `calculate()` (via `tools/ir-gen`, local-path dep on the
kernel) over each fixture's embedded plan JSON and writes
`tests/testdata/<fixture>.ir.json`. Never hand-edit those files.
