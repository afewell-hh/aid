// Unit tests — Algorithms 3 (rail-optimized) & 4 (alternating).
// Expected values derived from the formulas in ALGORITHMS.md.

test "Algorithm 4: alternating round-robin" {
  assert_eq(switch_index_alternating(0, 2), 0)
  assert_eq(switch_index_alternating(1, 2), 1)
  assert_eq(switch_index_alternating(2, 2), 0)
  assert_eq(switch_index_alternating(3, 2), 1)
  assert_eq(switch_index_alternating(5, 4), 1)
}

test "Algorithm 3A: rail-optimized capacity-sharing" {
  // floor((server_index * ports_per_connection + port_index) / ports_per_switch_zone)
  assert_eq(switch_index_rail_shared(0, 8, 0, 16), 0)
  assert_eq(switch_index_rail_shared(2, 8, 0, 16), 1)
  assert_eq(switch_index_rail_shared(3, 8, 7, 16), 1)
  assert_eq(switch_index_rail_shared(4, 8, 0, 16), 2)
}

test "Algorithm 3B: rail-optimized domain-based" {
  // servers_per_domain = floor(effective_quantity / total_rails)
  // switch_index = rail * servers_per_domain + floor(server_index / servers_per_domain)
  assert_eq(switch_index_rail_domain(0, 0, 8, 4), 0)
  assert_eq(switch_index_rail_domain(1, 0, 8, 4), 2)
  assert_eq(switch_index_rail_domain(3, 1, 8, 4), 6)
  assert_eq(switch_index_rail_domain(2, 3, 8, 4), 5)
}
