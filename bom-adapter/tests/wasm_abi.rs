//! RED wasm ABI smoke test — proves the D16 JSON-over-linear-memory boundary,
//! not just the native transform. Builds the `wasm32-unknown-unknown` artifact
//! (on demand), loads it in a pure-Rust wasm interpreter, and drives the exact
//! export contract: `alloc(len) -> ptr`, write JSON, `export_bom(ptr,len) ->
//! (out_ptr<<32)|out_len`, read JSON back.
//!
//! FAILS now: the stub returns an `err` result, so the `ok` assertion fails
//! (the wasm artifact itself builds — the ABI shell is real; GREEN fills in the
//! rendering behind it).

use std::fs;
use std::path::PathBuf;
use std::process::Command;

use wasmi::{Engine, Linker, Module, Store};

fn manifest(rel: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(rel)
}

/// Build the wasm artifact if it isn't present, then load its bytes.
fn load_wasm() -> Vec<u8> {
    let path = manifest("target/wasm32-unknown-unknown/release/bom_adapter.wasm");
    if !path.exists() {
        let status = Command::new(env!("CARGO"))
            .args(["build", "--release", "--target", "wasm32-unknown-unknown"])
            .current_dir(manifest("."))
            .status()
            .expect("run cargo build for wasm artifact");
        assert!(status.success(), "failed to build wasm32 artifact");
    }
    fs::read(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()))
}

#[test]
fn export_bom_over_wasm_abi_renders_switch_bom() {
    let wasm = load_wasm();
    let engine = Engine::default();
    let module = Module::new(&engine, &wasm[..]).expect("compile module");
    let mut store = Store::new(&engine, ());
    let linker = <Linker<()>>::new(&engine);
    let instance = linker
        .instantiate(&mut store, &module)
        .expect("instantiate")
        .start(&mut store)
        .expect("start");

    let memory = instance
        .get_memory(&store, "memory")
        .expect("export `memory`");
    let alloc = instance
        .get_typed_func::<u32, u32>(&store, "alloc")
        .expect("export `alloc`");
    let export_bom = instance
        .get_typed_func::<(u32, u32), u64>(&store, "export_bom")
        .expect("export `export_bom`");

    // Input = { "boms": <vendored switch-bom BOMs>, "options": {...} }, the exact
    // wire payload a Phase-6 host would hand the component.
    let boms_json = fs::read_to_string(manifest("tests/testdata/switch-bom.boms.json"))
        .expect("read switch-bom BOMs");
    let input = format!(
        "{{\"boms\":{boms_json},\"options\":{{\"format\":\"json\",\"include_fleet_totals\":true}}}}"
    );
    let bytes = input.as_bytes();
    let len = bytes.len() as u32;

    let in_ptr = alloc.call(&mut store, len).expect("alloc");
    memory
        .write(&mut store, in_ptr as usize, bytes)
        .expect("write input to linear memory");

    let packed = export_bom
        .call(&mut store, (in_ptr, len))
        .expect("call export_bom");
    let out_ptr = (packed >> 32) as usize;
    let out_len = (packed & 0xffff_ffff) as usize;
    assert!(out_len > 0, "non-empty result");

    let mut buf = vec![0u8; out_len];
    memory
        .read(&store, out_ptr, &mut buf)
        .expect("read result from linear memory");
    let result = String::from_utf8(buf).expect("utf-8 result");

    // Result must be the `ok` variant carrying a JSON BOM document.
    let value: serde_json::Value = serde_json::from_str(&result).expect("parse result JSON");
    let ok = value.get("ok").expect("ok variant (got error result)");
    assert_eq!(ok["format"].as_str(), Some("json"), "format echoed");
    let content = ok["content"].as_str().expect("ok.content string");
    let doc: serde_json::Value = serde_json::from_str(content).expect("parse content JSON");
    let boms = doc["boms"].as_array().expect("boms array");
    assert_eq!(boms.len(), 1, "one switch BOM");
    assert_eq!(boms[0]["entry_id"].as_str(), Some("leaf-switches"));
}
