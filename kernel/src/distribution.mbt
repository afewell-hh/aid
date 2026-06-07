// Algorithms 3 & 4 — switch distribution (ALGORITHMS.md).
//
// RED phase: stubs return -1 so unit tests fail on assertions.

/// Algorithm 4 — alternating (round-robin) distribution.
/// switch_index = port_index % effective_quantity.
pub fn switch_index_alternating(port_index : Int, effective_quantity : Int) -> Int {
  let _ = port_index
  let _ = effective_quantity
  -1
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
  let _ = server_index
  let _ = ports_per_connection
  let _ = port_index
  let _ = ports_per_switch_zone
  -1
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
  let _ = rail
  let _ = server_index
  let _ = effective_quantity
  let _ = total_rails
  -1
}
