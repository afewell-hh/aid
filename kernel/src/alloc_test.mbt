// Unit tests — Algorithm 8 (zone-cursor port allocation + pre-flight check).

test "Algorithm 8: port range parsing" {
  assert_eq(port_range_count("1-48"), 48)
  assert_eq(port_range_count("49-56"), 8)
  assert_eq(port_range_count("33-36"), 4)
  assert_eq(port_range_count("1-32"), 32)
  assert_eq(port_range_count("1-64"), 64)
}

test "Algorithm 8: sequential allocation advances the cursor (non-overlap)" {
  assert_eq(allocate_sequential(0, 5), [0, 1, 2, 3, 4])
  assert_eq(allocate_sequential(48, 3), [48, 49, 50])
  // empty demand yields no ports.
  assert_eq(allocate_sequential(0, 0), [])
}

test "Algorithm 8: pre-flight capacity check" {
  assert_true(zone_capacity_ok(8, 48))
  assert_true(zone_capacity_ok(48, 48))
  assert_false(zone_capacity_ok(50, 48))
}
