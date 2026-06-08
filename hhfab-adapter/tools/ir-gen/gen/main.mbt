// ir-gen — regenerate vendored topology-ir JSON for the hhfab adapter.
//
// Interim test tooling (NOT the production boundary): runs the merged Phase-3
// kernel (`aid/kernel`) `calculate()` on each fixture's embedded plan JSON and
// serializes the resulting typed `topology-ir` to the snake_case JSON wire
// shape the hhfab adapter consumes. This is the single-sourced Layer-1 -> Layer-2
// contract documented in ../../IR_CONTRACT.md (mapped field-for-field to
// wit/types.wit `topology-ir`). Per the D16 extension, the Phase-6 kernel
// encoder must emit these same bytes; this hand-rolled encoder is the interim
// stand-in for it (no `kernel/` edits — strictly additive).
//
// Output: a single JSON object {fixture-name: <ir>} on stdout; ../gen-ir.sh
// splits it into tests/testdata/<fixture>.ir.json.

// --- minimal JSON encoding (full control over integer vs float tokens) ------

fn esc(s : String) -> String {
  let mut out = "\""
  for c in s {
    if c == '"' {
      out = out + "\\\""
    } else if c == '\\' {
      out = out + "\\\\"
    } else if c == '\n' {
      out = out + "\\n"
    } else if c == '\r' {
      out = out + "\\r"
    } else if c == '\t' {
      out = out + "\\t"
    } else {
      out = out + c.to_string()
    }
  }
  out + "\""
}

fn opt_str(o : String?) -> String {
  match o {
    Some(s) => esc(s)
    None => "null"
  }
}

fn opt_uint(o : UInt?) -> String {
  match o {
    Some(n) => n.to_string()
    None => "null"
  }
}

fn node_type_str(t : @kernel.NodeType) -> String {
  match t {
    @kernel.NodeType::Server => "\"server\""
    @kernel.NodeType::Switch => "\"switch\""
    @kernel.NodeType::Spine => "\"spine\""
  }
}

fn enc_array(parts : Array[String]) -> String {
  let mut out = "["
  for i in 0..<parts.length() {
    if i > 0 {
      out = out + ","
    }
    out = out + parts[i]
  }
  out + "]"
}

fn enc_metadata(m : @kernel.PlanMetadata) -> String {
  "{\"plan_id\":" +
  esc(m.plan_id) +
  ",\"plan_name\":" +
  esc(m.plan_name) +
  ",\"customer_name\":" +
  esc(m.customer_name) +
  "}"
}

fn enc_node(n : @kernel.TopologyNode) -> String {
  "{\"node_id\":" +
  esc(n.node_id) +
  ",\"name\":" +
  esc(n.name) +
  ",\"node_type\":" +
  node_type_str(n.node_type) +
  ",\"device_class_id\":" +
  esc(n.device_class_id) +
  ",\"fabric\":" +
  opt_str(n.fabric) +
  ",\"hedgehog_role\":" +
  opt_str(n.hedgehog_role) +
  ",\"instance_index\":" +
  n.instance_index.to_string() +
  "}"
}

fn enc_edge(e : @kernel.TopologyEdge) -> String {
  "{\"edge_id\":" +
  esc(e.edge_id) +
  ",\"node_a_id\":" +
  esc(e.node_a_id) +
  ",\"node_b_id\":" +
  esc(e.node_b_id) +
  ",\"speed_gbps\":" +
  e.speed_gbps.to_string() +
  ",\"fabric\":" +
  esc(e.fabric) +
  ",\"zone\":" +
  esc(e.zone) +
  ",\"breakout_index\":" +
  opt_uint(e.breakout_index) +
  ",\"connection_type\":" +
  esc(e.connection_type) +
  ",\"port_a\":" +
  esc(e.port_a) +
  ",\"port_b\":" +
  esc(e.port_b) +
  "}"
}

fn enc_fabric(f : @kernel.FabricSummary) -> String {
  "{\"fabric_name\":" +
  esc(f.fabric_name) +
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

fn enc_ir(ir : @kernel.TopologyIr) -> String {
  let nodes = []
  for n in ir.nodes {
    nodes.push(enc_node(n))
  }
  let edges = []
  for e in ir.edges {
    edges.push(enc_edge(e))
  }
  let fabrics = []
  for f in ir.fabrics {
    fabrics.push(enc_fabric(f))
  }
  "{\"metadata\":" +
  enc_metadata(ir.metadata) +
  ",\"nodes\":" +
  enc_array(nodes) +
  ",\"edges\":" +
  enc_array(edges) +
  ",\"fabrics\":" +
  enc_array(fabrics) +
  "}"
}

fn calc_ir(plan : String) -> String {
  match @kernel.calculate(plan) {
    Ok(out) => enc_ir(out.ir)
    Err(_) => "null"
  }
}

fn main {
  let s = "{\"clos-small\":" +
    calc_ir(@kernel.clos_small_plan_json) +
    ",\"mesh-two-switch\":" +
    calc_ir(@kernel.mesh_two_switch_plan_json) +
    ",\"switch-bom\":" +
    calc_ir(@kernel.switch_bom_plan_json) +
    "}"
  println(s)
}
