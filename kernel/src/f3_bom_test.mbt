// F3 BOM-scale tests (Issue #56). Verify the scaling primitive `f3_scale` and the
// `f3_bom` walk route through the proven I4 cores (@proofs.child_qpu/fleet_quantity)
// and reproduce the BOM-scaling identity for the real-model shapes.

test "f3_scale routes a root line through the cores" {
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
  let (qpu, fleet) = f3_scale(1, 24, 3)
  assert_eq(qpu, 24)
  assert_eq(fleet, 72)
  // Nested: a cage under the 8× CX-7 (parent qpu 8), one per CX-7 → qpu 8.
  let (cage_qpu, cage_fleet) = f3_scale(8, 1, 2)
  assert_eq(cage_qpu, 8)
  assert_eq(cage_fleet, 16)
}

test "f3_bom empty plan -> empty well-formed output" {
  assert_eq(f3_bom("{\"nodes\":[]}"), "[]")
  assert_eq(f3_bom("not json"), "[]")
}

test "f3_bom walks a root + nested slot + nested line through the cores" {
  // root server (plan_quantity 1), an 8× CX-7 slot, the CX-7's own AOC line (qpi 1).
  // Fleets: root 1×1=1, slot 8×1=8, AOC child_qpu(8,1)=8 → fleet 8.
  let plan =
    #|{"nodes":[
    #|{"parent_index":-1,"quantity_per_parent":1,"plan_quantity":1},
    #|{"parent_index":0,"quantity_per_parent":8,"plan_quantity":1},
    #|{"parent_index":1,"quantity_per_parent":1,"plan_quantity":1}
    #|]}
  assert_eq(f3_bom(plan), "[1,8,8]")
}

test "f3_bom scales the same tree at 2x linearly" {
  let plan =
    #|{"nodes":[
    #|{"parent_index":-1,"quantity_per_parent":1,"plan_quantity":2},
    #|{"parent_index":0,"quantity_per_parent":8,"plan_quantity":2}
    #|]}
  assert_eq(f3_bom(plan), "[2,16]")
}
