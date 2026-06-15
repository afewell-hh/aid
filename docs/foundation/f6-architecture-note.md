# F6 Clos topology + switch-count derivation (xoc-256) — architecture note (Issue #63)

**Status:** proposed — awaiting lead + devb sign-off before RED.
**Author:** deva. **Scope:** D24 *Clos topology + switch-count derivation* —
reproduce **xoc-256 (2× OPG-128) `clos-ro`** end-to-end from its committed
`training_xoc256_2xopg128_clos_ro.yaml` + the AID catalog: the **calculated**
switch counts (no `override_quantity`), `bom.csv`, and both managed-fabric wiring
files (`hhfab validate` + §3B structural equivalence) **including leaf↔spine
fabric links**.

This is the phase that closes the **F2-flagged derivation gap**: xoc-64/128 are
override-only mesh; xoc-256 is the first composition where switch count is
**derived from demand**. It exercises the proved `leaf_count`/`spine_count` cores
(`kernel/src/switch_count.mbt`) that have shipped green but never been on a
production path (mesh used override).

**Out of scope (D24, deferred):** authored-plan→training **normalization**
(xoc-256 has no plan→training transform — the training form *is* the map, so we
ingest it directly); CLI/REST/GUI **surfaces**; xoc-512/1024 (optional add-on,
§5). D22 still holds (NetBox is not a validation target).

Everything below is grounded in the live engines + the committed reference under
`gitignored/refs/xoc/compositions/xoc/xoc-256/2x-OPG-128/clos-ro--cx7-1x400g--bf3-2x200g--storage-conv-2x200g/`
and HNP `netbox_hedgehog/utils/topology_calculations.py` (read-only; cited by
line). All counts in this note were recomputed from the vendored artifacts and
cross-checked against the reference `bom.csv` / `wiring/*.yaml` (§7 Evidence).

---

## 0. Headline finding — the math already exists; four engines need *wiring*, not invention

The proved arithmetic cores are already in the tree and machine-checked by
`moon prove`:

- `kernel/src/switch_count.mbt::leaf_count` — Algorithm 1 (ceil-div + alternating
  ≥2 / MCLAG even-round / ESLAG clamp), routes through `@proofs.ceil_div_pos`,
  `@proofs.leaf_adjust_non_eslag`, `@proofs.leaf_clamp_eslag`.
- `kernel/src/switch_count.mbt::spine_count` — Algorithm 2 (`ceil_div(total leaf
  uplink demand, spine fabric port capacity)`), routes through
  `@proofs.ceil_div_pos`. **Never exercised on any path.**

The leaf path is *already wired and correct* for Clos: `f2_calc.mbt::compute_switch_qty`
computes per-server-zone `ceil(demand/capacity)` maxed across zones, then the
alternating/redundancy floor — which already reproduces `fe-leaf=2` and
`be-rail-leaf=4` (verified, §1.1–1.2). What is missing is four pieces of
*wiring*:

1. **Spine derivation** — `compute_switch_qty` only sums **server/oob** zones; a
   spine class has only a **fabric** zone and **no feeding connections**, so it
   returns 0 today. Spine count needs a *cross-class* pass (find leaves in the
   same fabric, sum their uplink-port demand). New kernel work; uses `spine_count`.
2. **Boundary extension** — the kernel needs each switch class's `fabric_name`
   and `hedgehog_role` to group leaves↔spine per fabric. Both already live on the
   ingested model (`topology.SwitchClassUse.FabricName/HedgehogRole`) but
   `calc.BuildCalcPlan` does not forward them, and `calc.SwitchClassIn` /
   `f2_types.SwitchClassIn` lack the fields.
3. **Fabric-link wiring renderer** — `wiring.go` renders server↔leaf (unbundled)
   + mesh today; Clos adds **leaf↔spine `fabric` Connections** and the **MCLAG
   `SwitchGroup`** + per-switch `groups`/`redundancy` block. New renderer work.
4. **BOM fabric optics** — `bom.switchTransceiverLines` counts server-facing
   cages (from F2 endpoints) + mesh-zone cages; it **misses** the leaf-uplink and
   spine-fabric optics (384 of the reference's 528 switch transceivers, §1.5).
   New BOM work.

The oracle harness (`internal/oracle`) is already parametric over `Compositions()`
(F5). Adding xoc-256 is one `Composition` row + one vendored snapshot + one
catalog overlay; every Layer-A/B test then runs against it automatically — so the
F6 deliverable is to make those existing tests **pass** for a derived (no-override)
Clos plan, not to write new drivers.

---

## 1. The five HNP rules, resolved (read-only `topology_calculations.py`, cited)

The xoc-256 `clos-ro` training form (verified) declares **4 switch classes with
NO `override_quantity`** (`fe-leaf`, `fe-spine`, `be-rail-leaf`, `be-spine`), 1
server class (`compute`, qty 32, 8 GPU), 9 connections (`fe` ppc=2 + 8 rails
ppc=1). `expected.counts: {server_classes: 1, switch_classes: 4, connections: 9}`.
Target derived counts: **fe-leaf=2, fe-spine=1, be-rail-leaf=4, be-spine=2**.

### 1.1 ⚠️ Rail-optimized leaf count → `be-rail-leaf = 4`

`calculate_switch_quantity` (`topology_calculations.py:458`) detects any
`distribution='rail-optimized'` connection (`:537-556`) and delegates to
`_calculate_rail_optimized_switches` (`:669-769`). That function **pools demand
across rails** (NOT per-rail isolation — the docstring's "2 rails per switch",
`:690-693`):

- available ports/switch = server-zone `physical_ports × breakout.logical_ports`,
  **zone-based ⇒ no uplink subtraction** (`:719-728`, the `not is_fallback` branch).
- `total_port_demand = Σ_rails (server_quantity × ports_per_connection)` (`:733-754`).
- `total_switches = ceil(total_port_demand / available)` (`:757`), then redundancy
  rounding (`:763-765`).

xoc-256 `be-rail-leaf`: server zone `be-server-ports` `port_spec "1-32"` = 32
physical, `b_2x400` ⇒ 2 logical ⇒ **available = 64**. Demand = 8 rails × 32
servers × 1 = **256**. `ceil(256/64) = 4`; no redundancy ⇒ **4**. ✔

**The "32 servers × 8 rails" framing (#63):** 256 = 32×8; the rail count never
appears as a divisor — it is pooled into one demand figure. So **for the count,
rail-optimized is just `ceil(pooled_demand / zone_capacity)`** — identical shape
to the standard per-zone path. AID's `compute_switch_qty` already sums *all*
feeding connections to the zone (`f2_calc.mbt::zone_demand`), which for
`be-server-ports` sums the 8 rails → 256 → `ceil(256/64)=4`. **It already returns
4** (verified by `TestLayerA_DerivedQuantities` once the snapshot is vendored).
The rail-*specific* logic (which switch a given rail lands on) is placement, not
count, and is already handled by `f2_switch_index` (`f2_calc.mbt:216-260`).

### 1.2 Standard leaf count + alternating/MCLAG → `fe-leaf = 2`

`calculate_switch_quantity` per-zone path (`:582-617`): per server zone,
`logical_per_switch = zone_port_count × breakout.logical_ports` (`:605`),
`zone_demand = Σ server_quantity × ports_per_connection` (`:609-611`),
`ceil(demand/logical_per_switch)` maxed across zones (`:613-615`). Then the
**alternating ≥2 floor** (`:655-658`) and **redundancy rounding** (`:660-662` →
`_apply_redundancy_rounding`, MCLAG = even, min 2, `:433-439`).

xoc-256 `fe-leaf`: `fe-server-ports` `port_spec "1-63:2"` = {1,3,…,63}=32
physical, `b_4x200` ⇒ 4 logical ⇒ 128 capacity. Demand = `fe` ppc=2 × 32 = 64.
`ceil(64/128)=1`; `fe` is `alternating` ⇒ floor to 2; MCLAG (even, ≥2) ⇒ **2**. ✔

AID parity: `compute_switch_qty` → `ceil_zone(64,128)=1`, `apply_redundancy(1,
alternating=true, redundancy)` → `@proofs.leaf_adjust_non_eslag(1, true, …)` = 2.
Already correct. **Count is 2 whether redundancy is `none` or `mclag`** (the
alternating floor already produces 2, which is even), so the count does not depend
on F6 wiring up MCLAG ingest — but the **wiring** does (§2.4, §2.6).

### 1.3 Per-fabric spine derivation → `fe-spine = 1`, `be-spine = 2`

`calculate_spine_quantity` (`:878`):

- leaves = same plan + same `fabric_name`, `hedgehog_role ∈ {server-leaf,
  border-leaf}`, excluding the spine itself (`:948-953`).
- `total_uplink_demand = Σ_leaf (leaf.effective_quantity × get_uplink_port_count(leaf))`
  (`:960-978`).
- spine downlink capacity = spine **FABRIC** zone `physical_ports ×
  breakout.logical_ports`, zone-based ⇒ no subtraction (`:989-1017`).
- `spines_needed = ceil(total_uplink_demand / available_downlink_ports)` (`:1024`).

`get_uplink_port_count` (`:205`): from `zone_type='uplink'` zones, **count of
parsed physical ports** `Σ len(PortSpecification(zone.port_spec).parse())`
(`:259-264`) — **physical, no breakout multiply**. (Override
`uplink_ports_per_switch=0` ⇒ a leaf documented as contributing no spine demand,
`:252-257`; in xoc-256 the `=0` appears only on the spine classes, which are never
counted as leaves, so it is a no-op here.)

xoc-256 frontend: leaves = `fe-leaf` qty 2; uplink zone `fe-uplinks`
`"2-64:2"`={2,4,…,64}=32 physical ⇒ demand `2×32=64`. Spine fabric zone
`fe-fabric-downlinks` `"1-64"`=64 × `b_1x800`(1) = 64. `ceil(64/64)=1`. ✔
backend: leaves = `be-rail-leaf` qty 4; uplink `be-uplinks` `"33-64"`=32 ⇒
demand `4×32=128`. Spine fabric `be-fabric-downlinks` `"1-64"`=64. `ceil(128/64)=2`. ✔

This matches #63's `ceil(2×32/64)=1` and `ceil(4×32/64)=2`.

### 1.4 Uplink reservation from `zone_type: uplink`

Two distinct uses, both keyed off `zone_type='uplink'`:

1. **Leaf server-count (§1.1–1.2):** xoc-256 server zones are zone-based, so the
   server-zone capacity *already excludes* uplink ports (they are disjoint
   physical ports) — uplink reservation = **0** in the leaf ceil-div. (AID's
   `compute_switch_qty` already only sums `server`/`oob` zones, so it never
   borrows uplink-zone capacity — equivalent.)
2. **Spine demand (§1.3):** each leaf's uplink-zone **physical port count** is the
   per-leaf uplink demand (`get_uplink_port_count`, physical, no breakout).

So in F6 the uplink zone is consumed in the **spine** pass (as demand), not the
leaf pass (as a reservation). The fallback `is_fallback` uplink-subtraction path
(`:636-640`, `:722-725`, `:1011-1014`) is **not** taken by xoc-256 (every zone is
zone-based) — we reproduce the zone-based branch only and document the fallback as
out of scope (no fixture exercises it).

### 1.5 Fabric-link port pairing + ECMP (§3B wiring)

Verified from the reference `wiring/*.yaml` — the deterministic rule:

- **Leaf uplink → spine split.** Each leaf's uplink ports (enumerated from its
  `uplink` zone, ascending) are split into **S contiguous groups** of
  `U/S` ports (U = leaf uplink port count, S = spine count); group *k* → spine *k*.
  - backend: U=32, S=2 ⇒ 16/spine. leaf ports `E1/33..48`→spine-01,
    `E1/49..64`→spine-02.
  - frontend: U=32, S=1 ⇒ leaf ports `E1/2..64(even)` all →spine-01.
- **Spine downlink assignment.** Each spine has its OWN cursor over its `fabric`
  zone ports (ascending), filled **in leaf order**, `U/S` per leaf.
  - be-spine-01: leaf-01→`E1/1..16`, leaf-02→`17..32`, leaf-03→`33..48`,
    leaf-04→`49..64`. be-spine-02: identical, cursor restarts at `E1/1`.
  - fe-spine-01: leaf-01→`E1/1..32`, leaf-02→`E1/33..64`.
- **One `fabric` Connection per (spine, leaf) pair**, `metadata.name =
  "{spine-dev}-fabric-{leaf-dev}"` (e.g. `be-spine-01-fabric-be-rail-leaf-01`),
  `spec.fabric.links: [{leaf:{port}, spine:{port}}, …]`. Connection order: by
  spine, then by leaf. Counts: frontend 2 (=2×1), backend 8 (=4×2). ✔
- **ECMP:** the reference carries **no `ecmp` field** — multi-spine fan-out *is*
  the per-leaf split across S spines; there is no explicit `ecmp:` map. We emit
  none (consistent with note §2.3's "no empty `ecmp: {}`"). "ECMP" in #63 = the
  even leaf-uplink split across spines, realized structurally by the links above.
- **MCLAG `SwitchGroup` (frontend only):** `fe-leaf` carries `redundancy_type:
  mclag`, `redundancy_group: fe-mclag` ⇒ one `SwitchGroup{name: fe-mclag, spec:
  {}}` + each `fe-leaf` Switch gets `groups: [fe-mclag]` and `redundancy: {type:
  mclag}`. backend leaves have no redundancy group ⇒ no SwitchGroup.

---

## 2. Mapping onto AID (the wiring work)

### 2.1 Boundary extension — carry `fabric_name` + `role` to the kernel

- `calc.SwitchClassIn` (`internal/calc/calc.go`): add `FabricName string
  json:"fabric_name"`, `Role string json:"role"`.
- `BuildCalcPlan` (`calc.go:251-267`): populate from `sw.FabricName`,
  `sw.HedgehogRole`.
- `f2_types.mbt::SwitchClassIn`: add `fabric_name : String`, `role : String`.
- `decode.mbt::d_switch_class_in` (`f2_calc.mbt:678-691`): decode both.

The `uplink`/`fabric` zones are **already on the wire** — `BuildCalcPlan` includes
*all* `plan.Spec.PortZones` regardless of `zone_type`, and `ZoneIn` carries
`ZoneType`. No zone-boundary change needed. Additive only; mesh plans simply carry
the new fields (mesh leaves keep working).

### 2.2 Kernel — spine derivation as a second pass, via the proved `spine_count`

Refactor `f2_run` (`f2_calc.mbt:481`) switch-quantity step into two passes:

1. **Leaves first** (`role != "spine"`): existing `compute_switch_qty` unchanged
   (already reproduces fe-leaf=2, be-rail-leaf=4).
2. **Spines** (`role == "spine"`, no override): for each spine class,
   `demand = Σ_{leaf : leaf.fabric_name == spine.fabric_name ∧ leaf.role ∈
   {server-leaf, border-leaf}} (leaf_qty × uplink_physical_ports(leaf))`, where
   `uplink_physical_ports = Σ_{z : z.zone_type=="uplink"} parse_port_spec(z.port_spec).length()`;
   `capacity = Σ_{z : z.zone_type=="fabric"} parse_port_spec(z.port_spec).length()
   × z.breakout_logical_ports`; quantity = `@switch_count.spine_count(demand,
   capacity)` (which is `@proofs.ceil_div_pos` under the guard). `override` still
   wins if present (none in xoc-256).

This **exercises `spine_count` on the production path** (D2: the proven arithmetic
*is* the code path). `leaf_count` is already mirrored by `compute_switch_qty`'s
`ceil_zone`+`apply_redundancy`; optionally we route the leaf path through
`switch_count.mbt::leaf_count` for one canonical core, but that is a
behavior-preserving cleanup, not required for correctness — propose it as a
follow-up to keep the F6 diff tight (decision for devb).

Honesty note: `uplink_physical_ports` deliberately does **not** apply breakout
(matches HNP `get_uplink_port_count`, physical), whereas spine `capacity` **does**
(matches HNP fabric capacity, logical). In xoc-256 both fabric/uplink use
`b_1x800` (1:1) so they coincide numerically; the asymmetry is faithful and will
matter only if a future fixture breaks out uplinks.

### 2.3 Endpoints / allocation — unchanged

F2 emits per-server endpoints (server↔leaf) only; rail placement on `be-rail-leaf`
already works via `f2_switch_index` rail-optimized (`f2_calc.mbt:235-248`).
Spine/fabric ports are **not** F2 endpoints — they are plan-time derivable
(uplink/fabric zones × switch quantity), produced by the wiring renderer (§2.4)
and BOM (§2.5), exactly as mesh ports are today (D6). No new endpoint records.

### 2.4 Wiring renderer — fabric links + MCLAG SwitchGroup

In `wiring.Render` (`internal/wiring/wiring.go`), per managed fabric, after the
unbundled/mesh blocks add:

- **MCLAG:** collect distinct `(redundancy_group)` over member leaf classes with
  `redundancy_type == mclag`; emit `SwitchGroup{name, spec:{}}`; in `switchCRD`,
  when the class has a redundancy group add `spec.groups: [group]` and
  `spec.redundancy: {type: mclag}`.
- **Fabric links:** identify leaf classes (`role ∈ {server-leaf, border-leaf}`)
  and spine classes (`role == spine`) in the fabric. Leaf instances ordered by
  device name, spine instances by device name. Compute the §1.5 split + per-spine
  downlink cursor; emit one `fabric` Connection per (spine, leaf) pair, named
  `{spine}-fabric-{leaf}`, in (spine, leaf) order. No `ecmp` field.

Requires `redundancy_type`/`redundancy_group` on the switch class (§2.6) and the
uplink/fabric `SwitchPortZone`s (already ingested). The `§3B`
`CompareWiringHhfab` comparator is structural and already iterates all
Connections/Switches/SwitchGroups; the negative control
`TestNegative_WiringComparatorNonVacuous` already proves it is non-vacuous (a
dropped fabric ⇒ fail) — no comparator change expected, but confirm it diffs
`fabric`/`SwitchGroup` kinds (devb check).

### 2.5 BOM — count fabric-zone optics

In `bom.switchTransceiverLines` (`internal/bom/bom.go:250`), in addition to F2
endpoint cages + mesh zones, add **uplink + fabric** zone optics. To stay exact
(don't over-count spare spine ports), count **one optic per actually-populated
fabric port = one per fabric link end**, i.e. derive from the same §1.5 pairing:
leaf-uplink optics = Σ leaf uplink ports used (all of them), spine-downlink optics
= Σ spine downlink ports used (= total link count). For xoc-256 every uplink and
every used spine port is populated, so this equals `Σ_zone countPorts(zone) ×
switch_qty` over `uplink`+`fabric` zones — **but** prefer the link-derived count
so a future under-subscribed spine (spare ports) is not over-counted. Verified
total: server-facing 144 + leaf-uplink 192 + spine-downlink 192 = **528** switch
transceivers (matches reference `bom.csv`); server transceivers 320 (32×(2 fe + 8
be)) already produced. ✔ (decision for devb: link-derived vs zone×qty — recommend
link-derived for forward-safety, noting they coincide here.)

### 2.6 Ingest — capture inline `redundancy_type` / `redundancy_group`

xoc-256 declares redundancy **inline on the switch class** (no `mclag_domains`
section). Today `calc` reads redundancy from `plan.Spec.MCLAGDomains`. Add
`RedundancyType`/`RedundancyGroup` to `rawSwitchClass` + `SwitchClassUse`
(`internal/topology/topology.go:242-249, 88-100`), ingest them, and have
`BuildCalcPlan` prefer the inline class redundancy (falling back to MCLAGDomains
for older fixtures). Re-emit in `Rebundle` for round-trip (guardrail 2). As noted
(§1.2) this does **not** change the count for xoc-256 (alternating already forces
the even 2), but it is required for the MCLAG wiring (§2.4) and is the
model-correct source.

### 2.7 Proofs stay green

No proof changes. `spine_count`/`leaf_count` and their `@proofs.*` cores are
unchanged — F6 only *calls* `spine_count` from the production path for the first
time. `moon prove` stays green (the goals are about the cores, not their callers).
The negative control's teeth (§4) live in Go oracle tests, not proofs.

---

## 3. What to vendor

### 3.1 Snapshot → `tests/oracle/xoc-256-2xopg128-clos-ro/`

From the reference `clos-ro--cx7-1x400g--bf3-2x200g--storage-conv-2x200g/`:
`generated/inputs/training_xoc256_2xopg128_clos_ro.yaml` (the ingest input),
`bom.csv`, `wiring/wiring-frontend.yaml`, `wiring/wiring-backend.yaml`, and (for
Layer-B) the real full BOM if present. Mirror the F5 vendoring layout + provenance
note (record source git SHA of the xoc refs).

### 3.2 `Composition` row (`internal/oracle/composition.go`)

```go
{
  Name:          "xoc-256-2xopg128-clos-ro",
  Overlay:       filepath.Join("tests","oracle","xoc-256-2xopg128-clos-ro","optic-overlay.yaml"),
  ServerClasses: 1, SwitchClasses: 4, Connections: 9,
  TotalServers:  32, BOMRows:  /* pin from vendored bom.csv: 11 raw lines = header + 9 data + footer; confirm against LoadCSV semantics at vendor time */,
  Managed:       []string{"backend","frontend"},
},
```

(`BOMRows` pinned from the vendored `bom.csv` line count in `TestLayerA_Tripwires`.)

### 3.3 Catalog overlay (the one real fixture change ⚠️, as in F5 §3.3)

The training `module_types`/`device_type_extensions` are bare; `bom.csv` carries
rich optic attributes (`QSFP112`/`MMF`/`MPO-12`/`200GBASE-SR2`/SR/850/…) and the
switch `Item.Model` must resolve to the hhfab profile `celestica-ds5000`
(`hedgehog_profile_name`). Author `xoc-256-2xopg128-clos-ro/optic-overlay.yaml`
that (a) maps `ds5000` → model `celestica-ds5000` (for `wiring` profile + `bom`
switch model) and (b) supplies the `QSFP112-200GBASE-SR2` optic `calc_profile`
attributes so `bom.csv` is byte-exact. The negative control
`TestNegative_OverlayIsLoadBearing` is extended to assert this overlay is
load-bearing (xoc-64 overlay ⇒ wrong BOM ⇒ fail).

---

## 4. Acceptance (reuse the parametric harness)

All via the existing `Compositions()`-driven Layer-A/B tests once §3 is vendored:

1. **Derived counts (no override):** `TestLayerA_DerivedQuantities` →
   `fe-leaf=2, fe-spine=1, be-rail-leaf=4, be-spine=2`.
2. **Self-check:** `expected.counts {1,4,9}` reproduced
   (`TestLayerA_ExpectedCounts_SelfCheck` + `TestLayerA_Tripwires`).
3. **BOM byte-exact:** `TestLayerA_BOMProjection` matches the vendored `bom.csv`
   (528 switch / 320 server transceivers; switch counts 2/1/4/2).
4. **Wiring:** `TestLayerA_WiringHhfab` — both fabrics structurally equal (§3B,
   **incl. the 2 frontend + 8 backend fabric Connections and the `fe-mclag`
   SwitchGroup**) and each passes `hhfab validate`.
5. **Derivation negative control with teeth (new):** perturb a derivation input
   so the count flips and assert the oracle fails — e.g. shrink the spine
   `fabric` zone (`be-fabric-downlinks` `"1-64"`→`"1-32"`) ⇒ `be-spine` becomes
   `ceil(128/32)=4` ≠ 2 ⇒ count/BOM/wiring diverge ⇒ test fails; and widen a
   leaf `uplink` zone ⇒ spine demand changes ⇒ fails. Commit these as the F6
   teeth (mirror `negative_control_test.go`).
6. **`moon prove` green** (unchanged cores; `spine_count` now on the path).
7. **No regressions:** xoc-64 + xoc-128 Layer-A/B stay green (additive boundary +
   role-gated spine pass; mesh classes have no `spine` role).

### Pre-commit teeth shown transiently in RED
Before §3 overlay/snapshot land, show the spine pass producing 0 (RED) and the
fabric-link/BOM diffs, so the GREEN delta is visible (F2/F3/F5 convention).

---

## 5. Scope honesty + optional ladder

- **Normalization deferred** (D24): we ingest the **training form** directly;
  xoc-256 has no `topology-plan`/`topology-map`→training transform to reproduce
  (only `topology-map.yaml` exists, no plan). The plan→training normalization
  (xoc-64 convergence, xoc-128 disaggregation) remains its own phase.
- **Surfaces deferred** (D24): no CLI/REST/GUI retarget here.
- **`is_fallback` uplink path** not reproduced (no zone-less fixture); documented.
- **xoc-512 / xoc-1024 (optional, if cheap):** same `clos-ro` architecture,
  larger; they should reproduce as added `Composition` rows + snapshots with **no
  engine change** (counts scale: be-rail-leaf 4→8→16, be-spine 2→4→8 per D24).
  Add them **only** if they surface no new behavior; **report** if any does (e.g.
  `determine_leaf_uplink_breakout` `:772` kicking in when spines exceed a leaf's
  physical uplink ports — a real divergence that would expand scope). Do not let
  them gate F6.

---

## 6. RED → GREEN plan (after sign-off)

1. **RED (deva):** vendor snapshot + overlay (§3); add the `Composition` row; add
   the derivation negative-control test. Extend boundary types
   (`calc.SwitchClassIn`, `f2_types.SwitchClassIn`, decode) and ingest
   (`redundancy_type/group`) as *type/plumbing only* so the suite compiles and
   fails on the missing spine count / fabric links / BOM optics. Capture the RED
   report (spine=0, wiring/BOM diffs, neg-control red). **Push, PAUSE for devb.**
2. **devb review** of the RED contract + boundary shape.
3. **GREEN (deva):** kernel spine pass via `spine_count` (§2.2); wiring fabric
   links + SwitchGroup (§2.4); BOM fabric optics (§2.5); `BuildCalcPlan` forwards
   fabric/role + inline redundancy. Make all of §4 pass; `moon prove` green;
   xoc-64/128 green. **Push, PAUSE for devb.**
4. **devb review** of GREEN.
5. **lead merges.** Never self-merge; PAUSE at each gate.

---

## 7. Evidence (verified before this note)

- HNP rules: `topology_calculations.py` `:458` `calculate_switch_quantity`,
  `:669-769` `_calculate_rail_optimized_switches`, `:878-1026`
  `calculate_spine_quantity`, `:205-297` `get_uplink_port_count`, `:433-455`
  `_apply_redundancy_rounding`.
- Counts recomputed from the vendored training form: fe-leaf `ceil(64/128)=1`→2
  (alt/mclag), be-rail-leaf `ceil(256/64)=4`, fe-spine `ceil(64/64)=1`, be-spine
  `ceil(128/64)=2`.
- Wiring shapes from `wiring/wiring-frontend.yaml` (2 `fabric` Connections,
  `SwitchGroup fe-mclag`, switch `redundancy: {type: mclag}` + `groups`) and
  `wiring/wiring-backend.yaml` (8 `fabric` Connections, 16 links each, no ecmp);
  leaf-uplink→spine split and per-spine downlink cursor confirmed by enumerating
  every link.
- BOM from `bom.csv`: switch counts 2/1/4/2; server_transceiver 320,
  switch_transceiver 528 = 144 server-facing + 192 leaf-uplink + 192
  spine-downlink (recomputed).
- AID engines: `internal/calc/calc.go`, `kernel/src/f2_calc.mbt`,
  `kernel/src/switch_count.mbt`, `internal/wiring/wiring.go`,
  `internal/bom/bom.go`, `internal/oracle/{composition,oracle_test,negative_control_test}.go`,
  `internal/topology/topology.go` (all read this session).
</content>
</invoke>
