// Issue #41 — ESLAG's [2,4] clamp must not silently under-provision a zone.
// Demand beyond four switches' worth of logical capacity gets a specific calc
// error instead of relying only on the generic allocation overflow backstop.

let eslag_over_capacity_plan : String =
  #|{"switch_classes":[{"switch_class_id":"sw","redundancy":"eslag","topology_mode":"clos","zones":[{"zone_name":"z","zone_type":"server","port_spec":"1-2","breakout_logical_ports":1,"allocation_strategy":"sequential"}]}],"server_classes":[{"server_class_id":"srv","quantity":9}],"connections":[{"connection_id":"c","server_class_id":"srv","server_quantity":9,"nic_slot_id":"n","port_index":0,"ports_per_connection":1,"speed":1,"distribution":"same-switch","target_switch_class":"sw","target_zone":"z"}]}

let eslag_fits_four_switches_plan : String =
  #|{"switch_classes":[{"switch_class_id":"sw","redundancy":"eslag","topology_mode":"clos","zones":[{"zone_name":"z","zone_type":"server","port_spec":"1-2","breakout_logical_ports":1,"allocation_strategy":"sequential"}]}],"server_classes":[{"server_class_id":"srv","quantity":8}],"connections":[{"connection_id":"c","server_class_id":"srv","server_quantity":8,"nic_slot_id":"n","port_index":0,"ports_per_connection":1,"speed":1,"distribution":"same-switch","target_switch_class":"sw","target_zone":"z"}]}

test "Issue #41: ESLAG over 4-switch capacity surfaces a specific error" {
  let out = f2_calculate(eslag_over_capacity_plan)
  assert_eq(contains_sub(out, "ESLAG_CAPACITY_EXCEEDED"), true)
}

test "Issue #41: ESLAG demand within 4-switch capacity has no ESLAG capacity error" {
  let out = f2_calculate(eslag_fits_four_switches_plan)
  assert_eq(contains_sub(out, "ESLAG_CAPACITY_EXCEEDED"), false)
}
