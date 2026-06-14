// F2 RED — distribution fixtures (note §2.4; device_generator.py:1242-1321).
// The existing distribution.mbt uses the INVENTED-model formulas; F2 re-derives
// the HNP-correct mapping. FAIL against the f2_calc.mbt stub (returns -1) until
// GREEN. (`_` args are unused by the strategy under test.)

test "F2 §2.4 alternating: switch[port_index % n]" {
  assert_eq(f2_switch_index("alternating", 3, 0, 0, 2, 0, 0, 0), 1) // 3 % 2
  assert_eq(f2_switch_index("alternating", 4, 0, 0, 2, 0, 0, 0), 0) // 4 % 2
}

test "F2 §2.4 same-switch: contiguous server partition" {
  // total_servers 4, n 2 -> servers {0,1}->sw0, {2,3}->sw1
  assert_eq(f2_switch_index("same-switch", 0, 1, 0, 2, 0, 4, 0), 0)
  assert_eq(f2_switch_index("same-switch", 0, 2, 0, 2, 0, 4, 0), 1)
}

test "F2 §2.4 rail-optimized capacity-sharing: n < total_rails (xoc-64 scale-out)" {
  // n 2 < total_rails 8 -> rails_per_switch = ceil(8/2) = 4 -> rail // 4
  assert_eq(f2_switch_index("rail-optimized", 0, 0, 3, 2, 8, 8, 0), 0) // rails 0-3 -> sw0
  assert_eq(f2_switch_index("rail-optimized", 0, 0, 4, 2, 8, 8, 0), 1) // rails 4-7 -> sw1
}

test "F2 §2.4 rail-optimized domain-based: n >= total_rails" {
  // n 6 >= total_rails 3, servers_per_domain 1 ->
  //   switch_index = (server_index // spd) * total_rails + rail
  assert_eq(f2_switch_index("rail-optimized", 0, 0, 2, 6, 3, 6, 1), 2) // (0//1)*3+2
  assert_eq(f2_switch_index("rail-optimized", 0, 1, 0, 6, 3, 6, 1), 3) // (1//1)*3+0
}
