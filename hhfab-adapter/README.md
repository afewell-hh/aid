# hhfab-adapter (AID Layer 2, Phase 5)

Rust WASM component that transforms a calculated `topology-ir` into **hhfab
wiring YAML** (Kubernetes CRDs, `wiring.githedgehog.com`). Implements
`wit/hhfab-adapter.wit`.

- **Pure transformation.** Consumes `topology-ir` **only** — never plan YAML,
  never NetBox, no other I/O.
- **Oracle.** Output is correct iff `hhfab validate` accepts it
  (hhfab v0.43.1 / fabric API v0.96.2). Golden snapshots lock regression after
  green.
- **Data ABI.** Core-wasm module realizing the WIT interface as JSON over linear
  memory (D16, extended to Layer 2): `alloc`/`dealloc` +
  `export_wiring(ptr,len) -> (out_ptr<<32)|len`, JSON `topology-ir` in, JSON
  `result<hhfab-output, hhfab-error>` out. See [`IR_CONTRACT.md`](IR_CONTRACT.md).

## Layout

```
hhfab-adapter/
  src/
    lib.rs      # WIT-facing API, JSON entry point, wasm ABI shell
    ir.rs       # topology-ir deserialization (IR_CONTRACT.md)
    crds.rs     # hhfab CRD serialization structs + WIT option/output/error types
    emit.rs     # IR -> CRD transformation (the core mapping)
  tests/
    unit.rs     # edge -> Connection mapping per variant; no-empty-ecmp rule
    validate.rs # per-fixture: export_wiring -> `hhfab validate` (acceptance)
    wasm_abi.rs # JSON-over-linear-memory ABI smoke test (proves the boundary)
    golden.rs   # snapshot regression for the emitted wiring YAML
    golden/     # committed wiring YAML snapshots (UPDATE_GOLDEN=1 to refresh)
    testdata/   # vendored topology-ir JSON (regenerate via tools/gen-ir.sh)
  tools/
    ir-gen/     # MoonBit generator: runs aid/kernel calculate() (additive)
    gen-ir.sh   # regenerate tests/testdata/*.ir.json
```

## Build & test

```sh
cargo build --release --target wasm32-unknown-unknown   # component artifact
cargo test                       # unit + acceptance (hhfab validate) + golden + wasm ABI smoke
UPDATE_GOLDEN=1 cargo test --test golden   # refresh golden snapshots after an intended change
hhfab-adapter/tools/gen-ir.sh    # regenerate vendored IR test data
```

`tests/wasm_abi.rs` builds the wasm artifact on demand if it isn't present, so a
bare `cargo test` exercises the JSON-over-memory boundary too.

## CRDs emitted

`VLANNamespace`, `IPv4Namespace` (`vpc.githedgehog.com/v1beta1`), `SwitchGroup`,
`Switch`, `Server`, `Connection` (variants: unbundled / bundled / mclag / eslag /
fabric / mesh).

### Synthesized defaults (overridable)

These are **fabric-deployment defaults**, not derived from the IR — present only
so `hhfab validate` sees a complete fabric:

- `VLANNamespace default` ranges `1000–2999`
- `IPv4Namespace default` subnets `10.0.0.0/16`
- `SwitchGroup empty` (`spec: {}`)
- `Switch.spec.profile: vs` — the validate-accepted virtual-switch profile;
  mapping `device-class-id` → a real hhfab SwitchProfile is a documented
  follow-up, out of scope for Phase 5.

Empty `ecmp: {}` / `redundancy: {}` are **never** emitted (a known `hhfab
validate` failure; omitting both is confirmed to validate).

## Acceptance set

`clos-small` and `mesh-two-switch` must pass `hhfab validate`. `switch-bom` has
no connections → switches-only output, **excluded** from the acceptance set.
