# F5 Mesh scale-out (xoc-128) — architecture note (Issue #62)

**Status:** proposed — awaiting lead + devb sign-off before RED.
**Author:** deva. **Scope:** D24 F5 — **mesh scale-out, xoc-128 only**. No new
architecture (xoc-128 is the same `mesh-conv-ro` as xoc-64, 2× OPG). Clos
(xoc-256+), switch-count **derivation**, and authored-plan→training
**normalization** are explicitly OUT of scope (D24). D22 still holds (NetBox is
not a validation target).

F5 makes the oracle/test harness **parametric over composition** and reproduces
xoc-128's quantities + `bom.csv` + all **5** managed-fabric wiring files
(`hhfab validate` + §3B structural equivalence) from its committed
`training_xoc128_2xopg64_mesh_conv_ro.yaml` + the AID catalog. It does **not**
close the F2-flagged derivation gap — xoc-128 is **override-only** (every switch
class `override_quantity: 2`); that stays Clos-phase-tracked, stated honestly in
the report.

Everything below is grounded in the live engines and the committed reference
snapshot under
`gitignored/refs/xoc/.../xoc-128/2x-opg-64/mesh-conv-ro--.../` (verified — see
§6 Evidence).

---

## 0. The headline finding — the engines already generalize; the *harness* does not

The F1–F4 engines (`internal/topology` ingest, `internal/calc`, `internal/bom`,
`internal/wiring`) are **already composition-agnostic**. They are data-driven off
the plan + catalog and contain no xoc-64 constants:

- **Ingest** maps the top-level `expected:` block → `plan.Status.Expected`
  (`topology.go:435`); xoc-128 uses the identical top-level shape. No change.
- **Wiring** groups output **per `fabric_name`** where `fabric_class == managed`
  and renders a 2-switch mesh for any class with `qty ≥ 2`
  (`wiring.go:66-76, 97, 229-236`). xoc-128's 5 managed fabrics, each one
  override-2 leaf class, fall straight out of this loop — no "2-fabric"
  assumption in the renderer.
- **calc / bom** consume `plan` + `cat` + `calcOut`; quantities flow from
  `override_quantity` / server `quantity`. No xoc-64 magic.

The work is therefore **almost entirely in the oracle harness and the vendored
fixtures**, plus **one catalog-overlay extension**. This is what keeps F5 tight
and low-risk, exactly as D24 intended. The risk is *not* engine rewrites; the
risk is leaving xoc-64 special-cased while pretending to be generic. §2 enumerates
every such spot.

---

## 1. The parametric oracle harness — composition as a parameter

### 1.1 The composition descriptor

Introduce a single table in `internal/oracle` driving all Layer-A oracle rows:

```go
// Composition is one vendored XOC oracle snapshot (mesh-conv-ro family for F5).
type Composition struct {
    Name        string // dir under tests/oracle, e.g. "xoc-128-2xopg64-mesh-conv-ro"
    OverlayPath string // AID optic/identity overlay for this composition (§3.3)

    // Pinned tripwires — vendored provenance, NOT derived (catch silent
    // corruption of the snapshot). Everything else is derived from the
    // vendored artifacts at test time.
    ServerClasses int      // expected.counts.server_classes
    SwitchClasses int      // expected.counts.switch_classes
    Connections   int      // expected.counts.connections
    TotalServers  int      // Σ server quantity (xoc-128 = 34)
    BOMRows       int      // bom.csv data+footer row count (xoc-128 = 23)
    Managed       []string // managed fabric_names (xoc-128 = 5)
}

func Compositions() []Composition {
    return []Composition{
        {Name: "xoc-64-mesh-conv-ro", OverlayPath: ".../optic-overlay.yaml",
            ServerClasses: 5, SwitchClasses: 3, Connections: 21, TotalServers: 20,
            BOMRows: 23, Managed: []string{"soc-storage-scale-out", "inb-mgmt"}},
        {Name: "xoc-128-2xopg64-mesh-conv-ro", OverlayPath: ".../optic-overlay-xoc128.yaml",
            ServerClasses: 8, SwitchClasses: 6, Connections: 38, TotalServers: 34,
            BOMRows: 23, Managed: []string{
                "scale-out-a", "scale-out-b", "soc-storage-a", "soc-storage-b", "inb-mgmt"}},
    }
}
```

`Dir()` replaces the hardcoded `LayerADir()`:

```go
func (c Composition) Dir() string { return filepath.Join(Root(), c.Name) }
```

**Design stance — derive, don't inline.** Every oracle *target* is read from the
vendored artifact, never from a magic number in Go:

| Oracle row | Target source (already-generic loader) |
|---|---|
| F1 expected.counts | `plan.Status.Expected.Counts` (from the composition's `training.yaml`) |
| F2 derived quantities | `LoadBOMQuantities(<dir>/bom.csv)` |
| F3 BOM projection | `CompareBOMProjection(got, <dir>/bom.csv)` |
| F3 full BOM (Layer B) | `real-server-bom.csv` (composition-independent; stays 1×/2×) |
| F4 wiring | `CompareWiringHhfab(computed, <dir>/wiring/)` (globs `wiring-*.yaml`) |
| F4 hhfab gate | every rendered managed fabric → `hhfab validate` |

The `Composition` struct's pinned fields are **tripwires only** — a small set of
headline totals asserted once per composition so a corrupted vendored snapshot
fails loudly rather than silently agreeing with itself. They are *not* the
comparison oracle.

### 1.2 The driver loop

Each existing `TestLayerA_*` becomes a `for _, c := range Compositions()` /
`t.Run(c.Name, ...)` subtest. The body is **identical** to today's xoc-64 body
with `LayerADir()` → `c.Dir()` and the inline expectations → derived-from-artifact
+ tripwire checks. CompareWiringHhfab already iterates **per fabric** inside one
composition, so the nesting is composition → fabric with zero new comparator
code.

This is the whole of the "parametric harness." It is **not** special-cased to
xoc-128: adding a third mesh composition later is one table row + one vendored
snapshot, no Go changes.

---

## 2. Every place xoc-64 is hardcoded today (and the fix)

Verified by grep across `internal/`. Two tiers:

### 2.1 The oracle harness — **must become composition-driven** (the F5 work)

| Location | Hardcoding | Fix |
|---|---|---|
| `oracle.go:47 LayerADir()` | returns the single `xoc-64-mesh-conv-ro` dir | replace with `Composition.Dir()`; keep a thin `LayerADir()` alias = `xoc-64`'s `Dir()` for the per-package engine tests (§2.2) |
| `oracle_test.go:39` | `NetboxCounts{128,21,481,259}` asserted inline | **drop from the parametric loop** — D22 makes NetBox a non-oracle; keep only a "file loads + has counts block" smoke (no magic numbers) |
| `oracle_test.go:101` | `{ServerClasses:5,SwitchClasses:3,Connections:21}` inline | derive from `plan.Status.Expected`; assert equals `c.{ServerClasses,SwitchClasses,Connections}` tripwire |
| `oracle_test.go:124-125` | `wantSwitch{soc_storage_scale_out_leaf:2,…}` / `wantServer{compute_xpu:8,…}` inline | **delete** — `LoadBOMQuantities(c.Dir()/bom.csv)` already *is* the oracle; the engine output is compared to it directly |
| `oracle_test.go:301` | `managed{soc-storage-scale-out,inb-mgmt}` inline | use `c.Managed` (or derive from the `wiring-*.yaml` glob — both are vendored truth); xoc-128 has 5 |
| `oracle.go` doc comments | "xoc-64 first", "xoc-64 is a 2-switch mesh" | generalize wording to "per composition" |

### 2.2 Per-package engine tests — **stay xoc-64-anchored** (deliberately, not laziness)

`topology_test.go`, `ingest_f1_test.go`, `wiring_test.go`, `bom_test.go` assert
**fine-grained engine mechanics** against xoc-64 — specific switch device names,
`boot.mac` values, NIC-slot joins, per-port breakout maps, cage bindings
(`wiring_test.go:214-333`, `ingest_f1_test.go:60-389`, `bom_test.go:42`). These
are **unit contracts for the engines**, and the engines are composition-agnostic
(§0). Re-asserting all of them for xoc-128 would be redundant and high-maintenance
for no added coverage.

**Decision:** the *end-to-end behavioral proof* that xoc-128 is reproduced lives
entirely in the **parametric oracle suite** (§1) — expected.counts, derived
quantities, byte-exact `bom.csv`, all-5-fabric `hhfab validate` + §3B. The
per-package tests remain xoc-64 mechanics tests. This is the right altitude:
oracle = "does AID reproduce the real composition end-to-end?", per-package =
"does this engine's transform obey its contract?". Acceptance ("both compositions
table-driven") is satisfied at the oracle layer, which is where "composition" is
a meaningful axis.

(If devb prefers, a *thin* xoc-128 smoke in `wiring_test.go` asserting the
5-fabric split + one mesh pair is cheap to add — flagged as optional, not
required.)

---

## 3. What to vendor — and provenance

Vendor a new oracle snapshot at `tests/oracle/xoc-128-2xopg64-mesh-conv-ro/`,
mirroring the xoc-64 dir, copied from the verified reference snapshot.

### 3.1 Files to vendor

| File | Source | Role |
|---|---|---|
| `training.yaml` | `…/generated/inputs/training_xoc128_2xopg64_mesh_conv_ro.yaml` | the ingested plan (spec + `expected.counts`) — **F1 self-check + the input to F2/F3/F4** |
| `bom.csv` | `…/bom.csv` | **F2 quantities + F3 byte-exact projection** oracle (23 rows) |
| `wiring/wiring-{scale-out-a,scale-out-b,soc-storage-a,soc-storage-b,inb-mgmt}.yaml` | `…/wiring/` | **F4** structural-equivalence + `hhfab validate` oracle (5 files) |
| `netbox_inventory.json`, `connectivity-map.csv` | `…/` | vendored **for parity only**; **NOT oracles** (D22) — loaded for the "file present" smoke, never compared |
| `translation-notes.md` | `…/generated/inputs/translation-notes.md` | provenance |
| `README.md` (new, short) | authored | provenance + "do not edit; regenerate from snapshot" |

**Do NOT vendor** a separate "derived counts" file — the counts are already
carried by `training.yaml` (`expected.counts`) and `bom.csv` (quantities). Adding
a third copy invites drift. The `Composition` struct's pinned tripwires are the
only restated numbers, and they live in code under review.

### 3.2 Provenance of the snapshot

The reference snapshot is HNP's generator output for the xoc-128 2×OPG-64
mesh-conv-ro composition. The committed wiring is authoritative: all **5**
`hhfab_validate_*.log` show `Fabricator config and wiring are valid` (hhfab
v0.45.5). AID's job is to **reproduce** these artifacts from `training.yaml` +
the AID-owned catalog — `bom.csv` byte-exact, wiring structurally equivalent +
hhfab-valid. We do not read oracle values back into the computation (no
circularity): the catalog identity facts are authored independently (§3.3).

### 3.3 Catalog overlay — the one real fixture change ⚠️

The AID optic/identity overlay (`tests/fixtures/f3/optic-overlay.yaml`) is **keyed
by class/transceiver id** and supplies (a) transceiver `calc_profile` facts for
`bom.csv` cols 7–19 and (b) per-class descriptive identity (`module_type_model` /
`manufacturer`, cols 2+5) for base server/switch rows. **It is xoc-64-specific in
two ways that xoc-128 breaks:**

1. **New transceiver SKU.** xoc-128's `bom.csv` has a `server_transceiver`
   **`osfp_200g_dr4`** (OSFP-200G-DR4; `host_serdes_gbps_per_lane: 50`,
   `200GBASE-DR4`) that the xoc-64 overlay does not define. (xoc-128 also has
   **no `switch_transceiver` rows** and a different row mix — but that falls out
   of the data; only the *missing SKU facts* need authoring.)
2. **New per-OPG class ids.** xoc-128's `_a/_b` disaggregation introduces class
   ids absent from the overlay: `compute_xpu_a/b`, `storage_srv_a/b`,
   `metadata_srv_a/b`, `scale_out_leaf_a/b`, `soc_storage_leaf_a/b`. (`hh_gateway`,
   `hh_controller`, `inb_mgmt_leaf`, `oob_leaf` are shared ids, already present.)
   The bom descriptive identity is an **inconsistent mix** the overlay must pin
   exactly: switches use the device-type **slug** (`celestica-ds5000`,
   `celestica-ds2000`) but `oob_leaf` uses the **model string**
   (`Celestica DS1000`); manufacturer is **title-cased** (`Generic`/`Celestica`)
   vs training's lowercase. This is precisely why AID owns these strings in the
   overlay rather than echoing a single training field.

**Recommendation — per-composition overlay.** Vendor a
`tests/oracle/xoc-128-2xopg64-mesh-conv-ro/optic-overlay.yaml` (or
`tests/fixtures/f3/optic-overlay-xoc128.yaml`) self-contained for xoc-128, rather
than growing the shared file. Rationale: the **class-identity** entries are
genuinely composition-specific (different class ids), so a shared file would
accumulate every composition's classes and obscure which belong to which.
Transceiver facts (`osfp_400g_dr4`, `osfp_200g_dr4`) are shared public optical
facts and may be duplicated cheaply or factored into a tiny shared `optics-base`
overlay that both compositions merge — devb's call; the note recommends the
simple self-contained per-composition file first. The `Composition.OverlayPath`
field (§1.1) already makes the overlay a per-composition parameter.

All authored values are **public optical-standard / device-catalog facts**
(same provenance discipline as the xoc-64 overlay header, D1/D12): authored by
hand, not read back from `bom.csv`.

---

## 4. Scope honesty — what F5 does NOT do

- **No derivation.** Every xoc-128 switch class is `override_quantity: 2`; calc
  reads the override, it does not *compute* switch counts from demand. The
  derivation path (the gap F2's note flagged) is **not exercised** here and
  remains **Clos-phase-tracked** (D24). The F5 report states this explicitly.
- **No normalization.** F5 ingests the **`training_*.yaml`** (already in training
  form), **not** the authored `topology-map.yaml`. The per-OPG disaggregation
  transform is a separate later phase (D24).
- **No Clos / spine support, no new CRD kinds, no kernel/proof changes.** `moon
  prove` stays green trivially (no proved-core touched).
- **No NetBox.** D22 unchanged — `netbox_inventory.json` is vendored for parity,
  never compared.

---

## 5. RED → GREEN plan (after sign-off)

**RED (branch `issue-62-f5-red`):**
1. Add the `Composition` table + `Dir()`; convert each `TestLayerA_*` to the
   `t.Run(c.Name)` loop; xoc-64 row stays green (pure refactor, no behavior
   change — verify green *before* adding xoc-128).
2. Vendor the xoc-128 snapshot (§3.1) **minus** the overlay; add the xoc-128
   table row. New subtests go RED for the right reason: F3 `bom.csv` projection
   and F4 wiring fail because the xoc-128 overlay/identity facts are absent
   (`optic-overlay-xoc128.yaml` missing) — not because the engines are broken.
3. Push, PAUSE for devb review of the RED (harness shape + tripwires + the
   "engines unchanged" claim).

**GREEN:**
4. Author `optic-overlay-xoc128.yaml` (`osfp_200g_dr4` + the 10 `_a/_b` class
   identities; §3.3). Wire `Composition.OverlayPath`.
5. Run the suite. Expect: F1 expected.counts {8,6,38} ✓; F2 quantities ==
   bom.csv ✓; F3 `bom.csv` **byte-exact** (23 rows) ✓; F4 all 5 fabrics
   structurally equivalent + `hhfab validate` valid ✓ (hhfab v0.45.5 is on PATH
   locally); xoc-64 still fully green; `moon prove` green; full `go test` + CI
   green. Mirror CI exactly: `go test $(go list ./... | grep -v gitignored)`.
6. Fix only the fixtures/overlay if a diff appears — engine code is expected to
   be untouched. **If an engine change proves necessary, STOP** and re-confirm
   with the lead (it would mean the "engines generalize" premise was wrong, which
   is a scope event).
7. Push, PAUSE for devb review, then **lead merges**. Never self-merge.

**Acceptance (restated):** xoc-128 expected.counts + quantities reproduced (6
switch classes incl. `_a/_b`; 8 server classes, 34 servers); `bom.csv` byte-exact
(23 rows); all 5 managed fabrics `hhfab validate` + §3B equivalent to committed
wiring; xoc-64 fully green (both compositions table-driven); `moon prove` green;
full suite + CI green. Report states derivation remains Clos-phase-tracked
(override-only here).

---

## 6. Evidence (verified before this note)

- **Composition shape** — `training_xoc128_2xopg64_mesh_conv_ro.yaml`: 6 switch
  classes (`scale_out_leaf_a/b`, `soc_storage_leaf_a/b`, `inb_mgmt_leaf`,
  `oob_leaf`), 5 `fabric_class: managed` fabric_names + `oob-mgmt` unmanaged, all
  `override_quantity: 2`; 8 server classes summing to 34 servers; top-level
  `expected.counts: {server_classes: 8, switch_classes: 6, connections: 38}`.
- **bom.csv** — 23 rows: 8 server + 6 switch + 5 nic + 2 `server_transceiver`
  (`osfp_200g_dr4`, `osfp_400g_dr4`) + 0 `switch_transceiver` + footer
  `# suppressed_switch_cable_assembly_count,0`. Manufacturer title-cased; switch
  model = slug except `oob_leaf` = model string.
- **wiring** — 5 files (`scale-out-a/b`, `soc-storage-a/b`, `inb-mgmt`); all 5
  committed `hhfab_validate_*.log` report `Fabricator config and wiring are
  valid` (hhfab v0.45.5).
- **Engines are generic** — `topology.go:435` (top-level `expected:`),
  `wiring.go:66-76,97,229-236` (per-`fabric_name`, qty≥2 mesh), `calc`/`bom`
  consume plan+cat+calcOut with no xoc-64 constants.
- **Overlay gap** — `optic-overlay.yaml` lacks `osfp_200g_dr4` and the `_a/_b`
  class-identity entries; supplies cols 2+5+7–19 by id (`optic-overlay.yaml:163-204`).
- **Hardcoding inventory** — `oracle.go:47`, `oracle_test.go:39,101,124-125,301`
  (the F5 work); per-package engine tests xoc-64-anchored by design (§2.2).
- **Tooling** — `hhfab` v0.45.5 on PATH; `moon prove` unaffected (no core touched).
