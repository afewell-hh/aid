// Read-only accessors over kernel output, used by tests (and later by callers)
// to query counts/BOM totals/validation without re-deriving them. These are
// pure projections of the output structs — not algorithm logic.

/// Total topology nodes (servers + switches + spines).
pub fn device_count(out : CalcOutput) -> Int {
  out.ir.nodes.length()
}

/// Total topology edges (cables).
pub fn cable_count(out : CalcOutput) -> Int {
  out.ir.edges.length()
}

/// Count switch/spine nodes in a fabric carrying a given plan-role label
/// (e.g. "server-leaf", "spine"). Role is carried on `hedgehog_role`.
pub fn switch_count_for(out : CalcOutput, fabric : String, role : String) -> Int {
  let mut n = 0
  for node in out.ir.nodes {
    let fabric_match = match node.fabric {
      Some(f) => f == fabric
      None => false
    }
    let role_match = match node.hedgehog_role {
      Some(r) => r == role
      None => false
    }
    if fabric_match && role_match {
      n = n + 1
    }
  }
  n
}

/// Whether a FabricSummary exists for the named fabric.
pub fn has_fabric_summary(out : CalcOutput, fabric_name : String) -> Bool {
  for fs in out.ir.fabrics {
    if fs.fabric_name == fabric_name {
      return true
    }
  }
  false
}

/// The oversubscription ratio recorded for a fabric, if present.
pub fn fabric_oversub(out : CalcOutput, fabric_name : String) -> Double? {
  for fs in out.ir.fabrics {
    if fs.fabric_name == fabric_name {
      return Some(fs.oversubscription_ratio)
    }
  }
  None
}

/// Total server-access bandwidth (Gbps) recorded for a fabric, if present.
pub fn fabric_server_bw(out : CalcOutput, fabric_name : String) -> Int? {
  for fs in out.ir.fabrics {
    if fs.fabric_name == fabric_name {
      return Some(fs.total_server_bandwidth_gbps.to_int())
    }
  }
  None
}

/// Total leaf-uplink bandwidth (Gbps) recorded for a fabric — the Algorithm 7 /
/// D17 oversubscription denominator, carried in the total_spine_bandwidth_gbps
/// WIT field. None if no such fabric.
pub fn fabric_uplink_bw(out : CalcOutput, fabric_name : String) -> Int? {
  for fs in out.ir.fabrics {
    if fs.fabric_name == fabric_name {
      return Some(fs.total_spine_bandwidth_gbps.to_int())
    }
  }
  None
}

/// Find the BOM for a plan entry by entry id.
pub fn bom_for(out : CalcOutput, entry_id : String) -> DeviceClassBom? {
  for b in out.boms {
    if b.entry_id == entry_id {
      return Some(b)
    }
  }
  None
}

/// Per-unit quantity of a device class within a BOM (summed across line items).
pub fn bom_line_per_unit(bom : DeviceClassBom, device_class_id : String) -> Int {
  let mut total = 0
  for li in bom.line_items {
    if li.device_class.id == device_class_id {
      total = total + li.quantity_per_unit.reinterpret_as_int()
    }
  }
  total
}

/// Fleet quantity of a device class within a BOM (summed across line items).
pub fn bom_line_fleet(bom : DeviceClassBom, device_class_id : String) -> Int {
  let mut total = 0
  for li in bom.line_items {
    if li.device_class.id == device_class_id {
      total = total + li.fleet_quantity.reinterpret_as_int()
    }
  }
  total
}

/// Convenience: per-unit quantity of a device class in entry `entry_id`'s BOM.
pub fn bom_out_per_unit(out : CalcOutput, entry_id : String, device_class_id : String) -> Int {
  match bom_for(out, entry_id) {
    Some(b) => bom_line_per_unit(b, device_class_id)
    None => 0
  }
}

/// Convenience: fleet quantity of a device class in entry `entry_id`'s BOM.
pub fn bom_out_fleet(out : CalcOutput, entry_id : String, device_class_id : String) -> Int {
  match bom_for(out, entry_id) {
    Some(b) => bom_line_fleet(b, device_class_id)
    None => 0
  }
}

/// Whether a validation result carries an error with the given code.
pub fn has_error_code(vr : ValidationResult, code : String) -> Bool {
  for issue in vr.errors {
    if issue.code == code {
      return true
    }
  }
  false
}

/// Whether a validation result carries a warning with the given code.
pub fn has_warning_code(vr : ValidationResult, code : String) -> Bool {
  for issue in vr.warnings {
    if issue.code == code {
      return true
    }
  }
  false
}
