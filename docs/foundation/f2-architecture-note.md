# F2 — Calculation Kernel: Architecture Note

**Status:** Draft for lead (devc) + devb sign-off. **No code until approved** (the
F2 workflow gate). Issue #52. Supersedes the invented-model kernel built in
Phases A–E (against `ALGORITHMS.md`); re-derives against the real diet/HNP engine.

This note resolves the F2 fork (where the calc lives, which HNP algorithms are
re-derived, which `moon prove` invariants are re-established) and fixes the oracle
bar before RED.

---

## 0. TL;DR / decisions to sign off

1. **The calc stays in the MoonBit kernel** (D2 stands) and is re-derived against
   the diet relational model. **Recommended; no Go-side reimplementation.**
2. **Boundary (D16 unchanged):** Go (`internal/topology` + `internal/catalog`)
   resolves the ingested plan into a **flat, numeric "calc-plan" JSON** and hands
   it to the kernel over the existing `wasmhost` `alloc`/`calculate(ptr,len)->(ptr,len)`
   ABI. The kernel returns a **"calc-output" JSON** (per-class switch quantity,
   port allocations, distribution assignments, transceiver verdicts). Go writes the
   switch/server quantities into `topology.Status.Computed` and the new oracle row
   compares them to `bom.csv`.
3. **Kernel owns the string/structural calc** the issue calls out — `port_spec`
   parse (comma-list + range + `start-end:step`), breakout expansion to
   `E1/{port}` / `E1/{port}/{lane}`, allocation cursor, distribution (incl.
   rail-optimized), transceiver verdict. **Go owns catalog resolution only** (e.g.
   `breakout_option` id → `logical_ports`), so the kernel stays a pure,
   catalog-free function — provable and matching D16's "pure kernel takes typed
   input only."
4. **`moon prove` invariants re-established against the real model:** (I1) port
   non-overlap over the *real* parsed+breakout sequence; (I3) switch-count lower
   bound for the *derived* path (`effective = override ?? calculated`); (I-new)
   allocation completeness/in-bounds. The pure-arithmetic cores already proved
   (`ceil_div_pos`, `ports_distinct`, `leaf_adjust_*`) are **reused**; BOM-scaling
   cores defer to F3; mesh-cable core defers to F4.
   **(Scope) Mesh-link pairing/cabling is deferred to F4** (§2.6): it is
   switch↔switch *wiring*, the same category F4 owns. F2 keeps only the mesh
   **structural gate** (2-or-3 switches) as a validation/proof invariant and emits
   **no** mesh-link IR. The acceptance plan also adds an explicit **derived count
   > 1** case so the non-override path is exercised beyond `ceil=1` (§6).
5. **Oracle bar:** for `xoc-64`, computed **switches per class** = `soc_storage_scale_out_leaf ×2,
   inb_mgmt_leaf ×1, oob_leaf ×1` and **server quantities** =
   `compute_xpu ×8, storage_srv ×3, metadata_srv ×3, hh_gateway ×2, hh_controller ×1`,
   validated against `tests/oracle/xoc-64-mesh-conv-ro/bom.csv`. New Layer-A
   **derived-quantities** oracle row moves SKIP→PASS. **`bom.csv` full reproduction
   (F3), wiring (F4), `netbox_inventory.json` (deferred, D22) rows stay SKIP.** CI
   green; no F0/F1 regression.

---

## 1. Where the calc lives — MoonBit kernel (recommended)

**Recommendation: keep the calculation kernel in MoonBit with `moon prove`, per
D2.** The diet calc is exactly the pure-function, zero-I/O, hard-invariant code D2
was written for (switch-count math, port non-overlap, allocation completeness).
The kernel↔Go WASM boundary, ABI shim, embed/build, and CI prove-gate already
exist and are green (`internal/wasmhost/wasmhost.go`, `kernel/wasm/abi.mbt`,
`embed/kernel.wasm`, `scripts/moon-prove-gate.sh`). Re-deriving in Go would discard
D2's machine-checked proofs and the working boundary for no benefit. No strong
reason to deviate was found, so D2 stands.

**What changes vs. the existing kernel.** The existing kernel decodes an *invented*
`TopologyPlan` (`device_catalog`, `fabric_domains`, `entries[]` with
`override_quantity` on entries). F2 swaps the **input shape** and the
**orchestration** to the diet model; the **pure algorithm cores and proofs are
largely reused** (§5).

### 1.1 Data flow (Go ↔ kernel, D16)

```
ingested plan + catalog            kernel (pure, provable)              Go
(internal/topology.Plan,           ───────────────────────             ──────────────────
 internal/catalog.Catalog)
        │  resolve catalog facts          calc-plan JSON  ──►  decode
        │  (breakout→logical_ports,                            derive switch qty (§4.1)
        │   port_spec string, uplink,                          parse port_spec (§4.2)
        │   zone transceiver attrs)                            allocate w/ cursor (§4.3)
        ▼                                                      distribute (§4.4)
  build calc-plan JSON  ──── wasmhost.Call("export_calculate", json) ──►  transceiver verdict (§4.5)
                                                               encode calc-output JSON
  decode calc-output  ◄──────────── (ptr<<32)|len ◄──────────  ◄──
        │
        ▼
  Status.Computed.SwitchQuantity / Counts   ──►  derived-quantities oracle vs bom.csv
```

**Identity is preserved across the boundary (devb gate).** Distribution keys on
`port_index` directly (`device_generator.py:1245-1257`) and the real input is keyed
per `connection_id` / `nic` / `port_index` (`training.yaml:607-745`); a single
connection *definition* fans out across `server_class.quantity` server instances,
and `same-switch`/`rail-optimized` send different server instances to different
switch instances. So the calc-plan carries **full per-connection identity in**, and
the calc-output emits **one record per realized endpoint** (per server instance ×
per port), never a per-definition summary.

**`calc-plan` JSON (Go → kernel) — the resolved, numeric input.** Per switch class:
`{ switch_class_id, override_quantity?, redundancy (none|mclag|eslag), topology_mode,
zones[] }` where each zone is `{ zone_name, zone_type, port_spec (string),
breakout_logical_ports (int, resolved by Go), allocation_strategy, transceiver_attrs }`.
Per server class: `{ server_class_id, quantity }`. Per connection (the **full
relational identity**, not a reduced view):
```
{ connection_id, server_class_id, server_quantity,
  nic_slot_id, port_index, ports_per_connection, speed,
  distribution, rail?, target_switch_class, target_zone,
  server_transceiver_attrs }      // optic attribute_data resolved by Go
```
Go resolves every catalog-dependent scalar (notably `breakout_option` →
`logical_ports`, and the optic `attribute_data` for both ends) so the **kernel
never touches the catalog** — it consumes typed numbers + the raw `port_spec`
string it is required to parse.

**`calc-output` JSON (kernel → Go).** Quantities for the oracle plus a
**per-realized-endpoint** allocation list (the F4-consumable IR):
```
{ switch_quantity: { class_id → int },
  server_quantity: { class_id → int },
  endpoints: [ {                       // one per (server instance, connection, port)
    server_class_id, server_index, connection_id, nic_slot_id, port_index,
    switch_class_id, switch_index, zone,
    port_slot: { physical_port, breakout_index?, name }   // name = E1/{p} | E1/{p}/{lane}
  } ],
  transceiver_verdicts: [ { connection_id, outcome (match|needs_review|blocked), reason_code } ] }
```
Every endpoint names *which* server instance got *which* switch instance and port
slot, so F4 can build wiring without re-running distribution. F2's headline
consumer is `switch_quantity`/`server_quantity` (the oracle); `endpoints`/`verdicts`
are computed and proved here but F2 emits no wiring CRDs.

This keeps the boundary identical to D16 (UTF-8 JSON over linear memory,
`alloc`/`dealloc`/`export_calculate`), so `internal/wasmhost` and `embed` are
untouched; only the JSON schemas on each side change.

---

## 2. HNP algorithms re-derived (cited)

All citations are to `gitignored/refs/hnp/netbox_hedgehog/` (reference only; never
imported, never surfaced to users — D1/D12). AID re-derives the *behavior*, not the
code.

### 2.1 Switch-count derivation
- **Effective quantity** — `models/topology_planning/topology_plans.py:577-589`
  (`effective_quantity`): `override_quantity if set else calculated_quantity else 0`.
- **Calculated quantity** — `utils/topology_calculations.py:458-666`
  (`calculate_switch_quantity`):
  - No connections → 0 (`:521-534`).
  - **Standard (per-zone) path** (`:558-648`): demand per zone =
    `Σ server_class.quantity × ports_per_connection` over the zone's connections;
    `logical_per_switch = zone_port_count × breakout.logical_ports`;
    `per_zone_switches = ceil(zone_demand / logical_per_switch)`;
    **switches = max over zones** (`:613`).
  - **Rail-optimized path** (`:536-556`, `_calculate_rail_optimized_switches`
    `:669-769`): aggregate demand per rail then
    `ceil(total_port_demand / available_ports_per_switch)` (`:754-757`), where
    `available = physical×logical − uplink` (`:719-728`).
  - **Alternating floor** — min 2 switches if any connection is `alternating` with
    demand (`:655-658`).
  - **Redundancy rounding** — `_apply_redundancy_rounding` at every return
    (`:662`, `:765`).

### 2.2 Port-spec parsing — `services/port_specification.py:39-77` (`parse`)
- Comma-split (`:54`); each part: `:` → stepped (`_parse_interleaved` `:117-150`,
  `range(start,end+1,step)`), `-` → range (`_parse_range` `:89-115`,
  `range(start,end+1)`), else single (`_parse_single` `:79-87`).
- Dedup via set, `sorted` output (`:52`, `:77`). Validation `0 < port ≤ 1024`
  (`_validate_port` `:152-162`). xoc-64 exercises all three:
  `26,28,30,32,34,36,38-63` (comma+range), `1-16` (range), `27,29,31,…` (comma).

### 2.3 Port allocation — `services/port_allocator.py`
- **Stateful cursor** keyed `(switch_name, zone.pk)` (`:32-34`); `allocate`
  (`:36-68`) slices `sequence[cursor:cursor+count]` then advances cursor →
  **non-overlap by construction**; over-allocation raises (`:46-55`).
- **Sequence build** (`:74-77`): `parse` → `_apply_strategy` → `_expand_breakouts`.
  Strategies (`:79-89`): `sequential` (as-is), `interleaved` (`ports[::2]+ports[1::2]`
  `:117-119`), `spaced` (halves-interleave `:121-133`), `custom` (explicit order).
  xoc-64 uses **sequential everywhere**.
- **Breakout expansion** (`:91-115`): `logical_ports==1` → `PortSlot(p, None, "E1/{p}")`;
  `>1` → lanes `1..N` → `PortSlot(p, lane, "E1/{p}/{lane}")`.

### 2.4 Distribution — `services/device_generator.py:1242-1321` (`_select_switch_instance`)
- `alternating` → `switch[port_index % n]` (`:1256-1257`).
- `same-switch` → contiguous server-index partition across switches, first
  `total%n` switches get +1 (`:1258-1274`).
- `rail-optimized` (`:1275-1319`): if `n ≥ total_rails` → domain-based
  `switch_index = (server_index // servers_per_domain)×total_rails + rail`
  (`:1303-1311`); else capacity-sharing
  `switch_index = rail // ceil(total_rails/n)` (`:1312-1315`); clamp to `n−1`
  (`:1317-1318`). Pre-compute at `:428-443`.

### 2.5 Transceiver selection — `services/transceiver_rules.py:160-256` (`evaluate_xcvr_pair`)
- Both null → match (`:184-185`); one null → `needs_review` intent-asymmetry
  (`:187-191`); cable-assembly far-end compares to far-end medium/cage
  (`:195-214`); **medium mismatch → BLOCKED, never downgraded** (`:216-223`);
  approved-asymmetric pair → match (`:225-235`); cage mismatch → `needs_review`
  (`:237-244`); connector mismatch → `needs_review` (`:246-253`); else match.
  Only **BLOCKED** halts generation (`device_generator.py:1121-1137`).

### 2.6 Mesh — deferred to F4 (structural gate kept in F2)
HNP mesh is **not** a byproduct of generic server-port allocation; it is its own
deterministic switch↔switch *cabling* flow:
`utils/mesh_allocator.py:39-121` (`allocate_mesh_links`) pairs **physical switch
instances** via `combinations(all_switch_names, 2)` (full mesh), assigns each pair a
stable `link_index`, leaves `subnet` blank (hhfab hydrates IPs), and
`services/device_generator.py:1659-1819` (`_create_mesh_connections`) allocates the
per-pair ports from the mesh zone and emits the mesh connections/CRDs.

That output is **wiring** — switch↔switch links validated by `hhfab validate` —
which is exactly F4's remit (foundation-redesign §5 F4). **Decision: F2 defers mesh
pairing + per-pair port allocation + mesh-link IR to F4.** F2 retains only:
- the **mesh structural gate** — a mesh fabric must have **2 or 3 switches**
  (`device_generator.py` mesh guard; the existing `mesh_cable_count`
  core in `kernel/proofs/cores.mbt:150-161` already encodes the `{2,3}` precondition)
  — surfaced as a validation result, since it is a topology-soundness invariant, not
  wiring; and
- **mesh-zone capacity awareness** in switch-count derivation *only if* a mesh class
  is on the derived (non-override) path. In `xoc-64` the sole mesh class
  (`soc_storage_scale_out_leaf`) is `override_quantity: 2`, so no mesh-derived count
  arises here; full mesh-zone-derived counting rides with the F4 mesh work.

Consequently `calc-output` carries **no mesh-link field** (its `endpoints` are
server↔switch only). The earlier note's "F2 computes mesh links / feeds F4" framing
is corrected to this explicit deferral.

---

## 3. Oracle / acceptance bar (core XOC assets — NOT netbox_inventory)

**Headline (derived-quantities row, SKIP→PASS).** For `xoc-64`
(`tests/oracle/xoc-64-mesh-conv-ro/training.yaml`) the kernel computes, and Go
compares to `bom.csv` switch/server rows:

| class | qty | source in xoc-64 | how F2 gets it |
|---|---|---|---|
| `soc_storage_scale_out_leaf` | 2 | `override_quantity: 2` (`training.yaml:455`) | `effective = override` |
| `inb_mgmt_leaf` | 1 | **derived** | zone `inb_mgmt_server_25g` `port_spec 1-24`×`b_1x25`(=1) = 24/switch; demand 17 (5 classes ×qty, same-switch, 1 port) → `ceil(17/24)=1` |
| `oob_leaf` | 1 | **derived** | zone `oob_server_1g` `port_spec 1-48`×`b_1x1`(=1) = 48/switch; demand 17 (bmc, 5 classes) → `ceil(17/48)=1` |
| servers | 8/3/3/2/1 | `server_classes[].quantity` | passthrough echo |

The two **genuinely derived** classes (`inb_mgmt_leaf`, `oob_leaf`) exercise the
real per-zone `ceil(demand/logical_capacity)` path; `soc_storage` exercises the
override path. (17 = 8+3+3+2+1; matches `bom.csv` server-transceiver rows
`RJ45-1000BASE-T ×17`, `SFP28-25GBASE-SR ×17`.) 21 connections = 8 rail-optimized
scale-out + 3 soc-storage (`ppc=2`) + 5 inb-mgmt + 5 oob.

**Required: a derived count > 1 case (lead + devb gate).** xoc-64's derived
classes both land at `ceil=1`, which does not stress the orchestration around
`effective = override ?? calculated` (a class that only ever returns 1 could pass
with broken zone-demand/capacity wiring). So the acceptance bar **additionally
requires** at least one explicit case on the **non-override** path with
`calculated_quantity > 1`, asserted independently of xoc-64:
- **Kernel unit fixture** `derived_multi`: a single server zone, `port_spec` giving
  e.g. 4 logical ports/switch, same-switch demand 10 → `ceil(10/4) = 3` switches; a
  variant adds the `alternating` floor (demand 1, but `alternating` ⇒ ≥ 2) and an
  MCLAG even-rounding variant (`ceil → 3` ⇒ rounded to 4). These hit
  `effective = calculated` with `> 1`, the alternating floor (`:655-658`), and
  redundancy rounding (`:662`).
- This is wired as a kernel `moon test` case **and** an I3 proof obligation (below),
  so both the runtime path and the proof exercise `calculated > 1`, not only the
  `ceil=1` xoc-64 values. (Synthetic fixture only — not a new XOC composition;
  scale-out compositions arrive in F5.)

**Also computed (proved, not asserted against an oracle until F4):** port
allocations (incl. `brk_2x400_osfp`=2-lane and `brk_4x200_osfp`=4-lane breakout
expansion on `soc_storage`), rail-optimized distribution of the 8 scale-out rails
across the 2 leaves, and per-cage transceiver verdicts. These feed F4 wiring.

**Stays SKIP/deferred:** `bom.csv` full 19-column reproduction (F3),
`wiring/*.yaml` + `hhfab validate` (F4), `netbox_inventory.json` /
`connectivity-map.csv` (deferred, **D22**). `moon prove` re-established and green
in the CI prove-gate.

---

## 4. `moon prove` invariants for the real model

Re-establish against the diet model; **discard** invariants tied to the invented
`device_catalog`/`entries[].override_quantity` shape. Pure-arithmetic cores in
`kernel/proofs/cores.mbt` are **reused unchanged** where the math is identical.

- **I1 — Port non-overlap (real sequence).** Within one `(switch, zone)`, distinct
  allocation cursor offsets map to distinct `PortSlot`s of the *real* parsed +
  strategy-ordered + breakout-expanded sequence. Reuse `port_at`/`ports_distinct`
  (`cores.mbt:24-42`); extend the wrapper so the injective domain is
  `(physical_port, breakout_index)` after expansion, not a bare cursor.
- **I3 — Switch-count lower bound (derived path).** For a non-overridden class,
  `calculated_quantity = ceil(zone_demand / logical_per_switch)` covers demand
  (`q·cap ≥ demand`) and is minimal — `ceil_div_pos` (`cores.mbt:46-62`) already
  proves this; restate the wrapper as `effective = override ?? calculated` and keep
  `leaf_adjust_non_eslag`/`leaf_clamp_eslag` (`:72-108`) for the alternating/MCLAG/
  ESLAG floors. **The I3 obligation must be discharged at `calculated > 1`** (the
  `derived_multi` fixture, §3), not only the xoc-64 `ceil=1` values.
- **I-new — Allocation completeness / in-bounds.** New pure core:
  `demand ≤ capacity ⇒ allocated == demand ∧ cursor_end ≤ |sequence|` (cursor never
  runs past the parsed sequence; every demanded port gets exactly one slot).
- **Deferred:** `fleet_quantity`/`child_qpu` BOM-scaling cores → F3;
  `mesh_cable_count` → F4. They remain proved (no regression) but are not F2's
  headline.

CI: the prove-gate job (`scripts/moon-prove-gate.sh`, wired in `.github/workflows/ci.yml`)
must stay green with `N == M` packages proved; a negative-control inversion must
still turn it red.

---

## 5. Salvage plan (reuse vs. rebuild)

| kernel area | disposition |
|---|---|
| `kernel/proofs/cores.mbt` (ceil_div, ports_distinct, leaf_adjust/clamp) | **reuse** (pure arithmetic; restate wrappers for I1/I3/I-new) |
| `switch_count.mbt`, `alloc.mbt`, `distribution.mbt`, `oversubscription.mbt` | **reuse logic**, re-anchor inputs to diet zones/connections; add real `port_spec` parse + breakout expand (new `port_spec.mbt`) |
| `types.mbt`, `decode.mbt` | **rebuild** to the `calc-plan` shape (§1.1); drop invented `device_catalog`/`fabric_domains`/`entries` |
| `calculate.mbt`, `encode.mbt` | **rebuild** orchestration + `calc-output` shape (switch/server qty + per-realized-endpoint records + verdicts; §1.1) |
| `bom.mbt`, `mesh.mbt`, `validate.mbt` (BOM/mesh parts) | **defer** to F3/F4 (keep compiling/proved; not on F2 path) |
| `internal/wasmhost`, `internal/components`, `embed`, ABI `kernel/wasm/abi.mbt` | **unchanged** (D16 boundary intact) |
| `internal/orchestrate` (invented path) | **rewire** to feed the new `calc-plan` from `internal/topology`+`catalog`, or add a parallel F2 entry; flagged for devb in RED |

**Model changes (F0/F1):** none anticipated beyond writing `Status.Computed`
(`topology.go:164-168` already exists). Any change is justified + flagged per the
issue constraint.

---

## 6. RED → GREEN plan (after sign-off)

- **RED:** (a) Go `oracle.CompareDerivedQuantities(computed, bom.csv)` + a new
  failing `TestLayerA_DerivedQuantities` (SKIP→fail without calc); (b) kernel
  `port_spec`/alloc/distribution/switch-count tests over xoc-64-shaped fixtures
  (failing), **incl. the `derived_multi` `calculated > 1` fixture (§3)**; (c) a
  kernel test asserting `calc-output` **endpoint identity** — for a same-switch and
  a rail-optimized connection, distinct `server_index` values land on the expected
  distinct `switch_index`/`port_slot` (proves the boundary preserves per-instance
  identity, devb finding 1); (d) inverted `moon prove` negative control. Push,
  PAUSE for devb.
- **GREEN:** implement §1.1 calc-plan resolver (Go) carrying full per-connection
  identity, the kernel calc (§2) emitting per-realized-endpoint records, the
  `calc-output` decode + `Status.Computed` population, and the re-established proofs
  (§4) incl. the `calculated > 1` I3 obligation. xoc-64 derived-quantities row PASS;
  `derived_multi` PASS; endpoint-identity test PASS; `moon prove` `N==M`; full
  `go test ./...` + `cd kernel && moon test` + CI green; BOM/wiring/mesh-link/netbox
  rows still SKIP/deferred. Push, PAUSE for devb; lead merges.

**Reproduction commands (to be recorded on the issue at GREEN):**
`cd kernel && moon test`; `bash scripts/moon-prove-gate.sh`; `make wasm && go test ./...`;
`go test -run TestLayerA_DerivedQuantities ./internal/oracle/`.

---

## 7. Scope guards
- No Python, no NetBox, no HNP names in user-facing output (D1/D12/D22).
- `netbox_inventory.json` / `connectivity-map.csv` not targeted (D22).
- Kernel stays catalog-free / pure (D16); Go owns all catalog resolution.
- F0/F1 model untouched except `Status.Computed` writes.
