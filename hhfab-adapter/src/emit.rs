//! IR -> CRD transformation (the core of the adapter).
//!
//! Pure mapping from `topology-ir` to hhfab wiring YAML. Correctness is defined
//! by `hhfab validate` (see `tests/validate.rs`).
//!
//! Port synthesis (ref issue #29): the IR's `topology-edge.port_a/port_b` carry
//! IR-internal zone refs (e.g. `nic-fe:0`), not concrete switch ports, so this
//! adapter synthesizes deterministic, non-overlapping hhfab port names. Each
//! edge consumes the next free port on each of its two endpoint nodes, walking
//! the edges in IR order: switch/spine ports are `<name>/E1/<n>` and server
//! ports are `<name>/enp2s<n>`, both 1-based per node. Because each switch
//! belongs to exactly one fabric, this is stable across combined / split /
//! fabric-scoped exports. The topologically meaningful endpoint assignment
//! comes from the edges' `node-a-id`/`node-b-id`; only the exact port number is
//! adapter-chosen (#29).

use std::collections::{BTreeMap, BTreeSet, HashMap, HashSet};

use crate::crds::*;
use crate::ir::{NodeIndex, NodeType, TopologyEdge, TopologyIr, TopologyNode};

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

const LABEL_COMBINED: &str = "combined";

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

/// Produce wiring YAML document(s) from a calculated topology IR.
///
/// `options.fabric = Some(name)` restricts output to one fabric;
/// `options.split_by_fabric = true` emits one self-contained document per
/// managed fabric, otherwise a single combined document.
pub fn export_wiring(
    ir: &TopologyIr,
    options: &HhfabOptions,
) -> Result<HhfabOutput, HhfabError> {
    let nodes = NodeIndex::build(&ir.nodes);

    // All fabrics present in the IR (on switch/spine nodes and on edges). The IR
    // does not carry fabric-class, so every fabric with topology is treated as a
    // managed, exportable fabric (documented assumption; fixtures are all
    // managed).
    let mut all_fabrics: BTreeSet<String> = BTreeSet::new();
    for n in &ir.nodes {
        if let Some(f) = &n.fabric {
            all_fabrics.insert(f.clone());
        }
    }
    for e in &ir.edges {
        all_fabrics.insert(e.fabric.clone());
    }

    // Apply the fabric filter.
    let selected: Vec<String> = match &options.fabric {
        Some(name) => {
            if !all_fabrics.contains(name) {
                return Err(HhfabError::UnsupportedTopology(format!(
                    "requested fabric `{name}` not present in IR"
                )));
            }
            vec![name.clone()]
        }
        None => all_fabrics.iter().cloned().collect(),
    };

    let documents = if options.split_by_fabric {
        let mut docs = Vec::with_capacity(selected.len());
        for fabric in &selected {
            let scope: HashSet<String> = std::iter::once(fabric.clone()).collect();
            docs.push(build_document(ir, &nodes, &scope, fabric.clone())?);
        }
        docs
    } else {
        let scope: HashSet<String> = selected.iter().cloned().collect();
        let label = if selected.len() == 1 {
            selected[0].clone()
        } else {
            LABEL_COMBINED.to_string()
        };
        vec![build_document(ir, &nodes, &scope, label)?]
    };

    Ok(HhfabOutput { documents })
}

// ---------------------------------------------------------------------------
// Document assembly
// ---------------------------------------------------------------------------

fn build_document(
    ir: &TopologyIr,
    nodes: &NodeIndex,
    fabrics: &HashSet<String>,
    label: String,
) -> Result<WiringDocument, HhfabError> {
    let scoped_edges: Vec<TopologyEdge> = ir
        .edges
        .iter()
        .filter(|e| fabrics.contains(&e.fabric))
        .cloned()
        .collect();

    let connections = connections_for_edges(&scoped_edges, nodes)?;

    // Switch/spine CRDs: every non-server node in scope, sorted by name.
    let mut switches: Vec<&TopologyNode> = ir
        .nodes
        .iter()
        .filter(|n| n.node_type != NodeType::Server)
        .filter(|n| n.fabric.as_ref().is_some_and(|f| fabrics.contains(f)))
        .collect();
    switches.sort_by(|a, b| a.name.cmp(&b.name));

    // Server CRDs: server nodes that are an endpoint of a scoped edge.
    let mut server_ids: BTreeSet<String> = BTreeSet::new();
    for e in &scoped_edges {
        for id in [&e.node_a_id, &e.node_b_id] {
            if let Some(n) = nodes.get(id) {
                if n.node_type == NodeType::Server {
                    server_ids.insert(id.clone());
                }
            }
        }
    }

    // Assemble: config CRDs, then Switches, Servers, Connections (sample order).
    let mut docs: Vec<String> = Vec::new();
    docs.push(to_yaml(&vlan_namespace())?);
    docs.push(to_yaml(&ipv4_namespace())?);
    docs.push(to_yaml(&switch_group())?);
    for n in &switches {
        docs.push(to_yaml(&switch_crd(n)?)?);
    }
    for id in &server_ids {
        let n = nodes
            .get(id)
            .ok_or_else(|| HhfabError::InvalidIr(format!("unknown server node `{id}`")))?;
        docs.push(to_yaml(&server_crd(n)?)?);
    }
    for c in &connections {
        docs.push(to_yaml(c)?);
    }

    Ok(WiringDocument {
        fabric: label,
        yaml: docs.join("---\n"),
    })
}

fn to_yaml<T: serde::Serialize>(value: &T) -> Result<String, HhfabError> {
    serde_yaml::to_string(value).map_err(|e| HhfabError::Internal(format!("yaml: {e}")))
}

// ---------------------------------------------------------------------------
// Config CRDs (synthesized defaults)
// ---------------------------------------------------------------------------

fn crd<S>(api: &str, kind: &str, name: &str, spec: S) -> Crd<S> {
    Crd {
        api_version: api.to_string(),
        kind: kind.to_string(),
        metadata: Metadata {
            name: name.to_string(),
        },
        spec,
    }
}

fn vlan_namespace() -> VlanNamespace {
    crd(
        WIRING_API,
        "VLANNamespace",
        "default",
        VlanNamespaceSpec {
            ranges: vec![VlanRange {
                from: DEFAULT_VLAN_FROM,
                to: DEFAULT_VLAN_TO,
            }],
        },
    )
}

fn ipv4_namespace() -> Ipv4Namespace {
    crd(
        VPC_API,
        "IPv4Namespace",
        "default",
        Ipv4NamespaceSpec {
            subnets: vec![DEFAULT_IPV4_SUBNET.to_string()],
        },
    )
}

fn switch_group() -> SwitchGroup {
    crd(
        WIRING_API,
        "SwitchGroup",
        DEFAULT_SWITCHGROUP_NAME,
        SwitchGroupSpec {},
    )
}

// ---------------------------------------------------------------------------
// Node CRDs
// ---------------------------------------------------------------------------

/// Map a switch/spine node to a `Switch` CRD. `ecmp`/`redundancy` are never
/// emitted (see `SwitchSpec`). MAC is synthesized deterministically from the
/// node id; `profile` is the synthesized validation default (`vs`).
pub fn switch_crd(node: &TopologyNode) -> Result<Switch, HhfabError> {
    let role = node.hedgehog_role.clone().ok_or_else(|| {
        HhfabError::InvalidIr(format!("switch node `{}` has no hedgehog-role", node.node_id))
    })?;
    Ok(crd(
        WIRING_API,
        "Switch",
        &node.name,
        SwitchSpec {
            boot: Boot {
                mac: synth_mac(&node.node_id),
            },
            description: node.name.clone(),
            profile: DEFAULT_SWITCH_PROFILE.to_string(),
            role,
        },
    ))
}

/// Map a server node to a `Server` CRD.
pub fn server_crd(node: &TopologyNode) -> Result<Server, HhfabError> {
    Ok(crd(
        WIRING_API,
        "Server",
        &node.name,
        ServerSpec {
            description: node.name.clone(),
        },
    ))
}

/// Deterministic, locally-unique MAC from a node id (FNV-1a, low 3 bytes under
/// the `0c:20:12` prefix). Stable across runs and independent of node ordering.
fn synth_mac(node_id: &str) -> String {
    let mut hash: u32 = 0x811c_9dc5;
    for b in node_id.bytes() {
        hash ^= b as u32;
        hash = hash.wrapping_mul(0x0100_0193);
    }
    let b = hash.to_le_bytes();
    format!("0c:20:12:{:02x}:{:02x}:{:02x}", b[0], b[1], b[2])
}

// ---------------------------------------------------------------------------
// Connections (edge -> CRD, with per-port edges aggregated into links[])
// ---------------------------------------------------------------------------

/// Map a set of topology edges to Connection CRDs, dispatching on
/// `connection_type` and aggregating multi-link connections:
///   - `unbundled`               -> one `spec.unbundled` Connection per edge
///   - `bundled`/`mclag`/`eslag` -> one Connection per server group (`links[]`)
///   - `uplink`                  -> one `spec.fabric` Connection per leaf<->spine
///                                  pair (`links[]` aggregated)
///   - `mesh`                    -> one `spec.mesh` Connection per switch<->switch
///                                  pair (`links[]` aggregated)
pub fn connections_for_edges(
    edges: &[TopologyEdge],
    nodes: &NodeIndex,
) -> Result<Vec<Connection>, HhfabError> {
    let ports = assign_ports(edges, nodes)?;
    let name_of = |id: &str| nodes.get(id).map(|n| n.name.clone());

    let mut out: Vec<Connection> = Vec::new();

    // Aggregators (BTreeMap for deterministic ordering).
    let mut server_groups: BTreeMap<(String, String, String), Vec<ServerSwitchLink>> =
        BTreeMap::new(); // (variant, server, switch) -> links
    let mut fabric_groups: BTreeMap<(String, String), Vec<LeafSpineLink>> = BTreeMap::new(); // (spine, leaf)
    let mut mesh_groups: BTreeMap<(String, String), Vec<MeshLink>> = BTreeMap::new(); // (a, b) sorted

    for e in edges {
        let (pa, pb) = ports
            .get(&e.edge_id)
            .ok_or_else(|| HhfabError::Internal(format!("no port for edge `{}`", e.edge_id)))?;
        let na = name_of(&e.node_a_id)
            .ok_or_else(|| HhfabError::InvalidIr(format!("edge `{}` -> unknown node `{}`", e.edge_id, e.node_a_id)))?;
        let nb = name_of(&e.node_b_id)
            .ok_or_else(|| HhfabError::InvalidIr(format!("edge `{}` -> unknown node `{}`", e.edge_id, e.node_b_id)))?;

        match e.connection_type.as_str() {
            "unbundled" => {
                // node_a = server, node_b = switch. One link per Connection.
                out.push(connection(
                    &format!("{na}--unbundled--{nb}"),
                    ConnectionSpec {
                        unbundled: Some(UnbundledSpec {
                            link: ServerSwitchLink {
                                server: PortRef { port: pa.clone() },
                                switch: PortRef { port: pb.clone() },
                            },
                        }),
                        ..Default::default()
                    },
                ));
            }
            v @ ("bundled" | "mclag" | "eslag") => {
                server_groups
                    .entry((v.to_string(), na.clone(), nb.clone()))
                    .or_default()
                    .push(ServerSwitchLink {
                        server: PortRef { port: pa.clone() },
                        switch: PortRef { port: pb.clone() },
                    });
            }
            "uplink" => {
                // node_a = leaf, node_b = spine. hhfab variant is `fabric`.
                fabric_groups
                    .entry((nb.clone(), na.clone()))
                    .or_default()
                    .push(LeafSpineLink {
                        leaf: PortRef { port: pa.clone() },
                        spine: PortRef { port: pb.clone() },
                    });
            }
            "mesh" => {
                // Unordered switch pair; keep link endpoints aligned with the
                // sorted (first, second) key so leaf1/leaf2 are stable.
                let (first, second, p_first, p_second) = if na <= nb {
                    (na.clone(), nb.clone(), pa.clone(), pb.clone())
                } else {
                    (nb.clone(), na.clone(), pb.clone(), pa.clone())
                };
                mesh_groups
                    .entry((first, second))
                    .or_default()
                    .push(MeshLink {
                        leaf1: PortRef { port: p_first },
                        leaf2: PortRef { port: p_second },
                    });
            }
            other => {
                return Err(HhfabError::UnsupportedTopology(format!(
                    "edge `{}` has unknown connection-type `{other}`",
                    e.edge_id
                )));
            }
        }
    }

    for ((variant, server, switch), links) in server_groups {
        let name = match variant.as_str() {
            "mclag" | "eslag" => format!("{server}--{variant}--{switch}"),
            _ => format!("{server}--bundled--{switch}"),
        };
        let spec = match variant.as_str() {
            "mclag" => ConnectionSpec {
                mclag: Some(ServerLinksSpec { links }),
                ..Default::default()
            },
            "eslag" => ConnectionSpec {
                eslag: Some(ServerLinksSpec { links }),
                ..Default::default()
            },
            _ => ConnectionSpec {
                bundled: Some(ServerLinksSpec { links }),
                ..Default::default()
            },
        };
        out.push(connection(&name, spec));
    }

    for ((spine, leaf), links) in fabric_groups {
        out.push(connection(
            &format!("{spine}--fabric--{leaf}"),
            ConnectionSpec {
                fabric: Some(FabricSpec { links }),
                ..Default::default()
            },
        ));
    }

    for ((a, b), links) in mesh_groups {
        out.push(connection(
            &format!("{a}--mesh--{b}"),
            ConnectionSpec {
                mesh: Some(MeshSpec { links }),
                ..Default::default()
            },
        ));
    }

    // Stable order + guard against duplicate metadata names (hhfab requires
    // unique Connection names).
    out.sort_by(|x, y| x.metadata.name.cmp(&y.metadata.name));
    let mut seen: HashSet<String> = HashSet::new();
    for c in &mut out {
        if !seen.insert(c.metadata.name.clone()) {
            let mut n = 2;
            loop {
                let candidate = format!("{}-{n}", c.metadata.name);
                if seen.insert(candidate.clone()) {
                    c.metadata.name = candidate;
                    break;
                }
                n += 1;
            }
        }
    }

    Ok(out)
}

fn connection(name: &str, spec: ConnectionSpec) -> Connection {
    crd(WIRING_API, "Connection", name, spec)
}

/// Assign deterministic, non-overlapping hhfab port names. Each edge consumes
/// the next free port on each endpoint, in IR edge order. See module docs (#29).
fn assign_ports(
    edges: &[TopologyEdge],
    nodes: &NodeIndex,
) -> Result<HashMap<String, (String, String)>, HhfabError> {
    let mut switch_cursor: HashMap<String, u32> = HashMap::new();
    let mut server_cursor: HashMap<String, u32> = HashMap::new();
    let mut map: HashMap<String, (String, String)> = HashMap::new();

    for e in edges {
        let a = nodes.get(&e.node_a_id).ok_or_else(|| {
            HhfabError::InvalidIr(format!("edge `{}` -> unknown node `{}`", e.edge_id, e.node_a_id))
        })?;
        let b = nodes.get(&e.node_b_id).ok_or_else(|| {
            HhfabError::InvalidIr(format!("edge `{}` -> unknown node `{}`", e.edge_id, e.node_b_id))
        })?;
        let pa = next_port(a, &mut switch_cursor, &mut server_cursor);
        let pb = next_port(b, &mut switch_cursor, &mut server_cursor);
        map.insert(e.edge_id.clone(), (pa, pb));
    }
    Ok(map)
}

fn next_port(
    node: &TopologyNode,
    switch_cursor: &mut HashMap<String, u32>,
    server_cursor: &mut HashMap<String, u32>,
) -> String {
    match node.node_type {
        NodeType::Server => {
            let c = server_cursor.entry(node.node_id.clone()).or_insert(1);
            let port = format!("{}/enp2s{}", node.name, *c);
            *c += 1;
            port
        }
        NodeType::Switch | NodeType::Spine => {
            let c = switch_cursor.entry(node.node_id.clone()).or_insert(1);
            let port = format!("{}/E1/{}", node.name, *c);
            *c += 1;
            port
        }
    }
}
