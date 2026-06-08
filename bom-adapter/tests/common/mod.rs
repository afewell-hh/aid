//! Shared test helpers: load vendored BOM test data, load the fixtures'
//! `bom_totals` acceptance baselines, and aggregate rendered CSV/JSON by slug.
//!
//! The acceptance oracle is the kernel-derived `bom_totals` block in each
//! fixture's `tests/fixtures/valid/<fixture>/expected.json` (per-unit + fleet,
//! keyed by plan-entry id). A faithful renderer must reproduce these exactly
//! when its output is parsed back and aggregated by device-class slug.

#![allow(dead_code)]

use std::collections::BTreeMap;
use std::path::PathBuf;

use bom_adapter::DeviceClassBom;
use serde_json::Value;

/// The three valid fixtures with BOM baselines. `switch-bom` is the non-server
/// case (a switch with a transceiver sub-component; root SKU included).
pub const FIXTURES: &[&str] = &["clos-small", "mesh-two-switch", "switch-bom"];

pub fn manifest(rel: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(rel)
}

/// Load a fixture's vendored `device-class-bom[]` (the adapter's sole input).
pub fn load_boms(fixture: &str) -> Vec<DeviceClassBom> {
    let p = manifest(&format!("tests/testdata/{fixture}.boms.json"));
    let s = std::fs::read_to_string(&p).unwrap_or_else(|e| panic!("read {}: {e}", p.display()));
    DeviceClassBom::list_from_json(&s).unwrap_or_else(|e| panic!("parse {fixture} boms: {e}"))
}

/// entry_id -> (slug -> quantity).
pub type Totals = BTreeMap<String, BTreeMap<String, u64>>;

/// Aggregated per-unit and fleet quantities, keyed by entry then slug.
pub struct Agg {
    pub per_unit: Totals,
    /// Empty per entry when fleet columns were omitted.
    pub fleet: Totals,
}

fn add(t: &mut Totals, entry: &str, slug: &str, qty: u64) {
    *t.entry(entry.to_string())
        .or_default()
        .entry(slug.to_string())
        .or_default() += qty;
}

/// Expected aggregate built from a fixture's `bom_totals` baseline.
pub fn expected_agg(fixture: &str) -> Agg {
    let p = manifest(&format!("../tests/fixtures/valid/{fixture}/expected.json"));
    let s = std::fs::read_to_string(&p).unwrap_or_else(|e| panic!("read {}: {e}", p.display()));
    let v: Value = serde_json::from_str(&s).expect("parse expected.json");
    let bt = v.get("bom_totals").expect("bom_totals block");
    let mut per_unit = Totals::new();
    let mut fleet = Totals::new();
    for (entry, obj) in bt.as_object().expect("bom_totals object") {
        for (slug, q) in obj["per_unit"].as_object().expect("per_unit") {
            add(&mut per_unit, entry, slug, q.as_u64().expect("per_unit qty"));
        }
        for (slug, q) in obj["fleet"].as_object().expect("fleet") {
            add(&mut fleet, entry, slug, q.as_u64().expect("fleet qty"));
        }
    }
    Agg { per_unit, fleet }
}

/// Parse rendered CSV and aggregate `quantity_per_unit` / `fleet_quantity` by
/// `(entry_id, slug)`. Tolerates the `fleet_quantity` column being absent.
pub fn aggregate_csv(content: &str) -> Agg {
    let mut rdr = csv::ReaderBuilder::new()
        .has_headers(true)
        .from_reader(content.as_bytes());
    let headers: Vec<String> = rdr
        .headers()
        .expect("csv headers")
        .iter()
        .map(|s| s.to_string())
        .collect();
    let col = |name: &str| headers.iter().position(|h| h == name);
    let i_entry = col("entry_id").expect("entry_id column");
    let i_slug = col("slug").expect("slug column");
    let i_pu = col("quantity_per_unit").expect("quantity_per_unit column");
    let i_fleet = col("fleet_quantity");

    let mut per_unit = Totals::new();
    let mut fleet = Totals::new();
    for rec in rdr.records() {
        let rec = rec.expect("csv record");
        let entry = &rec[i_entry];
        let slug = &rec[i_slug];
        let pu: u64 = rec[i_pu].parse().expect("quantity_per_unit u64");
        add(&mut per_unit, entry, slug, pu);
        if let Some(fi) = i_fleet {
            let fq: u64 = rec[fi].parse().expect("fleet_quantity u64");
            add(&mut fleet, entry, slug, fq);
        }
    }
    Agg { per_unit, fleet }
}

/// Parse rendered JSON and aggregate `quantity_per_unit` / `fleet_quantity` by
/// `(entry_id, slug)`. Tolerates `fleet_quantity` being absent on line-items.
pub fn aggregate_json(content: &str) -> Agg {
    let v: Value = serde_json::from_str(content).expect("parse rendered JSON");
    let boms = v.get("boms").and_then(|b| b.as_array()).expect("boms array");
    let mut per_unit = Totals::new();
    let mut fleet = Totals::new();
    for b in boms {
        let entry = b["entry_id"].as_str().expect("entry_id");
        for li in b["line_items"].as_array().expect("line_items") {
            let slug = li["slug"].as_str().expect("line-item slug");
            let pu = li["quantity_per_unit"].as_u64().expect("quantity_per_unit");
            add(&mut per_unit, entry, slug, pu);
            if let Some(fq) = li.get("fleet_quantity").and_then(|x| x.as_u64()) {
                add(&mut fleet, entry, slug, fq);
            }
        }
    }
    Agg { per_unit, fleet }
}
