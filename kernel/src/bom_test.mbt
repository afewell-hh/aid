// Unit tests — Algorithm 6 (recursive DeviceClass BOM; D6, D13).
//
// These pin the two behaviours the issue #6 fixture table encodes:
//   - SERVER-role entries list sub-components only (root server excluded).
//   - SWITCH-role entries include the switch SKU itself plus sub-components.
// and the recursive multiply-down-the-path / fleet scaling.

fn t_sub(slot : String, dcid : String, qty : Int) -> SubComponent {
  { slot_id: slot, device_class_id: dcid, quantity_per_parent: qty.reinterpret_as_uint() }
}

fn t_dc(id : String, subs : Array[SubComponent]) -> DeviceClass {
  {
    id,
    name: id,
    slug: id,
    category: "",
    manufacturer: None,
    part_number: None,
    description: None,
    attributes: [],
    ports: [],
    sub_components: subs,
  }
}

fn t_entry(eid : String, dcid : String, qty : Int, role : PlanRole) -> PlanEntry {
  {
    entry_id: eid,
    device_class_id: dcid,
    quantity: qty.reinterpret_as_uint(),
    role,
    label: None,
    fabric_id: None,
    override_quantity: None,
    topology_mode: None,
    redundancy: RedNone,
    port_zones: [],
    connections: [],
  }
}

test "Algorithm 6: server BOM excludes root, recurses and scales by fleet" {
  let catalog = [
    t_dc("server", [
      t_sub("chassis", "chassis", 1),
      t_sub("gpu", "gpu", 8),
      t_sub("nic", "nic", 2),
    ]),
    t_dc("chassis", []),
    t_dc("gpu", []),
    t_dc("nic", [t_sub("xcvr", "xcvr", 1)]),
    t_dc("xcvr", []),
  ]
  let bom = bom_for_entry(t_entry("svrs", "server", 4, PlanRole::Server), catalog)
  // per-unit (root server excluded)
  assert_eq(bom_line_per_unit(bom, "chassis"), 1)
  assert_eq(bom_line_per_unit(bom, "gpu"), 8)
  assert_eq(bom_line_per_unit(bom, "nic"), 2)
  assert_eq(bom_line_per_unit(bom, "xcvr"), 2) // 2 NICs x 1 transceiver, recursed
  assert_eq(bom_line_per_unit(bom, "server"), 0) // server root is not a BOM line
  // fleet = per-unit x plan_quantity(4)
  assert_eq(bom_line_fleet(bom, "gpu"), 32)
  assert_eq(bom_line_fleet(bom, "xcvr"), 8)
}

test "Algorithm 6: switch BOM includes the switch SKU root" {
  let catalog = [
    t_dc("leaf800", [t_sub("xcvr", "xcvr800", 64)]),
    t_dc("xcvr800", []),
  ]
  let bom = bom_for_entry(t_entry("leafs", "leaf800", 2, PlanRole::ServerLeaf), catalog)
  assert_eq(bom_line_per_unit(bom, "leaf800"), 1) // switch root IS a BOM line
  assert_eq(bom_line_per_unit(bom, "xcvr800"), 64)
  assert_eq(bom_line_fleet(bom, "leaf800"), 2)
  assert_eq(bom_line_fleet(bom, "xcvr800"), 128) // 64 x 2
}
