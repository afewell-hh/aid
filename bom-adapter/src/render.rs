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

use crate::bom::{BomLineItem, DeviceClassBom, DeviceClassSummary};

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
/// Faithful rendering: line-items are emitted in the kernel's order (no
/// re-sorting), per-unit columns are always present, `include_fleet_totals`
/// toggles the fleet column/field, and quantities are printed as given — the
/// adapter never recomputes the BOM or re-derives the role-based root rule.
pub fn export_bom(boms: &[DeviceClassBom], options: &BomOptions) -> Result<BomOutput, BomError> {
    validate(boms)?;
    let content = match options.format {
        BomFormat::Csv => render_csv(boms, options.include_fleet_totals)?,
        BomFormat::Json => render_json(boms, options.include_fleet_totals)?,
    };
    Ok(BomOutput {
        format: options.format,
        content,
    })
}

/// Structural checks. `empty-bom` when there is nothing to render; `invalid-bom`
/// when a line-item carries a zero quantity or violates the per-unit x
/// plan-quantity = fleet invariant. Never panics.
fn validate(boms: &[DeviceClassBom]) -> Result<(), BomError> {
    let total_lines: usize = boms.iter().map(|b| b.line_items.len()).sum();
    if total_lines == 0 {
        return Err(BomError::EmptyBom);
    }
    for bom in boms {
        for li in &bom.line_items {
            if li.quantity_per_parent < 1 || li.quantity_per_unit < 1 {
                return Err(BomError::InvalidBom(format!(
                    "entry {} line {}: zero quantity (per_parent={}, per_unit={})",
                    bom.entry_id,
                    path_str(&li.path, &li.device_class),
                    li.quantity_per_parent,
                    li.quantity_per_unit,
                )));
            }
            let expected_fleet = li.quantity_per_unit as u64 * bom.plan_quantity as u64;
            if li.fleet_quantity as u64 != expected_fleet {
                return Err(BomError::InvalidBom(format!(
                    "entry {} line {}: fleet_quantity {} != quantity_per_unit {} * plan_quantity {}",
                    bom.entry_id,
                    path_str(&li.path, &li.device_class),
                    li.fleet_quantity,
                    li.quantity_per_unit,
                    bom.plan_quantity,
                )));
            }
        }
    }
    Ok(())
}

/// A readable identifier for a line in error messages: the slot path, or the
/// device-class slug for the root SKU line (empty path).
fn path_str(path: &[String], dc: &DeviceClassSummary) -> String {
    if path.is_empty() {
        format!("<root {}>", dc.slug)
    } else {
        path.join("/")
    }
}

/// CSV: one flat table for all BOMs, rows contiguous per `entry_id`. Header
/// columns per BOM_CONTRACT.md; the `fleet_quantity` column is dropped when
/// `include_fleet` is false (per-unit columns always present).
fn render_csv(boms: &[DeviceClassBom], include_fleet: bool) -> Result<String, BomError> {
    let mut wtr = csv::Writer::from_writer(Vec::new());

    let mut header = vec![
        "entry_id",
        "plan_quantity",
        "level",
        "path",
        "slug",
        "name",
        "category",
        "manufacturer",
        "part_number",
        "quantity_per_parent",
        "quantity_per_unit",
    ];
    if include_fleet {
        header.push("fleet_quantity");
    }
    wtr.write_record(&header)
        .map_err(|e| BomError::Internal(format!("csv header: {e}")))?;

    for bom in boms {
        for li in &bom.line_items {
            let dc = &li.device_class;
            let mut row = vec![
                bom.entry_id.clone(),
                bom.plan_quantity.to_string(),
                li.path.len().to_string(),
                li.path.join("/"),
                dc.slug.clone(),
                dc.name.clone(),
                dc.category.clone(),
                dc.manufacturer.clone().unwrap_or_default(),
                dc.part_number.clone().unwrap_or_default(),
                li.quantity_per_parent.to_string(),
                li.quantity_per_unit.to_string(),
            ];
            if include_fleet {
                row.push(li.fleet_quantity.to_string());
            }
            wtr.write_record(&row)
                .map_err(|e| BomError::Internal(format!("csv row: {e}")))?;
        }
    }

    let bytes = wtr
        .into_inner()
        .map_err(|e| BomError::Internal(format!("csv flush: {e}")))?;
    String::from_utf8(bytes).map_err(|e| BomError::Internal(format!("csv utf-8: {e}")))
}

// ---------------------------------------------------------------------------
// JSON output shape (BOM_CONTRACT.md). Per-unit fields always present;
// `fleet_quantity` omitted from each line-item when fleet totals are off.
// ---------------------------------------------------------------------------

#[derive(Serialize)]
struct JsonOutput<'a> {
    include_fleet_totals: bool,
    boms: Vec<JsonBom<'a>>,
}

#[derive(Serialize)]
struct JsonBom<'a> {
    entry_id: &'a str,
    plan_quantity: u32,
    device_class: &'a DeviceClassSummary,
    line_items: Vec<JsonLine<'a>>,
}

#[derive(Serialize)]
struct JsonLine<'a> {
    path: &'a [String],
    level: usize,
    slug: &'a str,
    name: &'a str,
    category: &'a str,
    manufacturer: &'a Option<String>,
    part_number: &'a Option<String>,
    quantity_per_parent: u32,
    quantity_per_unit: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    fleet_quantity: Option<u32>,
}

fn json_line(li: &BomLineItem, include_fleet: bool) -> JsonLine<'_> {
    let dc = &li.device_class;
    JsonLine {
        path: &li.path,
        level: li.path.len(),
        slug: &dc.slug,
        name: &dc.name,
        category: &dc.category,
        manufacturer: &dc.manufacturer,
        part_number: &dc.part_number,
        quantity_per_parent: li.quantity_per_parent,
        quantity_per_unit: li.quantity_per_unit,
        fleet_quantity: include_fleet.then_some(li.fleet_quantity),
    }
}

fn render_json(boms: &[DeviceClassBom], include_fleet: bool) -> Result<String, BomError> {
    let out = JsonOutput {
        include_fleet_totals: include_fleet,
        boms: boms
            .iter()
            .map(|bom| JsonBom {
                entry_id: &bom.entry_id,
                plan_quantity: bom.plan_quantity,
                device_class: &bom.device_class,
                line_items: bom
                    .line_items
                    .iter()
                    .map(|li| json_line(li, include_fleet))
                    .collect(),
            })
            .collect(),
    };
    serde_json::to_string_pretty(&out).map_err(|e| BomError::Internal(format!("json encode: {e}")))
}
