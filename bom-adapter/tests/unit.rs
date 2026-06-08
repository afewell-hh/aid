//! RED unit tests — per-unit invariance across fleet sizes, the
//! `include-fleet-totals` toggle, and the structured error variants. All FAIL
//! now: `export_bom` is a stub returning `internal`.

mod common;

use bom_adapter::{
    export_bom, BomError, BomFormat, BomLineItem, BomOptions, DeviceClassBom, DeviceClassSummary,
};
use common::{aggregate_json, load_boms};

fn summary(slug: &str) -> DeviceClassSummary {
    DeviceClassSummary {
        id: slug.to_string(),
        name: slug.to_string(),
        slug: slug.to_string(),
        category: "x".to_string(),
        manufacturer: None,
        part_number: None,
    }
}

/// A switch-shaped BOM (root SKU + 64 transceivers) at a given fleet size.
/// Per-unit quantities are independent of `plan_q`; fleet scales by `plan_q`.
fn switch_bom_at(plan_q: u32) -> DeviceClassBom {
    DeviceClassBom {
        device_class: summary("leaf"),
        entry_id: "leaves".to_string(),
        plan_quantity: plan_q,
        line_items: vec![
            BomLineItem {
                path: vec![],
                device_class: summary("leaf"),
                quantity_per_parent: 1,
                quantity_per_unit: 1,
                fleet_quantity: plan_q,
            },
            BomLineItem {
                path: vec!["xcvr".to_string()],
                device_class: summary("xcvr"),
                quantity_per_parent: 64,
                quantity_per_unit: 64,
                fleet_quantity: 64 * plan_q,
            },
        ],
    }
}

fn opts(format: BomFormat, fleet: bool) -> BomOptions {
    BomOptions {
        format,
        include_fleet_totals: fleet,
    }
}

/// Per-unit BOM is identical regardless of fleet quantity; fleet = per-unit x
/// plan-quantity at each tree level.
#[test]
fn per_unit_invariant_across_fleet_sizes() {
    let a = export_bom(&[switch_bom_at(2)], &opts(BomFormat::Json, true)).expect("render a");
    let b = export_bom(&[switch_bom_at(5)], &opts(BomFormat::Json, true)).expect("render b");
    let agg_a = aggregate_json(&a.content);
    let agg_b = aggregate_json(&b.content);

    // Per-unit columns are independent of fleet size.
    assert_eq!(
        agg_a.per_unit, agg_b.per_unit,
        "per-unit must not change with plan_quantity"
    );

    // Fleet scales: fleet = per-unit x plan-quantity at each level.
    assert_eq!(agg_a.fleet["leaves"]["leaf"], 1 * 2);
    assert_eq!(agg_a.fleet["leaves"]["xcvr"], 64 * 2);
    assert_eq!(agg_b.fleet["leaves"]["leaf"], 1 * 5);
    assert_eq!(agg_b.fleet["leaves"]["xcvr"], 64 * 5);
}

#[test]
fn include_fleet_totals_toggles_csv_fleet_column() {
    let boms = load_boms("switch-bom");
    let with = export_bom(&boms, &opts(BomFormat::Csv, true)).expect("csv with fleet");
    let without = export_bom(&boms, &opts(BomFormat::Csv, false)).expect("csv without fleet");
    assert!(
        with.content.contains("fleet_quantity"),
        "fleet column present when enabled"
    );
    assert!(
        !without.content.contains("fleet_quantity"),
        "fleet column absent when disabled"
    );
    // Per-unit column is always present.
    assert!(without.content.contains("quantity_per_unit"));
}

#[test]
fn include_fleet_totals_toggles_json_fleet_field() {
    let boms = load_boms("switch-bom");
    let with = export_bom(&boms, &opts(BomFormat::Json, true)).expect("json with fleet");
    let without = export_bom(&boms, &opts(BomFormat::Json, false)).expect("json without fleet");
    assert!(with.content.contains("fleet_quantity"));
    assert!(!without.content.contains("fleet_quantity"));
    assert!(without.content.contains("quantity_per_unit"));
}

#[test]
fn empty_input_list_is_empty_bom_error() {
    let err = export_bom(&[], &opts(BomFormat::Csv, true)).unwrap_err();
    assert_eq!(err, BomError::EmptyBom);
}

#[test]
fn all_empty_line_items_is_empty_bom_error() {
    let bom = DeviceClassBom {
        device_class: summary("leaf"),
        entry_id: "leaves".to_string(),
        plan_quantity: 2,
        line_items: vec![],
    };
    let err = export_bom(&[bom], &opts(BomFormat::Csv, true)).unwrap_err();
    assert_eq!(err, BomError::EmptyBom);
}

#[test]
fn zero_quantity_is_invalid_bom_error() {
    let bom = DeviceClassBom {
        device_class: summary("leaf"),
        entry_id: "leaves".to_string(),
        plan_quantity: 2,
        line_items: vec![BomLineItem {
            path: vec!["xcvr".to_string()],
            device_class: summary("xcvr"),
            quantity_per_parent: 0,
            quantity_per_unit: 0,
            fleet_quantity: 0,
        }],
    };
    let err = export_bom(&[bom], &opts(BomFormat::Csv, true)).unwrap_err();
    assert!(
        matches!(err, BomError::InvalidBom(_)),
        "expected invalid-bom, got {err:?}"
    );
}

#[test]
fn broken_fleet_invariant_is_invalid_bom_error() {
    // fleet_quantity != quantity_per_unit * plan_quantity (2 * 1 = 2, not 99).
    let bom = DeviceClassBom {
        device_class: summary("leaf"),
        entry_id: "leaves".to_string(),
        plan_quantity: 2,
        line_items: vec![BomLineItem {
            path: vec![],
            device_class: summary("leaf"),
            quantity_per_parent: 1,
            quantity_per_unit: 1,
            fleet_quantity: 99,
        }],
    };
    let err = export_bom(&[bom], &opts(BomFormat::Csv, true)).unwrap_err();
    assert!(
        matches!(err, BomError::InvalidBom(_)),
        "expected invalid-bom, got {err:?}"
    );
}
