// Unit tests — Algorithms 1 (leaf) & 2 (spine) and the ceil_div helper.
// Expected values are derived from ALGORITHMS.md and the clos-small fixture
// calculation_notes.

test "ceil_div rounds up" {
  assert_eq(ceil_div(8, 48), 1)
  assert_eq(ceil_div(48, 48), 1)
  assert_eq(ceil_div(49, 48), 2)
  assert_eq(ceil_div(0, 48), 0)
  assert_eq(ceil_div(16, 32), 1)
}

test "Algorithm 1: leaf_count base demand and alternating >= 2 floor" {
  // clos-small: demand 8 logical ports into a 48-port zone, no reservation.
  // ceil(8/48) = 1, raised to 2 by the alternating-distribution floor.
  assert_eq(leaf_count(8, 48, 0, Alternating, RedNone), 2)
  // Without the alternating floor, same-switch keeps the raw ceil.
  assert_eq(leaf_count(8, 48, 0, SameSwitch, RedNone), 1)
  assert_eq(leaf_count(100, 48, 0, SameSwitch, RedNone), 3)
}

test "Algorithm 1: uplink reservation reduces usable capacity" {
  // usable = 56 - 8 = 48; ceil(48/48) = 1.
  assert_eq(leaf_count(48, 56, 8, SameSwitch, RedNone), 1)
  // ceil(49/48) = 2.
  assert_eq(leaf_count(49, 56, 8, SameSwitch, RedNone), 2)
}

test "Algorithm 1: MCLAG rounds up to even" {
  // ceil(100/48) = 3 -> next even = 4.
  assert_eq(leaf_count(100, 48, 0, SameSwitch, RedMclag({ switch_count: 2 })), 4)
  // ceil(48/48) = 1 -> floored to 2 (even and >= 2).
  assert_eq(leaf_count(48, 48, 0, SameSwitch, RedMclag({ switch_count: 2 })), 2)
}

test "Algorithm 1: ESLAG clamps to [2, 4]" {
  // ceil(1000/48) = 21 -> clamped to 4.
  assert_eq(leaf_count(1000, 48, 0, SameSwitch, RedEslag({ switch_count: 2 })), 4)
  // ceil(1/48) = 1 -> clamped up to 2.
  assert_eq(leaf_count(1, 48, 0, SameSwitch, RedEslag({ switch_count: 2 })), 2)
}

test "Algorithm 2: spine_count" {
  // clos-small: 2 leaves x 8 uplinks = 16 into a 32-port spine fabric zone.
  assert_eq(spine_count(16, 32), 1)
  assert_eq(spine_count(40, 32), 2)
  assert_eq(spine_count(32, 32), 1)
}
