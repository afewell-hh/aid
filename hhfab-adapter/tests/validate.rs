//! RED acceptance tests — the oracle. Each test runs the adapter on vendored
//! topology-ir and pipes every emitted document through `hhfab validate`,
//! asserting success. All FAIL now because `export_wiring` is a stub; GREEN
//! implements the transform until `hhfab validate` accepts the YAML.
//!
//! Acceptance set: clos-small + mesh-two-switch. switch-bom is switches-only
//! (no connections) and is explicitly excluded from the validate acceptance set
//! (asserted separately).

use std::fs;
use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicU32, Ordering};

use hhfab_adapter::{export_wiring, HhfabOptions, TopologyIr};

fn load_ir(fixture: &str) -> TopologyIr {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("tests/testdata")
        .join(format!("{fixture}.ir.json"));
    let json = fs::read_to_string(&path)
        .unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
    TopologyIr::from_json(&json).unwrap_or_else(|e| panic!("parse {fixture} IR: {e}"))
}

static WORKDIR_SEQ: AtomicU32 = AtomicU32::new(0);

/// Run `hhfab validate` over a single wiring YAML document in an isolated,
/// freshly-initialized hhfab workdir. Returns (passed, combined-output).
fn hhfab_validate(wiring_yaml: &str) -> (bool, String) {
    let seq = WORKDIR_SEQ.fetch_add(1, Ordering::SeqCst);
    let dir = std::env::temp_dir().join(format!(
        "hhfab-adapter-{}-{seq}",
        std::process::id()
    ));
    let _ = fs::remove_dir_all(&dir);
    fs::create_dir_all(&dir).expect("create workdir");

    let init = Command::new("hhfab")
        .args(["init", "--dev"])
        .current_dir(&dir)
        .output()
        .expect("run `hhfab init` (is hhfab on PATH?)");
    assert!(
        init.status.success(),
        "hhfab init failed:\n{}",
        String::from_utf8_lossy(&init.stderr)
    );

    fs::create_dir_all(dir.join("include")).expect("create include/");
    fs::write(dir.join("include/wiring.yaml"), wiring_yaml).expect("write wiring");

    let out = Command::new("hhfab")
        .args(["validate", "--brief"])
        .current_dir(&dir)
        .output()
        .expect("run `hhfab validate`");
    let combined = format!(
        "{}\n{}",
        String::from_utf8_lossy(&out.stdout),
        String::from_utf8_lossy(&out.stderr)
    );
    let _ = fs::remove_dir_all(&dir);
    (out.status.success(), combined)
}

fn assert_documents_validate(ir: &TopologyIr, options: &HhfabOptions) -> usize {
    let output = export_wiring(ir, options).expect("export_wiring should succeed");
    assert!(
        !output.documents.is_empty(),
        "expected at least one wiring document"
    );
    for doc in &output.documents {
        let (ok, log) = hhfab_validate(&doc.yaml);
        assert!(ok, "hhfab validate failed for fabric `{}`:\n{log}", doc.fabric);
    }
    output.documents.len()
}

#[test]
fn clos_small_validates() {
    let ir = load_ir("clos-small");
    assert_documents_validate(&ir, &HhfabOptions::default());
}

#[test]
fn mesh_two_switch_validates() {
    let ir = load_ir("mesh-two-switch");
    assert_documents_validate(&ir, &HhfabOptions::default());
}

#[test]
fn fabric_scope_some_validates() {
    // `fabric = Some(name)` restricts output to one fabric.
    let ir = load_ir("clos-small");
    let options = HhfabOptions {
        fabric: Some("frontend".to_string()),
        split_by_fabric: false,
    };
    let n = assert_documents_validate(&ir, &options);
    assert_eq!(n, 1, "single-fabric scope -> one document");
}

#[test]
fn split_by_fabric_validates() {
    // `split_by_fabric = true` emits one self-contained document per managed
    // fabric; each must validate independently. clos-small has one managed
    // fabric (`frontend`).
    let ir = load_ir("clos-small");
    let options = HhfabOptions {
        fabric: None,
        split_by_fabric: true,
    };
    let n = assert_documents_validate(&ir, &options);
    assert_eq!(n, 1, "clos-small has one managed fabric");
}

#[test]
fn switch_bom_is_switches_only_and_excluded_from_acceptance() {
    // switch-bom has no connections -> no Connection CRDs (switches-only).
    // Documented as out of the `hhfab validate` acceptance set.
    let ir = load_ir("switch-bom");
    let output = export_wiring(&ir, &HhfabOptions::default())
        .expect("export_wiring should succeed");
    let combined: String = output.documents.iter().map(|d| d.yaml.clone()).collect();
    assert!(
        !combined.contains("kind: Connection"),
        "switch-bom must yield no Connection CRDs (switches-only):\n{combined}"
    );
}
