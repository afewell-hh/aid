//! RED acceptance tests — the rendered CSV and JSON must reproduce every
//! fixture's `bom_totals` baseline exactly (per-unit AND fleet), at every tree
//! level. The oracle is the kernel-derived `bom_totals` in each fixture's
//! `expected.json`. All FAIL now: `export_bom` is a stub returning `internal`.

mod common;

use bom_adapter::{export_bom, BomFormat, BomOptions};
use common::{aggregate_csv, aggregate_json, expected_agg, load_boms, FIXTURES};

fn opts(format: BomFormat) -> BomOptions {
    BomOptions {
        format,
        include_fleet_totals: true,
    }
}

#[test]
fn csv_reproduces_bom_totals_for_every_fixture() {
    for &fixture in FIXTURES {
        let boms = load_boms(fixture);
        let out = export_bom(&boms, &opts(BomFormat::Csv))
            .unwrap_or_else(|e| panic!("{fixture}: export_bom(csv) failed: {e}"));
        let got = aggregate_csv(&out.content);
        let want = expected_agg(fixture);
        assert_eq!(
            got.per_unit, want.per_unit,
            "{fixture}: CSV per-unit totals mismatch"
        );
        assert_eq!(
            got.fleet, want.fleet,
            "{fixture}: CSV fleet totals mismatch"
        );
    }
}

#[test]
fn json_reproduces_bom_totals_for_every_fixture() {
    for &fixture in FIXTURES {
        let boms = load_boms(fixture);
        let out = export_bom(&boms, &opts(BomFormat::Json))
            .unwrap_or_else(|e| panic!("{fixture}: export_bom(json) failed: {e}"));
        let got = aggregate_json(&out.content);
        let want = expected_agg(fixture);
        assert_eq!(
            got.per_unit, want.per_unit,
            "{fixture}: JSON per-unit totals mismatch"
        );
        assert_eq!(
            got.fleet, want.fleet,
            "{fixture}: JSON fleet totals mismatch"
        );
    }
}

/// `switch-bom` is the non-server case: a switch whose root SKU IS a line item
/// (`leaf-switch-800g`, per-unit 1) plus a transceiver sub-component
/// (`osfp-800g-xcvr`, per-unit 64). Proves non-server BOMs render correctly.
#[test]
fn switch_bom_renders_non_server_root_line() {
    let boms = load_boms("switch-bom");
    let out = export_bom(&boms, &opts(BomFormat::Csv)).expect("export switch-bom");
    let got = aggregate_csv(&out.content);
    let leaf = &got.per_unit["leaf-switches"];
    assert_eq!(leaf["leaf-switch-800g"], 1, "root switch SKU present per-unit");
    assert_eq!(leaf["osfp-800g-xcvr"], 64, "transceiver sub-component per-unit");
    let fleet = &got.fleet["leaf-switches"];
    assert_eq!(fleet["leaf-switch-800g"], 2);
    assert_eq!(fleet["osfp-800g-xcvr"], 128);
}
