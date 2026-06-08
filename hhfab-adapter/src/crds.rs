//! hhfab Kubernetes CRD types (the output shapes) and the WIT-facing option /
//! output / error types.
//!
//! CRD shapes are oracle-derived: every shape here was produced by
//! `hhfab vlab generate` and confirmed by `hhfab validate` on hhfab v0.43.1
//! (fabric API v0.96.2). `hhfab validate` is the authority; these structs are
//! the serialization target only.
//!
//! API groups (verified): everything is `wiring.githedgehog.com/v1beta1`
//! EXCEPT `IPv4Namespace`, which is `vpc.githedgehog.com/v1beta1`.

use serde::{Deserialize, Serialize};

pub const WIRING_API: &str = "wiring.githedgehog.com/v1beta1";
pub const VPC_API: &str = "vpc.githedgehog.com/v1beta1";

// ---------------------------------------------------------------------------
// WIT-facing types (wit/hhfab-adapter.wit)
// ---------------------------------------------------------------------------

/// `hhfab-options`.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct HhfabOptions {
    /// Restrict output to a single fabric by name; `None` exports all fabrics.
    pub fabric: Option<String>,
    /// Emit one document per managed fabric instead of one combined document.
    #[serde(default)]
    pub split_by_fabric: bool,
}

/// `wiring-document`.
#[derive(Debug, Clone, Serialize)]
pub struct WiringDocument {
    pub fabric: String,
    pub yaml: String,
}

/// `hhfab-output`.
#[derive(Debug, Clone, Serialize)]
pub struct HhfabOutput {
    pub documents: Vec<WiringDocument>,
}

/// `hhfab-error` variant. Serialized as `{ "kind": <variant>, "message": ... }`.
#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(tag = "kind", content = "message", rename_all = "snake_case")]
pub enum HhfabError {
    /// IR contains a topology shape the adapter cannot express.
    UnsupportedTopology(String),
    /// IR was structurally invalid (e.g. edge referencing an unknown node).
    InvalidIr(String),
    /// Unexpected internal failure.
    Internal(String),
}

impl std::fmt::Display for HhfabError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            HhfabError::UnsupportedTopology(m) => write!(f, "unsupported-topology: {m}"),
            HhfabError::InvalidIr(m) => write!(f, "invalid-ir: {m}"),
            HhfabError::Internal(m) => write!(f, "internal: {m}"),
        }
    }
}

impl std::error::Error for HhfabError {}

// ---------------------------------------------------------------------------
// CRD building blocks
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Serialize)]
pub struct Metadata {
    pub name: String,
}

/// A switch/server port reference, e.g. `leaf-01/E1/1` or `server-01/enp2s1`.
#[derive(Debug, Clone, Serialize)]
pub struct PortRef {
    pub port: String,
}

// ---- config CRDs (synthesized; not present in topology-ir) -----------------

#[derive(Debug, Clone, Serialize)]
pub struct VlanRange {
    pub from: u32,
    pub to: u32,
}

#[derive(Debug, Clone, Serialize)]
pub struct VlanNamespaceSpec {
    pub ranges: Vec<VlanRange>,
}

#[derive(Debug, Clone, Serialize)]
pub struct Ipv4NamespaceSpec {
    pub subnets: Vec<String>,
}

// ---- Switch ----------------------------------------------------------------

#[derive(Debug, Clone, Serialize)]
pub struct Boot {
    pub mac: String,
}

/// Switch spec. NOTE the deliberate omissions: `ecmp` and `redundancy` are NOT
/// fields here — emitting empty `ecmp: {}` is a known `hhfab validate` failure
/// (issue #9 hard rule), and omitting both is confirmed to still validate.
#[derive(Debug, Clone, Serialize)]
pub struct SwitchSpec {
    pub boot: Boot,
    pub description: String,
    /// Synthesized validation default (the virtual-switch profile accepted by
    /// `hhfab validate`). Mapping `device-class-id` -> a real hhfab SwitchProfile
    /// is a documented follow-up, out of scope for Phase 5.
    pub profile: String,
    pub role: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct ServerSpec {
    pub description: String,
}

// ---- Connection variants ---------------------------------------------------

#[derive(Debug, Clone, Serialize)]
pub struct ServerSwitchLink {
    pub server: PortRef,
    pub switch: PortRef,
}

#[derive(Debug, Clone, Serialize)]
pub struct UnbundledSpec {
    pub link: ServerSwitchLink,
}

/// `bundled` / `mclag` / `eslag` all use a list of server<->switch links.
#[derive(Debug, Clone, Serialize)]
pub struct ServerLinksSpec {
    pub links: Vec<ServerSwitchLink>,
}

#[derive(Debug, Clone, Serialize)]
pub struct LeafSpineLink {
    pub leaf: PortRef,
    pub spine: PortRef,
}

#[derive(Debug, Clone, Serialize)]
pub struct FabricSpec {
    pub links: Vec<LeafSpineLink>,
}

#[derive(Debug, Clone, Serialize)]
pub struct MeshLink {
    pub leaf1: PortRef,
    pub leaf2: PortRef,
}

#[derive(Debug, Clone, Serialize)]
pub struct MeshSpec {
    pub links: Vec<MeshLink>,
}

/// Connection spec — exactly one variant key is present per Connection. Unset
/// variants are omitted from the YAML.
#[derive(Debug, Clone, Default, Serialize)]
pub struct ConnectionSpec {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub unbundled: Option<UnbundledSpec>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bundled: Option<ServerLinksSpec>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mclag: Option<ServerLinksSpec>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub eslag: Option<ServerLinksSpec>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub fabric: Option<FabricSpec>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mesh: Option<MeshSpec>,
}

// ---------------------------------------------------------------------------
// Top-level CRD envelope
// ---------------------------------------------------------------------------

/// One Kubernetes CRD document (`apiVersion`/`kind`/`metadata`/`spec`).
#[derive(Debug, Clone, Serialize)]
pub struct Crd<S> {
    #[serde(rename = "apiVersion")]
    pub api_version: String,
    pub kind: String,
    pub metadata: Metadata,
    pub spec: S,
}

pub type VlanNamespace = Crd<VlanNamespaceSpec>;
pub type Ipv4Namespace = Crd<Ipv4NamespaceSpec>;
pub type SwitchGroup = Crd<SwitchGroupSpec>;
pub type Switch = Crd<SwitchSpec>;
pub type Server = Crd<ServerSpec>;
pub type Connection = Crd<ConnectionSpec>;

/// SwitchGroup spec is an empty object (`spec: {}`).
#[derive(Debug, Clone, Default, Serialize)]
pub struct SwitchGroupSpec {}
