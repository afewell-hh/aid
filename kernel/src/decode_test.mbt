// Unit tests — JSON -> TopologyPlan decoder (D16 ingress edge).
// Uses the embedded fixture plan JSON (snake_case plan-schema shape).

test "decode: clos-small plan JSON decodes to a typed plan" {
  match decode_plan(clos_small_plan_json) {
    Ok(plan) => {
      assert_eq(plan.id, "clos-small")
      assert_eq(plan.customer_name, "Reference Lab")
      assert_eq(plan.device_catalog.length(), 7)
      assert_eq(plan.entries.length(), 3)
      assert_eq(plan.fabric_domains.length(), 1)
    }
    Err(_) => assert_true(false) // RED: decoder unimplemented
  }
}

test "decode: nested enums, options and numbers round-trip" {
  match decode_plan(clos_small_plan_json) {
    Ok(plan) => {
      let e0 = plan.entries[0]
      assert_eq(e0.entry_id, "gpu-servers")
      assert_eq(e0.quantity.reinterpret_as_int(), 4)
      assert_true(e0.role == PlanRole::Server)
      assert_eq(e0.connections.length(), 2)
      let c0 = e0.connections[0]
      assert_true(c0.distribution == Alternating)
      assert_eq(c0.speed_gbps.reinterpret_as_int(), 400)
    }
    Err(_) => assert_true(false)
  }
}

test "decode: malformed JSON yields invalid-plan, never a panic" {
  match decode_plan("{ not valid json ]") {
    Ok(_) => assert_true(false) // must not accept garbage
    Err(InvalidPlan(_)) => assert_true(true)
    Err(Internal(_)) => assert_true(false) // wrong error arm
  }
}
