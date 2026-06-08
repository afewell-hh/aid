//! Golden snapshot regression (post-green). Locks the exact generated wiring
//! YAML for the acceptance fixtures so unintended drift is caught. The YAML
//! itself is gated by `hhfab validate` in `tests/validate.rs`; these snapshots
//! guard against silent regressions in the emitted bytes.
//!
//! Regenerate after an intentional change:  `UPDATE_GOLDEN=1 cargo test --test golden`

use std::fs;
use std::path::PathBuf;

use hhfab_adapter::{export_wiring, HhfabOptions, TopologyIr};

fn manifest(rel: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(rel)
}

fn combined_yaml(fixture: &str) -> String {
    let json = fs::read_to_string(manifest(&format!("tests/testdata/{fixture}.ir.json")))
        .unwrap_or_else(|e| panic!("read {fixture} IR: {e}"));
    let ir = TopologyIr::from_json(&json).unwrap_or_else(|e| panic!("parse {fixture} IR: {e}"));
    let out = export_wiring(&ir, &HhfabOptions::default()).expect("export_wiring");
    assert_eq!(out.documents.len(), 1, "{fixture}: expected one combined document");
    out.documents[0].yaml.clone()
}

fn check(fixture: &str) {
    let got = combined_yaml(fixture);
    let path = manifest(&format!("tests/golden/{fixture}.wiring.yaml"));
    if std::env::var("UPDATE_GOLDEN").is_ok() {
        fs::create_dir_all(path.parent().unwrap()).unwrap();
        fs::write(&path, &got).unwrap();
        return;
    }
    let want = fs::read_to_string(&path).unwrap_or_else(|_| {
        panic!("missing golden {} (run `UPDATE_GOLDEN=1 cargo test --test golden`)", path.display())
    });
    assert_eq!(got, want, "golden mismatch for {fixture}");
}

#[test]
fn golden_clos_small() {
    check("clos-small");
}

#[test]
fn golden_mesh_two_switch() {
    check("mesh-two-switch");
}
