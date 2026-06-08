// Unit tests — invalid fixtures must produce structured ValidationResult.errors
// with identifiable codes (issue #6 condition 4), never panics or calc-error.
// Driven through the wire `validate(json)` entry point.

test "validate: mesh-four-switch emits MESH_SWITCH_COUNT error" {
  match validate(mesh_four_switch_plan_json) {
    Ok(vr) => {
      assert_false(vr.is_valid)
      assert_true(has_error_code(vr, code_mesh_switch_count))
    }
    Err(_) => assert_true(false) // domain issues must be data, not calc-error
  }
}

test "validate: mclag-odd-count emits MCLAG_SWITCH_COUNT error" {
  match validate(mclag_odd_count_plan_json) {
    Ok(vr) => {
      assert_false(vr.is_valid)
      assert_true(has_error_code(vr, code_mclag_switch_count))
    }
    Err(_) => assert_true(false)
  }
}
