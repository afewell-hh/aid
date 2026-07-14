// Unit tests — Algorithm 2 (spine) and the ceil_div helper. (Algorithm 1
// leaf_count was retired with the legacy path in #85; the live F2 leaf derivation
// is covered by f2_switchcount_test.mbt.) Expected values are derived from
// ALGORITHMS.md and the clos-small fixture calculation_notes.

test "ceil_div rounds up" {
  assert_eq(ceil_div(8, 48), 1)
  assert_eq(ceil_div(48, 48), 1)
  assert_eq(ceil_div(49, 48), 2)
  assert_eq(ceil_div(0, 48), 0)
  assert_eq(ceil_div(16, 32), 1)
}

test "Algorithm 2: spine_count" {
  // clos-small: 2 leaves x 8 uplinks = 16 into a 32-port spine fabric zone.
  assert_eq(spine_count(16, 32), 1)
  assert_eq(spine_count(40, 32), 2)
  assert_eq(spine_count(32, 32), 1)
}
