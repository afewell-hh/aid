//! AID Layer-2 BOM export adapter (Phase 4, issue #8).
//!
//! Pure transformation: `device-class-bom[]` -> a human-reviewable CSV document
//! and a structured JSON document. It consumes `device-class-bom[]` ONLY — it
//! never reads plan YAML, never queries NetBox, never performs other I/O. It
//! RENDERS the kernel-computed BOM faithfully; it does not recompute the BOM or
//! re-derive the role-based root-inclusion rule (Algorithm 6, applied in the
//! kernel).
//!
//! Data ABI (D16, extended to Layer 2): the component is a core-wasm module that
//! realizes the `bom-adapter` WIT interface as UTF-8 JSON over linear memory.
//! `wit/bom-adapter.wit` + `wit/types.wit` remain the contract of record; the
//! wasm exports below are a thin JSON shell over the native [`export_bom`].
//! Tests exercise the native typed API directly (fast, oracle on the rendered
//! strings); a wasm smoke test exercises the ABI shell.

pub mod bom;
pub mod render;

pub use bom::{BomLineItem, DeviceClassBom, DeviceClassSummary};
pub use render::{export_bom, BomError, BomFormat, BomOptions, BomOutput};

use serde::{Deserialize, Serialize};

/// JSON input envelope for the wasm boundary: the BOMs plus export options.
#[derive(Debug, Deserialize)]
struct WireInput {
    boms: Vec<DeviceClassBom>,
    #[serde(default)]
    options: BomOptions,
}

/// JSON output envelope: `result<bom-output, bom-error>`.
#[derive(Debug, Serialize)]
#[serde(rename_all = "snake_case")]
enum WireResult {
    Ok(BomOutput),
    Err(BomError),
}

/// Native JSON entry point: parse `{ "boms": ..., "options": ... }`, run the
/// transform, and serialize `result<bom-output, bom-error>` as JSON. This is the
/// function the wasm ABI shell wraps and the contract Phase-6 hosts call.
pub fn export_bom_json(input_json: &str) -> String {
    let result = match serde_json::from_str::<WireInput>(input_json) {
        Ok(input) => match export_bom(&input.boms, &input.options) {
            Ok(out) => WireResult::Ok(out),
            Err(e) => WireResult::Err(e),
        },
        Err(e) => WireResult::Err(BomError::InvalidBom(format!("malformed BOM JSON: {e}"))),
    };
    serde_json::to_string(&result).unwrap_or_else(|e| {
        format!("{{\"err\":{{\"kind\":\"internal\",\"message\":\"{e}\"}}}}")
    })
}

// ---------------------------------------------------------------------------
// JSON-over-linear-memory ABI (core wasm). D16 extended to Layer 2:
//   alloc(len) -> ptr ; dealloc(ptr, len) ; export_bom(ptr,len) -> packed
// where packed = (out_ptr << 32) | out_len, payloads are UTF-8 JSON.
// ---------------------------------------------------------------------------
#[cfg(target_arch = "wasm32")]
pub mod abi {
    use super::export_bom_json;

    /// Allocate `len` bytes in linear memory and return the pointer.
    #[no_mangle]
    pub extern "C" fn alloc(len: u32) -> *mut u8 {
        let mut buf = Vec::<u8>::with_capacity(len as usize);
        let ptr = buf.as_mut_ptr();
        std::mem::forget(buf);
        ptr
    }

    /// Free `len` bytes previously returned by [`alloc`].
    ///
    /// # Safety
    /// `ptr`/`len` must come from a prior [`alloc`] call.
    #[no_mangle]
    pub unsafe extern "C" fn dealloc(ptr: *mut u8, len: u32) {
        drop(Vec::from_raw_parts(ptr, 0, len as usize));
    }

    /// Read JSON BOMs+options at `(in_ptr, in_len)`, run the adapter, write the
    /// JSON result to freshly-allocated memory, and return `(out_ptr<<32)|len`.
    ///
    /// # Safety
    /// `(in_ptr, in_len)` must describe a valid UTF-8 JSON buffer in memory.
    #[no_mangle]
    pub unsafe extern "C" fn export_bom(in_ptr: *const u8, in_len: u32) -> u64 {
        let input = std::slice::from_raw_parts(in_ptr, in_len as usize);
        let input = std::str::from_utf8(input).unwrap_or("");
        let out = export_bom_json(input).into_bytes();
        let out_len = out.len() as u32;
        let out_ptr = alloc(out_len);
        std::ptr::copy_nonoverlapping(out.as_ptr(), out_ptr, out.len());
        ((out_ptr as u64) << 32) | (out_len as u64)
    }
}
