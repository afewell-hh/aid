//! Layer-1 -> Layer-2 input contract: the `topology-ir` wire shape.
//!
//! These structs deserialize the snake_case JSON emitted by `tools/ir-gen`
//! (the merged Phase-3 kernel `calculate()` output). Per the D16 extension to
//! Layer 2, that JSON is the single-sourced wire contract between the kernel
//! (Layer 1) and this adapter (Layer 2) — the same bytes the Phase-6 Go host
//! will hand the adapter. Every field maps one-to-one to `wit/types.wit`
//! `topology-ir`; see `IR_CONTRACT.md` for the field-by-field mapping table.
//!
//! This module ONLY models the IR. The adapter never reads plan YAML or NetBox.

use serde::Deserialize;

/// `types.wit` enum `node-type`. Serialized lowercase by the kernel encoder.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NodeType {
    Server,
    Switch,
    Spine,
}

/// `types.wit` record `plan-metadata`.
#[derive(Debug, Clone, Deserialize)]
pub struct PlanMetadata {
    pub plan_id: String,
    pub plan_name: String,
    pub customer_name: String,
}

/// `types.wit` record `topology-node`.
#[derive(Debug, Clone, Deserialize)]
pub struct TopologyNode {
    pub node_id: String,
    pub name: String,
    pub node_type: NodeType,
    pub device_class_id: String,
    pub fabric: Option<String>,
    pub hedgehog_role: Option<String>,
    pub instance_index: u32,
}

/// `types.wit` record `topology-edge`. `connection_type` is a free string in
/// the IR (the kernel emits `unbundled` / `uplink` / `mesh`); the adapter maps
/// it to an hhfab Connection variant (note: kernel `uplink` -> hhfab `fabric`).
#[derive(Debug, Clone, Deserialize)]
pub struct TopologyEdge {
    pub edge_id: String,
    pub node_a_id: String,
    pub node_b_id: String,
    pub speed_gbps: u32,
    pub fabric: String,
    pub zone: String,
    pub breakout_index: Option<u32>,
    pub connection_type: String,
    pub port_a: String,
    pub port_b: String,
}

/// `types.wit` record `fabric-summary`.
#[derive(Debug, Clone, Deserialize)]
pub struct FabricSummary {
    pub fabric_name: String,
    pub switch_count: u32,
    pub total_server_bandwidth_gbps: u64,
    pub total_spine_bandwidth_gbps: u64,
    pub oversubscription_ratio: f64,
}

/// `types.wit` record `topology-ir` — the sole input to the adapter.
#[derive(Debug, Clone, Deserialize)]
pub struct TopologyIr {
    pub metadata: PlanMetadata,
    pub nodes: Vec<TopologyNode>,
    pub edges: Vec<TopologyEdge>,
    pub fabrics: Vec<FabricSummary>,
}

impl TopologyIr {
    /// Parse the IR from its snake_case JSON wire form.
    pub fn from_json(s: &str) -> Result<Self, serde_json::Error> {
        serde_json::from_str(s)
    }
}

/// Lookup from `node-id` to node — built once per export so edge endpoints can
/// be resolved to nodes (for names, roles, and port synthesis).
pub struct NodeIndex {
    by_id: std::collections::HashMap<String, TopologyNode>,
}

impl NodeIndex {
    pub fn build(nodes: &[TopologyNode]) -> Self {
        let mut by_id = std::collections::HashMap::with_capacity(nodes.len());
        for n in nodes {
            by_id.insert(n.node_id.clone(), n.clone());
        }
        NodeIndex { by_id }
    }

    pub fn get(&self, node_id: &str) -> Option<&TopologyNode> {
        self.by_id.get(node_id)
    }
}
