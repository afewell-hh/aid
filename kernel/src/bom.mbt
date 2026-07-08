// LEGACY / NON-LIVE (#84): pre-rebuild kernel surface (legacy DeviceClass BOM)
// retained for quarantine and the #38 guard only. No host/Go consumer after the
// foundation rebuild (D18–D27; the live BOM is the Go internal/bom over F2/F3).
// Reachable only via kernel unit tests. Retire-vs-reconcile deferred to a follow-up.
//
// Algorithm 6 — recursive DeviceClass BOM traversal (ALGORITHMS.md; D6, D13).
//
//   - quantity_per_unit(node) = product of quantity_per_parent from root to node
//   - fleet_quantity(node)    = quantity_per_unit(node) * plan_entry.quantity
//   - root quantity_per_unit  = 1
//   - SERVER-role entries list their sub-components only (the assembled server is
//     not itself a procurement line); SWITCH-role entries include the switch SKU
//     itself plus its sub-components. (Role-based root inclusion — confirmed
//     against the fixture baselines; see ALGORITHMS.md Algorithm 6 as clarified
//     in PR #24. The recursion/type model itself is uniform per D13.)

/// Recursive BOM for one plan entry's device class, resolved against the
/// normalized device catalog (sub-components reference catalog entries by id).
pub fn bom_for_entry(entry : PlanEntry, catalog : Array[DeviceClass]) -> DeviceClassBom {
  let items : Array[BomLineItem] = []
  let plan_qty = entry.quantity.reinterpret_as_int()
  let root_opt = find_device_class(catalog, entry.device_class_id)
  let root_summary = match root_opt {
    Some(root) => summary_of(root)
    None => empty_summary(entry.device_class_id)
  }
  match root_opt {
    Some(root) => {
      // Switch entries include the switch SKU itself as a root BOM line;
      // server entries do not (their components are the procurement lines).
      if is_server_role(entry.role) {
        ()
      } else {
        items.push({
          path: [],
          device_class: root_summary,
          quantity_per_parent: 1U,
          quantity_per_unit: 1U,
          // fleet = quantity_per_unit (1) × plan_quantity — @proofs.fleet_quantity (I4).
          fleet_quantity: @proofs.fleet_quantity(1, plan_qty).reinterpret_as_uint(),
        })
      }
      bom_traverse(root, catalog, [], 1, plan_qty, items)
    }
    None => ()
  }
  {
    device_class: root_summary,
    entry_id: entry.entry_id,
    plan_quantity: entry.quantity,
    line_items: items,
  }
}

fn bom_traverse(
  parent : DeviceClass,
  catalog : Array[DeviceClass],
  parent_path : Array[String],
  parent_qpu : Int,
  plan_qty : Int,
  items : Array[BomLineItem]
) -> Unit {
  for sub in parent.sub_components {
    let qpp = sub.quantity_per_parent.reinterpret_as_int()
    // qpu(child) = qpu(parent) × quantity_per_parent — @proofs.child_qpu (I4).
    let qpu = @proofs.child_qpu(parent_qpu, qpp)
    let path = parent_path.copy()
    path.push(sub.slot_id)
    let child_opt = find_device_class(catalog, sub.device_class_id)
    let summary = match child_opt {
      Some(child) => summary_of(child)
      None => empty_summary(sub.device_class_id)
    }
    items.push({
      path,
      device_class: summary,
      quantity_per_parent: sub.quantity_per_parent,
      quantity_per_unit: qpu.reinterpret_as_uint(),
      // fleet = quantity_per_unit × plan_quantity — @proofs.fleet_quantity (I4).
      fleet_quantity: @proofs.fleet_quantity(qpu, plan_qty).reinterpret_as_uint(),
    })
    match child_opt {
      Some(child) => bom_traverse(child, catalog, path, qpu, plan_qty, items)
      None => ()
    }
  }
}
