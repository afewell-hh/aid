# Oracle snapshot — xoc-128 / 2×OPG-64 / mesh-conv-ro (Issue #62, F5)

**Do not hand-edit.** This is a vendored, verbatim copy of HNP's generator output
for the xoc-128 `mesh-conv-ro` composition. It is the F5 behavioral oracle (D24
mesh scale-out; F1–F4 reproduction targets). Regenerate by re-copying from the
reference snapshot, never by editing to make a test pass — if a tripwire or a
comparison fails, investigate the snapshot, do not adjust the expectation.

## Provenance

Source (gitignored reference clone):
`gitignored/refs/xoc/compositions/xoc/xoc-128/2x-opg-64/mesh-conv-ro--cx7-1x400g--bf3-2x200g--storage-conv-2x200g--inb-2x25g/`

| File here | Source | Oracle role |
|---|---|---|
| `training.yaml` | `generated/inputs/training_xoc128_2xopg64_mesh_conv_ro.yaml` | ingested plan (spec + `expected.counts`); input to F2/F3/F4 |
| `bom.csv` | `bom.csv` | F2 quantities + F3 byte-exact projection (23 rows) |
| `wiring/wiring-*.yaml` | `wiring/` | F4 structural equivalence + `hhfab validate` (5 managed fabrics) |
| `netbox_inventory.json` | `netbox_inventory.json` | **parity only — NOT an oracle** (D22) |
| `connectivity-map.csv` | `connectivity-map.csv` | **parity only — NOT an oracle** (D22) |
| `translation-notes.md` | `generated/inputs/translation-notes.md` | HNP translation provenance |
| `optic-overlay.yaml` | **AID-authored** (not from the snapshot) | per-composition AID catalog overlay (cols 2/5/7–19); see its own header |

## Shape (verified)

- 8 server classes (`compute_xpu_a/b`, `storage_srv_a/b`, `metadata_srv_a/b`,
  `hh_gateway`, `hh_controller`), Σ quantity = **34 servers**.
- 6 switch classes (`scale_out_leaf_a/b`, `soc_storage_leaf_a/b`, `inb_mgmt_leaf`,
  `oob_leaf`), **all `override_quantity: 2`** (override-only — **no derivation**;
  derivation stays Clos-phase-tracked, D24).
- 5 `fabric_class: managed` fabrics → `scale-out-a`, `scale-out-b`,
  `soc-storage-a`, `soc-storage-b`, `inb-mgmt`; `oob-mgmt` is unmanaged.
- `expected.counts: {server_classes: 8, switch_classes: 6, connections: 38}`.
- `bom.csv`: 23 rows (header + 8 server + 6 switch + 5 nic + 2 `server_transceiver`
  + footer `# suppressed_switch_cable_assembly_count,0`; **no `switch_transceiver`
  rows**).
- All 5 committed `diagrams/hhfab/hhfab_validate_*.log` (in the source snapshot)
  report `Fabricator config and wiring are valid` (hhfab v0.45.5).
