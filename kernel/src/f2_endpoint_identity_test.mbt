// F2 RED — endpoint identity across the boundary (note §1.1; devb finding 1).
// One connection definition fans out across `server_quantity` server instances;
// the output must name WHICH server instance got WHICH switch instance + port
// slot. FAIL against the f2_calc.mbt stub (returns []) until GREEN.

// Four allocated slots for a 48-port OOB zone (only the first four are needed).
fn oob_zone_ports() -> Array[PortSlot] {
  [
    PortSlot::{ physical_port: 1, breakout_index: None, name: "E1/1" },
    PortSlot::{ physical_port: 2, breakout_index: None, name: "E1/2" },
    PortSlot::{ physical_port: 3, breakout_index: None, name: "E1/3" },
    PortSlot::{ physical_port: 4, breakout_index: None, name: "E1/4" },
  ]
}

test "F2 §1.1 same-switch fan-out preserves per-instance identity" {
  let conn : ConnIn = {
    connection_id: "oob-mgmt",
    server_class_id: "compute_xpu",
    server_quantity: 4,
    nic_slot_id: "bmc",
    port_index: 0,
    ports_per_connection: 1,
    speed: 1,
    distribution: "same-switch",
    rail: None,
    target_switch_class: "oob_leaf",
    target_zone: "oob_server_1g",
  }
  // effective_quantity 2 so the partition splits {0,1}->sw0, {2,3}->sw1.
  let eps = f2_endpoints_for_connection(conn, 2, 0, 0, oob_zone_ports())
  assert_eq(eps.length(), 4) // one realized endpoint per server instance
  // server instance 0 -> switch 0, identity fields carried through
  assert_eq(eps[0].server_index, 0)
  assert_eq(eps[0].switch_index, 0)
  assert_eq(eps[0].connection_id, "oob-mgmt")
  assert_eq(eps[0].nic_slot_id, "bmc")
  // server instance 2 -> switch 1 (distinct instance -> distinct switch)
  assert_eq(eps[2].server_index, 2)
  assert_eq(eps[2].switch_index, 1)
}

test "F2 §1.1 rail-optimized fan-out maps rails to distinct switches" {
  let conn : ConnIn = {
    connection_id: "scale-out-rail-4",
    server_class_id: "compute_xpu",
    server_quantity: 1,
    nic_slot_id: "scale_out",
    port_index: 0,
    ports_per_connection: 1,
    speed: 400,
    distribution: "rail-optimized",
    rail: Some(4),
    target_switch_class: "soc_storage_scale_out_leaf",
    target_zone: "scale_out_server_2x400",
  }
  // n 2 < total_rails 8 -> rail 4 lands on switch 1 (rail // ceil(8/2)).
  let eps = f2_endpoints_for_connection(conn, 2, 8, 0, oob_zone_ports())
  assert_eq(eps.length(), 1)
  assert_eq(eps[0].switch_index, 1)
  assert_eq(eps[0].connection_id, "scale-out-rail-4")
}
