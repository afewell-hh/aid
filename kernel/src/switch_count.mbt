// Algorithm 2 — spine switch quantity + the shared ceil_div helper (ALGORITHMS.md).
//
// (Algorithm 1 `leaf_count` was retired with the legacy topology-calculator path in
// #85; the live F2 kernel derives leaf quantities in f2_calc.mbt, routing the same
// I3/I6 redundancy floors through @proofs.leaf_adjust_non_eslag / leaf_clamp_eslag
// directly. `ceil_div` and `spine_count` below remain live — F2 uses both.)

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

/// Algorithm 2 — spine switch quantity.
/// ceil(total_leaf_uplink_demand / spine_fabric_port_capacity).
pub fn spine_count(
  total_leaf_uplink_demand : Int,
  spine_fabric_port_capacity : Int
) -> Int {
  ceil_div(total_leaf_uplink_demand, spine_fabric_port_capacity)
}
