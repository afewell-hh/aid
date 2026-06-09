// Algorithm 5 — mesh inter-switch link count (ALGORITHMS.md).

/// Cables per switch pair for a mesh fabric.
///   - 2-switch mesh: cables_per_pair = logical_mesh_ports_per_switch
///   - 3-switch mesh: cables_per_pair = floor(logical_mesh_ports_per_switch / 2)
///     (must be even — enforced as a hard validation constraint elsewhere)
/// switch_count must be 2 or 3 (validated separately; see validate_plan).
/// Routes through the proven `@proofs.mesh_cable_count` core (Issue #7 I5):
/// well-formed (>= 0, <= ports) under the `switch_count ∈ {2,3}` guard that
/// `validate_plan` enforces.
pub fn mesh_cable_count(switch_count : Int, logical_mesh_ports_per_switch : Int) -> Int {
  @proofs.mesh_cable_count(switch_count, logical_mesh_ports_per_switch)
}
