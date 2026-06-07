// Algorithms 1 & 2 — leaf and spine switch quantities (ALGORITHMS.md).
//
// RED phase: stubs return placeholder values so the unit tests fail on their
// assertions (not on missing symbols). GREEN replaces the bodies.

/// Integer ceiling division: ceil(numerator / denominator). denominator > 0.
/// Underpins both switch-count algorithms.
pub fn ceil_div(numerator : Int, denominator : Int) -> Int {
  let _ = numerator
  let _ = denominator
  -1
}

/// Algorithm 1 — leaf switch quantity for a single (zone, speed) demand.
///
/// raw = ceil(demand_logical_ports / (zone_logical_capacity - uplink_reservation)),
/// then constraints: >= 2 when alternating; rounded up to even when MCLAG;
/// clamped to [2, 4] when ESLAG. (The max-over-(zone, speed) combination across
/// multiple demands is composed by the caller.)
pub fn leaf_count(
  demand_logical_ports : Int,
  zone_logical_capacity : Int,
  uplink_reservation : Int,
  distribution : Distribution,
  redundancy : Redundancy
) -> Int {
  let _ = demand_logical_ports
  let _ = zone_logical_capacity
  let _ = uplink_reservation
  let _ = distribution
  let _ = redundancy
  -1
}

/// Algorithm 2 — spine switch quantity.
/// ceil(total_leaf_uplink_demand / spine_fabric_port_capacity).
pub fn spine_count(
  total_leaf_uplink_demand : Int,
  spine_fabric_port_capacity : Int
) -> Int {
  let _ = total_leaf_uplink_demand
  let _ = spine_fabric_port_capacity
  -1
}
