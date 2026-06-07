// Unit tests for the spike allocator model (`moon test`).
// These are executable checks; the formal guarantees live in `moon prove`.

test "allocate satisfies full demand when it fits" {
  assert_eq(allocate(48, 10), 10)
}

test "allocate clamps to capacity when over-subscribed" {
  assert_eq(allocate(48, 100), 48)
}

test "allocate handles zero demand" {
  assert_eq(allocate(48, 0), 0)
}

test "allocate fills the zone exactly at capacity" {
  assert_eq(allocate(32, 32), 32)
}

test "distinct connection offsets never collide on a port" {
  assert_true(non_overlap_holds(0, 3, 7))
  assert_true(non_overlap_holds(16, 0, 31))
}
