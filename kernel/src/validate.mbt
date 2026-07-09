// LEGACY / NON-LIVE (#84): pre-rebuild kernel surface retained for quarantine
// and the #38 WIT-mirror consistency guard only. No host/Go consumer after the
// foundation rebuild (D18–D27); the live wasm boundary is export_f2_calculate /
// export_f3_bom. The old export_validate wasm shell was removed in #84 —
// `validate` stays compiled only via kernel unit tests. Do not extend.
//
// Constraint validation -> ValidationResult (ALGORITHMS.md; D11).
//
// Domain violations are returned as data (errors with identifiable codes),
// never as panics or calc-error. Errors block generation; warnings (e.g.
// oversubscription) do not.

/// Demand and the set of distribution modes for server connections that target
/// a given switch zone. Returns (demand_logical_ports, has_rail, has_alt).
fn zone_demand_and_dists(plan : TopologyPlan, zone_id : String) -> (Int, Bool, Bool) {
  let mut demand = 0
  let mut has_rail = false
  let mut has_alt = false
  for entry in plan.entries {
    if is_server_role(entry.role) {
      let server_count = entry.quantity.reinterpret_as_int()
      for conn in entry.connections {
        if conn.target_zone_id == zone_id {
          demand = demand + server_count * conn.ports_per_connection.reinterpret_as_int()
          match conn.distribution {
            RailOptimized => has_rail = true
            Alternating => has_alt = true
            SameSwitch => ()
          }
        }
      }
    }
  }
  (demand, has_rail, has_alt)
}

/// Validate a decoded plan and return structured errors/warnings.
pub fn validate_plan(plan : TopologyPlan) -> ValidationResult {
  let errors : Array[ValidationIssue] = []
  let warnings : Array[ValidationIssue] = []

  // Mesh switch count (Algorithm 5): each mesh fabric must list 2 or 3 switches.
  for fab in plan.fabric_domains {
    match fab.topology_mode {
      TopologyMode::Mesh => {
        let n = fab.switch_entry_ids.length()
        if n != 2 && n != 3 {
          errors.push({
            severity: IssueSeverity::Error,
            code: code_mesh_switch_count,
            message: "Mesh fabric '" +
              fab.fabric_id +
              "' lists " +
              n.to_string() +
              " switch entries; a mesh fabric must have exactly 2 or 3 switches.",
            entry_id: None,
            fabric_id: Some(fab.fabric_id),
            zone_id: None,
          })
        }
      }
      _ => ()
    }
  }

  // Redundancy: MCLAG even and >= 2; ESLAG within 2..4.
  for entry in plan.entries {
    match entry.redundancy {
      RedMclag(cfg) => {
        let c = cfg.switch_count.reinterpret_as_int()
        if c < 2 || c % 2 != 0 {
          errors.push({
            severity: IssueSeverity::Error,
            code: code_mclag_switch_count,
            message: "Entry '" +
              entry.entry_id +
              "' uses MCLAG with switch_count " +
              c.to_string() +
              "; MCLAG requires an even switch_count of at least 2.",
            entry_id: Some(entry.entry_id),
            fabric_id: entry.fabric_id,
            zone_id: None,
          })
        }
      }
      RedEslag(cfg) => {
        let c = cfg.switch_count.reinterpret_as_int()
        if c < 2 || c > 4 {
          errors.push({
            severity: IssueSeverity::Error,
            code: code_eslag_switch_count,
            message: "Entry '" +
              entry.entry_id +
              "' uses ESLAG with switch_count " +
              c.to_string() +
              "; ESLAG requires a switch_count between 2 and 4.",
            entry_id: Some(entry.entry_id),
            fabric_id: entry.fabric_id,
            zone_id: None,
          })
        }
      }
      RedNone => ()
    }
  }

  // Per switch zone: Algorithm 8 capacity pre-flight + mixed-distribution check.
  for entry in plan.entries {
    for zone in entry.port_zones {
      let (demand, has_rail, has_alt) = zone_demand_and_dists(plan, zone.zone_id)
      let capacity = zone_logical_capacity(zone)
      if !zone_capacity_ok(demand, capacity) {
        errors.push({
          severity: IssueSeverity::Error,
          code: code_zone_capacity_deficit,
          message: "Zone '" +
            zone.zone_id +
            "' on entry '" +
            entry.entry_id +
            "' is over capacity: demand " +
            demand.to_string() +
            " > capacity " +
            capacity.to_string() +
            ".",
          entry_id: Some(entry.entry_id),
          fabric_id: entry.fabric_id,
          zone_id: Some(zone.zone_id),
        })
      }
      if has_rail && has_alt {
        errors.push({
          severity: IssueSeverity::Error,
          code: code_mixed_distribution,
          message: "Zone '" +
            zone.zone_id +
            "' is targeted by both rail-optimized and alternating connections; a zone cannot mix the two.",
          entry_id: Some(entry.entry_id),
          fabric_id: entry.fabric_id,
          zone_id: Some(zone.zone_id),
        })
      }
    }
  }

  { is_valid: errors.length() == 0, errors, warnings }
}
