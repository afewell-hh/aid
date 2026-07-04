# AID Domain Model

> **Status (2026-07): current.** This document was rewritten to match the
> **real diet/XOC relational model** AID uses today (D18/D19). It **supersedes**
> the earlier "universal `DeviceClass` with recursive sub-components" model,
> which was an **invented** design that shared essentially zero keys with the
> real topology-plan shape and was replaced during the foundation rebuild.
> Authoritative: `DECISIONS.md` (D18, D19, D21, D22, D26, D27),
> `docs/foundation-redesign.md`, `schema/`, and `internal/topology/topology.go`
> (the Go types below are quoted from there).

## Design principles (as built)

- **The model is the real diet/XOC shape, not an invented one** (D18). A plan is
  a bundled DIET YAML document: `meta` + `reference_data` (the catalog) + a
  relational `spec` (+ optional `status`/`expected`). AID ingests exactly what
  HNP authors; it does not translate an invented topology language (D25).
- **Relational, not a device tree** (D19). Topology intent is expressed as
  *classes*, *port zones*, and *connections* referencing a *reference-data
  catalog* by pinned id — **not** as a recursive `DeviceClass` hardware tree.
- **Spec vs status** (D21). `spec` is authored input and the *only* thing that
  drives calculation. `status`/`expected` holds computed/expected values (a
  Kubernetes-style status) used for self-check tests; it never drives the calc.
- **The catalog is a separate, AID-owned artifact** (D21) with a two-layer
  component graph (bare hardware types vs. configured classes, D19). An
  optic/identity **overlay** supplies BOM optic columns + wiring `profile`.

## The plan document (top level)

```
meta:            { case_id, name, description, version, ... }   # identity (planstore keys on meta.case_id)
reference_data:  { device_types, device_type_extensions, breakout_options, module_types }   # the catalog
spec / plan:     { server_classes, switch_classes, switch_port_zones,
                   server_connections, mesh_links?, mclag_domains? }
status/expected: { counts, ... }                                # computed/expected; self-check only (D21)
```

## Core relational types (`internal/topology`)

**`Plan`** = `Meta` (identity; `case_id`, `name`) + `Spec` (authored input) +
optional `Status` (computed/expected). **`Spec`** is the relational core:

| Field (`spec.…`) | Go type | Meaning |
|---|---|---|
| `server_classes[]` | `ServerClassUse` | a class of servers used in this plan (id + `class_ref` into the catalog + quantity + its NIC slots) |
| `switch_classes[]` | `SwitchClassUse` | a class of switches (see below) |
| `switch_port_zones[]` | `SwitchPortZone` | a named range of ports on a switch class with a role, port spec, breakout, and allocation strategy |
| `server_connections[]` | `ServerConnection` | one server-NIC-port → switch-zone connection intent |
| `mesh_links[]` | `MeshLink` | leaf↔leaf mesh links (mesh topology) |
| `mclag_domains[]` | `MCLAGDomain` | MCLAG pairing (when not inline on the class) |

**`SwitchClassUse`** — `switch_class_id`, `class_ref`, `fabric_name`,
`fabric_class` (`managed`|`unmanaged` — gates which fabrics get rendered as hhfab
wiring), `hedgehog_role` (Switch CRD `spec.role`), `override_quantity?` (else the
count is *derived*), `topology_mode` (`spine-leaf`|`mesh`), and inline
`redundancy_type` (`mclag`|`eslag`) / `redundancy_group` (the wiring `SwitchGroup`).

**`SwitchPortZone`** — `switch_class`, `zone_name`, `zone_type`
(`server`|`uplink`|`fabric`|`mesh`|…), `port_spec` (e.g. `"1-63:2"`),
`breakout_option`, `allocation_strategy` (`sequential`|…). Uplink/fabric zones
are what the switch-count derivation and Clos spine wiring key on.

**`ServerConnection`** — `server_class`, `connection_id`, `nic` (NIC slot),
`ports_per_connection`, `hedgehog_conn_type` (`unbundled`|`bundled`|`mclag`|`eslag`),
`distribution` (`same-switch`|`alternating`|`rail-optimized`), a `target_zone`
that resolves to `target_switch_class` + `target_zone_name`, `speed`, optional
`rail` (rail-optimized backends), and `transceiver_module_type` (the optic
selection, resolved by ingest into the class's cage binding).

## The catalog (`reference_data`, `internal/catalog`)

A two-layer, AID-owned component graph referenced by the classes above (D19):

- **`device_types`** — bare hardware device identities (manufacturer/model/slug).
- **`device_type_extensions`** — configured extensions of a device type (the
  ports/cages/capabilities a class actually uses).
- **`breakout_options`** — named breakout configs (`port_spec` → logical ports).
- **`module_types`** — transceiver/optic module types (e.g. `osfp_400g_dr4`,
  `sfp28_25gbase_sr`, `rj45_1000base_t`). The **overlay** supplies their optic
  identity (the BOM's cols 7–19) + descriptive strings; these are AID-owned
  public facts, authored (not read back from `bom.csv`).

Classes bind these by **pinned `ID{name, version}`**. The built-in **Library**
(`internal/library`, Epic #75) is the strict dedup-by-pinned-id *union* of the
catalogs derived from the shipped reference templates — which is why the shipped
reference data must be identity-consistent across templates (D27).

## From plan to outputs (where the model goes)

`internal/topology.IngestBundled` parses this document into the relational `Plan`
+ resolves the `reference_data` into a `*catalog.Catalog`. From there:
`internal/calc` derives switch/server quantities + the per-endpoint allocation IR
(running the proved MoonBit kernel); `internal/bom` reduces the resolved model to
the `bom.csv` projection + the full purchasable BOM; `internal/wiring` renders
per-fabric hhfab CRDs. The BOM/wiring are **projections of one resolved model**,
not a second independent count. See `ARCHITECTURE.md`.

## What was retired (so old references don't mislead)

- **`DeviceClass` (universal hardware template) + recursive `sub_components`** →
  the relational classes/zones/connections + the two-layer reference-data
  catalog above (D18/D19). There is no universal device tree.
- **`PlanEntry` / `FabricDomain` / `device_catalog: DeviceClass[]`** → the
  `spec.*` relational lists + `reference_data` catalog.
- **Recursive `DeviceClassBOM`** → the plan-time relational BOM reduction
  (`internal/bom`, D23) — one resolve, two renders (projection + full BOM).
- The **AS-4126GS "DeviceClass with nested slots" example** was illustrative of
  the invented model; the real full-purchasable-BOM requirement is met by the
  catalog's `bom_line_templates` + per-cage transceivers scaled per instance
  (see `docs/foundation-redesign.md` §4).
