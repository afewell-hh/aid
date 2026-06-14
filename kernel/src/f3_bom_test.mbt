// F3 BOM-scale tests (Issue #56). Verify the scaling primitive `f3_scale` routes
// through the proven I4 cores (@proofs.child_qpu/fleet_quantity) and reproduces
// the BOM-scaling identity for the real-model shapes.

test "f3_scale routes a root line through the cores" {
  // Root: parent_qpu = 1, qpp = 1, plan_quantity = 8 servers.
  let (qpu, fleet) = f3_scale(1, 1, 8)
  assert_eq(qpu, 1)
  assert_eq(fleet, 8)
}

test "f3_scale scales the 8x CX-7 quantity-bearing slot" {
  // 8× CX-7 slot on a server with plan_quantity 2 → qpu 8, fleet 16 (linear).
  let (qpu, fleet) = f3_scale(1, 8, 2)
  assert_eq(qpu, 8)
  assert_eq(fleet, 16)
}

test "f3_scale composes child-of-child qpu (24x memory)" {
  // 24× memory line under a root server (qpu 1), plan_quantity 3 → qpu 24, fleet 72.
  let (qpu, fleet) = f3_scale(1, 24, 3)
  assert_eq(qpu, 24)
  assert_eq(fleet, 72)
  // Nested: a cage under the 8× CX-7 (parent qpu 8), one per CX-7 → qpu 8.
  let (cage_qpu, cage_fleet) = f3_scale(8, 1, 2)
  assert_eq(cage_qpu, 8)
  assert_eq(cage_fleet, 16)
}

test "f3_bom is a RED stub (empty well-formed output)" {
  assert_eq(f3_bom("{\"nodes\":[]}"), "{\"lines\":[]}")
}
