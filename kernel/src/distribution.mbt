// Algorithms 3 & 4 — switch distribution (ALGORITHMS.md).

/// Algorithm 4 — alternating (round-robin) distribution.
/// switch_index = port_index % effective_quantity.
pub fn switch_index_alternating(port_index : Int, effective_quantity : Int) -> Int {
  if effective_quantity <= 0 {
    0
  } else {
    port_index % effective_quantity
  }
}

/// Algorithm 3 sub-case A — rail-optimized, capacity-sharing
/// (effective_quantity < total_rails):
/// switch_index = floor((server_index * ports_per_connection + port_index)
///                      / ports_per_switch_zone).
pub fn switch_index_rail_shared(
  server_index : Int,
  ports_per_connection : Int,
  port_index : Int,
  ports_per_switch_zone : Int
) -> Int {
  if ports_per_switch_zone <= 0 {
    0
  } else {
    (server_index * ports_per_connection + port_index) / ports_per_switch_zone
  }
}

/// Algorithm 3 sub-case B — rail-optimized, domain-based
/// (effective_quantity >= total_rails):
/// servers_per_domain = floor(effective_quantity / total_rails);
/// switch_index = rail * servers_per_domain + floor(server_index / servers_per_domain).
pub fn switch_index_rail_domain(
  rail : Int,
  server_index : Int,
  effective_quantity : Int,
  total_rails : Int
) -> Int {
  if total_rails <= 0 {
    0
  } else {
    let servers_per_domain = effective_quantity / total_rails
    if servers_per_domain <= 0 {
      rail
    } else {
      rail * servers_per_domain + server_index / servers_per_domain
    }
  }
}
