// Algorithm 5 — mesh inter-switch link count (ALGORITHMS.md).
//
// RED phase: stub returns -1 so the unit test fails on its assertion.

/// Cables per switch pair for a mesh fabric.
///   - 2-switch mesh: cables_per_pair = logical_mesh_ports_per_switch
///   - 3-switch mesh: cables_per_pair = floor(logical_mesh_ports_per_switch / 2)
///     (must be even — enforced as a hard validation constraint elsewhere)
/// switch_count must be 2 or 3 (validated separately; see validate_plan).
pub fn mesh_cable_count(switch_count : Int, logical_mesh_ports_per_switch : Int) -> Int {
  let _ = switch_count
  let _ = logical_mesh_ports_per_switch
  -1
}
