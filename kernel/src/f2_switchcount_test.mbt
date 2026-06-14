// F2 RED — switch-count derivation fixtures (note §2.1, §3).
//   effective_quantity: topology_plans.py:577-589
//   calculated_quantity: topology_calculations.py:558-648,655-662
// FAIL against the f2_calc.mbt stubs (return -1) until GREEN.

test "F2 §2.1 effective_quantity: override wins (soc_storage_scale_out_leaf = 2)" {
  assert_eq(effective_quantity(Some(2), 99), 2)
}

test "F2 §2.1 effective_quantity: no override -> calculated" {
  assert_eq(effective_quantity(None, 1), 1)
}

test "F2 §2.1 calculated_quantity: xoc-64 derived classes both ceil=1" {
  // inb_mgmt_leaf: demand 17 servers, zone 1-24 (×b_1x25=1) = 24/switch
  assert_eq(calculated_quantity(17, 24, false, "none"), 1)
  // oob_leaf: demand 17, zone 1-48 (×b_1x1=1) = 48/switch
  assert_eq(calculated_quantity(17, 48, false, "none"), 1)
}

// The lead's main caveat (note §3): exercise the non-override path at
// calculated > 1, not only the xoc-64 ceil=1 values.
test "F2 §3 calculated_quantity: derived_multi > 1 (ceil(10/4)=3)" {
  assert_eq(calculated_quantity(10, 4, false, "none"), 3)
}

test "F2 §2.1 calculated_quantity: alternating floor forces >= 2" {
  // demand fits one switch, but an alternating connection requires >= 2
  assert_eq(calculated_quantity(1, 48, true, "none"), 2)
}

test "F2 §2.1 calculated_quantity: MCLAG rounds up to even (ceil=3 -> 4)" {
  assert_eq(calculated_quantity(10, 4, false, "mclag"), 4)
}
