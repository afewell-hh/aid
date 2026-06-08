// Unit test — Algorithm 7 (oversubscription ratio) and the D11 warning rule.

test "Algorithm 7: oversubscription ratio" {
  assert_eq(oversubscription_ratio(800, 400), 2.0)
  assert_eq(oversubscription_ratio(400, 400), 1.0)
  assert_eq(oversubscription_ratio(3200, 12800), 0.25)
}

test "Algorithm 7: ratio > 1.0 is oversubscribed (warning per D11)" {
  assert_true(is_oversubscribed(2.0))
  assert_false(is_oversubscribed(1.0))
  assert_false(is_oversubscribed(0.5))
}
