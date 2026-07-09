// LEGACY / NON-LIVE (#84): pre-rebuild kernel surface retained for quarantine and
// the #38 guard only. No host/Go consumer after the foundation rebuild (D18–D27);
// serves the legacy calculate/validate/BOM path, reachable only via kernel unit
// tests. Retire-vs-reconcile deferred to a follow-up.
//
// Shared, package-internal lookups over the plan/catalog, used by the BOM,
// validation and calculate orchestration. Pure linear scans (fixtures are small).

fn find_device_class(catalog : Array[DeviceClass], id : String) -> DeviceClass? {
  for d in catalog {
    if d.id == id {
      return Some(d)
    }
  }
  None
}

fn summary_of(dc : DeviceClass) -> DeviceClassSummary {
  {
    id: dc.id,
    name: dc.name,
    slug: dc.slug,
    category: dc.category,
    manufacturer: dc.manufacturer,
    part_number: dc.part_number,
  }
}

/// Plan-role rendered as the user-facing kebab-case string (carried on
/// TopologyNode.hedgehog_role and used for per-fabric switch-role counts).
fn role_str(r : PlanRole) -> String {
  match r {
    Server => "server"
    Spine => "spine"
    ServerLeaf => "server-leaf"
    BorderLeaf => "border-leaf"
    OobLeaf => "oob-leaf"
    Hhg => "hhg"
  }
}

fn is_server_role(r : PlanRole) -> Bool {
  match r {
    Server => true
    _ => false
  }
}

/// Number of device instances an entry expands to: servers use `quantity`;
/// switch entries honour `override_quantity` when present (the plan's explicit
/// resolved count, per the fixture calculation_notes), else fall back to
/// `quantity`. The demand-derived Algorithm 1/2 counts are unit-tested directly.
fn instance_count(entry : PlanEntry) -> Int {
  if is_server_role(entry.role) {
    entry.quantity.reinterpret_as_int()
  } else {
    match entry.override_quantity {
      Some(n) => n.reinterpret_as_int()
      None => entry.quantity.reinterpret_as_int()
    }
  }
}

/// The switch entry that owns a given port zone (by zone id), if any.
fn entry_owning_zone(plan : TopologyPlan, zone_id : String) -> PlanEntry? {
  for entry in plan.entries {
    for zone in entry.port_zones {
      if zone.zone_id == zone_id {
        return Some(entry)
      }
    }
  }
  None
}

/// Logical port capacity of a zone = physical ports * breakout factor.
fn zone_logical_capacity(zone : SwitchPortZone) -> Int {
  let physical = port_range_count(zone.port_range)
  match zone.breakout {
    Some(b) => physical * b.logical_ports.reinterpret_as_int()
    None => physical
  }
}
