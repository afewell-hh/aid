// Kernel WASM JSON-over-linear-memory ABI shell (DECISIONS.md D16, amended by D28).
//
// UTF-8 JSON over linear memory is the live MVP convention the Go host drives:
//
//   alloc(len) -> ptr ; dealloc(ptr,len)
//   export_f2_calculate(ptr,len) -> packed (ptr<<32)|len  # calc-plan JSON -> calc-output JSON
//   export_f3_bom(ptr,len)       -> packed (ptr<<32)|len  # bom-scale plan JSON -> scaled-lines JSON
//
// LIVE host boundary: `export_f2_calculate` / `export_f3_bom` only — the entries
// `internal/components` names and `internal/wasmhost` invokes after the
// foundation rebuild (D18–D27; F7d, #64/#35).
//
// The live boundary CONTRACT is the F2/F3 JSON shapes (kernel/src/f2_types.mbt +
// f3_bom.mbt, mirrored in Go by internal/calc / internal/bom) PLUS the executable
// golden contract tests in internal/wasmhost/golden_boundary_test.go — NOT a WIT
// interface. The invented pre-rebuild WIT contract (`calculate`/`validate` +
// topology-ir) and its #38 drift guard were RETIRED in #85 (Option A; D28); the
// old `export_calculate`/`export_validate` shells were already removed in #84.
//
// Mechanics (verified against moon 0.1.20260529 in the Phase-6 spike): use ONLY
// whitelisted raw wasm instructions (i32.load8_u / i32.store8) plus the 8-byte
// Bytes object header (data pointer = obj + 8). The $moonbit.incref/decref
// inline calls used by older community libraries are NOT whitelisted on this
// toolchain; buffers are instead kept alive via a MoonBit global.

extern "wasm" fn load8(pos : Int) -> Int =
  #|(func (param i32) (result i32) local.get 0 i32.load8_u)

extern "wasm" fn store8(pos : Int, value : Int) =
  #|(func (param i32) (param i32) local.get 0 local.get 1 i32.store8)

// Data pointer of a Bytes object = object pointer + 8-byte header.
#borrow(b)
extern "wasm" fn data_ptr(b : Bytes) -> Int =
  #|(func (param i32) (result i32) local.get 0 i32.const 8 i32.add)

// Keep host-visible buffers alive so their pointers stay valid across the call.
let live : Array[Bytes] = []

/// Allocate `size` bytes in linear memory; return the data pointer.
pub fn alloc(size : Int) -> Int {
  let b = Bytes::new(size)
  live.push(b)
  data_ptr(b)
}

/// Free a buffer previously returned by `alloc`.
pub fn dealloc(ptr : Int, _len : Int) -> Unit {
  for i in 0..<live.length() {
    if data_ptr(live[i]) == ptr {
      live.remove(i) |> ignore
      break
    }
  }
}

fn read_input(ptr : Int, len : Int) -> Bytes {
  let arr : Array[Byte] = []
  for i in 0..<len {
    arr.push(load8(ptr + i).to_byte())
  }
  Bytes::from_array(arr)
}

fn write_output(out : Bytes) -> Int64 {
  let len = out.length()
  // Keep the encoded source buffer alive across the destination allocation, the
  // same lifetime strategy `alloc` already applies to host-visible buffers. Under
  // the raw-instruction ABI the toolchain otherwise treats `out` as reclaimable
  // once `alloc(len)` needs comparable space, so a large output's tail bytes get
  // reused before the copy loop reads them — corrupting the JSON near its end
  // (Issue #59). Retaining `out` forces `alloc` to return a genuinely disjoint
  // buffer, so the byte copy stays exact for any size.
  live.push(out)
  let ptr = alloc(len)
  for i in 0..<len {
    store8(ptr + i, out[i].to_int())
  }
  (ptr.to_int64() << 32) | len.to_int64()
}

/// F2 calc-plan JSON -> F2 calc-output JSON (note §1.1; Issue #52). The F2
/// boundary uses the same JSON-over-linear-memory ABI; the kernel emits a bare
/// calc-output envelope (no ok/err wrapper) — a malformed plan yields an empty
/// but well-formed calc-output, so the Go host always decodes a valid result.
pub fn export_f2_calculate(ptr : Int, len : Int) -> Int64 {
  let plan_json = @utf8.decode_lossy(read_input(ptr, len)[:])
  let json = @src.f2_calculate(plan_json)
  write_output(@utf8.encode(json[:]))
}

/// F3 bom-scale plan JSON -> F3 scaled-lines JSON (note §1.1, §4; Issue #56). Same
/// JSON-over-linear-memory ABI; GREEN routes every qpu/fleet multiply through the
/// proven I4 cores (@proofs.child_qpu/fleet_quantity) via @src.f3_scale. F3 RED:
/// the kernel body (@src.f3_bom) is a stub returning an empty, well-formed output.
pub fn export_f3_bom(ptr : Int, len : Int) -> Int64 {
  let plan_json = @utf8.decode_lossy(read_input(ptr, len)[:])
  let json = @src.f3_bom(plan_json)
  write_output(@utf8.encode(json[:]))
}
