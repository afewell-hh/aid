// F2 RED — breakout expansion fixtures (note §2.3; port_allocator.py:91-115).
// FAIL against the f2_calc.mbt stub (returns []) until GREEN.

test "F2 §2.3 expand_breakout: logical_ports == 1 -> E1/{port}, no lane" {
  assert_eq(expand_breakout([27], 1), [
    PortSlot::{ physical_port: 27, breakout_index: None, name: "E1/27" },
  ])
}

test "F2 §2.3 expand_breakout: logical_ports == 4 -> lanes E1/{port}/{1..4}" {
  assert_eq(expand_breakout([27], 4), [
    PortSlot::{ physical_port: 27, breakout_index: Some(1), name: "E1/27/1" },
    PortSlot::{ physical_port: 27, breakout_index: Some(2), name: "E1/27/2" },
    PortSlot::{ physical_port: 27, breakout_index: Some(3), name: "E1/27/3" },
    PortSlot::{ physical_port: 27, breakout_index: Some(4), name: "E1/27/4" },
  ])
}

test "F2 §2.3 expand_breakout: 2x400 breakout over two physical ports preserves order" {
  // soc_storage scale_out_server_2x400: brk_2x400_osfp -> 2 lanes per port
  assert_eq(expand_breakout([1, 2], 2), [
    PortSlot::{ physical_port: 1, breakout_index: Some(1), name: "E1/1/1" },
    PortSlot::{ physical_port: 1, breakout_index: Some(2), name: "E1/1/2" },
    PortSlot::{ physical_port: 2, breakout_index: Some(1), name: "E1/2/1" },
    PortSlot::{ physical_port: 2, breakout_index: Some(2), name: "E1/2/2" },
  ])
}
