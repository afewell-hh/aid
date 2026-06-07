# HNP Reference — Developer Context Only

This document is for AID developers only. It documents the relationship between AID and
the HNP (Hedgehog NetBox Plugin) internal tool from which AID's algorithms and behavioral
contract are derived.

This document is NOT for AID users, documentation, or OCP contributions. AID is an
independent project. HNP was never released and has no users. Do not reference HNP in
any user-facing AID material.

---

## What HNP Provides

HNP contains:

1. **Validated algorithms**: switch quantity calculation, port allocation, rail-optimized
   distribution, mesh link enumeration, oversubscription detection. These have been
   validated against real AI cluster topology designs.

2. **Reference architecture fixture library**: YAML topology plan files for all major
   AI cluster variants (OPG-64 through XOC-1024, Clos RO/SH, dual-plane, mesh-converged).
   These are AID's behavioral specification — if AID produces different output for the
   same fixture input, it is wrong.

3. **Expected output counts**: for each fixture, the expected device count, interface
   count, cable count, and per-fabric switch counts. These are AID's acceptance test
   baselines.

4. **Transceiver compatibility rules**: the rule engine structure and approved-pair
   registry. These rules reflect real hardware vendor specifications.

5. **hhfab wiring YAML format**: the output schema that AID's hhfab-adapter must produce.
   HNP's yaml_generator.py is the reference implementation.

---

## HNP GitHub Location

HNP is at: `https://github.com/afewell-hh/hh-netbox-plugin`

Branch for current stable: `main`

Key files for AID reference:

| HNP file | AID relevance |
|----------|--------------|
| `netbox_hedgehog/services/device_generator.py` | Algorithm reference: switch selection, Clos wiring, mesh, rail distribution |
| `netbox_hedgehog/services/yaml_generator.py` | Output format reference: hhfab CRD schema |
| `netbox_hedgehog/services/transceiver_rules.py` | Transceiver rule engine (already pure Python, direct port candidate) |
| `netbox_hedgehog/services/port_allocator.py` | Port allocation algorithm reference |
| `netbox_hedgehog/services/bom_export.py` | BOM structure reference (note: inventory-based; AID improves this) |
| `netbox_hedgehog/test_cases/training_opg128_clos_ro.yaml` | Reference fixture: OPG-128 Clos RO |
| `netbox_hedgehog/test_cases/training_xoc64_1xopg64_mesh_conv_ro.yaml` | Reference fixture: XOC-64 mesh-converged |
| `netbox_hedgehog/test_cases/training_xoc128_2xopg64_mesh_conv_ro.yaml` | Reference fixture: XOC-128 mesh-converged (A/B split) |
| `netbox_hedgehog/test_cases/training_opg256_dual_plane.yaml` | Reference fixture: dual-plane |
| `netbox_hedgehog/models/topology_planning/` | Domain model reference (Django version) |

---

## Behavioral Contract Extraction

To extract AID's acceptance test baselines from HNP:

1. Clone HNP and set up the local NetBox dev environment (see HNP `AGENTS.md`)
2. For each fixture in `netbox_hedgehog/test_cases/`, run:
   ```bash
   cd /path/to/netbox-docker
   docker compose exec -T netbox python manage.py apply_diet_test_case --case <case_id> --clean
   docker compose exec -T netbox python manage.py generate_devices <plan_id>
   ```
3. Record device_count, interface_count, cable_count from GenerationState
4. Record switch counts per switch class
5. Export wiring YAML and run `hhfab validate` — capture the pass/fail per fabric

These become the expected outputs in `aid/tests/fixtures/<case_id>/expected.json`.

---

## Key Algorithmic Differences (AID Improves on HNP)

AID must match HNP's output for valid plans. AID additionally:

1. **Reports oversubscription ratio**: HNP does not compute or report this. AID adds it
   as `FabricSummary.oversubscription_ratio`.

2. **Plan-derived BOM**: HNP's BOM reads from NetBox inventory (post-generation).
   AID derives BOM from the plan model at plan time. BOM totals must match HNP's
   inventory-based BOM for generated plans.

3. **Pre-flight feasibility check**: HNP checks zone capacity mid-generation and fails
   with a transaction rollback. AID checks before any topology is built and returns a
   `ValidationResult` with per-zone deficit details.

4. **ServerConnection owned by ServerNIC**: HNP has ServerConnection owned by ServerClass
   (with a reference FK to ServerNIC). AID corrects the ownership hierarchy.
   The behavioral output is identical; only the model structure changes.

5. **No NetBox ORM**: AID produces the same topology without writing to a database first.
   The YAML export path no longer requires a prior DCIM generation step.

---

## Known HNP Limitations That AID Should NOT Inherit

- Mixed-speed-within-zone under-counting (issue #319, deferred in HNP)
- Mixed rail-optimized + alternating connections to same switch class: silently wrong in HNP
- Mesh constraint checked at generation time (not plan-edit time) in HNP
- No global port-balance invariant check in HNP

These are documented in `ALGORITHMS.md` as correctness properties that AID must verify.
