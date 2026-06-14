# F3 — BOM Reducer: Architecture Note

**Status:** Draft for lead (devc) + devb sign-off. **No code until approved** (the
F3 workflow gate). Issue #56. Builds on F0 (object substrate + catalog), F1
(ingest → relational model), F2 (calc kernel: switch/server quantities + per-realized
endpoint IR). Implements design `docs/foundation-redesign.md` §2.2 / §2.6 / §4.2 / §4.4;
decisions D2, D6, D16, D20, **D22**.

This note resolves the two F3 forks the issue calls out — (1) **where the reducer
lives and how its quantity/scaling math stays proved**, and (2) **the anti-drift
gate** (one resolver; the HNP projection is a *filtered view*, never a second counted
path) — and fixes the oracle bar before RED.

---

## 0. TL;DR / decisions to sign off

1. **The reducer is a new Go package `internal/bom`.** It resolves the catalog +
   topology + F2 calc-output into **one resolved instance graph**, then renders **two
   views** of it: the full purchasable BOM and the HNP 19-column projection. No
   rework of the Rust `bom-adapter` (it renders the *old* invented `DeviceClass`
   model; the D16 boundary AID uses is JSON-over-memory to the **MoonBit kernel**, not
   Rust — §1).
2. **Scaling math goes through the proven kernel cores (D2/§4.4).** Every quantity
   multiply — `quantity_per_unit(child) = parent_qpu × quantity_per_parent` and
   `fleet = quantity_per_unit × plan_quantity` — is computed by the already-proved
   cores `@proofs.child_qpu` / `@proofs.fleet_quantity` (I4, `kernel/proofs/cores.mbt`,
   gated green in CI). F3 routes the **real diet model** through them via a new
   kernel export `export_f3_bom` (the F2 pattern: Go resolves catalog facts, kernel
   does the proved arithmetic, Go renders). **Only CSV/JSON rendering is impure Go**
   (§1.2, §4).
3. **One resolver, two renders (the anti-drift gate, load-bearing — §4.4).** There is
   exactly **one** `Resolve(plan, catalog, calcOut) → ResolvedModel`. Both outputs are
   pure renderers over that single value: `RenderFullBOM(m)` and `RenderProjection(m)`.
   The projection **selects and regroups lines that already exist in the resolved
   graph** — it never re-counts from the plan. A test asserts the projection's
   physical lines are a structural **subset** of the full BOM's lines, so the two
   provably cannot drift (§2).
4. **The 19-column optic attributes are an AID-owned catalog attribute plane, not
   input-derived (D12 provenance).** training.yaml `module_types` carry **only
   `{id, manufacturer, model}`** — *none* of bom.csv columns 7–19
   (cage_type/medium/connector/standard/reach/wavelength/lanes/breakout). Those are
   public optical-standard facts AID owns in `calc_profile` per transceiver/NIC type
   (§4.2). F3 authors them as an **AID catalog overlay** keyed by model; the renderer
   reads them. We do **not** read bom.csv (circular) and do **not** import HNP
   `seed_catalog` (D1/D12). (§3.)
5. **Oracle bar.** Projection == `tests/oracle/xoc-64-mesh-conv-ro/bom.csv` **exactly**
   (19 cols, every row, the `# suppressed_switch_cable_assembly_count,0` footer); full
   BOM == `docs/requirements/real-server-bom.csv` incl. non-physical + nested
   CX-7(×8)/BF3 + per-cage transceivers, with explicit **1× and 2×** scaling. Both
   Layer-A **BOM-projection** and Layer-B **full-BOM** oracle rows move SKIP→PASS.
   BOM-scaling `moon prove` green for the real model. D6 plan-time preserved. Wiring
   (F4) / netbox (D22) stay SKIP. CI green; no F0/F1/F2 regression. (§5.)

---

## 1. Where the reducer lives — Go `internal/bom`, scaling through the proven kernel cores

**Recommendation: a new Go package `internal/bom`, mirroring the F2 split.** F2
established the pattern (`docs/foundation/f2-architecture-note.md` §1): **Go resolves
every catalog-dependent fact; the MoonBit kernel does the pure, provable arithmetic;
Go renders.** F3 is the same shape — the catalog-dependent resolution (which slots,
which NIC cages, which selected optic, which switch ports F2 allocated) is impure Go;
the **scaling arithmetic** is the hard D2 invariant and belongs in the proved kernel.

**Why not the alternatives.**
- *Rework the Rust `bom-adapter`* — rejected. It renders the **old invented
  `DeviceClass` BOM** (`export_bom`, `kernel/src/bom.mbt` over `types.mbt`). The
  rebuild's boundary (D16) is JSON-over-linear-memory to the **MoonBit kernel**
  (`internal/wasmhost`, `export_f2_calculate`); the Rust adapter is on the disposition
  list (§4.6) and is not the F3 path.
- *Compute scaling in Go, cross-check the cores* — rejected. §4.4 + D2 require the
  math to **go through** the proven path, not merely be spot-checked against it. A
  parallel Go multiply is exactly the drift D2 exists to prevent.

### 1.1 Data flow (Go ↔ kernel, D16)

```
ingested plan + catalog  ──┐
(topology.Plan,            │  Go internal/bom.Resolve (impure: catalog resolution)
 catalog.Catalog)          │   • expand server fleet  = ServerClassUse.Quantity        (F2 server_quantity)
F2 calc-output  ───────────┤   • expand switch fleet  = F2 switch_quantity
(calc.CalcOutput:          │   • walk component_slots recursively → (node, qpp) tree
 SwitchQuantity,           │   • attach per-cage selected transceiver (cage_bindings)
 ServerQuantity,           │   • attach switch-side optic per F2 endpoint (zone transceiver)
 Endpoints)                │   • collect bom_line_templates (physical + non-physical)
                           ▼
                    flat numeric "bom-scale plan" JSON
                           │  wasmhost.Call("export_f3_bom", json)
                           ▼
        kernel (PURE, PROVABLE) — for every node:
          qpu(child) = @proofs.child_qpu(parent_qpu, qpp)          # I4
          fleet      = @proofs.fleet_quantity(qpu, plan_quantity)  # I4
                           │  scaled-line JSON  (each line: identity + fleet qty)
                           ▼
                    Go internal/bom (impure rendering only):
                      RenderFullBOM(m)    → real-server-bom.csv shape
                      RenderProjection(m) → bom.csv 19-col shape + footer
```

The kernel input is **flat and numeric** (the F2 contract): Go has already resolved
every catalog fact into `(node_id, quantity_per_parent, plan_quantity, is_root_line)`
tuples, so the kernel stays **catalog-free** and provable. The kernel returns each
line's **scaled fleet quantity**; Go attaches the rendering attributes (SKU,
description, optic columns) it already holds and emits CSV. (Attributes never cross
into the kernel — they are not part of the proved arithmetic.)

### 1.2 What is pure vs impure (the D2 line)

- **Pure / proved (kernel):** the qpu/fleet recursion — `child_qpu`, `fleet_quantity`
  (I4). The traversal *wiring* over the slot array is `[bridge]` (test-covered), as in
  the existing `bom_traverse`; the per-node arithmetic is proved.
- **Impure (Go):** catalog resolution, instance/cage expansion, optic-attribute
  lookup, CSV/JSON serialization, row aggregation/sorting, the suppressed-cable
  footer. None of this is scaling arithmetic.

---

## 2. The anti-drift gate — one resolver, projection-is-a-view (§4.4, load-bearing)

This is the single most important guard in F3. The design (§4.4 implementation gate,
devb re-review) mandates: **one resolver; both outputs are views of its result; the
HNP projection is never a second independently-counted BOM path.**

**Enforcement (structural, not by convention):**

1. **One resolve, two renders.** The package exposes exactly one resolver,
   `Resolve(plan, catalog, calcOut) (*ResolvedModel, error)`, and two **pure**
   renderers that take `*ResolvedModel` and nothing else:
   `RenderFullBOM(*ResolvedModel)` and `RenderProjection(*ResolvedModel)`. Neither
   renderer takes the plan, the catalog, or the calc-output — they **cannot** re-count
   even if a future edit tried to. The type signature is the gate.
2. **The projection is a filter+regroup of the full line set.** `ResolvedModel` holds
   a single `[]ResolvedLine` (every instantiated line, fleet-scaled). `RenderFullBOM`
   renders all of them in the owner shape. `RenderProjection` **filters** that same
   slice to the HNP-physical kinds (`server`, `switch` base devices; `nic`/`dpu`
   modules; `server_transceiver`; `switch_transceiver`), regroups into the 19-column
   shape, and appends the footer. Same input slice; the projection adds no quantity.
3. **A subset invariant test.** A test asserts that for the xoc-64 model, every
   physical projection row's `(kind, model, fleet_qty)` is accounted for by the full
   BOM's lines (the projection is provably a subset). If a refactor ever introduced a
   second counted path, the quantities would diverge and this test goes red.

> Note on the two oracle inputs. bom.csv (xoc-64 topology, HNP synthetic NIC modeling)
> and real-server-bom.csv (a standalone B200 single-server, AID 8×-one-cage-CX-7
> modeling) are **different resolved models** — they exercise different inputs, not two
> renders of one input. The "one resolver, two views" invariant is *within a single
> resolve*: for any given resolved model the projection is a view of the full BOM. F3
> validates the **projection face** on the xoc-64 model and the **full-BOM face** on the
> B200 model; the reducer code is the same for both.

---

## 3. The two outputs — detail & provenance

### 3.1 Full purchasable BOM (Layer B → real-server-bom.csv)

For every instantiated server/switch instance: its own `bom_line_templates`
(physical + non-physical) + every required `component_slot`'s line templates
recursively + the selected transceiver per populated cage — each **× instance
count** (linear). The B200 server (`smc-b200-8gpu`, already stubbed in
`tests/oracle/xoc-64-mesh-conv-ro/catalog.yaml`) is extended in a **dedicated F3
catalog fixture** to carry all 13 line types of real-server-bom.csv:

- **base/physical line templates:** Barebone `AS-4126GS-NBR-LCC`, GPU Board, CPU ×2,
  MEMORY ×24, Drive ×2 / ×1, Accessory `CBL-PWEX-1174-60`.
- **non-physical line templates (`physical:false`):** warranty `EWCSC`, support
  `SVC-NVSTDSWSUP-3Y`, assembly `MC0037`, onsite `OSNBD3`.
- **nested, quantity-bearing component_slots:** `8× CX-7` as **one slot `quantity:8`**
  over the faithful one-QSFP112-cage CX-7 type (`AOC-CX766003N-SQ0`); `1× BF3` over a
  BF3 DPU type with **one `fixed_interface` 1000BASE-T BMC** (`requires_transceiver:false`)
  + **two QSFP112 `transceiver_cage`s @ 200G**.
- **per-cage transceivers** are added by the reducer from the class cage_bindings —
  they are **not** flat lines in the CSV (README.md:17-21). real-server-bom.csv has no
  transceiver rows, so the catalog's cage_bindings select a transceiver that the
  **full-BOM renderer of this fixture suppresses from the flat CSV** but the projection
  would surface. (This is precisely R3/R5; the 1×/2× tests assert linear scaling of
  every line incl. the nested CX-7/BF3 quantities.)

Scaling tests: resolve a plan of `quantity:1` and `quantity:2` B200 servers; assert
every quantity (incl. 8×CX-7→8/16, 24×MEM→24/48, etc.) scales exactly linearly.

### 3.2 HNP 19-column projection (Layer A → bom.csv)

The same resolved model, filtered to HNP-physical rows and rendered in the bom.csv
header order, with HNP's section classification and the suppressed-cable footer.
Every quantity is derivable from the F2 resolved model — verified by hand against the
committed bom.csv:

| projection rows | source in the resolved model | check |
|---|---|---|
| `server` ×N per class | F2 `ServerQuantity` | 8/1/2/3/3 ✓ |
| `switch` ×N per class | F2 `SwitchQuantity` | 2/1/1 ✓ |
| `nic` modules | server_nics slot × server fleet | BMC ×17 (all servers), 25GbE ×17, 200G ×6, 8x400G ×8, 2x200G ×8 ✓ |
| `server_transceiver` | NIC cage count × server fleet, per cage_binding optic | OSFP-400G-DR4 ×64 (8 cages×8), QSFP112-SR2 ×28, RJ45 ×17, SFP28 ×17 ✓ |
| `switch_transceiver` | F2 `Endpoints` (populated switch ports) × zone optic | R4113-VR ×11, OSFP-400G-DR4 ×32, RJ45 ×17, SFP28 ×17 |
| footer | suppressed cable-assembly count | `,0` ✓ |

**Optic columns 7–19 provenance (§0.4, D12).** training.yaml carries only model names,
so the optic attributes are an **AID-owned `calc_profile` plane** authored as a catalog
overlay keyed by transceiver/NIC model (public optical-standard facts:
OSFP-400G-DR4 → `OSFP,SMF,MPO-12,400GBASE-DR4,DR,1310,4,100,DR4,…,1x`). The renderer
reads these. We do **not** read bom.csv (circular) and do **not** import HNP
`seed_catalog.py` (D1/D12). The `r4113_a9220_vr` 800G `2x400g` breakout row is the only
non-`1x` `breakout_topology` and is the explicit edge case in the RED fixtures.

> **Switch-transceiver count = physical cages, not logical ports.** A breakout cage
> holds **one** optic serving multiple logical ports. The switch-side count walks F2
> `Endpoints` reduced to **distinct physical (switch_index, physical_port)** pairs per
> zone optic — not the logical-port count. This is the one non-trivial projection
> derivation and gets its own RED fixture.

---

## 4. `moon prove` — BOM-scaling invariant re-established for the real model

The I4 cores (`child_qpu`, `fleet_quantity`) are **already proved and CI-gated**
(`scripts/moon-prove-gate.sh … kernel/proofs`). F3 does **not** weaken or rewrite
them; it **routes the real diet model through them** via `export_f3_bom`, so the
proved identity `fleet = qpu × plan_quantity` (and `qpu = parent × qpp` at every
nesting level) holds for the real BOM **by construction**. The traversal over the
slot array stays `[bridge]` (test-covered), exactly as documented for the existing
`bom_traverse`. No new proof goals are required for F3's core obligation; if review
prefers, an additional `proof_ensure` restating non-negativity of the composed
fleet at the F3 entry is cheap to add, but the I4 cores already discharge it.

CI prove-gate line is unchanged (`spikes/moonbit-port-proof kernel/proofs`); it stays
green because the cores are untouched. A negative-control (flip a `+` to `-` in a
scratch copy and watch the gate go red) is captured in the F3 report, per the F2
precedent.

---

## 5. Oracle bar / acceptance (D20/D22)

- **Layer A — projection:** `internal/bom.RenderProjection(model)` == committed
  `bom.csv` **exactly** — all 19 columns, every data row, and the
  `# suppressed_switch_cable_assembly_count,0` footer. `oracle.CompareBOMProjection`
  becomes real; `TestLayerA_BOMProjection` moves SKIP→PASS.
- **Layer B — full BOM:** `RenderFullBOM(model)` == `real-server-bom.csv` exact line
  set incl. non-physical (`EWCSC`, `SVC-NVSTDSWSUP-3Y`, `MC0037`, `OSNBD3`) + nested
  CX-7(×8)/BF3 + per-cage transceivers, at **1× and 2×**.
  `oracle.CompareFullBOM(_, _, scale)` becomes real; `TestLayerB_Scaling` moves
  SKIP→PASS for scale ∈ {1,2}.
- **Scaling proof:** BOM-scaling `moon prove` green for the real model (via I4 cores).
- **D6 preserved:** the BOM is derived plan-time from the model — no inventory DB
  write; the projection equals the inventory-derived numbers HNP would generate,
  checked vs bom.csv.
- **Stay SKIP:** wiring/`hhfab` (F4), `connectivity-map`/`netbox_inventory` (D22).
- **No regression:** F0/F1/F2 + golden path + CI green.

---

## 6. RED test plan (devb review gate before GREEN)

New `internal/bom` tests (failing against a stub reducer) + flip the two oracle rows
from pending→executing:

1. `TestProjection_XOC64_ExactBOMCsv` — full 19-column, row-exact, footer-exact equality
   to bom.csv (the headline projection oracle).
2. `TestFullBOM_B200_RealServerBom` — exact line-set equality to real-server-bom.csv at 1×.
3. `TestFullBOM_B200_LinearScaling` — 2× scales every line linearly (and a 1× control).
4. `TestProjection_IsSubsetOfFullBOM` — the anti-drift structural invariant (§2.3).
5. `TestProjection_SwitchTransceiver_PerPhysicalCage` — the breakout cage-vs-logical-port
   edge case (R4113-VR ×11; OSFP-400G-DR4 ×32).
6. `TestFullBOM_NonPhysicalAndNested` — EWCSC/SVC/MC0037/OSNBD3 present; 8×CX-7 + BF3
   (fixed BMC + 2 cages) nested correctly.
7. `oracle.CompareBOMProjection` / `CompareFullBOM` lose their `ErrNotImplemented`
   stubs; `TestLayerA_BOMProjection` / `TestLayerB_Scaling` move SKIP→PASS.

All RED for the right reason (reducer is a stub), no other phase touched.

---

## 7. Risks / open questions for sign-off

1. **Catalog authoring scope.** F3 must author (a) the optic `calc_profile` overlay for
   ~8 transceiver/NIC models (bom.csv cols 7–19) and (b) the full B200 line-template
   fixture (real-server-bom.csv). These are AID-owned public-fact data, not HNP
   imports (D12). **Confirm this is in F3 scope** (it is the R1–R5 catalog the design
   makes F3 responsible for), vs. split into a tiny F3a catalog-enrichment step.
2. **bom.csv `manufacturer`/`description` columns** also need AID-owned values
   (`Celestica`, `NVIDIA`, `Generic`, and the SR2/RJ45/SFP28 descriptions). Same
   overlay mechanism; flagging that "reproduce bom.csv exactly" pulls in descriptive
   strings, not just optics.
3. **New kernel export vs extend `f2_calculate`.** Recommendation is a separate
   `export_f3_bom` (clean contract, F2 untouched). Alternative: fold BOM scaling into
   the F2 output. Separate export preferred for review isolation — **confirm.**
4. **Switch-transceiver derivation** depends on F2 `Endpoints` carrying enough to
   reduce to distinct physical cages per zone optic. F2 emits one record per realized
   endpoint with `(switch_index, port_slot.physical_port, zone)` — sufficient. Calling
   it out so review can confirm no F2 boundary change is needed (it is not).

---

*Prepared by deva. Awaiting lead (devc) + devb sign-off before RED.*
