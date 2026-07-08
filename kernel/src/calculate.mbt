// LEGACY / NON-LIVE (#84): pre-rebuild kernel surface retained for quarantine
// and the #38 WIT-mirror consistency guard only. No host/Go consumer after the
// foundation rebuild (D18–D27); the live wasm boundary is export_f2_calculate /
// export_f3_bom (kernel/wasm/abi.mbt). The old export_calculate/export_validate
// wasm shells were removed in #84 — `calculate`/`validate` stay compiled only via
// kernel unit tests. Do not extend or re-export; retire-vs-reconcile is deferred
// to a follow-up.
//
// Top-level kernel entry points: plan JSON -> calc-output / validation.
//
// Logical realizations of the `topology-calculator` WIT interface (`calculate`,
// `validate`). Per D16 the MVP wasm boundary is JSON-over-memory; these typed
// functions are what the boundary edge (Phase 6) will wrap. The pure kernel
// takes the JSON string, decodes to typed input, and computes — no I/O.

fn opt_or_empty(o : String?) -> String {
  match o {
    Some(s) => s
    None => ""
  }
}

fn entry_in_fabric(entry : PlanEntry, fabric_id : String) -> Bool {
  match entry.fabric_id {
    Some(f) => f == fabric_id
    None => false
  }
}

fn is_spine_role(r : PlanRole) -> Bool {
  match r {
    Spine => true
    _ => false
  }
}

fn conn_type_str(c : ConnectionType) -> String {
  match c {
    Unbundled => "unbundled"
    Bundled => "bundled"
    ConnectionType::Mclag => "mclag"
    ConnectionType::Eslag => "eslag"
  }
}

// First Uplink-typed zone on an entry (the spine fabric zone / leaf uplink zone).
fn uplink_zone(entry : PlanEntry) -> SwitchPortZone? {
  for zone in entry.port_zones {
    match zone.zone_type {
      Uplink => return Some(zone)
      _ => ()
    }
  }
  None
}

fn fabric_has_spine(plan : TopologyPlan, fabric_id : String) -> Bool {
  for entry in plan.entries {
    if entry_in_fabric(entry, fabric_id) && is_spine_role(entry.role) {
      return true
    }
  }
  false
}

fn first_spine_node(plan : TopologyPlan, fabric_id : String) -> String {
  for entry in plan.entries {
    if entry_in_fabric(entry, fabric_id) && is_spine_role(entry.role) {
      return entry.entry_id + "-0"
    }
  }
  ""
}

// Physical mesh ports per switch in a fabric (first Mesh-typed zone found).
fn mesh_ports_per_switch(plan : TopologyPlan, fabric_id : String) -> Int {
  for entry in plan.entries {
    if entry_in_fabric(entry, fabric_id) {
      for zone in entry.port_zones {
        match zone.zone_type {
          ZoneType::Mesh => return port_range_count(zone.port_range)
          _ => ()
        }
      }
    }
  }
  0
}

fn count_switches_in_fabric(plan : TopologyPlan, fabric_id : String) -> Int {
  let mut n = 0
  for entry in plan.entries {
    if entry_in_fabric(entry, fabric_id) && !is_server_role(entry.role) {
      n = n + instance_count(entry)
    }
  }
  n
}

// Representative fabric link speed = max server-connection speed into the fabric.
fn fabric_speed(plan : TopologyPlan, fabric_id : String) -> Int {
  let mut speed = 0
  for entry in plan.entries {
    if is_server_role(entry.role) {
      for conn in entry.connections {
        match entry_owning_zone(plan, conn.target_zone_id) {
          Some(te) =>
            if entry_in_fabric(te, fabric_id) {
              let s = conn.speed_gbps.reinterpret_as_int()
              if s > speed {
                speed = s
              }
            }
          None => ()
        }
      }
    }
  }
  speed
}

// Algorithm 7 inputs --------------------------------------------------------

fn total_server_bw(plan : TopologyPlan, fabric_id : String) -> Int {
  let mut bw = 0
  for entry in plan.entries {
    if is_server_role(entry.role) {
      let server_count = entry.quantity.reinterpret_as_int()
      for conn in entry.connections {
        match entry_owning_zone(plan, conn.target_zone_id) {
          Some(te) =>
            if entry_in_fabric(te, fabric_id) {
              bw = bw +
                server_count *
                conn.ports_per_connection.reinterpret_as_int() *
                conn.speed_gbps.reinterpret_as_int()
            }
          None => ()
        }
      }
    }
  }
  bw
}

// Algorithm 7 denominator (DECISIONS.md D17): total declared leaf-uplink
// bandwidth = sum over the fabric's leaf switch classes of
// (leaf_count × uplink_zone_logical_ports × uplink_speed), taken from each leaf
// class's explicitly declared UPLINK zone (zone_type = uplink). Uplinks are
// declared, never inferred; a leaf class with no UPLINK zone contributes nothing
// (=> oversubscription N/A for that fabric). This is NOT the spine's fabric-port
// capacity (the older form, replaced per D17).
fn total_uplink_bw(plan : TopologyPlan, fabric_id : String) -> Int {
  let speed = fabric_speed(plan, fabric_id)
  let mut bw = 0
  for entry in plan.entries {
    if entry_in_fabric(entry, fabric_id) &&
      !is_server_role(entry.role) &&
      !is_spine_role(entry.role) {
      match uplink_zone(entry) {
        Some(uz) => {
          let n = instance_count(entry)
          let ports = zone_logical_capacity(uz)
          bw = bw + n * ports * speed
        }
        None => ()
      }
    }
  }
  bw
}

// IR builders ---------------------------------------------------------------

fn build_nodes(plan : TopologyPlan) -> Array[TopologyNode] {
  let nodes = []
  for entry in plan.entries {
    let count = instance_count(entry)
    let server = is_server_role(entry.role)
    let nt = if server {
      NodeType::Server
    } else if is_spine_role(entry.role) {
      NodeType::Spine
    } else {
      NodeType::Switch
    }
    let fabric = if server { None } else { entry.fabric_id }
    let role = role_str(entry.role)
    for i in 0..<count {
      let id = entry.entry_id + "-" + i.to_string()
      nodes.push({
        node_id: id,
        name: id,
        node_type: nt,
        device_class_id: entry.device_class_id,
        fabric,
        hedgehog_role: Some(role),
        instance_index: i.reinterpret_as_uint(),
      })
    }
  }
  nodes
}

fn build_edges(plan : TopologyPlan) -> Array[TopologyEdge] {
  let edges = []
  let mut eid = 0

  // 1) Server -> switch connection edges (one per server instance per connection).
  for entry in plan.entries {
    if is_server_role(entry.role) {
      let count = instance_count(entry)
      for i in 0..<count {
        for conn in entry.connections {
          let (node_b, fabric) = match entry_owning_zone(plan, conn.target_zone_id) {
            Some(te) => {
              let swc = instance_count(te)
              let idx = switch_index_alternating(
                conn.port_index.reinterpret_as_int(),
                swc,
              )
              (te.entry_id + "-" + idx.to_string(), opt_or_empty(te.fabric_id))
            }
            None => ("", "")
          }
          edges.push({
            edge_id: "e" + eid.to_string(),
            node_a_id: entry.entry_id + "-" + i.to_string(),
            node_b_id: node_b,
            speed_gbps: conn.speed_gbps,
            fabric,
            zone: conn.target_zone_id,
            breakout_index: None,
            connection_type: conn_type_str(conn.connection_type),
            port_a: conn.nic_slot_id + ":" + conn.port_index.to_string(),
            port_b: conn.target_zone_id,
          })
          eid = eid + 1
        }
      }
    }
  }

  // 2) Clos leaf -> spine uplink edges (leaf_count x uplinks_per_leaf), only when
  //    the fabric actually has a spine tier.
  for fab in plan.fabric_domains {
    match fab.topology_mode {
      Clos =>
        if fabric_has_spine(plan, fab.fabric_id) {
          let spine_node = first_spine_node(plan, fab.fabric_id)
          for entry in plan.entries {
            if entry_in_fabric(entry, fab.fabric_id) &&
              !is_server_role(entry.role) &&
              !is_spine_role(entry.role) {
              match uplink_zone(entry) {
                Some(uz) => {
                  let leaf_n = instance_count(entry)
                  let uplinks = port_range_count(uz.port_range)
                  for li in 0..<leaf_n {
                    for p in 0..<uplinks {
                      edges.push({
                        edge_id: "e" + eid.to_string(),
                        node_a_id: entry.entry_id + "-" + li.to_string(),
                        node_b_id: spine_node,
                        speed_gbps: 0U,
                        fabric: fab.fabric_id,
                        zone: uz.zone_id,
                        breakout_index: None,
                        connection_type: "uplink",
                        port_a: uz.zone_id + ":" + p.to_string(),
                        port_b: "fabric",
                      })
                      eid = eid + 1
                    }
                  }
                }
                None => ()
              }
            }
          }
        }
      _ => ()
    }
  }

  // 3) Mesh inter-switch edges (Algorithm 5).
  for fab in plan.fabric_domains {
    match fab.topology_mode {
      TopologyMode::Mesh => {
        let sw_count = fab.switch_entry_ids.length()
        let ports = mesh_ports_per_switch(plan, fab.fabric_id)
        let cables = mesh_cable_count(sw_count, ports)
        let a = if fab.switch_entry_ids.length() > 0 {
          fab.switch_entry_ids[0] + "-0"
        } else {
          ""
        }
        let b = if fab.switch_entry_ids.length() > 1 {
          fab.switch_entry_ids[1] + "-0"
        } else {
          ""
        }
        for c in 0..<cables {
          edges.push({
            edge_id: "e" + eid.to_string(),
            node_a_id: a,
            node_b_id: b,
            speed_gbps: 0U,
            fabric: fab.fabric_id,
            zone: "mesh",
            breakout_index: None,
            connection_type: "mesh",
            port_a: "mesh:" + c.to_string(),
            port_b: "mesh:" + c.to_string(),
          })
          eid = eid + 1
        }
      }
      _ => ()
    }
  }

  edges
}

fn build_fabric_summaries(plan : TopologyPlan) -> Array[FabricSummary] {
  let out = []
  for fab in plan.fabric_domains {
    let switch_count = count_switches_in_fabric(plan, fab.fabric_id)
    let server_bw = total_server_bw(plan, fab.fabric_id)
    // Denominator is declared leaf-uplink bandwidth (D17), carried in the
    // total_spine_bandwidth_gbps WIT field (the leaf->spine uplink bandwidth).
    let uplink_bw = total_uplink_bw(plan, fab.fabric_id)
    out.push({
      fabric_name: fab.fabric_name,
      switch_count: switch_count.reinterpret_as_uint(),
      total_server_bandwidth_gbps: server_bw.to_uint64(),
      total_spine_bandwidth_gbps: uplink_bw.to_uint64(),
      oversubscription_ratio: oversubscription_ratio(server_bw, uplink_bw),
    })
  }
  out
}

fn build_ir(plan : TopologyPlan) -> TopologyIr {
  {
    metadata: {
      plan_id: plan.id,
      plan_name: plan.name,
      customer_name: plan.customer_name,
    },
    nodes: build_nodes(plan),
    edges: build_edges(plan),
    fabrics: build_fabric_summaries(plan),
  }
}

/// Validate and calculate the full topology from a plan JSON string.
pub fn calculate(plan_json : String) -> Result[CalcOutput, CalcError] {
  match decode_plan(plan_json) {
    Err(e) => Err(e)
    Ok(plan) => {
      let validation = validate_plan(plan)
      let ir = build_ir(plan)
      let boms = []
      for entry in plan.entries {
        boms.push(bom_for_entry(entry, plan.device_catalog))
      }
      Ok({ ir, boms, validation })
    }
  }
}

/// Validate a plan JSON string without producing the full IR/BOMs.
pub fn validate(plan_json : String) -> Result[ValidationResult, CalcError] {
  match decode_plan(plan_json) {
    Err(e) => Err(e)
    Ok(plan) => Ok(validate_plan(plan))
  }
}
