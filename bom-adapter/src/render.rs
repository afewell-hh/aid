//! WIT-facing option / output / error types and the rendering entry point.
//!
//! `wit/bom-adapter.wit` is the contract of record:
//!   export-bom(boms, options) -> result<bom-output, bom-error>
//! with `bom-format { csv, json }`, `bom-options { format, include-fleet-totals }`,
//! `bom-output { format, content }`, and `bom-error { empty-bom, invalid-bom(string),
//! internal(string) }`.
//!
//! The adapter RENDERS `device-class-bom[]` faithfully: per-unit columns are
//! always present, `include-fleet-totals` toggles the fleet columns, and the
//! kernel's emitted line-item order (pre-order DFS — switch root SKU first, then
//! its sub-components) is preserved. It never recomputes the BOM or re-derives
//! the role-based root rule.

use serde::{Deserialize, Serialize};

use crate::bom::DeviceClassBom;

/// `bom-format`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum BomFormat {
    Csv,
    Json,
}

/// `bom-options`. Per-unit columns are always present; `include_fleet_totals`
/// toggles the fleet columns.
#[derive(Debug, Clone, Deserialize)]
pub struct BomOptions {
    pub format: BomFormat,
    #[serde(default)]
    pub include_fleet_totals: bool,
}

impl Default for BomOptions {
    fn default() -> Self {
        BomOptions {
            format: BomFormat::Csv,
            include_fleet_totals: true,
        }
    }
}

/// `bom-output`. `content` is CSV text or JSON text per `format`.
#[derive(Debug, Clone, Serialize)]
pub struct BomOutput {
    pub format: BomFormat,
    pub content: String,
}

/// `bom-error` variant. Serialized as `{ "kind": <variant>, "message": ... }`
/// (the `empty_bom` unit variant carries no message).
#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(tag = "kind", content = "message", rename_all = "snake_case")]
pub enum BomError {
    /// No BOM lines to render.
    EmptyBom,
    /// A BOM was structurally invalid (e.g. negative/zero quantities, or a
    /// broken per-unit x plan-quantity = fleet invariant).
    InvalidBom(String),
    /// Unexpected internal failure.
    Internal(String),
}

impl std::fmt::Display for BomError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            BomError::EmptyBom => write!(f, "empty-bom"),
            BomError::InvalidBom(m) => write!(f, "invalid-bom: {m}"),
            BomError::Internal(m) => write!(f, "internal: {m}"),
        }
    }
}

impl std::error::Error for BomError {}

/// Render one or more device-class BOMs to a single CSV or JSON document.
///
/// RED STUB: not yet implemented — every call returns `internal`, so all
/// rendering/acceptance/golden/wasm tests fail until GREEN.
pub fn export_bom(_boms: &[DeviceClassBom], _options: &BomOptions) -> Result<BomOutput, BomError> {
    Err(BomError::Internal(
        "export_bom not implemented (RED stub)".to_string(),
    ))
}
