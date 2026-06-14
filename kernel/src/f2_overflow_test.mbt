// F2 — allocation in-bounds guard (note §4; port_allocator.py:57-67; devb GREEN
// review finding 2). An overfull zone must SURFACE a ZONE_OVERFLOW error, not
// silently wrap/reuse ports; a zone that fits must report no error.

// Minimal substring search (no dependency on a stdlib String.contains shape).
fn contains_sub(haystack : String, needle : String) -> Bool {
  let h = haystack.length()
  let n = needle.length()
  if n == 0 {
    return true
  }
  let mut i = 0
  while i + n <= h {
    let mut j = 0
    let mut matched = true
    while j < n {
      if haystack[i + j] != needle[j] {
        matched = false
        break
      }
      j = j + 1
    }
    if matched {
      return true
    }
    i = i + 1
  }
  false
}

// One switch (override 1), one 2-port server zone (capacity 2), one same-switch
// connection fanning out across 5 server instances → 5 demanded ports on the one
// switch > 2 capacity → over-allocation.
let overfull_plan : String =
  #|{"switch_classes":[{"switch_class_id":"sw","override_quantity":1,"redundancy":"none","topology_mode":"clos","zones":[{"zone_name":"z","zone_type":"server","port_spec":"1-2","breakout_logical_ports":1,"allocation_strategy":"sequential"}]}],"server_classes":[{"server_class_id":"srv","quantity":5}],"connections":[{"connection_id":"c","server_class_id":"srv","server_quantity":5,"nic_slot_id":"n","port_index":0,"ports_per_connection":1,"speed":1,"distribution":"same-switch","target_switch_class":"sw","target_zone":"z"}]}

// Same shape but an 8-port zone (capacity 8) that comfortably fits the 5 ports.
let fits_plan : String =
  #|{"switch_classes":[{"switch_class_id":"sw","override_quantity":1,"redundancy":"none","topology_mode":"clos","zones":[{"zone_name":"z","zone_type":"server","port_spec":"1-8","breakout_logical_ports":1,"allocation_strategy":"sequential"}]}],"server_classes":[{"server_class_id":"srv","quantity":5}],"connections":[{"connection_id":"c","server_class_id":"srv","server_quantity":5,"nic_slot_id":"n","port_index":0,"ports_per_connection":1,"speed":1,"distribution":"same-switch","target_switch_class":"sw","target_zone":"z"}]}

test "F2 §4 over-allocation surfaces ZONE_OVERFLOW (no silent wrap)" {
  assert_eq(contains_sub(f2_calculate(overfull_plan), "ZONE_OVERFLOW"), true)
}

test "F2 §4 a zone that fits reports no allocation error" {
  let out = f2_calculate(fits_plan)
  assert_eq(contains_sub(out, "ZONE_OVERFLOW"), false)
  // the errors array is present and empty for a sound plan.
  assert_eq(contains_sub(out, "\"errors\":[]"), true)
}
