//! Layer-1 -> Layer-2 input contract: the `device-class-bom[]` wire shape.
//!
//! These structs deserialize the snake_case JSON emitted by `tools/bom-gen`
//! (the merged Phase-3 kernel `calculate()` `boms` slice). Per the D16 extension
//! to Layer 2, that JSON is the single-sourced wire contract between the kernel
//! (Layer 1) and this adapter (Layer 2) — the same bytes the Phase-6 Go host
//! will hand the adapter. Every field maps one-to-one to `wit/types.wit`
//! `device-class-bom` / `bom-line-item` / `device-class-summary`; see
//! `BOM_CONTRACT.md` for the field-by-field mapping table.
//!
//! This module ONLY models the BOM. The adapter never reads plan YAML or NetBox,
//! and it RENDERS these values — it never recomputes them or re-derives the
//! role-based root-inclusion rule (the kernel already applied it).

use serde::{Deserialize, Serialize};

/// `types.wit` record `device-class-summary` — a flat identity snapshot.
/// `Serialize` so the JSON renderer can embed it verbatim as `device_class`.
#[derive(Debug, Clone, Deserialize, Serialize, PartialEq, Eq)]
pub struct DeviceClassSummary {
    pub id: String,
    pub name: String,
    pub slug: String,
    pub category: String,
    #[serde(default)]
    pub manufacturer: Option<String>,
    #[serde(default)]
    pub part_number: Option<String>,
}

/// `types.wit` record `bom-line-item` — one resolved BOM line, flattened from
/// the recursive sub-component tree. `path` is the slot path from the root
/// device class (empty for the root SKU line, present only on switch entries).
#[derive(Debug, Clone, Deserialize, PartialEq, Eq)]
pub struct BomLineItem {
    pub path: Vec<String>,
    pub device_class: DeviceClassSummary,
    pub quantity_per_parent: u32,
    /// Product of `quantity_per_parent` along the path (per root unit).
    pub quantity_per_unit: u32,
    /// `quantity_per_unit * plan_quantity`.
    pub fleet_quantity: u32,
}

/// `types.wit` record `device-class-bom` — the BOM for one plan entry's device
/// class. Per-unit values are independent of fleet size; fleet values scale by
/// `plan_quantity`.
#[derive(Debug, Clone, Deserialize, PartialEq, Eq)]
pub struct DeviceClassBom {
    pub device_class: DeviceClassSummary,
    pub entry_id: String,
    pub plan_quantity: u32,
    pub line_items: Vec<BomLineItem>,
}

impl DeviceClassBom {
    /// Parse a `device-class-bom[]` list from its snake_case JSON wire form.
    pub fn list_from_json(s: &str) -> Result<Vec<DeviceClassBom>, serde_json::Error> {
        serde_json::from_str(s)
    }
}
