//! IR -> CRD transformation (the core of the adapter).
//!
//! RED PHASE: every mapping function below is an unimplemented stub that returns
//! an error, so the acceptance and unit tests fail for the right reason — the
//! adapter emits no wiring yet. GREEN replaces these bodies with the real
//! IR->CRD mapping until `hhfab validate` accepts the output.

use crate::crds::*;
use crate::ir::{NodeIndex, TopologyIr};

// Synthesized fabric-deployment defaults (NOT in topology-ir). These are
// overridable defaults, present solely so `hhfab validate` sees a complete
// fabric. Values match the hhfab-validated reference sample.
pub const DEFAULT_VLAN_FROM: u32 = 1000;
pub const DEFAULT_VLAN_TO: u32 = 2999;
pub const DEFAULT_IPV4_SUBNET: &str = "10.0.0.0/16";
pub const DEFAULT_SWITCHGROUP_NAME: &str = "empty";
/// Synthesized switch profile accepted by `hhfab validate` (virtual switch).
/// Mapping `device-class-id` -> a real SwitchProfile is a documented follow-up.
pub const DEFAULT_SWITCH_PROFILE: &str = "vs";

const RED_STUB: &str = "RED stub: not implemented (GREEN phase)";

/// Produce wiring YAML document(s) from a calculated topology IR.
///
/// `options.fabric = Some(name)` restricts output to one fabric;
/// `options.split_by_fabric = true` emits one document per managed fabric,
/// otherwise a single combined document.
pub fn export_wiring(
    _ir: &TopologyIr,
    _options: &HhfabOptions,
) -> Result<HhfabOutput, HhfabError> {
    Err(HhfabError::Internal(RED_STUB.to_string()))
}

/// Map a switch/spine node to a `Switch` CRD. `ecmp`/`redundancy` are never
/// emitted (see `SwitchSpec`).
pub fn switch_crd(_node: &crate::ir::TopologyNode) -> Result<Switch, HhfabError> {
    Err(HhfabError::Internal(RED_STUB.to_string()))
}

/// Map a server node to a `Server` CRD.
pub fn server_crd(_node: &crate::ir::TopologyNode) -> Result<Server, HhfabError> {
    Err(HhfabError::Internal(RED_STUB.to_string()))
}

/// Map a set of topology edges to Connection CRDs, dispatching on
/// `connection_type` and aggregating multi-link connections:
///   - `unbundled`           -> one `spec.unbundled` Connection per edge
///   - `bundled`/`mclag`/`eslag` -> one Connection per server group (link list)
///   - `uplink`              -> one `spec.fabric` Connection per leaf<->spine
///                              pair (links aggregated)
///   - `mesh`                -> one `spec.mesh` Connection per switch<->switch
///                              pair (links aggregated)
pub fn connections_for_edges(
    _edges: &[crate::ir::TopologyEdge],
    _nodes: &NodeIndex,
) -> Result<Vec<Connection>, HhfabError> {
    Err(HhfabError::Internal(RED_STUB.to_string()))
}
