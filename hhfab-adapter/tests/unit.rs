//! RED unit tests — edge -> Connection mapping per variant, and the
//! no-empty-`ecmp` rule. All FAIL now because the emit functions are stubs;
//! GREEN implements the mapping until these pass.

use hhfab_adapter::crds::*;
use hhfab_adapter::emit::{connections_for_edges, server_crd, switch_crd};
use hhfab_adapter::ir::{NodeIndex, NodeType, TopologyEdge, TopologyNode};

fn node(id: &str, nt: NodeType, role: &str, fabric: Option<&str>) -> TopologyNode {
    TopologyNode {
        node_id: id.to_string(),
        name: id.to_string(),
        node_type: nt,
        device_class_id: "dc".to_string(),
        fabric: fabric.map(|s| s.to_string()),
        hedgehog_role: Some(role.to_string()),
        instance_index: 0,
    }
}

fn edge(id: &str, a: &str, b: &str, conn_type: &str, zone: &str) -> TopologyEdge {
    TopologyEdge {
        edge_id: id.to_string(),
        node_a_id: a.to_string(),
        node_b_id: b.to_string(),
        speed_gbps: 400,
        fabric: "frontend".to_string(),
        zone: zone.to_string(),
        breakout_index: None,
        connection_type: conn_type.to_string(),
        port_a: format!("{zone}:0"),
        port_b: zone.to_string(),
    }
}

fn yaml(c: &Connection) -> String {
    serde_yaml::to_string(c).expect("serialize connection")
}

#[test]
fn unbundled_edge_maps_to_unbundled_connection() {
    let nodes = vec![
        node("srv-0", NodeType::Server, "server", None),
        node("leaf-0", NodeType::Switch, "server-leaf", Some("frontend")),
    ];
    let idx = NodeIndex::build(&nodes);
    let edges = vec![edge("e0", "srv-0", "leaf-0", "unbundled", "leaf-server")];

    let conns = connections_for_edges(&edges, &idx).expect("map unbundled edge");
    assert_eq!(conns.len(), 1, "one unbundled edge -> one Connection");
    let c = &conns[0];
    assert_eq!(c.kind, "Connection");
    assert_eq!(c.api_version, WIRING_API);
    assert!(c.spec.unbundled.is_some(), "spec.unbundled present");
    let y = yaml(c);
    assert!(y.contains("unbundled:"), "yaml has unbundled key:\n{y}");
    assert!(!y.contains("fabric:"), "no other variant leaks:\n{y}");
}

#[test]
fn uplink_edges_aggregate_into_one_fabric_connection() {
    // The kernel emits per-port `uplink` edges; hhfab wants ONE fabric
    // Connection per leaf<->spine pair with the links aggregated.
    let nodes = vec![
        node("leaf-0", NodeType::Switch, "server-leaf", Some("frontend")),
        node("spine-0", NodeType::Spine, "spine", Some("frontend")),
    ];
    let idx = NodeIndex::build(&nodes);
    let edges = vec![
        edge("e0", "leaf-0", "spine-0", "uplink", "leaf-uplink"),
        edge("e1", "leaf-0", "spine-0", "uplink", "leaf-uplink"),
    ];

    let conns = connections_for_edges(&edges, &idx).expect("map uplink edges");
    assert_eq!(conns.len(), 1, "2 uplink edges, 1 pair -> 1 fabric Connection");
    let spec = conns[0].spec.fabric.as_ref().expect("spec.fabric present");
    assert_eq!(spec.links.len(), 2, "links aggregated, not split into CRDs");
    let y = yaml(&conns[0]);
    assert!(y.contains("fabric:"), "kernel `uplink` -> hhfab `fabric`:\n{y}");
}

#[test]
fn mesh_edges_aggregate_into_one_mesh_connection() {
    let nodes = vec![
        node("sw-a", NodeType::Switch, "server-leaf", Some("converged")),
        node("sw-b", NodeType::Switch, "server-leaf", Some("converged")),
    ];
    let idx = NodeIndex::build(&nodes);
    let edges = vec![
        edge("e0", "sw-a", "sw-b", "mesh", "mesh"),
        edge("e1", "sw-a", "sw-b", "mesh", "mesh"),
    ];

    let conns = connections_for_edges(&edges, &idx).expect("map mesh edges");
    assert_eq!(conns.len(), 1, "2 mesh edges, 1 pair -> 1 mesh Connection");
    let spec = conns[0].spec.mesh.as_ref().expect("spec.mesh present");
    assert_eq!(spec.links.len(), 2);
    let y = yaml(&conns[0]);
    assert!(y.contains("mesh:") && y.contains("leaf1") && y.contains("leaf2"), "{y}");
}

#[test]
fn bundled_edge_maps_to_bundled_connection() {
    let nodes = vec![
        node("srv-0", NodeType::Server, "server", None),
        node("leaf-0", NodeType::Switch, "server-leaf", Some("frontend")),
    ];
    let idx = NodeIndex::build(&nodes);
    let edges = vec![edge("e0", "srv-0", "leaf-0", "bundled", "leaf-server")];

    let conns = connections_for_edges(&edges, &idx).expect("map bundled edge");
    assert_eq!(conns.len(), 1);
    assert!(conns[0].spec.bundled.is_some(), "spec.bundled present");
}

#[test]
fn switch_crd_omits_empty_ecmp_and_redundancy() {
    // Hard rule (issue #9): no empty `ecmp: {}` (a known hhfab validate
    // failure). Omitting both ecmp and redundancy is confirmed to still
    // validate, so they must never appear in emitted Switch YAML.
    let n = node("leaf-0", NodeType::Switch, "server-leaf", Some("frontend"));
    let sw = switch_crd(&n).expect("map switch node");
    assert_eq!(sw.kind, "Switch");
    assert_eq!(sw.spec.profile, "vs", "synthesized validation default profile");
    let y = serde_yaml::to_string(&sw).expect("serialize switch");
    assert!(!y.contains("ecmp"), "no empty ecmp key:\n{y}");
    assert!(!y.contains("redundancy"), "no empty redundancy key:\n{y}");
}

#[test]
fn server_crd_has_description() {
    let n = node("srv-0", NodeType::Server, "server", None);
    let srv = server_crd(&n).expect("map server node");
    assert_eq!(srv.kind, "Server");
    assert!(!srv.spec.description.is_empty());
}
