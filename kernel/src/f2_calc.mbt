// F2 calculation kernel — the re-derived calc functions (HNP diet engine).
//
// Re-derived against the real engine (cited per function); NOT the invented
// ALGORITHMS.md. See docs/foundation/f2-architecture-note.md §2 and Issue #52.
//
// F2 RED: every function below is a STUB returning an obviously-wrong sentinel so
// the fixtures in f2_*_test.mbt fail "for the right reason" (computed != expected).
// GREEN replaces the bodies; the moon-prove cores they route through
// (`@proofs.ceil_div_pos`, `@proofs.ports_distinct`, `@proofs.alloc_end_in_bounds`,
// `@proofs.leaf_adjust_non_eslag`) are already proved.

// ----------------------------------------------------------------------
// §2.2 Port-spec parsing — port_specification.py:39-77
//   comma-list + range (`start-end`) + stepped range (`start-end:step`);
//   dedup + sorted ascending.
// ----------------------------------------------------------------------

/// Parse a `port_spec` into the sorted, de-duplicated list of physical ports.
/// xoc-64 exercises all three forms: `26,28,30,32,34,36,38-63`, `1-16`, `1-32:2`.
pub fn parse_port_spec(_spec : String) -> Array[Int] {
  [] // F2 RED stub
}

// ----------------------------------------------------------------------
// §2.3 Breakout expansion — port_allocator.py:91-115
//   logical_ports == 1 -> PortSlot(p, None, "E1/{p}")
//   logical_ports  > 1 -> lanes 1..N -> PortSlot(p, lane, "E1/{p}/{lane}")
// ----------------------------------------------------------------------

pub fn expand_breakout(
  _physical_ports : Array[Int],
  _logical_ports : Int
) -> Array[PortSlot] {
  [] // F2 RED stub
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
pub fn effective_quantity(_override_qty : Int?, _calculated : Int) -> Int {
  -1 // F2 RED stub
}

/// calculated_quantity for one server zone: ceil(demand / logical_per_switch),
/// then the alternating "≥2" / MCLAG even-rounding floor. `redundancy` ∈
/// {none,mclag,eslag}; `alternating` is true if any feeding connection is
/// alternating. MUST be exercised at calculated > 1 (note §3, the lead caveat),
/// not only the xoc-64 ceil=1 values.
pub fn calculated_quantity(
  _demand : Int,
  _logical_per_switch : Int,
  _alternating : Bool,
  _redundancy : String
) -> Int {
  -1 // F2 RED stub
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

pub fn f2_switch_index(
  _distribution : String,
  _port_index : Int,
  _server_index : Int,
  _rail : Int,
  _effective_quantity : Int,
  _total_rails : Int,
  _total_servers : Int,
  _servers_per_domain : Int
) -> Int {
  -1 // F2 RED stub
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
  _server : XcvrAttrs?,
  _zone : XcvrAttrs?
) -> (String, String) {
  ("", "") // F2 RED stub: (outcome, reason_code)
}

// ----------------------------------------------------------------------
// Integration — one connection definition fans out across `server_quantity`
// server instances into per-realized-endpoint records (devb finding 1). Each
// endpoint names the server instance, the chosen switch instance, and the
// allocated port slot. (`ports_per_connection > 1` expands across consecutive
// port_index lanes; xoc-64's soc-storage uses ppc=2.)
// ----------------------------------------------------------------------

pub fn f2_endpoints_for_connection(
  _conn : ConnIn,
  _effective_quantity : Int,
  _total_rails : Int,
  _servers_per_domain : Int,
  _zone_ports : Array[PortSlot]
) -> Array[Endpoint] {
  [] // F2 RED stub
}

// ----------------------------------------------------------------------
// Boundary entry — JSON calc-plan string in, JSON calc-output string out
// (D16; wired to a new wasm export in GREEN). RED stub returns empty.
// ----------------------------------------------------------------------

pub fn f2_calculate(_calc_plan_json : String) -> String {
  "" // F2 RED stub
}
