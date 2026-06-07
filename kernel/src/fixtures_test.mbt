// Fixture-acceptance tests — drive the full `calculate` path on each valid
// fixture and assert the EXACT baselines from tests/fixtures/*/expected.json
// (and the issue #6 fixture table). The kernel stays pure: the embedded JSON
// (fixtures_gen.mbt) is passed as a string; no file I/O here.

fn calc(json : String) -> CalcOutput {
  match calculate(json) {
    Ok(o) => o
    Err(e) => abort("calculate error: " + calc_error_msg(e))
  }
}

fn has_oversub(out : CalcOutput, fabric : String) -> Bool {
  match fabric_oversub(out, fabric) {
    Some(_) => true
    None => false
  }
}

// ---------------------------------------------------------------- clos-small

test "clos-small: device, cable and per-fabric switch counts" {
  let out = calc(clos_small_plan_json)
  assert_eq(device_count(out), 7)
  assert_eq(cable_count(out), 24)
  assert_eq(switch_count_for(out, "frontend", "server-leaf"), 2)
  assert_eq(switch_count_for(out, "frontend", "spine"), 1)
}

test "clos-small: recursive BOM per-unit and fleet totals" {
  let out = calc(clos_small_plan_json)
  // per-unit (gpu-server root excluded; recursion through nic -> xcvr)
  assert_eq(bom_out_per_unit(out, "gpu-servers", "server-chassis"), 1)
  assert_eq(bom_out_per_unit(out, "gpu-servers", "h100-gpu"), 8)
  assert_eq(bom_out_per_unit(out, "gpu-servers", "connectx-7-nic"), 2)
  assert_eq(bom_out_per_unit(out, "gpu-servers", "osfp-400g-xcvr"), 2)
  // fleet (x plan_quantity 4)
  assert_eq(bom_out_fleet(out, "gpu-servers", "server-chassis"), 4)
  assert_eq(bom_out_fleet(out, "gpu-servers", "h100-gpu"), 32)
  assert_eq(bom_out_fleet(out, "gpu-servers", "connectx-7-nic"), 8)
  assert_eq(bom_out_fleet(out, "gpu-servers", "osfp-400g-xcvr"), 8)
  // switch entries include the switch SKU itself
  assert_eq(bom_out_fleet(out, "server-leafs", "leaf-switch"), 2)
  assert_eq(bom_out_fleet(out, "spines", "spine-switch"), 1)
}

test "clos-small: oversubscription summary present for the fabric" {
  let out = calc(clos_small_plan_json)
  assert_true(has_fabric_summary(out, "frontend"))
  assert_true(has_oversub(out, "frontend"))
}

// ------------------------------------------------------------ mesh-two-switch

test "mesh-two-switch: device, cable and switch counts" {
  let out = calc(mesh_two_switch_plan_json)
  assert_eq(device_count(out), 4)
  assert_eq(cable_count(out), 6) // 4 mesh + 2 server-to-switch
  assert_eq(switch_count_for(out, "converged", "server-leaf"), 2)
}

test "mesh-two-switch: recursive BOM fleet totals" {
  let out = calc(mesh_two_switch_plan_json)
  assert_eq(bom_out_fleet(out, "mesh-servers", "cx7-single-nic"), 2)
  assert_eq(bom_out_fleet(out, "mesh-servers", "qsfp112-200g-xcvr"), 2)
  assert_eq(bom_out_fleet(out, "mesh-switch-a", "mesh-switch"), 1)
  assert_eq(bom_out_fleet(out, "mesh-switch-b", "mesh-switch"), 1)
}

test "mesh-two-switch: oversubscription summary present for the fabric" {
  let out = calc(mesh_two_switch_plan_json)
  assert_true(has_fabric_summary(out, "converged"))
  assert_true(has_oversub(out, "converged"))
}

// ----------------------------------------------------------------- switch-bom

test "switch-bom: device and switch counts, no cables" {
  let out = calc(switch_bom_plan_json)
  assert_eq(device_count(out), 2)
  assert_eq(cable_count(out), 0)
  assert_eq(switch_count_for(out, "bom-fabric", "server-leaf"), 2)
}

test "switch-bom: non-server recursive BOM includes the switch SKU" {
  let out = calc(switch_bom_plan_json)
  assert_eq(bom_out_per_unit(out, "leaf-switches", "leaf-switch-800g"), 1)
  assert_eq(bom_out_per_unit(out, "leaf-switches", "osfp-800g-xcvr"), 64)
  assert_eq(bom_out_fleet(out, "leaf-switches", "leaf-switch-800g"), 2)
  assert_eq(bom_out_fleet(out, "leaf-switches", "osfp-800g-xcvr"), 128) // 64 x 2
}

test "switch-bom: oversubscription summary present for the fabric" {
  let out = calc(switch_bom_plan_json)
  assert_true(has_fabric_summary(out, "bom-fabric"))
  assert_true(has_oversub(out, "bom-fabric"))
}
