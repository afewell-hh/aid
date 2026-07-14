// PARTIALLY LEGACY (#84): the topology-ir / device-class-bom / calc-result
// encoders in this file are NON-LIVE pre-rebuild surface (no host/Go consumer
// after D18–D27), retained for quarantine and the #38 guard only. The small JSON
// primitives (j_esc, j_arr) ARE live — the F2/F3 encoders reuse them — so this
// file must NOT be deleted wholesale. Retire-vs-reconcile deferred to a follow-up.
//
// Canonical calc-output -> JSON encoder (DECISIONS.md D16 consolidation).
//
// HISTORICAL (pre-rebuild): when authored, this was the single source of the
// snake_case JSON wire shapes the Phase-6 Go host and the Layer-2 adapters
// consumed:
//   - topology-ir   -> IR_CONTRACT.md  (hhfab adapter input)
//   - device-class-bom[] -> BOM_CONTRACT.md (bom adapter input)
//   - validation-result, calc-output, result<calc-output, calc-error>
// After the rebuild (D18–D27) those consumers are gone (adapters retired F7d; the
// live encoders are the F2/F3 ones in f2_calc.mbt / f3_bom.mbt), so these encoders
// are NON-LIVE. Only the shared j_esc / j_arr primitives below remain live. Pure
// string building — no I/O, no externs.
//
// Field names are snake_case and mirror wit/types.wit. Integer-typed fields emit
// integer tokens; only oversubscription-ratio is a float.

// ---- legacy-only JSON primitives -------------------------------------------
// The shared j_esc / j_arr helpers moved to json_util.mbt (#85) because the live
// F2/F3 encoders reuse them; the option/string-array helpers below are used only
// by this retired encoder and go away with it.

fn j_opt_str(o : String?) -> String {
  match o {
    Some(s) => j_esc(s)
    None => "null"
  }
}

fn j_opt_uint(o : UInt?) -> String {
  match o {
    Some(n) => n.to_string()
    None => "null"
  }
}

fn j_str_arr(parts : Array[String]) -> String {
  let mut out = "["
  for i in 0..<parts.length() {
    if i > 0 {
      out = out + ","
    }
    out = out + j_esc(parts[i])
  }
  out + "]"
}

// ---- enum string forms -----------------------------------------------------

fn j_node_type(t : NodeType) -> String {
  match t {
    NodeType::Server => "\"server\""
    NodeType::Switch => "\"switch\""
    NodeType::Spine => "\"spine\""
  }
}

fn j_severity(s : IssueSeverity) -> String {
  match s {
    IssueSeverity::Error => "\"error\""
    IssueSeverity::Warning => "\"warning\""
  }
}

// ---- topology-ir (mirrors IR_CONTRACT.md / interim ir-gen) ------------------

fn encode_metadata(m : PlanMetadata) -> String {
  "{\"plan_id\":" +
  j_esc(m.plan_id) +
  ",\"plan_name\":" +
  j_esc(m.plan_name) +
  ",\"customer_name\":" +
  j_esc(m.customer_name) +
  "}"
}

fn encode_node(n : TopologyNode) -> String {
  "{\"node_id\":" +
  j_esc(n.node_id) +
  ",\"name\":" +
  j_esc(n.name) +
  ",\"node_type\":" +
  j_node_type(n.node_type) +
  ",\"device_class_id\":" +
  j_esc(n.device_class_id) +
  ",\"fabric\":" +
  j_opt_str(n.fabric) +
  ",\"hedgehog_role\":" +
  j_opt_str(n.hedgehog_role) +
  ",\"instance_index\":" +
  n.instance_index.to_string() +
  "}"
}

fn encode_edge(e : TopologyEdge) -> String {
  "{\"edge_id\":" +
  j_esc(e.edge_id) +
  ",\"node_a_id\":" +
  j_esc(e.node_a_id) +
  ",\"node_b_id\":" +
  j_esc(e.node_b_id) +
  ",\"speed_gbps\":" +
  e.speed_gbps.to_string() +
  ",\"fabric\":" +
  j_esc(e.fabric) +
  ",\"zone\":" +
  j_esc(e.zone) +
  ",\"breakout_index\":" +
  j_opt_uint(e.breakout_index) +
  ",\"connection_type\":" +
  j_esc(e.connection_type) +
  ",\"port_a\":" +
  j_esc(e.port_a) +
  ",\"port_b\":" +
  j_esc(e.port_b) +
  "}"
}

fn encode_fabric(f : FabricSummary) -> String {
  "{\"fabric_name\":" +
  j_esc(f.fabric_name) +
  ",\"switch_count\":" +
  f.switch_count.to_string() +
  ",\"total_server_bandwidth_gbps\":" +
  f.total_server_bandwidth_gbps.to_string() +
  ",\"total_spine_bandwidth_gbps\":" +
  f.total_spine_bandwidth_gbps.to_string() +
  ",\"oversubscription_ratio\":" +
  f.oversubscription_ratio.to_string() +
  "}"
}

/// Encode a topology-ir to its snake_case JSON wire shape (IR_CONTRACT.md).
pub fn encode_ir(ir : TopologyIr) -> String {
  let nodes = []
  for n in ir.nodes {
    nodes.push(encode_node(n))
  }
  let edges = []
  for e in ir.edges {
    edges.push(encode_edge(e))
  }
  let fabrics = []
  for f in ir.fabrics {
    fabrics.push(encode_fabric(f))
  }
  "{\"metadata\":" +
  encode_metadata(ir.metadata) +
  ",\"nodes\":" +
  j_arr(nodes) +
  ",\"edges\":" +
  j_arr(edges) +
  ",\"fabrics\":" +
  j_arr(fabrics) +
  "}"
}

// ---- device-class-bom[] (mirrors BOM_CONTRACT.md / interim bom-gen) ---------

fn encode_summary(s : DeviceClassSummary) -> String {
  "{\"id\":" +
  j_esc(s.id) +
  ",\"name\":" +
  j_esc(s.name) +
  ",\"slug\":" +
  j_esc(s.slug) +
  ",\"category\":" +
  j_esc(s.category) +
  ",\"manufacturer\":" +
  j_opt_str(s.manufacturer) +
  ",\"part_number\":" +
  j_opt_str(s.part_number) +
  "}"
}

fn encode_line_item(li : BomLineItem) -> String {
  "{\"path\":" +
  j_str_arr(li.path) +
  ",\"device_class\":" +
  encode_summary(li.device_class) +
  ",\"quantity_per_parent\":" +
  li.quantity_per_parent.to_string() +
  ",\"quantity_per_unit\":" +
  li.quantity_per_unit.to_string() +
  ",\"fleet_quantity\":" +
  li.fleet_quantity.to_string() +
  "}"
}

fn encode_bom(b : DeviceClassBom) -> String {
  let items = []
  for li in b.line_items {
    items.push(encode_line_item(li))
  }
  "{\"device_class\":" +
  encode_summary(b.device_class) +
  ",\"entry_id\":" +
  j_esc(b.entry_id) +
  ",\"plan_quantity\":" +
  b.plan_quantity.to_string() +
  ",\"line_items\":" +
  j_arr(items) +
  "}"
}

/// Encode device-class-bom[] to its snake_case JSON wire shape (BOM_CONTRACT.md).
pub fn encode_boms(boms : Array[DeviceClassBom]) -> String {
  let parts = []
  for b in boms {
    parts.push(encode_bom(b))
  }
  j_arr(parts)
}

// ---- validation-result -----------------------------------------------------

fn encode_issue(i : ValidationIssue) -> String {
  "{\"severity\":" +
  j_severity(i.severity) +
  ",\"code\":" +
  j_esc(i.code) +
  ",\"message\":" +
  j_esc(i.message) +
  ",\"entry_id\":" +
  j_opt_str(i.entry_id) +
  ",\"fabric_id\":" +
  j_opt_str(i.fabric_id) +
  ",\"zone_id\":" +
  j_opt_str(i.zone_id) +
  "}"
}

/// Encode a validation-result to its snake_case JSON wire shape.
pub fn encode_validation(v : ValidationResult) -> String {
  let errors = []
  for e in v.errors {
    errors.push(encode_issue(e))
  }
  let warnings = []
  for w in v.warnings {
    warnings.push(encode_issue(w))
  }
  "{\"is_valid\":" +
  (if v.is_valid { "true" } else { "false" }) +
  ",\"errors\":" +
  j_arr(errors) +
  ",\"warnings\":" +
  j_arr(warnings) +
  "}"
}

// ---- calc-output and result<calc-output, calc-error> -----------------------

/// Encode a calc-output: { ir, boms, validation }.
pub fn encode_calc_output(out : CalcOutput) -> String {
  "{\"ir\":" +
  encode_ir(out.ir) +
  ",\"boms\":" +
  encode_boms(out.boms) +
  ",\"validation\":" +
  encode_validation(out.validation) +
  "}"
}

fn encode_error(err : CalcError) -> String {
  let (kind, msg) = match err {
    InvalidPlan(m) => ("invalid_plan", m)
    Internal(m) => ("internal", m)
  }
  "{\"kind\":\"" + kind + "\",\"message\":" + j_esc(msg) + "}"
}

/// Encode result<calc-output, calc-error> as {"ok":...} | {"err":...}.
pub fn encode_calc_result(r : Result[CalcOutput, CalcError]) -> String {
  match r {
    Ok(out) => "{\"ok\":" + encode_calc_output(out) + "}"
    Err(e) => "{\"err\":" + encode_error(e) + "}"
  }
}

/// Encode result<validation-result, calc-error> as {"ok":...} | {"err":...}.
pub fn encode_validation_result(r : Result[ValidationResult, CalcError]) -> String {
  match r {
    Ok(v) => "{\"ok\":" + encode_validation(v) + "}"
    Err(e) => "{\"err\":" + encode_error(e) + "}"
  }
}
