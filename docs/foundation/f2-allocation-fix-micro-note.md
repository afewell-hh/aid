# F2 allocation-fidelity fix — architecture micro-note (Issue #61)

**Status:** proposed — awaiting lead (devc) + devb sign-off before RED.
**Author:** deva. Blocks F4 (#60). Touches the proved kernel — `moon prove` stays green.

Goal: F2's IR endpoints reproduce HNP's deterministic per-(switch,zone) port
assignment so F4's §3B Connection-set passes both fabrics (156 → 0 mismatches),
with quantities (`bom.csv`) unchanged and all proofs intact. Reproduce HNP's
rule — don't invent one.

## Trace result (grounds the fix)

HNP `device_generator.py:390 _create_connections` allocates in this order:
```
for server_class in plan.server_classes.all():   # alphabetical (PlanServerClass.Meta.ordering=['plan','server_class_id'])
  for server_index in range(quantity):
    for connection_def in server_class.connections.all():   # rail order
      … advance the single per-(switch,zone) cursor
```
i.e. **server_class(alpha) → server_index → connection(rail)**, one cursor per
(switch instance, zone), shared across everything in that zone.

AID's cursor (`kernel/src/f2_calc.mbt:443 alloc_slot`, `alloc.mbt:45
allocate_sequential`) is **already a correct single linear cursor** — `cursors[sw]`
→ offset, offset++, slot = port_at(0, offset). And `expand_breakout`
(`f2_calc.mbt:93`) is **already port-first/lane-inner**. The only divergence is the
**loop nesting** in `f2_run` (`f2_calc.mbt:499-558`), which is **connection-outer /
server-inner** — so it consumes the cursor rail-major (all servers' rail-0, then
all servers' rail-1 …), spreading rails to `E1/1/1, 5/1, 9/1, 13/1` instead of
HNP's consecutive `E1/1/1, 1/2, 2/1, 2/2`. The same nesting makes the same-switch
inb zone consume server classes in plan-declaration order, not alphabetical.

## The two fixes — where each lands

**(1) Server-class consumption order → alphabetical (Go, no proof impact).**
`internal/calc/calc.go` `BuildCalcPlan`: stable-sort the connections fed to the
kernel (`cp.Connections`) by `server_class_id` (and the `cp.ServerClasses` echo,
for tidiness) — the faithful analogue of HNP's `Meta.ordering` data-layer sort,
applied before the allocator. Within a class, connection order (rail-0…rail-7) is
preserved (stable). For xoc-64 → `compute_xpu, hh_controller, hh_gateway,
metadata_srv, storage_srv`. Pure feed-order; no kernel/proof change.

**(2) Cursor consumption order → server-outer/rail-inner (kernel).**
`kernel/src/f2_calc.mbt` `f2_run`: reorder the per-zone endpoint loop from
connection-outer/server-inner to HNP's **server_class(arrival) → server_index →
connection(rail) → ppc**, keeping the one zone-shared `cursors` array. Concretely:
collect the distinct server classes feeding `(switch_class, zone)` in arrival
order (which (1) makes alphabetical), then for each class iterate `server_index`,
then its connections, then `ports_per_connection`, calling the **unchanged**
`f2_switch_index` + `alloc_slot`. Transceiver verdicts stay one-per-connection (a
separate pass). **Unchanged:** `alloc_slot`, `allocate_sequential`,
`expand_breakout`, `f2_switch_index`, `compute_switch_qty`, and the
per-connection helper `f2_endpoints_for_connection` (test-only, not on the IR
path).

`allocation_strategy`: xoc-64's server zones are all `sequential`; AID's
`parse_port_spec` already yields sorted physical ports and `expand_breakout` is
port-first, so `sequential` is faithfully realized once consumption is linear.
`interleaved`/`spaced`/`custom` are **not** exercised by xoc-64 and are out of
scope here (flag for a follow-up if ever needed) — this fix changes consumption
order only, not strategy.

## Why proofs + quantities survive (order/assignment-only)

- **No count change.** Same number of endpoints; same per-leaf cursor demand
  (same number of `alloc_slot` calls per cursor); switch/server quantities
  (`compute_switch_qty`, the echo) untouched → `bom.csv` unchanged; F3's
  distinct-physical-cage set per zone is identical (same ports filled, reassigned
  across servers) → F3 projection/full-BOM unchanged.
- **Proof cores untouched.** The proved goals live in `kernel/proofs`
  (`port_at`, `ports_distinct`/I1 non-overlap, `alloc_end_in_bounds`,
  `ceil_div_pos` lower bound); `f2_run` is orchestration (not proof-enabled) and
  still routes every slot through `port_at` under the same in-bounds guard.
  Reordering *which* (server,rail) takes *which* offset changes neither the
  cursor map's injectivity nor the bound. `moon prove` re-proves all goals
  unchanged; the negative control stays red. (Re-run in GREEN to confirm.)

## Validation (D22-clean — no connectivity-map / netbox oracle)

The acceptance signal is the **existing F4 §3B Connection-set comparator**
(`internal/oracle.CompareWiringHhfab`, derived from the committed `wiring/*.yaml`)
going 156 → 0 for both fabrics + `hhfab validate` still passing. No
`connectivity-map.csv` / `netbox_inventory.json` oracle is introduced (D22).

**Proposed mechanics (needs lead confirm):** branch #61 on top of
`issue-60-f4-red` so `TestLayerA_WiringHhfab` is the live RED→GREEN signal
(RED: 156 mismatches; GREEN: 0). Lead then merges the F2 fix and F4 (#60) greens
unchanged. (Alternative if #61 must branch off main: add a small `internal/calc`
endpoint test whose expected (server-port, switch-port) slots are derived from
the `wiring/*.yaml` oracle — same truth source, not NetBox.) Either way: report
the before/after Connection-set diff for both fabrics + `moon prove` output.

## Plan after sign-off
RED (failing acceptance + the fix stubbed/visible) → devb → GREEN (both fixes;
156→0; `moon prove` green; full suite + CI green; no F0/F1/F3 regression) → devb →
lead verifies both fabrics + proofs, merges F2, then merges F4. Push and PAUSE at
each gate; never self-merge.
