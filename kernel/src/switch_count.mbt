// Algorithms 1 & 2 — leaf and spine switch quantities (ALGORITHMS.md).

/// Integer ceiling division: ceil(numerator / denominator). denominator > 0.
/// The positive-divisor case routes through the proven `@proofs.ceil_div_pos`
/// core (lower-bound + minimality, Issue #7 I3); the defensive `denominator <= 0`
/// guard is kept here (returns 0, as before).
pub fn ceil_div(numerator : Int, denominator : Int) -> Int {
  if denominator <= 0 {
    0
  } else {
    @proofs.ceil_div_pos(numerator, denominator)
  }
}

/// Algorithm 1 — leaf switch quantity for a single (zone, speed) demand.
///
/// raw = ceil(demand_logical_ports / (zone_logical_capacity - uplink_reservation)),
/// then constraints: >= 2 when alternating; rounded up to even (and >= 2) when
/// MCLAG; clamped to [2, 4] when ESLAG.
/// Behavior-preserving routing through proven cores (Issue #7):
///   - base = ceil_div(demand, usable)              -> @proofs.ceil_div_pos lower bound
///   - non-ESLAG (None/MCLAG) adjustments           -> @proofs.leaf_adjust_non_eslag
///       monotone (result >= base, so the I3 lower bound `leaf_count >=
///       ceil_div` survives) and MCLAG output even & >= 2 (I6).
///   - ESLAG hard clamp to [2,4]                    -> @proofs.leaf_clamp_eslag (I3-ESLAG)
/// The Alternating "at least 2" bump (`force_two`) is folded into the non-ESLAG
/// core; for ESLAG the clamp subsumes it (a sub-2 base is raised to 2 either way),
/// so the result is identical to the original sequential adjustment.
pub fn leaf_count(
  demand_logical_ports : Int,
  zone_logical_capacity : Int,
  uplink_reservation : Int,
  distribution : Distribution,
  redundancy : Redundancy
) -> Int {
  let usable = zone_logical_capacity - uplink_reservation
  let base = ceil_div(demand_logical_ports, usable)
  let force_two = distribution == Alternating
  match redundancy {
    RedMclag(_) => @proofs.leaf_adjust_non_eslag(base, force_two, true)
    RedEslag(_) => @proofs.leaf_clamp_eslag(base)
    RedNone => @proofs.leaf_adjust_non_eslag(base, force_two, false)
  }
}

/// Algorithm 2 — spine switch quantity.
/// ceil(total_leaf_uplink_demand / spine_fabric_port_capacity).
pub fn spine_count(
  total_leaf_uplink_demand : Int,
  spine_fabric_port_capacity : Int
) -> Int {
  ceil_div(total_leaf_uplink_demand, spine_fabric_port_capacity)
}
