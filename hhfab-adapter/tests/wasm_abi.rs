//! wasm ABI smoke test — proves the D16 JSON-over-linear-memory boundary works,
//! not just the native transform. Builds the `wasm32-unknown-unknown` artifact
//! (on demand), loads it in a pure-Rust wasm interpreter, and drives the exact
//! export contract: `alloc(len) -> ptr`, write JSON, `export_wiring(ptr,len) ->
//! (out_ptr<<32)|out_len`, read JSON back.

use std::fs;
use std::path::PathBuf;
use std::process::Command;

use wasmi::{Engine, Linker, Module, Store};

fn manifest(rel: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(rel)
}

/// Build the wasm artifact if it isn't present, then load its bytes.
fn load_wasm() -> Vec<u8> {
    let path = manifest("target/wasm32-unknown-unknown/release/hhfab_adapter.wasm");
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
fn export_wiring_over_wasm_abi_validates_clos_small() {
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
    let export_wiring = instance
        .get_typed_func::<(u32, u32), u64>(&store, "export_wiring")
        .expect("export `export_wiring`");

    // Input = { "ir": <vendored clos-small IR>, "options": {} }, the exact wire
    // payload a Phase-6 host would hand the component.
    let ir_json = fs::read_to_string(manifest("tests/testdata/clos-small.ir.json"))
        .expect("read clos-small IR");
    let input = format!("{{\"ir\":{ir_json},\"options\":{{}}}}");
    let bytes = input.as_bytes();
    let len = bytes.len() as u32;

    let in_ptr = alloc.call(&mut store, len).expect("alloc");
    memory
        .write(&mut store, in_ptr as usize, bytes)
        .expect("write input to linear memory");

    let packed = export_wiring
        .call(&mut store, (in_ptr, len))
        .expect("call export_wiring");
    let out_ptr = (packed >> 32) as usize;
    let out_len = (packed & 0xffff_ffff) as usize;
    assert!(out_len > 0, "non-empty result");

    let mut buf = vec![0u8; out_len];
    memory
        .read(&store, out_ptr, &mut buf)
        .expect("read result from linear memory");
    let result = String::from_utf8(buf).expect("utf-8 result");

    // Result must be the `ok` variant carrying a document with real CRDs.
    let value: serde_json::Value = serde_json::from_str(&result).expect("parse result JSON");
    let docs = value
        .get("ok")
        .and_then(|ok| ok.get("documents"))
        .and_then(|d| d.as_array())
        .expect("ok.documents array (got error result)");
    assert_eq!(docs.len(), 1, "one combined document");
    let yaml = docs[0]["yaml"].as_str().expect("document yaml");
    assert!(yaml.contains("kind: Connection"), "has Connection CRDs");
    assert!(yaml.contains("server-leafs-0"), "has clos-small switch");
    assert!(!yaml.contains("ecmp"), "no empty ecmp over the ABI either");
}
