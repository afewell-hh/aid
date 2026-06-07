# AID Core WIT Package

This directory holds the WASM Component Model contracts (WIT) for the AID core
MVP: the topology calculator, the hhfab wiring adapter, and the BOM adapter.
These interfaces are the architectural boundary between the pure MoonBit
calculation kernel, the export adapters, and the Go CLI/API orchestration layer.
Downstream schema, fixture, kernel, adapter, and CLI work depends on them.

Package: `aid:core@0.1.0`

## Layout

| File | Contents |
|------|----------|
| `types.wit` | `interface types` — all shared domain, IR, BOM, and validation types |
| `topology-calculator.wit` | `interface topology-calculator` — Layer 1 kernel contract |
| `hhfab-adapter.wit` | `interface hhfab-adapter` — Layer 2 wiring export contract |
| `bom-adapter.wit` | `interface bom-adapter` — Layer 2 BOM export contract |
| `world.wit` | `world aid-core` — aggregate contract world (binding-generation entry point) |
| `README.md` | this design note |

All five `.wit` files share the single package declaration
`package aid:core@0.1.0;`.

## Interface boundaries

The boundaries mirror the five-layer architecture in `ARCHITECTURE.md` and the
decisions in `DECISIONS.md` (D6, D8, D13). Components never call each other
directly — the Go CLI/API passes data between them (D8).

- **`topology-calculator`** (Layer 1, pure): `topology-plan` →
  `calc-output { ir, boms, validation }`. No filesystem, HTTP, database, or
  NetBox I/O. Domain constraint violations (mesh=4, MCLAG odd count, …) are
  returned as `validation-result` **data**, not as `calc-error`. `calc-error`
  is reserved for inputs that cannot be processed at all.
  - `calculate: func(plan) -> result<calc-output, calc-error>`
  - `validate:  func(plan) -> result<validation-result, calc-error>`
- **`hhfab-adapter`** (Layer 2): consumes `topology-ir` **only** — never plan
  YAML. Pure transformation to wiring YAML.
  - `export-wiring: func(ir, hhfab-options) -> result<hhfab-output, hhfab-error>`
- **`bom-adapter`** (Layer 2): consumes `device-class-bom[]` **only** — never
  NetBox or any database (D6).
  - `export-bom: func(boms, bom-options) -> result<bom-output, bom-error>`

`world aid-core` exports all three interfaces. It is a **contract / binding
surface**, not a deployment claim: a single default world keeps the package
self-validating and lets `wit-bindgen` run without a `--world` flag. In the
implementation phases each component is its own component exporting exactly one
interface; the per-component worlds are trivial and intentionally deferred:

```wit
world topology-calculator-component { export topology-calculator; }
world hhfab-adapter-component        { export hhfab-adapter; }
world bom-adapter-component          { export bom-adapter; }
```

## Modeling decisions

1. **Normalized catalog + ID references (no cyclic graphs).** WIT records
   cannot be recursive, and cross-record cycles are avoided. So:
   - `device-class` lives once in `topology-plan.device-catalog`. Composition
     is expressed by `sub-component.device-class-id` (an ID into the catalog),
     breaking the `DeviceClass → SubComponent → DeviceClass` cycle.
   - `topology-edge` references endpoints by `node-a-id` / `node-b-id` rather
     than embedding `topology-node` (the `DOMAIN_MODEL.md` shape embedded the
     nodes; here the IR is a normalized, acyclic graph).
   - `fabric-domain.switch-entry-ids`, `plan-connection.target-zone-id`,
     `switch-port-zone.peer-zone-id`, and the `transceiver-intent-id` fields are
     all ID references for the same reason.
   String-backed ID aliases (`plan-id`, `device-class-id`, `entry-id`,
   `fabric-id`, `zone-id`, `node-id`, `edge-id`, `connection-id`) document
   intent while remaining plain strings on the wire.

2. **Attributes as `list<attribute>`, not a map.** WIT has no native map type,
   and the modeling guidance requires a typed representation, so arbitrary
   key/value specs use `list<attribute>` (`{ key, value }`).

3. **`topology-ir` does not embed `boms` / `validation`** — a deliberate
   deviation from `DOMAIN_MODEL.md`, where `TopologyIR` lists them inline.
   Rationale: the IR is the stable handoff the **hhfab** adapter consumes, and
   it does not need BOM or validation data; the **bom** adapter consumes
   `device-class-bom[]` as a clean standalone input. The calculator therefore
   returns all three together in `calc-output { ir, boms, validation }`
   (matching the stated calculator outputs: `TopologyIR`, `DeviceClassBOM[]`,
   `ValidationResult`) instead of duplicating large BOM data inside the IR.
   This aligns with the leaner `TopologyIR` shown in `ARCHITECTURE.md`.

4. **`plan-entry` is one flat record with role-dependent fields.** WIT has no
   inheritance. To match `DOMAIN_MODEL.md` (a single `PlanEntry` with a `role`
   discriminator) the record keeps switch-specific fields
   (`fabric-id`, `override-quantity`, `topology-mode`, `redundancy`,
   `port-zones`) and the server-specific field (`connections`) on one record.
   Applicability is governed by `role`: inapplicable list fields are empty and
   inapplicable options are `none`. `redundancy` is a `variant`
   (`none | mclag | eslag`) rather than two optional configs.

5. **Numeric types.** Counts, quantities, capacities, speeds, and indices use
   unsigned integers (`u32`, with `u64` for aggregate bandwidth);
   `oversubscription-ratio` is `f64` (DECISIONS D11).

6. **Structured results everywhere.** Every entry point returns
   `result<_, variant>` with a typed error (`calc-error`, `hhfab-error`,
   `bom-error`) — no raw-string-only failure channel.

7. **BOM carries a `device-class-summary` snapshot, not an ID or the full
   `device-class`.** `device-class-bom` and `bom-line-item` each embed a
   `device-class-summary` (`id`, `name`, `slug`, `category`, `manufacturer`,
   `part-number`). This keeps the BOM adapter input **self-contained**: it can
   render human-reviewable CSV/JSON (`ROADMAP.md` Phase 4) from
   `device-class-bom[]` alone, without consuming `topology-plan` or looking up
   the catalog — preserving the adapter boundary. `DOMAIN_MODEL.md` models
   `DeviceClassBOM.device_class` / `BOMLineItem.device_class` as the full
   `DeviceClass`; the summary is a flat projection of its identity fields,
   deliberately omitting `sub-components`, `ports`, and `attributes` so it
   stays non-recursive and cannot reintroduce a cycle. Quantities (not the
   recursive structure) are what the BOM needs, and those live on
   `bom-line-item`. No plan metadata is added to `bom-options`/`export-bom`:
   the summary fields cover the per-device-class sections the adapter renders.

8. **Minor additions beyond `DOMAIN_MODEL.md`, for round-tripping:**
   `device-class-bom.entry-id` (maps a BOM back to its originating plan entry),
   `topology-node.node-id` and the id-based edges (item 1), and
   `topology-ir.metadata` (`plan-metadata`, matching the `metadata` field of the
   `ARCHITECTURE.md` IR).

## Domain coverage (`DOMAIN_MODEL.md` → WIT)

| Domain concept | WIT representation |
|----------------|--------------------|
| `TopologyPlan` | `types.topology-plan` |
| `DeviceClass`, `Attribute`, `PortSpec`, `SubComponent` | `device-class`, `attribute`, `port-spec`, `sub-component` |
| `FabricDomain` (+ `fabric_class`, `topology_mode`) | `fabric-domain`, `fabric-class`, `topology-mode` |
| `PlanEntry`, `PlanRole` | `plan-entry`, `plan-role` (+ `redundancy`, `mclag-config`, `eslag-config`) |
| `SwitchPortZone`, port range / breakout / allocation | `switch-port-zone`, `port-range` (string), `breakout-option`, `allocation-strategy`, `zone-type` |
| `PlanConnection`, connection / distribution / port-type enums | `plan-connection`, `connection-type`, `distribution`, `port-type` |
| `TopologyIR`, `TopologyNode`, `TopologyEdge`, `FabricSummary` | `topology-ir`, `topology-node`, `topology-edge`, `fabric-summary` (+ `node-type`, `plan-metadata`) |
| `DeviceClassBOM`, `BOMLineItem` (`.device_class`) | `device-class-bom`, `bom-line-item` (each carries a `device-class-summary`) |
| `ValidationResult`, `ValidationIssue` | `validation-result`, `validation-issue`, `issue-severity` |
| hhfab export options / result | `hhfab-options`, `hhfab-output`, `wiring-document`, `hhfab-error` |
| BOM export options / result | `bom-options`, `bom-output`, `bom-format`, `bom-error` |

## Validation

Run from the repo root unless noted. Results captured with
`wasm-tools 1.251.0` and `wit-bindgen-cli 0.57.1`.

```bash
wasm-tools component wit wit                                  # parse → prints package
wasm-tools component wit wit --wasm -o /tmp/aid-core-wit.wasm # encode to a WIT-package wasm
wasm-tools validate /tmp/aid-core-wit.wasm                    # validate the encoded package

rm -rf /tmp/aid-witgen-rust
wit-bindgen rust wit --out-dir /tmp/aid-witgen-rust          # Rust bindings

# NOTE: wit-bindgen moonbit writes part of its output (interface/, world/,
# moon.mod.json) RELATIVE TO THE CURRENT DIRECTORY, regardless of --gen-dir.
# Run it from a scratch directory so it does not write into the repo:
rm -rf /tmp/aid-witgen-moonbit /tmp/aid-mbt-scratch && mkdir -p /tmp/aid-mbt-scratch
( cd /tmp/aid-mbt-scratch && wit-bindgen moonbit /ABS/PATH/TO/wit --gen-dir /tmp/aid-witgen-moonbit )
```

All commands above succeed against this package. Generated bindings are written
only to `/tmp` (or the scratch dir) and are **not** committed.

## Deferrals

- **NetBox WIT** — deferred to Backlog Step 99 / issue #13. No
  `netbox-adapter.wit` is created here.
- **Per-component worlds** — deferred to each component's implementation phase
  (one-line worlds shown above).
- **JSON Schema, fixtures, and any implementation/binding code** — out of scope
  for this ticket.
