// F2 calculation kernel — the re-derived calc functions (HNP diet engine).
//
// Re-derived against the real engine (cited per function); NOT the invented
// ALGORITHMS.md. See docs/foundation/f2-architecture-note.md §2 and Issue #52.
//
// The pure cores these route through (`@proofs.ceil_div_pos`,
// `@proofs.ports_distinct`/`port_at`, `@proofs.alloc_end_in_bounds`,
// `@proofs.leaf_adjust_non_eslag`/`leaf_clamp_eslag`) are machine-checked by
// `moon prove`; the kernel calls them on the production path so the proven
// arithmetic IS the code path (D2), not a parallel copy.

// ----------------------------------------------------------------------
// small string helpers (the kernel parses the raw port_spec string itself)
// ----------------------------------------------------------------------

fn split_char(s : String, sep : Char) -> Array[String] {
  let out = []
  let mut cur = ""
  for c in s {
    if c == sep {
      out.push(cur)
      cur = ""
    } else {
      cur = cur + c.to_string()
    }
  }
  out.push(cur)
  out
}

fn parse_int_str(s : String) -> Int {
  let mut n = 0
  for c in s {
    if c >= '0' && c <= '9' {
      n = n * 10 + (c.to_int() - '0'.to_int())
    }
  }
  n
}

// Insert into a sorted array, dropping duplicates (the HNP `sorted(set(...))`).
fn insert_sorted_unique(arr : Array[Int], v : Int) -> Unit {
  let mut i = 0
  while i < arr.length() && arr[i] < v {
    i = i + 1
  }
  if i < arr.length() && arr[i] == v {
    return
  }
  arr.insert(i, v)
}

// ----------------------------------------------------------------------
// §2.2 Port-spec parsing — port_specification.py:39-77
//   comma-list + range (`start-end`) + stepped range (`start-end:step`);
//   dedup + sorted ascending.
// ----------------------------------------------------------------------

/// Parse a `port_spec` into the sorted, de-duplicated list of physical ports.
/// xoc-64 exercises all three forms: `26,28,30,32,34,36,38-63`, `1-16`, `1-32:2`.
pub fn parse_port_spec(spec : String) -> Array[Int] {
  let out = []
  for part in split_char(spec, ',') {
    if part != "" {
      // a `start-end:step` part splits into the `start-end` range and the step.
      let colon = split_char(part, ':')
      let range_part = colon[0]
      let step = if colon.length() > 1 { parse_int_str(colon[1]) } else { 1 }
      let step = if step <= 0 { 1 } else { step }
      let dash = split_char(range_part, '-')
      if dash.length() > 1 {
        let lo = parse_int_str(dash[0])
        let hi = parse_int_str(dash[1])
        let mut p = lo
        while p <= hi {
          insert_sorted_unique(out, p)
          p = p + step
        }
      } else {
        insert_sorted_unique(out, parse_int_str(range_part))
      }
    }
  }
  out
}

// ----------------------------------------------------------------------
// §2.3 Breakout expansion — port_allocator.py:91-115
//   logical_ports == 1 -> PortSlot(p, None, "E1/{p}")
//   logical_ports  > 1 -> lanes 1..N -> PortSlot(p, lane, "E1/{p}/{lane}")
// ----------------------------------------------------------------------

pub fn expand_breakout(
  physical_ports : Array[Int],
  logical_ports : Int
) -> Array[PortSlot] {
  let out = []
  for p in physical_ports {
    if logical_ports <= 1 {
      out.push({ physical_port: p, breakout_index: None, name: "E1/" + p.to_string() })
    } else {
      let mut lane = 1
      while lane <= logical_ports {
        out.push({
          physical_port: p,
          breakout_index: Some(lane),
          name: "E1/" + p.to_string() + "/" + lane.to_string(),
        })
        lane = lane + 1
      }
    }
  }
  out
}

// ----------------------------------------------------------------------
// §2.1 Switch-count derivation —
//   effective_quantity: topology_plans.py:577-589 (override ?? calculated ?? 0)
//   calculated_quantity: topology_calculations.py:558-648
//     per-zone ceil(demand / (port_count × logical_ports)), max over zones.
//   Routes the lower bound through @proofs.ceil_div_pos and the
//   alternating/mclag floors through @proofs.leaf_adjust_non_eslag.
// ----------------------------------------------------------------------

/// effective = override if present else calculated.
pub fn effective_quantity(override_qty : Int?, calculated : Int) -> Int {
  match override_qty {
    Some(v) => v
    None => calculated
  }
}

// ceil(demand / logical_per_switch) via the proven lower-bound core (guarded for
// a non-positive capacity, which has no demand it can satisfy).
fn ceil_zone(demand : Int, logical_per_switch : Int) -> Int {
  if logical_per_switch <= 0 {
    0
  } else {
    @proofs.ceil_div_pos(demand, logical_per_switch)
  }
}

// Apply the alternating "≥2" / redundancy floors to a base count, routing
// through the proven monotone adjustment cores (so the ceil-div lower bound
// survives — I3).
fn apply_redundancy(base : Int, alternating : Bool, redundancy : String) -> Int {
  if redundancy == "mclag" {
    @proofs.leaf_adjust_non_eslag(base, alternating, true)
  } else if redundancy == "eslag" {
    @proofs.leaf_clamp_eslag(base)
  } else {
    @proofs.leaf_adjust_non_eslag(base, alternating, false)
  }
}

/// calculated_quantity for one server zone: ceil(demand / logical_per_switch),
/// then the alternating "≥2" / MCLAG even-rounding floor. `redundancy` ∈
/// {none,mclag,eslag}; `alternating` is true if any feeding connection is
/// alternating. Exercised at calculated > 1 (note §3, the lead caveat) by the
/// `derived_multi` fixture, not only the xoc-64 ceil=1 values.
pub fn calculated_quantity(
  demand : Int,
  logical_per_switch : Int,
  alternating : Bool,
  redundancy : String
) -> Int {
  apply_redundancy(ceil_zone(demand, logical_per_switch), alternating, redundancy)
}

// ----------------------------------------------------------------------
// §2.4 Distribution — device_generator.py:1242-1321
//   alternating    : switch[port_index % n]
//   same-switch    : contiguous server partition (first total%n switches get +1)
//   rail-optimized : n >= total_rails -> domain-based
//                      (server_index // servers_per_domain)*total_rails + rail
//                    n <  total_rails -> capacity-sharing
//                      rail // ceil(total_rails / n)
//                    clamped to n-1.
// ----------------------------------------------------------------------

// Contiguous server-index partition across n switches; the first `total % n`
// switches carry one extra server (device_generator.py:1258-1274).
fn same_switch_index(server_index : Int, total_servers : Int, n : Int) -> Int {
  if n <= 0 {
    return 0
  }
  let base = total_servers / n
  let rem = total_servers % n
  let mut s = 0
  let mut start = 0
  while s < n {
    let size = if s < rem { base + 1 } else { base }
    if server_index < start + size {
      return s
    }
    start = start + size
    s = s + 1
  }
  n - 1
}

/// Domain-based rail pre-compute (device_generator.py:438-443): the number of
/// servers one switch serves for its rail = `max(1, zone_capacity // ppc)`,
/// where `zone_capacity` is the parsed+breakout-expanded slot count of the zone.
/// Feeds the `n >= total_rails` branch of `f2_switch_index`. NOT `eff / total_rails`.
pub fn servers_per_domain(zone_capacity : Int, ports_per_connection : Int) -> Int {
  let ppc = if ports_per_connection > 0 { ports_per_connection } else { 1 }
  let q = zone_capacity / ppc
  if q < 1 {
    1
  } else {
    q
  }
}

pub fn f2_switch_index(
  distribution : String,
  port_index : Int,
  server_index : Int,
  rail : Int,
  effective_quantity : Int,
  total_rails : Int,
  total_servers : Int,
  servers_per_domain : Int
) -> Int {
  let n = effective_quantity
  let raw = if distribution == "alternating" {
    if n <= 0 {
      0
    } else {
      port_index % n
    }
  } else if distribution == "same-switch" {
    same_switch_index(server_index, total_servers, n)
  } else if distribution == "rail-optimized" {
    if n >= total_rails {
      // domain-based: each domain owns `servers_per_domain` servers.
      let spd = if servers_per_domain <= 0 { 1 } else { servers_per_domain }
      server_index / spd * total_rails + rail
    } else {
      // capacity-sharing: rails_per_switch = ceil(total_rails / n).
      let rps = ceil_zone(total_rails, n)
      if rps <= 0 {
        0
      } else {
        rail / rps
      }
    }
  } else {
    0
  }
  // clamp to [0, n-1] (device_generator.py:1317-1318).
  if n > 0 && raw > n - 1 {
    n - 1
  } else if raw < 0 {
    0
  } else {
    raw
  }
}

// ----------------------------------------------------------------------
// §2.5 Transceiver verdict — transceiver_rules.py:160-256
//   Pairs the connection's server-side optic against the target zone's optic
//   (both already catalog-resolved by Go). Returns (outcome, reason_code):
//     both None                  -> ("match", "R_NULL")
//     one None                   -> ("needs_review", "R_INTENT_ASYMMETRY")
//     medium mismatch            -> ("blocked", "R_MEDIUM_MISMATCH")   [never downgraded]
//     cage_type mismatch         -> ("needs_review", "R_CAGE_MISMATCH")
//     connector mismatch         -> ("needs_review", "R_CONNECTOR_MISMATCH")
//     otherwise                  -> ("match", "R_MATCH")
//   Only "blocked" halts generation (device_generator.py:1121-1137).
// ----------------------------------------------------------------------

pub fn f2_xcvr_verdict(
  server : XcvrAttrs?,
  zone : XcvrAttrs?
) -> (String, String) {
  match (server, zone) {
    (None, None) => ("match", "R_NULL")
    (Some(_), None) => ("needs_review", "R_INTENT_ASYMMETRY")
    (None, Some(_)) => ("needs_review", "R_INTENT_ASYMMETRY")
    (Some(s), Some(z)) =>
      if s.medium != z.medium {
        ("blocked", "R_MEDIUM_MISMATCH")
      } else if s.cage_type != z.cage_type {
        ("needs_review", "R_CAGE_MISMATCH")
      } else if s.connector != z.connector {
        ("needs_review", "R_CONNECTOR_MISMATCH")
      } else {
        ("match", "R_MATCH")
      }
  }
}

// ----------------------------------------------------------------------
// Integration — one connection definition fans out across `server_quantity`
// server instances into per-realized-endpoint records (devb finding 1). Each
// endpoint names the server instance, the chosen switch instance, and the
// allocated port slot. (`ports_per_connection > 1` expands across consecutive
// port_index lanes; xoc-64's soc-storage uses ppc=2.)
// ----------------------------------------------------------------------

pub fn f2_endpoints_for_connection(
  conn : ConnIn,
  effective_quantity : Int,
  total_rails : Int,
  servers_per_domain : Int,
  zone_ports : Array[PortSlot]
) -> Array[Endpoint] {
  let eps = []
  let rail = match conn.rail {
    Some(r) => r
    None => 0
  }
  // Per-switch-instance cursors (port_allocator.py keys on (switch, zone)), so a
  // distinct server instance lands on a distinct switch's own port pool.
  let cursors = Array::make(
    if effective_quantity > 0 { effective_quantity } else { 1 },
    0,
  )
  for si in 0..<conn.server_quantity {
    for pp in 0..<conn.ports_per_connection {
      let port_index = conn.port_index + pp
      let switch_index = f2_switch_index(
        conn.distribution,
        port_index,
        si,
        rail,
        effective_quantity,
        total_rails,
        conn.server_quantity,
        servers_per_domain,
      )
      let (slot, _) = alloc_slot(zone_ports, cursors, switch_index)
      eps.push({
        server_class_id: conn.server_class_id,
        server_index: si,
        connection_id: conn.connection_id,
        nic_slot_id: conn.nic_slot_id,
        port_index,
        switch_class_id: conn.target_switch_class,
        switch_index,
        zone: conn.target_zone,
        port_slot: slot,
      })
    }
  }
  eps
}

// ----------------------------------------------------------------------
// Boundary entry — JSON calc-plan string in, JSON calc-output string out
// (D16; wired to the `export_f2_calculate` wasm export). Decode the calc-plan,
// derive switch/server quantities + per-realized-endpoint records + verdicts,
// and encode the calc-output. On a malformed plan it returns an empty (but
// well-formed) calc-output so the Go host always decodes a valid envelope.
// ----------------------------------------------------------------------

pub fn f2_calculate(plan_json : String) -> String {
  let plan = try f2_decode_plan_inner(plan_json) catch {
    _ =>
      return f2_encode_output({
        switch_quantity: [],
        server_quantity: [],
        endpoints: [],
        transceiver_verdicts: [],
        errors: [{ code: "INVALID_PLAN", message: "calc-plan JSON is malformed" }],
      })
  }
  f2_encode_output(f2_run(plan))
}

// Demand for one (switch class, zone): Σ server_quantity × ports_per_connection
// over the connections feeding that zone; also reports whether any feeder is
// `alternating` (topology_calculations.py:558-648,655-658).
fn zone_demand(
  conns : Array[ConnIn],
  switch_class_id : String,
  zone_name : String
) -> (Int, Bool) {
  let mut demand = 0
  let mut alternating = false
  for c in conns {
    if c.target_switch_class == switch_class_id && c.target_zone == zone_name {
      demand = demand + c.server_quantity * c.ports_per_connection
      if c.distribution == "alternating" {
        alternating = true
      }
    }
  }
  (demand, alternating)
}

// effective switch quantity for a class: override if present, else the per-zone
// ceil(demand/capacity) maxed across the class's server/oob zones, then floors.
fn compute_switch_qty(sc : SwitchClassIn, conns : Array[ConnIn]) -> Int {
  match sc.override_quantity {
    Some(v) => v
    None => {
      let mut base = 0
      let mut alternating = false
      for z in sc.zones {
        if z.zone_type == "server" || z.zone_type == "oob" {
          let (demand, alt) = zone_demand(conns, sc.switch_class_id, z.zone_name)
          if alt {
            alternating = true
          }
          if demand > 0 {
            let capacity = parse_port_spec(z.port_spec).length() *
              z.breakout_logical_ports
            let zc = ceil_zone(demand, capacity)
            if zc > base {
              base = zc
            }
          }
        }
      }
      if base == 0 {
        0
      } else {
        apply_redundancy(base, alternating, sc.redundancy)
      }
    }
  }
}

fn qty_for(qtys : Array[ClassQty], class_id : String) -> Int {
  for q in qtys {
    if q.class_id == class_id {
      return q.quantity
    }
  }
  0
}

// Membership test over a string array (used to collect the distinct server
// classes feeding a zone, in arrival order — Issue #61 fix 2).
fn contains_str(xs : Array[String], s : String) -> Bool {
  for x in xs {
    if x == s {
      return true
    }
  }
  false
}

// Allocate the next slot for switch instance `sw` from this zone's expanded
// sequence, advancing that switch's own cursor (port_allocator.py keys the
// cursor on (switch, zone)). Returns (slot, in_bounds): the bool is false when
// the switch's cursor would run past the parsed+breakout sequence — an
// over-allocation that mirrors port_allocator.py:57-67's raise. The caller
// surfaces it as a calc-output error rather than silently wrapping/reusing ports.
fn alloc_slot(
  zone_ports : Array[PortSlot],
  cursors : Array[Int],
  sw : Int
) -> (PortSlot, Bool) {
  let capacity = zone_ports.length()
  if capacity == 0 {
    ({ physical_port: 0, breakout_index: None, name: "" }, true)
  } else {
    let idx = if sw >= 0 && sw < cursors.length() { sw } else { 0 }
    let offset = cursors[idx]
    // In-bounds guard: `zone_capacity_ok(offset+1, capacity)` is exactly the
    // `demand <= capacity` pre-flight the `@proofs.alloc_end_in_bounds` core
    // encodes (note §4; cores.mbt:114-134). Only on a pass do we index the slot.
    if zone_capacity_ok(offset + 1, capacity) {
      // The slot for this cursor offset = `@proofs.port_at(0, offset)`, the cursor
      // map proven INJECTIVE by `@proofs.ports_distinct` (I1). `offset < capacity`
      // by the guard, so it indexes directly — no modulo, no reuse.
      let slot_index = @proofs.port_at(0, offset)
      cursors[idx] = offset + 1
      (zone_ports[slot_index], true)
    } else {
      ({ physical_port: -1, breakout_index: None, name: "" }, false)
    }
  }
}

fn f2_run(plan : CalcPlan) -> CalcOutput2 {
  // 1. switch quantity per class (override ?? derived).
  let switch_quantity = []
  for sc in plan.switch_classes {
    switch_quantity.push({
      class_id: sc.switch_class_id,
      quantity: compute_switch_qty(sc, plan.connections),
    })
  }
  // 2. server quantity — passthrough echo.
  let server_quantity = []
  for srv in plan.server_classes {
    server_quantity.push({ class_id: srv.server_class_id, quantity: srv.quantity })
  }
  // 3. per-realized-endpoint allocation + per-connection transceiver verdicts.
  let endpoints = []
  let verdicts = []
  let errors = []
  for sc in plan.switch_classes {
    let eff = qty_for(switch_quantity, sc.switch_class_id)
    for z in sc.zones {
      let zone_ports = expand_breakout(
        parse_port_spec(z.port_spec),
        z.breakout_logical_ports,
      )
      let zone_capacity = zone_ports.length()
      // One cursor per switch instance — port_allocator.py keys (switch, zone).
      let cursors = Array::make(if eff > 0 { eff } else { 1 }, 0)
      let mut overflowed = false
      // Allocation order — HNP's per-(switch,zone) linear cursor is consumed
      // server_class -> server_index -> connection(rail) -> ppc
      // (device_generator.py:390 _create_connections). Connections arrive already
      // sorted by server_class_id (calc.go BuildCalcPlan, Issue #61 fix 1), so
      // collecting the distinct classes in ARRIVAL order reproduces HNP's
      // alphabetical order; the shared `cursors` then fills consecutive slots per
      // server (Issue #61 fix 2). alloc_slot / f2_switch_index / expand_breakout
      // are unchanged — only the consumption order changes.
      let zone_classes = []
      for c in plan.connections {
        if c.target_switch_class == sc.switch_class_id && c.target_zone == z.zone_name {
          if contains_str(zone_classes, c.server_class_id) {
            ()
          } else {
            zone_classes.push(c.server_class_id)
          }
        }
      }
      for scid in zone_classes {
        // This class's connections to this zone, in arrival (rail) order.
        let conns_sc = []
        for c in plan.connections {
          if c.target_switch_class == sc.switch_class_id &&
            c.target_zone == z.zone_name &&
            c.server_class_id == scid {
            conns_sc.push(c)
          }
        }
        // server_quantity is identical across a class's connections.
        let sq = conns_sc[0].server_quantity
        // total_rails for THIS (server class, zone): one per rail-optimized
        // connection (_get_total_rails_for_target, device_generator.py:432-435).
        let mut total_rails = 0
        for c in conns_sc {
          if c.distribution == "rail-optimized" {
            total_rails = total_rails + 1
          }
        }
        for si in 0..<sq {
          for c in conns_sc {
            let ppc = if c.ports_per_connection > 0 {
              c.ports_per_connection
            } else {
              1
            }
            // Domain-based pre-compute (device_generator.py:438-443) — used only
            // on the n >= total_rails branch of f2_switch_index.
            let spd = servers_per_domain(zone_capacity, ppc)
            let rail = match c.rail {
              Some(r) => r
              None => 0
            }
            for pp in 0..<ppc {
              let port_index = c.port_index + pp
              let switch_index = f2_switch_index(
                c.distribution,
                port_index,
                si,
                rail,
                eff,
                total_rails,
                sq,
                spd,
              )
              let (slot, in_bounds) = alloc_slot(zone_ports, cursors, switch_index)
              endpoints.push({
                server_class_id: c.server_class_id,
                server_index: si,
                connection_id: c.connection_id,
                nic_slot_id: c.nic_slot_id,
                port_index,
                switch_class_id: c.target_switch_class,
                switch_index,
                zone: c.target_zone,
                port_slot: slot,
              })
              if in_bounds {
                ()
              } else {
                overflowed = true
              }
            }
          }
        }
      }
      // Transceiver verdicts — one per connection, in arrival order.
      for c in plan.connections {
        if c.target_switch_class == sc.switch_class_id && c.target_zone == z.zone_name {
          let (outcome, reason_code) = f2_xcvr_verdict(
            c.server_transceiver_attrs,
            z.transceiver_attrs,
          )
          verdicts.push({ connection_id: c.connection_id, outcome, reason_code })
        }
      }
      if overflowed {
        errors.push({
          code: "ZONE_OVERFLOW",
          message: "zone '" +
          z.zone_name +
          "' on switch class '" +
          sc.switch_class_id +
          "' over-allocated: demand exceeds " +
          zone_capacity.to_string() +
          " port slots",
        })
      }
    }
  }
  {
    switch_quantity,
    server_quantity,
    endpoints,
    transceiver_verdicts: verdicts,
    errors,
  }
}

// ----------------------------------------------------------------------
// calc-plan JSON decode / calc-output JSON encode (the F2 wire shapes,
// note §1.1). Reuses the package JSON primitives (d_obj/d_field/d_str/j_esc/
// j_arr) so the snake_case contract stays in one place.
// ----------------------------------------------------------------------

fn d_field_int(m : Map[String, Json], k : String) -> Int raise DecodeErr {
  let j = d_field(m, k)
  if j is Number(n, ..) {
    n.to_int()
  } else {
    raise DecodeErr("expected number: " + k)
  }
}

// Optional string field defaulting to "" when absent (mesh plans carry no
// fabric_name/role; the boundary stays backward-compatible — F6).
fn d_str_or_empty(m : Map[String, Json], k : String) -> String raise DecodeErr {
  match m.get(k) {
    Some(j) => d_str(j)
    None => ""
  }
}

fn d_opt_int(m : Map[String, Json], k : String) -> Int? raise DecodeErr {
  match m.get(k) {
    Some(j) => if j is Number(n, ..) { Some(n.to_int()) } else { None }
    None => None
  }
}

fn d_xcvr_opt(m : Map[String, Json], k : String) -> XcvrAttrs? raise DecodeErr {
  match m.get(k) {
    Some(j) =>
      if j is Object(om) {
        Some({
          medium: d_field_str(om, "medium"),
          cage_type: d_field_str(om, "cage_type"),
          connector: d_field_str(om, "connector"),
        })
      } else {
        None
      }
    None => None
  }
}

fn d_zone_in(j : Json) -> ZoneIn raise DecodeErr {
  let m = d_obj(j)
  {
    zone_name: d_field_str(m, "zone_name"),
    zone_type: d_field_str(m, "zone_type"),
    port_spec: d_field_str(m, "port_spec"),
    breakout_logical_ports: d_field_int(m, "breakout_logical_ports"),
    allocation_strategy: d_field_str(m, "allocation_strategy"),
    transceiver_attrs: d_xcvr_opt(m, "transceiver_attrs"),
  }
}

fn d_switch_class_in(j : Json) -> SwitchClassIn raise DecodeErr {
  let m = d_obj(j)
  let zones = []
  for el in d_arr_or_empty(m, "zones") {
    zones.push(d_zone_in(el))
  }
  {
    switch_class_id: d_field_str(m, "switch_class_id"),
    fabric_name: d_str_or_empty(m, "fabric_name"),
    role: d_str_or_empty(m, "role"),
    override_quantity: d_opt_int(m, "override_quantity"),
    redundancy: d_field_str(m, "redundancy"),
    topology_mode: d_field_str(m, "topology_mode"),
    zones,
  }
}

fn d_server_class_in(j : Json) -> ServerClassIn raise DecodeErr {
  let m = d_obj(j)
  {
    server_class_id: d_field_str(m, "server_class_id"),
    quantity: d_field_int(m, "quantity"),
  }
}

fn d_conn_in(j : Json) -> ConnIn raise DecodeErr {
  let m = d_obj(j)
  {
    connection_id: d_field_str(m, "connection_id"),
    server_class_id: d_field_str(m, "server_class_id"),
    server_quantity: d_field_int(m, "server_quantity"),
    nic_slot_id: d_field_str(m, "nic_slot_id"),
    port_index: d_field_int(m, "port_index"),
    ports_per_connection: d_field_int(m, "ports_per_connection"),
    speed: d_field_int(m, "speed"),
    distribution: d_field_str(m, "distribution"),
    rail: d_opt_int(m, "rail"),
    target_switch_class: d_field_str(m, "target_switch_class"),
    target_zone: d_field_str(m, "target_zone"),
    server_transceiver_attrs: d_xcvr_opt(m, "server_transceiver_attrs"),
  }
}

fn f2_decode_plan_inner(plan_json : String) -> CalcPlan raise {
  let j = @json.parse(plan_json)
  let m = d_obj(j)
  let scs = []
  for el in d_arr_or_empty(m, "switch_classes") {
    scs.push(d_switch_class_in(el))
  }
  let srvs = []
  for el in d_arr_or_empty(m, "server_classes") {
    srvs.push(d_server_class_in(el))
  }
  let conns = []
  for el in d_arr_or_empty(m, "connections") {
    conns.push(d_conn_in(el))
  }
  { switch_classes: scs, server_classes: srvs, connections: conns }
}

fn f2_j_class_qty(c : ClassQty) -> String {
  "{\"class_id\":" + j_esc(c.class_id) + ",\"quantity\":" + c.quantity.to_string() + "}"
}

fn f2_j_port_slot(p : PortSlot) -> String {
  let bi = match p.breakout_index {
    Some(i) => i.to_string()
    None => "null"
  }
  "{\"physical_port\":" +
  p.physical_port.to_string() +
  ",\"breakout_index\":" +
  bi +
  ",\"name\":" +
  j_esc(p.name) +
  "}"
}

fn f2_j_endpoint(e : Endpoint) -> String {
  "{\"server_class_id\":" +
  j_esc(e.server_class_id) +
  ",\"server_index\":" +
  e.server_index.to_string() +
  ",\"connection_id\":" +
  j_esc(e.connection_id) +
  ",\"nic_slot_id\":" +
  j_esc(e.nic_slot_id) +
  ",\"port_index\":" +
  e.port_index.to_string() +
  ",\"switch_class_id\":" +
  j_esc(e.switch_class_id) +
  ",\"switch_index\":" +
  e.switch_index.to_string() +
  ",\"zone\":" +
  j_esc(e.zone) +
  ",\"port_slot\":" +
  f2_j_port_slot(e.port_slot) +
  "}"
}

fn f2_j_verdict(v : Verdict) -> String {
  "{\"connection_id\":" +
  j_esc(v.connection_id) +
  ",\"outcome\":" +
  j_esc(v.outcome) +
  ",\"reason_code\":" +
  j_esc(v.reason_code) +
  "}"
}

fn f2_j_issue(e : CalcIssue) -> String {
  "{\"code\":" + j_esc(e.code) + ",\"message\":" + j_esc(e.message) + "}"
}

fn f2_encode_output(out : CalcOutput2) -> String {
  let sq = []
  for c in out.switch_quantity {
    sq.push(f2_j_class_qty(c))
  }
  let srq = []
  for c in out.server_quantity {
    srq.push(f2_j_class_qty(c))
  }
  let eps = []
  for e in out.endpoints {
    eps.push(f2_j_endpoint(e))
  }
  let vs = []
  for v in out.transceiver_verdicts {
    vs.push(f2_j_verdict(v))
  }
  let errs = []
  for e in out.errors {
    errs.push(f2_j_issue(e))
  }
  "{\"switch_quantity\":" +
  j_arr(sq) +
  ",\"server_quantity\":" +
  j_arr(srq) +
  ",\"endpoints\":" +
  j_arr(eps) +
  ",\"transceiver_verdicts\":" +
  j_arr(vs) +
  ",\"errors\":" +
  j_arr(errs) +
  "}"
}
