// Small constructors for empty/default output values, shared by the RED stubs.

/// Human-readable rendering of a CalcError (for test failure messages).
pub fn calc_error_msg(err : CalcError) -> String {
  match err {
    InvalidPlan(m) => "invalid-plan: " + m
    Internal(m) => "internal: " + m
  }
}

pub fn empty_summary(id : String) -> DeviceClassSummary {
  { id, name: "", slug: "", category: "", manufacturer: None, part_number: None }
}

pub fn empty_validation() -> ValidationResult {
  { is_valid: true, errors: [], warnings: [] }
}

pub fn empty_ir() -> TopologyIr {
  {
    metadata: { plan_id: "", plan_name: "", customer_name: "" },
    nodes: [],
    edges: [],
    fabrics: [],
  }
}

pub fn empty_calc_output() -> CalcOutput {
  { ir: empty_ir(), boms: [], validation: empty_validation() }
}
