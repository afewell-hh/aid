//! RED golden snapshot regression. Locks the exact rendered CSV and JSON for
//! each acceptance fixture so unintended drift is caught. The quantities are
//! gated against `bom_totals` in `tests/acceptance.rs`; these snapshots guard
//! against silent regressions in the emitted bytes (column order, tree layout,
//! formatting). All FAIL now: `export_bom` is a stub, and the goldens are
//! captured during GREEN.
//!
//! Regenerate after an intentional change:
//!   UPDATE_GOLDEN=1 cargo test --test golden

mod common;

use std::fs;
use std::path::PathBuf;

use bom_adapter::{export_bom, BomFormat, BomOptions};
use common::{load_boms, FIXTURES};

fn manifest(rel: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(rel)
}

fn render(fixture: &str, format: BomFormat) -> String {
    let boms = load_boms(fixture);
    let out = export_bom(
        &boms,
        &BomOptions {
            format,
            include_fleet_totals: true,
        },
    )
    .unwrap_or_else(|e| panic!("{fixture}: export_bom failed: {e}"));
    out.content
}

fn check(fixture: &str, ext: &str, format: BomFormat) {
    let got = render(fixture, format);
    let path = manifest(&format!("tests/golden/{fixture}.bom.{ext}"));
    if std::env::var("UPDATE_GOLDEN").is_ok() {
        fs::create_dir_all(path.parent().unwrap()).unwrap();
        fs::write(&path, &got).unwrap();
        return;
    }
    let want = fs::read_to_string(&path).unwrap_or_else(|_| {
        panic!(
            "missing golden {} (run `UPDATE_GOLDEN=1 cargo test --test golden`)",
            path.display()
        )
    });
    assert_eq!(got, want, "golden mismatch for {fixture}.{ext}");
}

#[test]
fn golden_csv_snapshots() {
    for &fixture in FIXTURES {
        check(fixture, "csv", BomFormat::Csv);
    }
}

#[test]
fn golden_json_snapshots() {
    for &fixture in FIXTURES {
        check(fixture, "json", BomFormat::Json);
    }
}
