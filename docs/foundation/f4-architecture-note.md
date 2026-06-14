# F4 Wiring renderer — architecture note (Issue #60)

**Status:** proposed — awaiting lead (devc) + devb sign-off before RED.
**Author:** deva. **Scope:** D22 (NetBox deferred → **wiring only**, no `netbox_inventory.json`).

F4 renders the F2 IR (`calc.CalcOutput`) + topology plan + catalog into hhfab
wiring CRDs (`wiring.githedgehog.com/v1beta1` + `vpc.githedgehog.com/v1beta1`)
that **`hhfab validate` accepts** and that are **structurally equivalent** to the
committed `tests/oracle/xoc-64-mesh-conv-ro/wiring/*.yaml`. Wiring is a **pure
transform** of the F2 IR + catalog — no new topology calc (§4.x, Issue #60
constraint).

Everything below is grounded in the live IR and the committed oracle (verified —
see §6 Evidence).

---

## 1. Where the renderer lives — **Go `internal/wiring`** (recommended)

A new Go package `internal/wiring`, mirroring F3's `internal/bom`:

```
Render(plan *topology.Plan, cat *catalog.Catalog, calcOut *calc.CalcOutput) ([]Doc, error)
// Doc{ Fabric string; YAML []byte } — one managed fabric per Doc.
```

**Rationale (recommend Go, not the Rust adapter):**

1. **Mirrors the established F3 pattern.** `internal/bom.Resolve(plan, cat,
   calcOut)` already consumes exactly these three Go-resident inputs and renders
   views of them. F4 is the same shape — base-device + endpoint walk → YAML
   instead of CSV. Same import graph, same test harness, same review surface.
2. **No proof obligation.** Wiring is rendering, not proved arithmetic (the
   proved invariants — switch_count, allocation, fleet scaling — already live in
   the F2/F3 kernel cores). MoonBit/Rust-over-WASM buys nothing here; it only
   adds a boundary to cross.
3. **The Rust `hhfab-adapter` (D3) is pre-rebuild salvage.** It is bound to the
   *old* invented kernel schema and the old D16 `topology-ir` envelope
   (`internal/orchestrate.ExportWiring` → `components.Hhfab()`), neither of which
   carries the rebuilt F2 IR. Reworking it would mean re-plumbing the D16
   boundary to ship the corrected endpoints + per-zone breakout labels + mesh
   ports across WASM — strictly more work than reading the Go IR directly, for a
   pure string transform. **We keep the Rust adapter's CRD struct shapes as
   reference** (`hhfab-adapter/src/crds.rs`: `unbundled.link.{server,switch}`,
   `mesh.links[].{leaf1,leaf2}`, `Switch.spec{boot,profile,role}` with **ecmp and
   redundancy deliberately omitted**) — those shapes are confirmed against the
   committed oracle and `hhfab validate`.
4. **D16 untouched.** No new boundary types; no kernel change. (Consistent with
   the F3 note §7.4 "additive accessor over the same F2 output.")

The legacy `internal/orchestrate` golden path (Rust adapter + old kernel) is left
**as-is** — not extended, not deleted — for this phase.

---

## 2. IR → CRD mapping

### 2.1 Inputs and per-fabric grouping

`training.yaml` is the bundled input; convergence already happened at translation
time (the class is `soc_storage_scale_out_leaf`, **not** AID-merged from
`scale_out_leaf` + `soc_storage_leaf`). So **F4 does no convergence** — it speaks
the IR's class ids directly.

One wiring document **per managed switch class** (`fabric_class: managed`):

| Switch class | fabric_class | Doc | qty |
|---|---|---|---|
| `soc_storage_scale_out_leaf` | managed | `wiring-soc-storage-scale-out.yaml` | 2 |
| `inb_mgmt_leaf` | managed | `wiring-inb-mgmt.yaml` | 1 |
| `oob_leaf` | **unmanaged** | *(excluded)* | 1 |

A Doc contains the endpoints/switches/servers whose `SwitchClassID` is that class.
`oob_leaf` is unmanaged → no wiring (matches the committed oracle: only two files).

### 2.2 Device-name normalization (§2.5)

- **Switch device name** = `hyphenate(switch_class_id) + "-" + pad2(switchIndex+1)`
  → `soc_storage_scale_out_leaf` → `soc-storage-scale-out-leaf-01` / `-02`.
- **Server device name** = `hyphenate(server_class_id) + "-" + pad3(serverIndex+1)`
  → `compute_xpu` → `compute-xpu-001`. (`metadata_srv`→`metadata-srv-001`, …)
- **Port suffixes preserve underscores**: server port =
  `{server_device}/{nic_slot_id}-{nic_iface_name}` → `compute-xpu-001/scale_out-so0`
  (the `scale_out` slot keeps its underscore; `so0` is the NIC's
  `PortTemplates[portIndex].Name`).
- Switch port = `{switch_device}/{PortSlot.Name}` → `…-leaf-01/E1/1/1` (the IR's
  `PortSlot.Name` already encodes `E1/{port}` or `E1/{port}/{lane}`).

### 2.3 CRD kinds

| Kind / apiVersion | How rendered |
|---|---|
| `VLANNamespace` (wiring) | constant: `ranges: [{from: 1000, to: 2999}]`, name/ns `default`. One per Doc. |
| `IPv4Namespace` (vpc) | constant: `subnets: [10.0.0.0/16]`. One per Doc. |
| `Switch` (wiring) | one per switch instance (`switchIndex` 0..qty-1). `spec.role` = `hedgehog_role`; `spec.profile` = catalog profile (`celestica-ds5000`/`-ds2000`); `spec.boot.mac` (§2.4); `portBreakouts`/`portSpeeds` (§2.5). **No `ecmp`, no `redundancy`** (xoc-64 has `mclag_pair: false`, no MCLAG domain). |
| `Server` (wiring) | one per distinct server instance that has ≥1 endpoint on this fabric. `spec: {}`. |
| `Connection` (wiring) | server→switch + mesh (§2.6). |

### 2.4 `boot.mac` — **verified formula**

`02` ⊕ `SHA256(underscore_device_name)[1:6]`, where `underscore_device_name` =
`{switch_class_id}-{pad2(idx+1)}` (the **inventory** form, underscores intact).
Verified against the oracle:

- `soc_storage_scale_out_leaf-01` → `02:d1:30:5d:84:0c` ✓
- `soc_storage_scale_out_leaf-02` → `02:b7:11:db:8a:74` ✓
- `inb_mgmt_leaf-01` → `02:95:80:2f:70:b5` ✓

(Structural equivalence does **not** require byte-identical MACs, but reproducing
them exactly is free and makes the output faithful.)

### 2.5 `portBreakouts` vs `portSpeeds`

Union over **all zones** of the switch class. For each zone, for each physical
port in `port_spec` (the diet grammar: comma-list + `A-B` + `A-B:step`), resolve
the zone's `breakout_option` from catalog `reference_data.breakout_options`:

- `from_speed >= 100` → `portBreakouts["E1/{port}"] = breakout_id` with `g→G`
  (`1x800g`→`1x800G`, `2x400g`→`2x400G`, `4x200g`→`4x200G`).
- `from_speed < 100` → `portSpeeds["E1/{port}"] = "{logical_speed}G"`
  (`b_1x25`→`25G`).

Emit whichever map is non-empty. ds5000 → `portBreakouts` only; ds2000 →
`portSpeeds` only — matching the oracle exactly (uplink + server + mesh zones
union to the committed 63-entry breakout map; the shared uplink/mesh ports
dedupe). Mesh-zone ports therefore appear in `portBreakouts` (`1x800G`) even
though they carry no server endpoint.

### 2.6 `Connection` variants

- **server→switch (`unbundled`)** — one per F2 endpoint. xoc-64's
  `hedgehog_conn_type` is `unbundled` for every server connection.
  ```yaml
  spec:
    unbundled:
      link:
        server: { port: compute-xpu-001/scale_out-so0 }
        switch: { port: soc-storage-scale-out-leaf-01/E1/1/1 }
  ```
  `distribution: rail-optimized` is **already baked into the endpoints** (which
  server instance lands on which leaf/port) — F4 does not re-derive it; it is
  order-insensitive over `calcOut.Endpoints`.
- **mesh** — F2 defers mesh-link pairing to F4 (calc.go §note; mesh ports are not
  in `Endpoints`). F4 enumerates the `zone_type: mesh` zone's `port_spec`
  (`26,28`) and, for the 2-switch mesh, pairs the *same* port on leaf-01↔leaf-02:
  ```yaml
  spec:
    mesh:
      links:
      - leaf1: { port: …-leaf-01/E1/26 }
        leaf2: { port: …-leaf-02/E1/26 }
      - leaf1: { port: …-leaf-01/E1/28 }
        leaf2: { port: …-leaf-02/E1/28 }
  ```
  `leaf1`/`leaf2` ordered by device name (alphabetical). For N>2 mesh the pairing
  generalizes to all unordered switch pairs; **xoc-64 is N=2** and that is the
  acceptance target — the N>2 case is noted, not the bar.
- **Designed-but-not-exercised by xoc-64:** `bundled`, `mclag`, `eslag`
  (server→redundant switches), `mclagDomain` (peer/session), `fabric`
  (leaf↔spine). F4 will route on `hedgehog_conn_type` / zone_type + redundancy so
  the structure is present, but only `unbundled` + `mesh` have a test oracle here.
  No `fabric`/spine in xoc-64 (mesh topology, no spines).

### 2.7 `Connection.metadata.name`

HNP rule (reference `yaml_generator.py`): sanitize (lowercase, non-`[a-z0-9-]`→`-`,
collapse/trim `-`), join, **truncate to 63 chars and strip a trailing `-`**:
- unbundled: `{server}-{nic_slot}-{iface}--unbundled--{switch}` → trunc63.
- mesh: `mesh-{leaf1}-{leaf2}` → trunc63.

Names are **not** part of the structural-equivalence set (that keys on endpoint
tuples), but they must be unique + DNS-label valid for `hhfab validate`. The
trunc-63 rule reproduces the committed names and keeps them unique for this
dataset.

---

## 3. What "equivalent" means (the acceptance definition)

**(A) Hard gate — `hhfab validate` passes.** For each managed fabric: `hhfab init
--dev` → write the rendered doc to `include/wiring.yaml` → `hhfab validate`
exits 0 with "Fabricator config and wiring are valid". Reuses the existing
harness pattern in `internal/orchestrate/golden_test.go` (`hhfabValidate`). Local
`hhfab v0.43.1` already accepts **both committed oracle files** (verified, §6) —
so the bar is "AID's render validates", not "a newer hhfab is needed".

**(B) Structural equivalence vs committed `wiring/*.yaml`** (semantic, not
byte-identical — naming/ordering may differ):

1. **CRD-kind counts** per fabric:
   | fabric | Connection | Server | Switch | VLANNamespace | IPv4Namespace |
   |---|---|---|---|---|---|
   | soc-storage-scale-out | **93** (92 unbundled + 1 mesh) | **14** | **2** | 1 | 1 |
   | inb-mgmt | **17** | **17** | **1** | 1 | 1 |
2. **Connection endpoint set** — order-insensitive set of normalized tuples.
   - server links: `(server_port, switch_port, "unbundled")`.
   - mesh: the set of `{leaf1_port, leaf2_port}` link pairs.
   Must equal the committed files' set exactly.
3. **Switch `portBreakouts` / `portSpeeds`** — equal as maps (key→label),
   order-insensitive.

This comparison is implemented in `internal/oracle.CompareWiringHhfab` (currently
`ErrNotImplemented` / SKIP) → it parses both AID's render and the committed file
and asserts (1)+(2)+(3), then shells `hhfab validate`. The **wiring oracle row
flips SKIP→PASS**.

---

## 4. Oracle / acceptance wiring

- `internal/oracle.CompareWiringHhfab(computedDir, oracleDir)` → renders via
  `internal/wiring`, runs `hhfab validate` (hard gate), and runs the structural
  comparison (§3B) for **both** managed fabrics.
- The CI `hhfab validate` harness actually runs (not skipped) — same skip-guard
  posture as the existing golden test (CI asserts no `--- SKIP`).
- No `netbox_inventory.json` produced (D22). No empty `ecmp: {}` (field omitted
  entirely). No Python / HNP names in output.

---

## 5. RED → GREEN plan (after sign-off)

- **RED:** add `internal/wiring` with the type + a stub `Render` returning
  `ErrNotImplemented`; add `internal/wiring` tests asserting the §3B structural
  facts (kind counts, endpoint set, portBreakouts) + a `hhfab validate` test, all
  failing; flip `CompareWiringHhfab` to call the (stub) renderer so the oracle row
  is RED. Push, PAUSE for devb.
- **GREEN:** implement `Render` (per-fabric walk over endpoints + mesh zone +
  switch portBreakouts/boot.mac); turn the oracle row PASS; full suite + CI green;
  no F0–F3 regression. Push, PAUSE for devb → lead merges.

No new calc; reuse `internal/bom`'s port-spec parsing approach (extended from
*count* to *enumerate* for mesh ports). YAML via the existing `yaml.v3` dep.

---

## 6. Evidence (verified before this note)

- **Live F2 IR** (`calc.Compute` on `training.yaml`): switch qty
  `{soc_storage_scale_out_leaf:2, inb_mgmt_leaf:1, oob_leaf:1}`; 126 endpoints —
  soc `scale_out_server_2x400`=64 + `soc_storage_server_4x200`=28 (=92), inb
  `inb_mgmt_server_25g`=17, oob `oob_server_1g`=17 (excluded). PortSlot.Name e.g.
  `E1/1/1` (breakout) / `E1/1` (non-breakout).
- **boot.mac** SHA256 formula reproduces all three oracle MACs (§2.4).
- **`hhfab validate` v0.43.1** accepts both committed
  `wiring-soc-storage-scale-out.yaml` and `wiring-inb-mgmt.yaml` (exit 0,
  "Fabricator config and wiring are valid").
- **breakout_options** carry `breakout_id` + `from_speed` + `logical_speed`
  (label derivation §2.5); **fabric_class** gates managed vs unmanaged; NIC
  `PortTemplates[].Name` supplies `so0…/ss0…/mgmt0` server port suffixes.
- Reference: HNP `yaml_generator.py` (naming/trunc/variants), Rust
  `hhfab-adapter/src/crds.rs` (CRD struct shapes, ecmp/redundancy omitted).
