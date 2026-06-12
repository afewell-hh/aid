# AID Foundation Redesign Proposal

**Status:** proposal — design only, no implementation (Issue #46). **Rev 2** incorporates devb's architecture review + lead synthesis (R1–R5; the 7 required changes). **devb re-reviewed Rev 2 (at 86b2e78) and approved it — "sound for lead + owner sign-off; no blocking architecture gaps."** This revision folds devb's three non-blocking *implementation gates* into §4.4 (projection = filter over one resolved graph), §4.2 (catalog capability vs plan binding), and Phase F0 (per-namespace/relation validation contracts). **Rev 3 (owner refinements, pre-sign-off):** the catalog is a **separate, AID-owned artifact** referenced by ID (CRD-style; ingest losslessly splits a bundled real file); the catalog has **two layers** — bare hardware *types* vs configured **server/switch classes** (reusable inventory; transceiver bound per **NIC port**; a different optic ⇒ a distinct class); and the plan schema gains a **`spec`/`status` (`expected`)** plane so one document is both a valid input and a self-checking test oracle. Folded into §4.1, §4.2, §4.3, D18/D19, and new **D21**. **devb reconfirmed Rev 3 (at `cc0ddf9`) — "Rev 3 holds; no blocking architecture concerns; approved for lead + owner sign-off"** — leaving four *non-blocking* F0 guardrails (catalog identity/version pinning; deterministic lossless bundled-file ingest; `status`/`expected` never drives production calc; deterministic `ports_per_connection>1` expansion), now captured in Phase F0 (§5).
**Scope:** re-derive AID's canonical plan schema, domain model, and validation strategy from the authoritative sources so a rebuilt AID can ingest the real reference `topology-plan.yaml` files, reproduce their committed expected outputs, **and emit a complete purchasable bill of materials** (the owner's `docs/requirements/real-server-bom.csv`).
**Decision:** this proposal recommends **discarding the invented plan schema and the universal recursive `DeviceClass` *topology root***, and **adopting (a) the real OCP/diet topology-plan shape for topology intent, (b) a relational topology-planning model, and (c) an AID-owned, NetBox-independent component catalog** that drives a complete purchasable BOM. It supersedes **D9, D10, D13** (drafts in §6) and the toy-fixture strategy.

Evidence is cited as `repo/path:line`. The two authoritative sources were cloned to `gitignored/refs/hnp` (HNP, developer-side reference) and `gitignored/refs/xoc` (OCP XOC, the public contract); the owner's requirements artifacts live on `main` at `docs/requirements/`. A throwaway parse spike confirmed the load-bearing claims; nothing under this ticket changes AID code or schema.

> **Rev 2 changelog (what changed vs Rev 1).** Rev 1's core reset direction (adopt the real topology-plan shape; relational topology model; XOC/HNP as the topology oracle; the salvage table) is **retained and extended, not re-litigated**. Rev 2 adds, per the review: (1) an **AID-owned NetBox-independent catalog** with non-physical item kinds; (2) **two attribute planes** per item (`calc_profile` / `purchase_profile`, R1); (3) arbitrary **`bom_line_templates`** incl. non-physical (R3); (4) **component/slot** relationships for nested purchasable parts, with an explicit choice for repeated NICs (R4/R5); (5) explicit **`PortTemplate`/`CageTemplate`** distinguishing fixed interfaces from transceiver cages (R4); (6) a **deterministic BOM reducer** emitting both the full purchasable BOM and the HNP 19-column projection (keeping D6 plan-time); (7) **two oracle layers + provenance gating** (XOC/HNP physical subset + `real-server-bom.csv` full-BOM with 1×/2× scaling; `training_*.yaml`-first). The catalog model is **F0** foundational work (§5). **Owner generalization (folded in):** the BOM is only the *first* consumer — the foundation is a **general extensible object model** (typed objects with open, namespaced attribute sets + arbitrary nested objects), so future features extend it by adding attributes/relations/projections rather than re-cutting the foundation (§4.2, D19).

---

## 0. Executive summary

1. **The invented foundation is disconnected from reality.** AID's `schema/topology-plan-v1.json` requires top-level `[id, name, customer_name, status, device_catalog, fabric_domains, entries]` (`schema/topology-plan-v1.json:8`). The real reference input requires `[meta, reference_data, plan, switch_classes, switch_port_zones, server_classes, server_nics, server_connections, expected]`. **The two vocabularies are disjoint — zero shared top-level keys.** AID cannot parse a single real `topology-plan.yaml`.
2. **The real input format and the HNP "diet" test-case format are the same schema.** Verified by parse: the XOC `topology-plan.yaml` and the HNP `training_*.yaml` have byte-identical top-level key lists (spike output, §2). The XOC `topology-plan.yaml` *is* an HNP diet TopologyPlan document.
3. **The toy fixtures are admittedly fabricated.** The fixtures' own README states they "**do not reproduce that tool's exact device/cable/switch counts, and the baselines here were not generated by it**" (`tests/fixtures/README.md:92-93`); baselines are hand-derived, "no generated tooling was used" (`tests/fixtures/valid/clos-small/expected.json:49`).
4. **HNP's `bom.csv` is a physical *subset*, not the complete purchasable BOM.** HNP's exporter "aggregates generated **Module** inventory" (`gitignored/refs/hnp/netbox_hedgehog/services/bom_export.py:1-8`) and its classifier emits only three module sections — `nic`, `server_transceiver`, `switch_transceiver` (`bom_export.py:454-460`). The owner's real BOM additionally requires **non-physical** lines — warranty (`EWCSC`), software support (`SVC-NVSTDSWSUP-3Y`), accessory (`CBL-PWEX`), assembly (`MC0037`), onsite service (`OSNBD3`) — and **nested embeddable components** (`8× CX-7`, `1× BF3 DPU` with their own cages/transceivers) scaled by instance count (`main:docs/requirements/README.md:12-28`; `main:docs/requirements/real-server-bom.csv:3-15`).
5. **HNP delegates the hardware catalog to NetBox; AID has no NetBox.** HNP deliberately maps switch/server/NIC/transceiver identity onto NetBox `dcim.DeviceType`/`ModuleType`/`InterfaceTemplate`, keeping only `BreakoutOption` + `DeviceTypeExtension` custom (`gitignored/refs/hnp/netbox_hedgehog/models/topology_planning/reference_data.py:4-13,142-158`). AID **cannot depend on NetBox seed state** and must own an equivalent catalog.
6. **Recommendation:** adopt the real topology-plan shape for topology intent (no converter); adopt the relational topology-planning model **plus an AID-owned component-graph catalog**; emit a **complete purchasable BOM** with the HNP 19-column CSV as a validation *projection*; replace the toy fixtures with **two oracle layers** — the XOC compositions (`xoc-64 → xoc-1024`) for the physical/wiring subset and `real-server-bom.csv` for the full purchasable BOM (incl. 1×/2× scaling).
7. **Salvageable:** the schema-agnostic infrastructure — `internal/wasmhost` (the WASM ABI host), `scripts/moon-prove-gate.sh`, `.github/workflows/ci.yml`, the `hhfab validate` harness, and the proof *technique*. The Go orchestration/CLI/REST/UI and the two Rust adapters are **rework** (good code, wrong contract). The kernel decode/type layer and all fixtures are **discard/regenerate**.

---

## 1. The diet feature set (from HNP)

"DIET" = **D**esign and **I**mplementation **E**xcellence **T**ools — the design-time topology-planning module of the NetBox-Hedgehog plugin (`gitignored/refs/hnp/netbox_hedgehog/models/topology_planning/topology_plans.py:2`). It is **not** a slimmed-down API; it is the planning subsystem, exposed for testing as a curated **declarative YAML contract** that captures planning *intent* (classes, zones, connection templates) from which the engine deterministically computes quantities, devices, cables, BOM, inventory, and wiring.

### 1.1 The domain model (the model AID must align to)

Authoritative model package: `gitignored/refs/hnp/netbox_hedgehog/models/topology_planning/`. It is **relational**, not a recursive composite. Eleven model classes; the load-bearing ones:

| Entity | Role | Key fields (cited) |
|---|---|---|
| `TopologyPlan` | root | `name, customer_name, status` — `topology_plans.py:33,42-58` |
| `PlanServerClass` | a group of identical servers | `server_class_id, quantity` ("PRIMARY USER INPUT"), `server_device_type`→`dcim.DeviceType`, `category, gpus_per_server` — `topology_plans.py:131,147,164,171` |
| `PlanServerNIC` | one physical NIC/DPU slot (DIET-294) | `server_class`→, `nic_id`, `module_type`→`dcim.ModuleType` — `topology_plans.py:200,217,226` |
| `PlanSwitchClass` | a group of switches | `switch_class_id, fabric_name, fabric_class (managed/unmanaged), hedgehog_role, device_type_extension, uplink_ports_per_switch, override_quantity, calculated_quantity, topology_mode, redundancy_type/group` — `topology_plans.py:276,292-401` |
| `SwitchPortZone` | a port-allocation zone on a switch class | `zone_name, zone_type, port_spec, breakout_option, allocation_strategy, priority, peer_zone, transceiver_module_type` — `port_zones.py:20,32-92` |
| `PlanServerConnection` | a connection template (NIC→zone) | `connection_id, nic`→NIC, `port_index, ports_per_connection, hedgehog_conn_type, distribution, target_zone`→`SwitchPortZone`, `speed, rail, port_type, transceiver_module_type` — `topology_plans.py:650,668-746` |
| `PlanMeshLink` | /31 point-to-point mesh link (DIET-309) | `switch_class_a/b, subnet, link_index, leaf{1,2}_port/name` — `topology_plans.py:592-618` |
| `PlanMCLAGDomain` | MCLAG/ESLAG domain | `domain_id, switch_class, peer/session_link_count, switch_group_name, redundancy_type` — `topology_plans.py:928,943-983` |
| `NamingTemplate` | device/iface naming | `device_category, pattern` (Python format string, e.g. `"{site}-{class}-{index:04d}"`) — `naming.py:15,22-25` |
| `GenerationState` | generation result/dirty-tracking | `device_count, interface_count, cable_count, snapshot` — `generation.py:19,39-54` |

**Reference data deliberately reuses NetBox core DCIM models** rather than custom duplicates; the comment block records that `SwitchModel`/`NICModel`/`SwitchPortGroup` were *removed* in favor of `dcim.DeviceType` (switches/servers), `dcim.ModuleType` (NICs/transceivers), and `dcim.InterfaceTemplate` (ports) (`reference_data.py:1-13,142-158`). **This is the central gap for AID:** HNP could omit a hardware catalog because *NetBox was the catalog* — switch/server/NIC/transceiver SKUs, port/interface templates, and module-bay structure all live in NetBox DCIM, seeded by `seed_catalog.py`/`populate_transceiver_bays.py`. AID is standalone (D1: no Python, no NetBox), so **AID must own an equivalent catalog** (§4.2). Only two custom reference models survive in HNP itself:
- `BreakoutOption` — breakout math (`breakout_id, from_speed, logical_ports, logical_speed`) — `reference_data.py:22,36-49`.
- `DeviceTypeExtension` — Hedgehog metadata one-to-one on a DeviceType (`mclag_capable, hedgehog_roles[], supported_breakouts[], native_speed, hedgehog_profile_name`) — `reference_data.py:65,77-123`.

Transceivers are `dcim.ModuleType` with the `'Network Transceiver'` profile carrying `attribute_data.reach_class`/`medium`, validated for copper-vs-optical compatibility at connection save (`topology_plans.py:746-757,850-884`).

### 1.2 The engine (the behavior AID must reproduce)

Authoritative engine: `gitignored/refs/hnp/netbox_hedgehog/services/`. Load-bearing behaviors:

- **Switch count is derived, not entered.** `PlanSwitchClass.effective_quantity = override_quantity else calculated_quantity else 0` (`topology_plans.py:577-589`); the generator iterates `range(switch_class.effective_quantity)` (`device_generator.py:320`). The calc oracle derives leaf/spine counts from server port demand (`tests/test_topology_planning/test_topology_calculations.py`).
- **Port allocation** is a stateful cursor over a zone's parsed `port_spec`, with strategies `sequential|interleaved|spaced|custom`, expanded by breakout into `E1/{port}` or `E1/{port}/{lane}` slots (the `E1/` prefix is hardcoded) — `port_allocator.py:16-22,51-67,104,112`; spec parser handles ranges, comma-lists and `start-end:step` — `port_specification.py:39-77`.
- **Transceiver rule engine** `evaluate_xcvr_pair(server_attrs, zone_attrs)` → `match|needs_review|blocked`; **only medium mismatch BLOCKS**; cage/connector mismatches degrade to `needs_review` — `transceiver_rules.py:160,217-253`.
- **BOM export** classifies each generated Module into `nic|server_transceiver|switch_transceiver` (plus base `server|switch`), suppresses switch-side cable assemblies (counted in a footer), and emits a fixed 19-column CSV (§2.2) — `bom_export.py:454-460,110-112,226-232,271`.
- **Wiring generation** is inventory-based: VLANNamespace(1000–2999)/IPv4Namespace(10.0.0.0/16)/Switch/SwitchGroup/Server/Connection CRDs at `wiring.githedgehog.com/v1beta1`; connection type routed by zone (server→unbundled/bundled/mclag/eslag, peer/session→mclagDomain, mesh→mesh links) — `yaml_generator.py:76-172,1525-1559,1354-1491`.
- **Preflight** validates transceiver-bay readiness: NIC ModuleType needs ≥1 ModuleBayTemplate; switch DeviceType needs `ModuleBayTemplates ≥ InterfaceTemplates` — `preflight.py:130-133,156-182`.

### 1.3 The diet interface (the input contract)

The diet test-case YAML (validated in `gitignored/refs/hnp/netbox_hedgehog/test_cases/schema.py:158-232`) has these sections — **this is exactly the XOC `topology-plan.yaml` shape**:

```
meta:               {case_id (^[a-z0-9_]+$), name, version:int, managed_by:"yaml", description?, tags?}
reference_data:     {manufacturers[], device_types[ (+interface_templates[]) ], device_type_extensions[],
                     breakout_options[], module_types[ (+attribute_data, interface_templates[]) ]}
plan:               {name, status∈{draft,review,approved,exported}, description?}
switch_classes[]:   {switch_class_id, fabric_name, fabric_class, hedgehog_role, device_type_extension,
                     topology_mode?, override_quantity?, uplink_ports_per_switch?, mclag_pair?,
                     redundancy_type?, redundancy_group?, groups?}
switch_port_zones[]:{switch_class, zone_name, zone_type∈{server,uplink,mclag,peer,session,oob,fabric,mesh},
                     port_spec, breakout_option?, allocation_strategy?, priority?, peer_zone?,
                     transceiver_module_type?}
server_classes[]:   {server_class_id, quantity, server_device_type, category?, gpus_per_server?, description?}
server_nics[]:      {server_class, nic_id, module_type}          # many-to-many join, NOT embedded
server_connections[]:{server_class, connection_id, nic, port_index, ports_per_connection,
                     hedgehog_conn_type, distribution, target_zone:"<switch_class>/<zone_name>",
                     speed, rail?, port_type, transceiver_module_type?}
expected:           {counts:{server_classes, switch_classes, connections}}    # self-check oracle
```
Citations: `test_cases/schema.py:158-232`, `test_cases/ingest.py:776-1197`, `docs/DIET_TOPOLOGY_PLAN_YAML_REFERENCE.md`. Connections reference a **NIC + zone** (DIET-294 NIC-first model); legacy `target_switch_class`/`nic_module_type` are hard-rejected (`ingest.py:1043-1102`). The harness ingests a case (`apply_diet_test_case --case <id>`), generates devices, and asserts `expected.counts` against `planning_counts` (`ingest.py:329-348`); a richer oracle compares `GenerationState` device/interface/cable counts (`assertions.py:88-116`).

**Feature surface AID's interface + model must support (concretely):** server classes with quantity + GPUs; a NIC join table; switch classes with managed/unmanaged fabric class, hedgehog role, derived-or-overridden quantity, spine-leaf **and** mesh topology modes, MCLAG/ESLAG redundancy; port zones (8 zone types) with `port_spec` (ranges + comma-lists + breakout steps) and 4 allocation strategies; NIC-first connection templates with `unbundled/bundled/mclag/eslag` types and `same-switch/alternating/rail-optimized` distribution (incl. per-rail index); reference-data catalog (manufacturers, device types + interface templates, device-type extensions, breakout options, module types/transceivers with optical attributes); INB/OOB management fabrics; and an `expected.counts` self-check.

---

## 2. The authoritative I/O contract (from XOC)

Anchor composition: `gitignored/refs/xoc/compositions/xoc/xoc-64/1x-OPG-64/mesh-conv-ro--cx7-1x400g--bf3-2x200g--storage-conv-2x200g--inb-2x25g/`.

### 2.1 Input — `topology-plan.yaml` (876 lines)

The 9 sections of §1.3, populated with real hardware. Parse-verified facts (spike):
- `switch_classes`: `[scale_out_leaf, soc_storage_leaf, inb_mgmt_leaf, oob_leaf]`; `expected.counts = {server_classes:5, switch_classes:4, connections:21}`.
- `server_classes`: `[(compute_xpu,8), (storage_srv,3), (metadata_srv,3), (hh_gateway,2), (hh_controller,1)]`.
- `reference_data` sub-keys: `[manufacturers, device_types, device_type_extensions, breakout_options, module_types]`.
- 21 `server_connections`, 14 `server_nics`. Example connection: `{server_class: compute_xpu, connection_id: scale-out-rail-0, nic: scale_out, ports_per_connection: 1, hedgehog_conn_type: unbundled, distribution: rail-optimized, target_zone: scale_out_leaf/scale_out_server_2x400, speed: 400, rail: 0, port_type: data, transceiver_module_type: osfp_400g_dr4}`.
- `port_spec` uses comma-lists + ranges, e.g. `26,28,30,32,34,36,38-63` (`topology-plan.yaml:483`) — the current AID `port_range` regex `^[0-9]+(-[0-9]+)?(:[0-9]+)?$` (`schema/topology-plan-v1.json:272-276`) **cannot express this**.

> **Important provenance nuance.** The committed outputs were generated from the **diet/training** form, not directly from `topology-plan.yaml`. For this composition the translation **collapses** `scale_out_leaf`+`soc_storage_leaf` into one shared DS5000 mesh pair `soc_storage_scale_out_leaf` (training has 3 switch_classes vs the plan's 4; spike output + `generated/inputs/translation-notes.md:30-32`). `netbox_inventory.json` records `plan_name: "Training XOC-64 1x OPG-64 Mesh Converged RO"`. So `topology-plan.yaml` is the human-authoring artifact and `training_*.yaml` (same schema) is what actually produced the oracle. The validation strategy (§4.5) accounts for this.

### 2.2 Output — `bom.csv` (19 columns; the BOM contract)

Header (`bom.csv:1`): `section, module_type_model, module_type_description, hedgehog_class, manufacturer, quantity, cage_type, medium, connector, standard, reach_class, wavelength_nm, host_lane_count, host_serdes_gbps_per_lane, optical_lane_pattern, gearbox_present, cable_assembly_type, breakout_topology, is_cable_assembly`. Full committed contents (xoc-64 mesh-conv-ro):

- Base devices: `server compute_xpu ×8`, `hh_controller ×1`, `hh_gateway ×2`, `metadata_srv ×3`, `storage_srv ×3`; `switch celestica-ds2000 (inb_mgmt_leaf) ×1`, `Celestica DS1000 (oob_leaf) ×1`, `celestica-ds5000 (soc_storage_scale_out_leaf) ×2`.
- NICs: `BMC Management Port ×17`, `Dual-Port 200G ×6`, `Dual-Port 25GbE ×17`, `xPU Scale-Out 8x400G ×8`, `xPU SoC/Storage 2x200G ×8`.
- Server transceivers: `OSFP-400G-DR4 ×64` (`OSFP,SMF,MPO-12,400GBASE-DR4,DR,1310,4,100,DR4,...,1x`), `QSFP112-200GBASE-SR2 ×28`, `RJ45-1000BASE-T ×17`, `SFP28-25GBASE-SR ×17`.
- Switch transceivers: `R4113-A9220-VR ×11` (`OSFP,MMF,Dual MPO-12,800GBASE-2xVR4,VR,850,8,100,VR4,...,2x400g` — the only non-`1x` breakout_topology), `OSFP-400G-DR4 ×32`, `RJ45-1000BASE-T ×17`, `SFP28-25GBASE-SR ×17`.
- Footer: `# suppressed_switch_cable_assembly_count,0` (`bom.csv:23`).

This is an **optics-aware procurement BOM** — cage/medium/connector/standard/reach/wavelength/lanes/breakout per transceiver SKU. The current AID BOM emits only `quantity_per_unit`/`fleet_quantity` per `DeviceClass` with none of these optical attributes.

### 2.3 Output — `connectivity-map.csv` (11 columns)

Header (`connectivity-map.csv:1`): `plan_id, cable_id, status, type, zone, a_device, a_interface, a_role, b_device, b_interface, b_role`. 128 cable rows, all `status=connected`. Endpoints use device name + port + role (`server`/`leaf`), e.g. server→leaf `compute_xpu-001,scale_out-so0,server, soc_storage_scale_out_leaf-01,E1/1/1,leaf` and the two inter-leaf mesh cables `…leaf-01,E1/26,leaf, …leaf-02,E1/26,leaf` (`connectivity-map.csv:4,128-129`). `cable_id` matches the `netbox_inventory.json` cable `id`.

### 2.4 Output — `netbox_inventory.json` (5 top-level keys)

`{cables, devices, interfaces, modules, metadata}`. `metadata.counts` pins: **devices 21, modules 259, interfaces 481, cables 128**. Devices carry `custom_field_data.{hedgehog_class, hedgehog_plan_id}`, role, site, status. Cables carry a/b terminations with `object_type:"Interface"`.

### 2.5 Output — `wiring/*.yaml` + hhfab validation

hhfab CRDs at `wiring.githedgehog.com/v1beta1` + `vpc.githedgehog.com/v1beta1`. `wiring-soc-storage-scale-out.yaml`: 93 `Connection`, 14 `Server`, 2 `Switch`, 1 `VLANNamespace`, 1 `IPv4Namespace`. Switch CRD carries `spec.profile: celestica-ds5000`, `role: server-leaf`, `boot.mac`, and a per-port `portBreakouts` map (`E1/1..16: 2x400G`, `E1/27.. : 4x200G`, mesh ports `1x800G`). **`hhfab validate` passes** (Fabricator v0.45.5): "`Fabricator config and wiring are valid`" in both `diagrams/hhfab/hhfab_validate_*.log`.

> **Note on device-name normalization:** `connectivity-map.csv`/`netbox_inventory.json` use underscores (`soc_storage_scale_out_leaf-01`), while the hhfab CRDs use hyphens for device names (`soc-storage-scale-out-leaf-01`) but preserve underscores in port suffixes (`scale_out-so0`). AID must reproduce both conventions.

### 2.6 The full purchasable BOM contract (owner artifact) — the *second* output layer

The XOC `bom.csv` (§2.2) is a **physical subset**: it is built by aggregating *generated NetBox Modules* (`bom_export.py:1-8`), its module classifier knows only `nic`/`server_transceiver`/`switch_transceiver` (`bom_export.py:454-460`), and its base-device rows come from generated `Device`s (`bom_export.py:167-172`). It carries **no** warranty, support, assembly, accessory, onsite-service, chassis, GPU-board, CPU, memory, or drive lines.

The owner's `docs/requirements/real-server-bom.csv` is the **complete purchasable BOM** AID must also emit — a real single-server BOM (4U HGX B200) whose 13 line types (`real-server-bom.csv:3-15`) include both physical components and **non-physical** items:

| Type (CSV) | SKU | Qty/server | Physical? |
|---|---|---|---|
| Barebone | `AS-4126GS-NBR-LCC` | 1 | chassis |
| **EWCSC** (warranty) | `EWCSC` | 1 | **non-physical** |
| GPU Board | `GPU-NVHGX-B200-8180-LC` | 1 | physical |
| CPU | `PSE-TUR9575F-1554` | 2 | physical |
| MEMORY | `MEM-DR512L-CL01-ER64` | 24 | physical |
| Drive | `HDS-MUN-…` / `HDS-SMN0-…` | 2 / 1 | physical |
| **AOC NETWORK** (NIC) | `AOC-CX766003N-SQ0` (CX-7) | **8** | physical, nested |
| **AOC NETWORK** (DPU) | `GPU-NVDPU-BA3220-C` (BF3) | **1** | physical, nested |
| **AOC Required Service** (support) | `SVC-NVSTDSWSUP-3Y` | 1 | **non-physical** |
| **Accessory** | `CBL-PWEX-1174-60` | 1 | physical accessory |
| **Assembly** | `MC0037` | 1 | **non-physical** |
| **Onsite Service** | `OSNBD3` | 1 | **non-physical** |

Two load-bearing requirements stated by the owner (`main:docs/requirements/README.md:17-28`):
1. **Transceivers are required subcomponents that are NOT line items in this flat CSV** — AID's model must add them per populated cage from the catalog. The `8× CX-7` each have **one QSFP112 cage @ 400G**; the `1× BF3` has **one fixed 1000BASE-T BMC port + two QSFP112 cages @ 200G** (`real-server-bom.csv:10-11`; `README.md:18-21`).
2. **The complete BOM = union of** base line items + required subcomponents (NICs, DPUs, transceivers) + arbitrary non-physical items, **scaled by instance count** (`README.md:23-28`).

This is the requirement HNP's exporter structurally cannot express, and it is why AID needs its own catalog + line-template model (§4.2–§4.4). The HNP 19-column `bom.csv` becomes a *projection* of AID's resolved model (§4.4), used for the XOC oracle; `real-server-bom.csv` is the acceptance oracle for the full BOM (§4.5).

---

## 3. Gap analysis — current AID vs the contract

### 3.1 What is invented / wrong

| Area | Current AID | Reality | Verdict |
|---|---|---|---|
| Plan schema | `id,name,customer_name,status,device_catalog,fabric_domains,entries` (`schema:8`) | `meta,reference_data,plan,switch_classes,switch_port_zones,server_classes,server_nics,server_connections,expected` | **Disjoint — wrong** |
| Hardware model | one recursive `DeviceClass` with embedded `sub_components`/`ports` (D13; `DOMAIN_MODEL.md:5-7`) | relational: `PlanServerClass` + `PlanServerNIC` join + `PlanSwitchClass` + reference-data tables on `dcim.*` (`topology_plans.py`, `reference_data.py:142-158`) | **Wrong abstraction** |
| Fabric | first-class `fabric_domain` object with `switch_entry_ids[]` (`schema:179-233`) | `fabric_name` is a string column on each switch class; no fabric object (`topology-plan.yaml:445`) | **Invented** |
| Connection target | single `target_zone_id` (`schema:336`) | 2-part `switch_class/zone_name` (`topology-plan.yaml:629`) | **Wrong shape** |
| Port spec | regex `^[0-9]+(-[0-9]+)?(:[0-9]+)?$` (`schema:272`) | comma-lists + ranges `26,28,30,...,38-63` (`topology-plan.yaml:483`) | **Too weak** |
| BOM output | `{quantity_per_unit, fleet_quantity}` per DeviceClass | 19-column optics-aware CSV (§2.2) | **Missing 17 columns** |
| Reference data | none (specs inline in catalog) | manufacturers/device_types/extensions/breakout_options/module_types catalog | **Missing** |
| Fixtures/oracle | 3 hand-invented toys; "do not reproduce real counts" (`tests/fixtures/README.md:92-93`) | 55 HNP cases + the XOC composition matrix with committed outputs | **Fabricated** |
| Outputs produced | IR + per-DeviceClass BOM + wiring | BOM + connectivity-map + netbox_inventory + wiring | **2 of 4 missing** |
| **Hardware catalog** | none (specs inline) | HNP: NetBox `dcim.*`; AID has no NetBox | **AID must own a catalog** |
| **Purchasable BOM** | per-DeviceClass qty only | physical subset (HNP) **+** full purchasable (owner) | **non-physical + nested missing** |
| **Port/cage model** | `port_spec` is a switch-zone string only | fixed-interface vs transceiver-cage distinction needed | **no fixed/cage distinction** |

**Strain 1 — BOM completeness (R3/R5).** Neither the invented AID BOM nor HNP's exporter can produce the owner's complete purchasable BOM (§2.6). HNP is Module-inventory-based and 3-section (`bom_export.py:1-8,454-460`); AID's recursive `DeviceClass` BOM emits only `quantity_per_unit`/`fleet_quantity`. Both lack non-physical line items and per-cage transceiver resolution. **Fix: §4.2–§4.4** (catalog + line templates + reducer).

**Strain 2 — port/cage modeling (R4).** HNP gives useful nested-transceiver primitives — `PlanServerNIC` = one physical NIC/DPU slot, one Module per server (`topology_plans.py:200-231`; `device_generator.py:399-411`); `PlanServerConnection.transceiver_module_type` selects a server-side cage transceiver, placed `device → NIC bay → NIC Module → cage-N bay → transceiver Module` (`topology_plans.py:742-757`; `device_generator.py:1383-1450`); switch zones carry a zone-level `transceiver_module_type` placed per physical parent cage with breakout dedupe (`port_zones.py:76-90`; `device_generator.py:1470-1539`); and transceiver attributes (cage/medium/connector/lanes/reach/breakout) are seeded (`seed_catalog.py:10-71`). **But HNP cannot distinguish a fixed interface from a pluggable cage:** `populate_transceiver_bays.py` creates a `cage-{index}` bay for *every* NIC InterfaceTemplate (`populate_transceiver_bays.py:63-85`), and the seeded BF3 has only two QSFP112 interface templates with **no fixed BMC port** (`seed_catalog.py:613-623`) — while the owner requires BF3 = 1 fixed 1000BASE-T BMC + 2 QSFP112 cages (`README.md:20-21`). XOC works around this by modeling BMC as a *separate* `bmc_module` NIC (`gitignored/refs/xoc/…/topology-plan.yaml:420-425,587`) — fine for the XOC oracle, insufficient for the real-server object model. **Fix: §4.2** (explicit `PortTemplate`/`CageTemplate`).

### 3.2 What is genuinely salvageable

| Component | Verdict | Why (cited) |
|---|---|---|
| `internal/wasmhost` | **REUSE** | Generic JSON-over-memory ABI host; zero schema knowledge — `internal/wasmhost/wasmhost.go:1-11` |
| `scripts/moon-prove-gate.sh` | **REUSE** | Pure proof-gate infra (parses `moon prove` stdout) |
| `.github/workflows/ci.yml` | **REUSE** | build-test + prove-gate + real `hhfab validate`, pinned toolchains |
| `hhfab validate` harness | **REUSE** | The acceptance oracle for wiring (CI shells real hhfab) |
| `internal/planstore` | **REUSE** (light) | YAML-file store; reads only `id/name/status` |
| Proof *technique* (`kernel/proofs` + gate) | **REUSE technique** | Pure Int/Bool cores provable; specific goals follow surviving algorithms |
| `internal/orchestrate`, `cmd/aid`, `ui/` | **REWORK** | Solid Go/MoonBit-JS scaffolding; contract is the invented IR/BOM/plan JSON |
| `hhfab-adapter`, `bom-adapter` (Rust) | **REWORK** | Mature pure transforms; consume the invented IR/BOM shapes (`hhfab-adapter/src/ir.rs`) — retarget to the corrected IR + add the BOM optics columns |
| `kernel/src` decode/types | **DISCARD/REWRITE** | `decode.mbt`/`types.mbt` bound to the snake_case invented schema (`kernel/src/decode.mbt:215,251,259`); calc logic (switch_count, mesh, allocation) is portable but must be re-derived against the diet model |
| `tests/fixtures/*` + `expected.json` | **DISCARD/REGENERATE** | Hand-invented; replaced by XOC oracles |

### 3.3 What is missing entirely

Reference-data catalog ingestion; the `server_nics` join; NIC-first connections; per-zone `port_spec` comma-lists; breakout catalog; transceiver selection + optical attributes; the optics-aware BOM columns; `connectivity-map.csv`; `netbox_inventory.json`; INB/OOB management fabrics; rail-optimized distribution as exercised by real plans; the `expected.counts` self-check.

---

## 4. The redesign

The redesign has **two coupled planes**: a **topology plane** (adopt the real topology-plan shape + relational planning model — the part Rev 1 got right) and a **catalog/BOM plane** (an AID-owned component-graph catalog driving a complete purchasable BOM — the part Rev 2 adds). They meet at the catalog: topology classes reference catalog items by SKU; the BOM reducer resolves those items' components and line templates.

### 4.1 Canonical topology input — **adopt the real topology-plan shape (no converter)**

AID's canonical *topology* input is the **diet/XOC `topology-plan.yaml` shape** (the 9 sections of §1.3). AID publishes a JSON Schema *for that real format*, not an invention. **Adopt directly, no converter:** (a) the real format and the diet engine format are the *same schema* (§2.1, spike), so a converter would convert a format to itself; (b) a converter re-introduces an AID-invented intermediate — the failure being corrected; (c) the committed oracles are defined against this format; (d) D9's version-controllable-YAML intent is better served by adopting the community format. The per-composition leaf-class **collapse** (§2.1) is a property of *how the oracle was generated*, gated in the harness (§4.5), not a general converter.

**The catalog is a separate, AID-owned artifact (canonical) — the plan references it by ID.** This matches HNP's *real* architecture: the plan **references** the catalog by foreign key (`PlanServerClass.server_device_type`, `PlanSwitchClass.device_type_extension`, `PlanServerConnection.transceiver_module_type` — `topology_plans.py:164,323,746`); the catalog itself lives in a separate store (NetBox DCIM), and `reference_data` in the diet/XOC YAML merely **seeds** it (`ingest.py:61-326` does `get_or_create` into `dcim.*`). So bundling `reference_data` into the file is a test-fixture *convenience*, not the architecture — and the plan body **already uses ID pointers** (`device_type_extension: sw_ds5000_scale_out_ext`, `transceiver_module_type: osfp_400g_dr4`). AID makes the separation canonical: the **catalog** (§4.2) is its own versioned artifact of independent objects (CRD-style — a switch object is a switch object, a server object is a server object), and a plan carries only **pointers** (server/switch **class** IDs + other catalog refs) plus topology intent. AID still ingests a real bundled `topology-plan.yaml` by **extracting its `reference_data` into the catalog** — a *lossless, deterministic relocation of identical objects* (IDs preserved → an equivalent pure-reference plan; the refs already exist), **not** a vocabulary converter, so the `training_*.yaml`-first oracle is unaffected. Canonical authoring is pure-reference; the ingest boundary accepts bundled. Plan refs **pin catalog identity + version/digest** (not a mutable friendly ID) so old plans and oracle fixtures stay reproducible (F0 guardrail).

**Plan schema = inputs (`spec`) + computed/expected (`status`) — double-duty as test documents.** Like a Kubernetes object (author `spec`, `status` is populated after running), an AID plan carries **input** fields plus an optional **status/expected** plane of computed values. A plan with inputs only is a valid input AID can ingest; the *same* plan with expected values populated is a self-checking **test oracle** — AID asserts its computed values match. This generalizes the real format's `expected.counts` (`topology-plan.yaml:872`). Line drawn: **scalar/summary computed values** (derived switch counts per class, totals, validation results, the `expected.counts`) live in the plan's status/expected; **bulky generated artifacts** (full `netbox_inventory.json`, wiring CRDs, the full BOM rows) stay separate output files — exactly XOC's own split (small `expected.counts` in the plan; big outputs as adjacent files).

### 4.2 A general, extensible object model — the catalog is its first consumer (R1, R4, R5)

This is the foundational addition, and it is deliberately **general, not BOM-specific**. The complete purchasable BOM (§2.6) is the *first motivating example* of a broader requirement the owner has stated: AID must model components fully and accurately for current **and future** features, many of which will add attributes, metadata, and nested objects to existing objects. A foundation that hard-codes today's fields would have to be re-cut for each new feature. So the substrate is a **general extensible object model**, and BOM/topology/wiring/inventory are *consumers* of it:

- **Objects are typed but open.** Every modelled thing — catalog item, server/switch class, zone, connection, port, … — is a typed object carrying an **open, namespaced attribute set**, not a fixed column list. A new feature adds attributes without re-foundationing the schema or migrating existing objects. The two attribute *planes* below (`calc_profile` / `purchase_profile`) are simply the first two attribute **namespaces**; more (power/thermal, lifecycle/EoL, cost/commercial, compliance, …) are added the same way.
- **Relationships and nesting are first-class and arbitrary.** Objects compose other objects to any depth via typed relationships (`component_slots`, `port_templates`, … and future relation kinds). An object may own as many nested objects as needed; depth and fan-out are bounded by the data, not the model.
- **Consumers are deterministic projections.** A feature (BOM, wiring, inventory, a future cost/power/thermal report) is a pure reducer/projection over the object graph (§4.4). Adding a consumer never changes the object model — only adds a projection. (This is exactly the schema-first-graph + generators/transformations pattern of §4.7.)

This generality costs little now (it is mostly *not* over-specifying fields) and is what lets AID grow without another foundation reset. The **component catalog** is the first consumer. AID owns it (it cannot read NetBox's seed state). A **catalog item** is one such typed object:

```
CatalogItem {                      # a typed object in the general model above
  id, kind,            # kind ∈ {server, switch, nic, dpu, transceiver, component, accessory,
                       #         warranty, software_support, assembly, onsite_service, …}  (open set)
  manufacturer, model, part_number (SKU), description, orderable,
  attributes: { <namespace>: { … } }   # open, namespaced attribute planes (extensible):
  calc_profile  { … }  # see (1) — namespace: drives topology calculation
  purchase_profile { … }# see (2) — namespace: drives the purchasable BOM
                       #            (future: power_profile, lifecycle, cost, … — same mechanism)
  port_templates[]     # see (4) — typed relationship: physical ports/cages
  component_slots[]    # see (3) — typed relationship: nested child objects (any depth)
  bom_line_templates[] # see (5) — physical + non-physical rows this item contributes
}
```

**Two layers of catalog object.** A `CatalogItem` is either:
- a **bare hardware *type*** — chassis, NIC, DPU, transceiver, component — declaring **capability only**: its `port_templates`/cages (with `allowed_transceivers`), `calc_profile`/`purchase_profile`, and `bom_line_templates`. Reusable across every design; **no** per-design selection baked in.
- a **configured *class*** (`server_class` / `switch_class`) — a composite that references hardware types via `component_slots` and **binds specific transceivers into specific NIC-port cages**, assembling a fully self-describing object with a complete, **context-free** BOM. Server/switch classes are first-class **reusable inventory** objects (a "static server inventory"): for a plan, a user **picks an existing class or defines a new one**, then references it from `topology.yaml`.

Because a class is a *configured composite over shared primitive types*, two classes that differ only by optic **share every base/component line and differ only in the one transceiver binding** — so the "different transceiver ⇒ different class" rule (gate in (4)) costs almost nothing and keeps each class independently orderable. (This is the recursive component graph D19 retains for the catalog — a `server_class` composes NIC types that compose cage→transceiver bindings.)

Provenance: HNP proves the *calc-plane* ingredients (switch `DeviceTypeExtension` roles/breakouts/native-speed `reference_data.py:65-129`; transceiver `Network Transceiver` profile + attrs `topology_plans.py:850-862`, `seed_catalog.py:10-71`); AID **adds the purchasing plane and owns the catalog** (HNP delegated it to NetBox, `reference_data.py:142-158`). `kind` extends HNP's implicit device/module roles with the owner's non-physical kinds (`real-server-bom.csv:4,12-15`).

**(1) `calc_profile`** — everything the topology kernel needs: declared ports/cages (via `port_templates`, below), max speeds, cage types, breakout support, switch role/profile (`hedgehog_role`, `hedgehog_profile_name`), and transceiver-compat attributes (cage/medium/connector/standard/reach/lanes/breakout — the HNP transceiver attribute set, `seed_catalog.py:10-71`). Mirrors HNP's `DeviceTypeExtension` + `ModuleType.attribute_data`.

**(2) `purchase_profile`** — everything procurement needs and the topology kernel must ignore: SKU/part number, procurement description, unit of measure, quantity rules, power/capacity (e.g. the `Total Capacity(GB)`/`Power(W)` columns of `real-server-bom.csv:1`), and arbitrary per-item attributes. **Two planes are required because the same item participates in both calculation and purchasing with different fields** (R1).

**(3) `component_slots[]`** — nested purchasable parts: `{slot_id, catalog_item_ref, quantity, required|optional, selection_constraints}`. This is how a `server` item owns chassis, GPU board, CPUs, memory, drives, NICs, DPU, **and** non-physical warranty/support/accessory/assembly/onsite items (`real-server-bom.csv:3-15`). HNP's `PlanServerNIC` (one NIC slot = one Module, `topology_plans.py:200-231`) is the topology-facing analogue; `component_slots` generalize it to *purchasable* nesting, including non-physical parts HNP never modeled.
> **Explicit modeling choice for repeated identical NICs (R4/R5):** the `8× CX-7` are modeled as **one `component_slot` with `quantity: 8`** referencing the single one-cage CX-7 catalog item — **not** a synthetic 8-port NIC, and not 8 hand-duplicated rows. Rationale: the catalog item stays a faithful 1× one-QSFP112-cage CX-7 (matching `AOC-CX766003N-SQ0`, `real-server-bom.csv:10`); the slot quantity drives both the BOM line (`8×`) and the generation of 8 distinct NIC instances/cages downstream. (HNP would instead require 8 named `PlanServerNIC` rows — `topology_plans.py:200-231`; AID's quantity-bearing slot is the cleaner faithful model.)

**(4) `port_templates[]` / cage templates (R4)** — the fixed-vs-pluggable distinction HNP lacks. Each catalog entry declares **capability only**: `{name, port_kind, max_speed_gbps, interface_type, cage_type?, requires_transceiver, allowed_transceivers[]}` with `port_kind ∈ {fixed_interface, transceiver_cage}`. (The *selected* transceiver is a plan-instance binding, not a catalog field — see the gate below; the only exception is a captive/fixed optic baked into the SKU.)
- A **`fixed_interface`** is a soldered port needing no optic — e.g. the BF3 **1000BASE-T BMC** port (`requires_transceiver=false`). HNP cannot express this: `populate_transceiver_bays.py:63-85` makes a cage for *every* InterfaceTemplate, and the seeded BF3 is two bare QSFP112 templates with no BMC (`seed_catalog.py:613-623`).
- A **`transceiver_cage`** is a pluggable slot that *can* be populated with a compatible transceiver — e.g. the CX-7's one QSFP112 @ 400G and the BF3's two QSFP112 @ 200G (`real-server-bom.csv:10-11`; `README.md:20-21`). This is the model object the GUI binds to for "select a transceiver to populate this cage," and it is what lets the BOM reducer add the per-cage transceiver as a required subcomponent (it is *not* a line in the flat CSV, `README.md:17-19`).
  > **Capability vs binding, across the two layers (devb gate + owner model).** A **bare hardware *type*'s** cage declares *capability only* — `cage_type`, `max_speed_gbps`, `requires_transceiver`, `allowed_transceivers[]` — and stays reusable across all designs (devb's concern: never bake a selection into the bare type). The **binding** — which transceiver populates which cage — is made on the **configured server/switch class**, **per NIC port**: a NIC commonly has multiple ports/cages that may attach to different zones, so selection (and connection) granularity is the **port**, not the NIC (`PlanServerConnection.nic` + `port_index`, `topology_plans.py:673,680`). That class is itself a reusable catalog object; the **plan** only references it. **Different transceiver selection ⇒ a distinct class**, even if nothing else differs — which keeps every class a complete, context-free, orderable BOM (and, per the two-layer note above, near-free since such classes share all non-optic lines). The only exception is a captive/fixed SKU optic, modelled as a `fixed_interface`.

**(5) `bom_line_templates[]` (R3)** — arbitrary rows a catalog item contributes to the purchasable BOM: `{category, catalog_item_ref?|inline SKU, quantity_per_instance, physical: bool, attributes}`. They cover **physical** (chassis, GPU board, CPU×2, memory×24, drives) and **non-physical** (warranty `EWCSC`, support `SVC-NVSTDSWSUP-3Y`, accessory `CBL-PWEX`, assembly `MC0037`, onsite `OSNBD3`) lines (`real-server-bom.csv:3-15`), and **scale linearly** with the count of the owning item (`README.md:23-28`). HNP has no equivalent — its BOM is generated-Module aggregation only (`bom_export.py:1-8,454-460`).

### 4.3 Corrected topology-planning model — **relational classes (over the catalog)**

For *topology* (unchanged from Rev 1, validated by review), replace the universal recursive `DeviceClass` *root* with the diet relational model (§1.1): `Plan → {ServerClass-ref, SwitchClass-ref, SwitchPortZone, ServerConnection, MeshLink, MCLAGDomain}`. A plan is **pointers + intent**: it references **server/switch classes by ID** (the configured catalog objects of §4.2) and carries the **plan-specific** parts — **`quantity`** per class (`topology_plans.py:171`) and **per-NIC-port connection intent**: each `(nic, port_index)` of a referenced class targets a switch port zone (`PlanServerConnection.nic` + `port_index` + `target_zone`, `topology_plans.py:673,680,711`), and a NIC's multiple ports may target different zones. Switch quantity is **derived** (`calculated_quantity`/`override_quantity`, `:344-356,577-589`). The transceiver in each NIC-port cage is a **class** property (§4.2); the real format specifies it at the **connection** level (`server_connections[].transceiver_module_type`, keyed by `(nic, port_index)`), which AID **resolves into the class** on ingest — server-optic ↔ switch-zone-optic compatibility stays a validated relationship (HNP's `transceiver_rules` does this today). AID adopts the *shape and semantics* in MoonBit/Rust/Go types — not Django/NetBox ORM (D1).

> The recursive *composite* is **dropped as the topology root** (D19) but **retained, corrected, for the catalog**: `component_slots` (§4.2-3) *is* a bounded component graph — a server item composes a DPU item that composes cage→transceiver items. The Rev 1 error was making that graph the topology entrypoint; the fix is to scope it to the catalog/BOM plane.

### 4.4 Deterministic BOM reducer — **full purchasable BOM + HNP projection** (R3/R5, keeps D6)

A single pure, plan-time reducer (no inventory DB write — preserves D6, `DECISIONS.md:105-115`) resolves the catalog + topology into **two outputs**:

1. **Full purchasable BOM** = for every instantiated server/switch instance: its own `bom_line_templates` + every required `component_slot`'s item line templates (recursively) + the **selected transceiver per populated cage** + all non-physical line templates — each **× instance count**. This is the union the owner specifies (`README.md:23-28`) and is validated against `real-server-bom.csv` (incl. 1×/2× scaling).
2. **HNP/XOC 19-column projection** = the *same resolved model*, **filtered** to HNP-compatible physical rows (base `server`/`switch`; `nic`/`dpu` modules; `server_transceiver`; `switch_transceiver`) and rendered into the 19-column shape (§2.2), reproducing HNP's section classification (`bom_export.py:454-460`) and the suppressed-cable-assembly footer. This is validated against the XOC `bom.csv`.

Because both outputs derive from one resolved model, they cannot drift; the projection is provably a subset of the full BOM. Plan-time derivation must still **equal the inventory-derived numbers** HNP would generate — a correctness obligation checked by the projection vs `bom.csv` and the counts vs `netbox_inventory.json`.

> **Implementation gate (devb re-review):** the HNP/XOC projection MUST be a *filtered projection over the same resolved object graph* — never a second, independently-counted BOM path. There is one resolver; both outputs are views of its result. (This is the single most important guard against the two BOMs silently diverging.)

### 4.5 Validation strategy — **two oracle layers + provenance gating** (R2/D20)

Replace the 3 toy fixtures with **two oracle layers**:

**Layer A — physical/topology subset (XOC/HNP).** For each XOC composition (`xoc-64 … xoc-1024`):

| AID output | Compared to | Comparison |
|---|---|---|
| BOM **projection** | `bom.csv` | exact 19-column row match incl. suppressed-count footer |
| connectivity map | `connectivity-map.csv` | set-equality of cable endpoint tuples (order-insensitive) |
| inventory counts | `netbox_inventory.json .metadata.counts` | exact `devices/modules/interfaces/cables` |
| wiring CRDs | `wiring/*.yaml` + `hhfab validate` | `hhfab validate` passes (existing CI harness) + CRD-kind counts |
| self-check | `expected.counts` | exact `server_classes/switch_classes/connections` |

**Layer B — full purchasable BOM (owner artifact).** AID's **full** BOM for the corresponding server is compared to `docs/requirements/real-server-bom.csv` — exact line set incl. non-physical rows and nested CX-7/BF3, **plus explicit 1× and 2× server-quantity scaling tests** (every quantity must scale linearly, `README.md:23-28`). This layer exercises R1–R5 that Layer A cannot.

**Provenance is a hard gate, not a caveat (§2.1).** The committed XOC outputs were generated from the **diet/training** form, which for `xoc-64` *collapses* `scale_out_leaf`+`soc_storage_leaf` (4 switch_classes) into `soc_storage_scale_out_leaf` (3) — authored `topology-plan.yaml:443-463` vs generated `training_xoc64_1xopg64_mesh_conv_ro.yaml:438-455`, and `bom.csv:7-9` uses the collapsed class. **Milestone gating:** the *first* oracle milestone targets `generated/inputs/training_*.yaml` **exactly** (1:1 with the committed outputs); the authored `topology-plan.yaml → training` normalization is a **separate, explicit milestone with expected mapping tests** — never assumed. The HNP `test_cases/` (55 plans) provide additional developer-side cross-checks via `expected.counts`/`GenerationState` (`assertions.py:88-116`), runnable only in HNP (Python), never imported (D1/D12).

### 4.6 Component disposition

Reuse the schema-agnostic infrastructure (§3.2). Rework orchestration/CLI/UI/adapters against the corrected IR + the new catalog/BOM model. Rewrite the kernel decode/type layer to the diet model; re-derive the calc kernel (switch-count, port allocation, mesh, transceiver selection) against the HNP engine's actual algorithms (§1.2). The `bom-adapter` is reworked to render the **two** BOM outputs (full + projection); add two new outputs (connectivity-map, netbox_inventory). Discard the fixtures.

### 4.7 Design influences (external, inspirational — not a dependency)

The catalog-graph + template-driven-BOM direction is **proven design, not novel**. OpsMill's Infrahub is an independent precedent worth borrowing *concepts* from (we add **no** dependency; D1 stands):
- **Schema-first graph source of truth** with arbitrary object types and relationships — corroborates modelling the catalog as a component graph rather than a fixed device tree.
- **"Model all your infrastructure, not just network devices"** — Infrahub explicitly models non-device business context; exactly the warranty/support/assembly/onsite lines the owner requires.
- **Generators** (templates + inputs → objects) and **Transformations** (data → artifacts via templates) — the conceptual shape of our `bom_line_templates` + the BOM reducer (resolved model → full BOM + HNP-projection artifacts). Two attribute planes (`calc_profile`/`purchase_profile`) is the same separation-of-concerns Infrahub gets from flexible per-object attributes + transformation queries.

We adopt the *ideas* (graph catalog, template→artifact reduction, multi-plane attributes), implemented natively in MoonBit/Rust/Go.

### 4.8 Honest caveats

- **BOM is inventory-derived in HNP** (`bom_export.py:1-8` counts generated Modules), whereas D6 makes AID's BOM plan-time. The redesign keeps plan-time derivation **but the projection must reproduce the inventory-derived numbers** — plan-time math must equal generation counts (checked vs `bom.csv` + `netbox_inventory.json`).
- **The full BOM has no public composite oracle beyond `real-server-bom.csv`** (one server type). Layer B proves the *mechanism* (nesting, non-physical, scaling) on a real example; broadening to more server SKUs is future catalog work, flagged honestly.
- **`topology-plan.yaml` ≠ oracle input for every composition** (the collapse) — never assume 1:1 without checking `translation-notes.md` per composition (§4.5 gate).

---

## 5. Phased re-implementation plan (high-level; implementation gated on approval)

Smallest real composition first; never frankenstein. Each phase is its own RED→GREEN with review gates.

- **Phase F0 — Object substrate + schema + catalog + model of record.** Define the **general extensible object substrate first** (§4.2: typed objects with open, namespaced attribute sets + arbitrary typed nested relationships) so later features extend by adding attributes/relations/projections. On it: adopt the diet/topology-plan JSON Schema **with a `spec`/`status` (`expected`) plane** (D21); define the **separate, AID-owned catalog artifact** (referenced by plans via ID; ingest splits a bundled real file) and the **two-layer** model — bare hardware *types* + configured **server/switch classes** (reusable inventory; per-NIC-port transceiver binding; distinct class per optic) — with `calc_profile`/`purchase_profile` namespaces, `bom_line_templates`, `component_slots` (incl. the quantity-bearing NIC slot), and `PortTemplate`/`CageTemplate` (`fixed_interface` vs `transceiver_cage`). Define the relational topology types (plan = class refs + `quantity` + per-NIC-port connection intent). Land decision records **D18/D19/D20**. Stand up both oracle layers (`xoc-64` training form for Layer A; `real-server-bom.csv` for Layer B) and the comparison harness skeleton (failing). No calc yet. **The catalog + planes + line-templates + slots + port/cage model are foundational here, not deferred.**
  > **Implementation gate (devb re-review): F0 must define validation contracts**, not just shapes. Per object/namespace/relation kind: **stable IDs**; **required fields per projection** (which attributes a consumer like the BOM/HNP-projection demands); **quantity semantics** (how `quantity`/`quantity_per_instance` compose down a nesting chain); **acyclicity** of `component_slots` (or explicit, tested cycle rejection); and **clear validation errors**. The "open attributes" generality must not become "anything goes" — each namespace/relation declares its contract.
  > **Non-blocking F0 guardrails (devb Rev 3 reconfirm) — keep explicit in implementation tickets:**
  > 1. **Pin catalog identity.** A plan's catalog refs pin **identity + version/digest**, not just a mutable friendly ID, so old plans and oracle fixtures stay reproducible (§4.1).
  > 2. **Deterministic, lossless bundled-file ingest.** Extracting `reference_data` out of a bundled real `topology-plan.yaml` must **round-trip deterministically** — IDs preserved, yielding an equivalent pure-reference plan (§4.1).
  > 3. **`status`/`expected` never drives production calculation.** It is ignored except in an explicit validation/self-check mode that compares expected vs computed (D21).
  > 4. **Deterministic `ports_per_connection > 1` expansion.** Expanding a multi-port connection into per-port cage bindings must be deterministic and validated against the configured class (§4.3).
- **Phase F1 — Ingest + catalog load.** Parse the real `training_*.yaml`/`topology-plan.yaml` into the topology model; load the AID catalog (incl. `server_nics` join, non-physical kinds, port/cage templates); reproduce `expected.counts`. Oracle: `xoc-64` self-check.
- **Phase F2 — Calculation kernel (re-derived).** Switch-count derivation, zone port allocation (comma-list `port_spec`, breakouts, strategies), mesh links, distribution (incl. rail-optimized), per-cage transceiver selection. Oracle: `xoc-64` device/interface/cable counts vs `netbox_inventory.json .metadata.counts`. Re-establish `moon prove` goals for the *real* invariants.
- **Phase F3 — BOM reducer (full + projection) + connectivity-map.** The deterministic reducer (§4.4) emitting **(a) the full purchasable BOM** and **(b) the HNP 19-column projection**, plus connectivity-map. Oracles: exact `bom.csv` (projection) + `connectivity-map.csv` for `xoc-64` **and** `real-server-bom.csv` for the full BOM **incl. 1×/2× linear-scaling tests**.
- **Phase F4 — Inventory + wiring + hhfab.** `netbox_inventory.json`; hhfab CRD generation; `hhfab validate` passes. Oracle: wiring + counts for `xoc-64` (both fabrics).
- **Phase F5 — Scale-out.** Add `xoc-128/256/512/1024` and clos/dual-plane/SH/RO/liquid/storage variants; close the authored `topology-plan.yaml → training` normalization as its **own gated milestone with mapping tests** (§4.5). Each new composition is a new oracle row.
- **Phase F6 — Surfaces.** Retarget CLI/REST/UI to the corrected model; the GUI must author a real plan **and select transceivers from the catalog to populate cages** (§4.2-4).

---

## 6. Draft decision records (D18–D21; supersede D9/D10/D13)

> Drafts for review; finalized into `DECISIONS.md` only after approval.

### D18 — Real topology-plan shape canonical for topology intent **+** an AID-owned NetBox-independent catalog (supersedes D9, D10)
**Decision.** AID's canonical *topology* input is the published OCP/diet `topology-plan.yaml` shape (`meta, reference_data, plan, switch_classes, switch_port_zones, server_classes, server_nics, server_connections, expected`), validated against a JSON Schema describing that real format — AID does **not** invent a topology vocabulary. **Additionally**, AID owns a NetBox-independent **component catalog** as a **separate, versioned artifact** that the plan **references by ID** (CRD-style independent objects) — because HNP delegated the catalog to NetBox (`reference_data.py:142-158`; the plan FK-references it, `topology_plans.py:164,323,746`) and AID has no NetBox. AID ingests a real *bundled* `topology-plan.yaml` by **losslessly extracting its `reference_data` into the catalog** (the plan body already uses ID refs); canonical authoring is pure-reference. This is **not a converter** — the topology shape is adopted as-is, the catalog is a separate layer AID owns to carry hardware/SKU/component identity and emit a purchasable BOM. **The plan schema is also expanded to `spec` (inputs) + `status`/`expected` (computed values)**, so one document is both a valid input and a self-checking test oracle (generalizing `expected.counts`); see D21. **Rationale.** The invented `topology-plan-v1.json` shares zero top-level keys with the real input and cannot parse a single reference file; the real format and the diet engine format are identical; committed oracles exist only for the real format. D9's version-controllable-YAML intent is preserved; D10's "publish an AID schema" becomes "adopt + document the community topology schema, and publish the AID catalog + plan-status schema."

### D19 — Relational topology classes **plus** a component-graph catalog (supersedes D13)
**Decision.** Two halves. **(Topology)** AID's topology model is the diet relational model — `ServerClass` (+`ServerNIC` join), `SwitchClass`, `SwitchPortZone`, `ServerConnection`, `MeshLink`, `MCLAGDomain`, with switch quantities derived (`calculated_quantity`/`override_quantity`). The universal recursive `DeviceClass` is **dropped as the topology root**. **(Catalog/BOM)** AID *retains a corrected recursive/component composite for the catalog*, expressed as instances of a **general extensible object model** (§4.2): typed objects with **open, namespaced attribute sets** + **arbitrary typed nested relationships**. The catalog has **two layers**: **bare hardware *types*** (chassis/NIC/DPU/transceiver/component — capability only, reusable) and **configured *classes*** (`server_class`/`switch_class` — composites that bind specific transceivers into specific **NIC-port** cages and assemble components, as **reusable inventory objects** with complete, context-free BOMs). The **binding lives on the class, per NIC port**, never on the bare type (honors the devb capability-vs-binding gate); **a different transceiver selection ⇒ a distinct class**. A plan references classes by ID and adds only `quantity` + per-port connection intent. The `CatalogItem` (with `component_slots`, `port_templates`/cage templates, `bom_line_templates`, and the `calc_profile`/`purchase_profile` attribute namespaces) is the first consumer; **future features extend the model by adding attribute namespaces, relation kinds, and projections — never by re-foundationing** (the owner intends to keep adding object attributes/metadata). **Rationale.** D13's "single universal recursive `DeviceClass`, no `ServerClass`/`ServerNIC`" is contradicted *as a topology root* by the authoritative relational model (NIC-first connections, switch-count derivation, zone allocation — `topology_plans.py`). But a bounded component graph is exactly right *for the catalog*: it is the only way to express the owner's nested purchasable parts, non-physical line items, and per-cage transceivers (`real-server-bom.csv:3-15`; `README.md:17-28`) that HNP's Module-aggregation BOM cannot (`bom_export.py:1-8,454-460`). Plan-time BOM derivation (D6) is retained via the §4.4 reducer.

### D20 — Two oracle layers: XOC/HNP physical subset **+** the owner full-purchasable-BOM artifact (supersedes the toy-fixture strategy)
**Decision.** The behavioral contract has two layers. **Layer A (physical/topology subset):** the XOC composition matrix (`xoc-64 … xoc-1024`) — AID reproduces the committed `bom.csv` (as the §4.4 **projection**), `connectivity-map.csv`, `netbox_inventory.json` counts, `wiring/*.yaml` (`hhfab validate`), and `expected.counts`. **Layer B (full purchasable BOM):** `docs/requirements/real-server-bom.csv` — AID's full BOM reproduces the complete line set (incl. non-physical and nested CX-7/BF3 + per-cage transceivers) **with 1×/2× linear-scaling tests**. The hand-authored `clos-small`/`mesh-two-switch`/`switch-bom` fixtures are removed. **Provenance is a hard gate:** the first oracle milestone targets `generated/inputs/training_*.yaml` exactly (1:1 with the committed outputs, which use the *collapsed* class set — `bom.csv:7-9`); the authored `topology-plan.yaml → training` normalization is a separate gated milestone with mapping tests. **Rationale.** The toy fixtures admit they "do not reproduce real device/cable/switch counts" (`tests/fixtures/README.md:92-93`); Layer A validates HNP-compatible behavior, but only Layer B exercises R1–R5 (catalog, planes, non-physical lines, nesting, scaling).

### D21 — Catalog is a separate artifact; plan schema is `spec` + `status`/`expected` (double-duty test documents)
**Decision.** **(a) Catalog separation.** The component catalog (§4.2) is a **separate, versioned, AID-owned artifact** of independent objects; topology plans carry only **pointers** (server/switch **class** IDs + catalog refs) plus topology intent. Server/switch **classes** are reusable catalog/inventory objects (two-layer model, D19): a plan picks an existing class or defines a new one. AID ingests a real *bundled* `topology-plan.yaml` by losslessly extracting `reference_data` into the catalog (not a converter). **(b) Plan spec/status.** An AID plan has an **input (`spec`)** plane and an optional **`status`/`expected`** plane of computed values (Kubernetes-style). Inputs-only ⇒ valid input; inputs + populated expected ⇒ a self-checking **test oracle**. Scalar/summary computed values (derived switch counts, totals, validation, `expected.counts`) live in the plan; bulky outputs (full inventory, wiring CRDs, full BOM rows) stay separate artifacts. **Rationale.** HNP's real architecture already separates the catalog (NetBox DCIM) from the plan and references it by FK (`topology_plans.py:164,323,746`); `reference_data` in the YAML is seed convenience (`ingest.py:61-326`). Separation gives CRD-style independent, reusable objects (a switch object is a switch object); the spec/status plane generalizes the real format's `expected.counts` (`topology-plan.yaml:872`) so the same document authors input *and* asserts output — strengthening D20's oracle story.

---

## 7. Evidence index

- **HNP model:** `gitignored/refs/hnp/netbox_hedgehog/models/topology_planning/{topology_plans,reference_data,port_zones,naming,generation}.py`
- **HNP engine:** `gitignored/refs/hnp/netbox_hedgehog/services/{device_generator,port_allocator,port_specification,transceiver_rules,bom_export,inventory_export,yaml_generator,preflight}.py`
- **HNP diet harness:** `gitignored/refs/hnp/netbox_hedgehog/test_cases/{schema,ingest,loader,runner,assertions}.py`; `management/commands/apply_diet_test_case.py`
- **XOC input/output:** `gitignored/refs/xoc/compositions/xoc/xoc-64/1x-OPG-64/mesh-conv-ro--.../{topology-plan.yaml,bom.csv,connectivity-map.csv,netbox_inventory.json,wiring/*.yaml,diagrams/hhfab/*.log,generated/inputs/{training_*.yaml,translation-notes.md}}`
- **HNP catalog/BOM evidence (Rev 2):** `gitignored/refs/hnp/netbox_hedgehog/models/topology_planning/reference_data.py:4-13,142-158` (catalog delegated to NetBox); `services/bom_export.py:1-8,167-172,454-460` (inventory-Module-based, 3-section); `services/device_generator.py:399-411,1383-1450,1470-1539` (nested cage placement); `models/topology_planning/port_zones.py:76-90`; `seed_catalog.py:10-71,613-623` (transceiver attrs; BF3 = two QSFP112, no BMC); `management/commands/populate_transceiver_bays.py:63-85` (one cage per InterfaceTemplate)
- **Owner requirements (Rev 2):** `main:docs/requirements/README.md:12-28`, `main:docs/requirements/real-server-bom.csv:3-15` (the full purchasable BOM, non-physical lines, nested CX-7/BF3)
- **Current AID:** `schema/topology-plan-v1.json`, `DOMAIN_MODEL.md`, `ALGORITHMS.md`, `DECISIONS.md` (D1/D2/D6/D9/D10/D12/D13/D16/D17), `tests/fixtures/`, `internal/`, `kernel/`, `hhfab-adapter/`, `bom-adapter/`, `ui/`
- **Spike (this ticket):** parse confirmation that XOC `topology-plan.yaml` ≡ HNP `training_*.yaml` top-level schema, and that current AID keys are disjoint (run under `gitignored/`).
- **Design influences (external, no dependency):** OpsMill Infrahub — schema-first graph source of truth, Generators (templates+inputs→objects), Transformations (data→artifacts): https://docs.infrahub.app/overview/concepts, https://opsmill.com/blog/modeling-all-your-infrastructure-not-just-network-devices/, https://opsmill.com/blog/infrahub-generators-faqs/
