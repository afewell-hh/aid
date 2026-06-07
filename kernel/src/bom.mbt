// Algorithm 6 — recursive DeviceClass BOM traversal (ALGORITHMS.md; D6, D13).
//
// RED phase: stub returns a BOM with no line items so the unit/fixture tests
// fail on their quantity assertions.
//
// GREEN contract (encoded by the tests):
//   - quantity_per_unit(node) = product of quantity_per_parent from root to node
//   - fleet_quantity(node)    = quantity_per_unit(node) * plan_entry.quantity
//   - root quantity_per_unit  = 1
//   - SERVER-role entries list their sub-components only (the assembled server is
//     not itself a procurement line); SWITCH-role entries include the switch SKU
//     itself plus its sub-components. (Per the issue #6 fixture table:
//     clos-small gpu-servers omits the server root; switch-bom includes the
//     switch root.)

/// Recursive BOM for one plan entry's device class, resolved against the
/// normalized device catalog (sub-components reference catalog entries by id).
pub fn bom_for_entry(entry : PlanEntry, catalog : Array[DeviceClass]) -> DeviceClassBom {
  let _ = catalog
  {
    device_class: empty_summary(entry.device_class_id),
    entry_id: entry.entry_id,
    plan_quantity: entry.quantity,
    line_items: [],
  }
}
