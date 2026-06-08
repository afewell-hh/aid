// Algorithms 1 & 2 — leaf and spine switch quantities (ALGORITHMS.md).

/// Integer ceiling division: ceil(numerator / denominator). denominator > 0.
pub fn ceil_div(numerator : Int, denominator : Int) -> Int {
  if denominator <= 0 {
    0
  } else {
    (numerator + denominator - 1) / denominator
  }
}

/// Algorithm 1 — leaf switch quantity for a single (zone, speed) demand.
///
/// raw = ceil(demand_logical_ports / (zone_logical_capacity - uplink_reservation)),
/// then constraints: >= 2 when alternating; rounded up to even (and >= 2) when
/// MCLAG; clamped to [2, 4] when ESLAG.
pub fn leaf_count(
  demand_logical_ports : Int,
  zone_logical_capacity : Int,
  uplink_reservation : Int,
  distribution : Distribution,
  redundancy : Redundancy
) -> Int {
  let usable = zone_logical_capacity - uplink_reservation
  let mut count = ceil_div(demand_logical_ports, usable)
  if distribution == Alternating && count < 2 {
    count = 2
  }
  match redundancy {
    RedMclag(_) => {
      if count < 2 {
        count = 2
      }
      if count % 2 != 0 {
        count = count + 1
      }
    }
    RedEslag(_) => {
      if count < 2 {
        count = 2
      }
      if count > 4 {
        count = 4
      }
    }
    RedNone => ()
  }
  count
}

/// Algorithm 2 — spine switch quantity.
/// ceil(total_leaf_uplink_demand / spine_fabric_port_capacity).
pub fn spine_count(
  total_leaf_uplink_demand : Int,
  spine_fabric_port_capacity : Int
) -> Int {
  ceil_div(total_leaf_uplink_demand, spine_fabric_port_capacity)
}
