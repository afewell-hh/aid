# AID Technology Stack

## Language Responsibilities

### MoonBit — Topology Calculation Kernel

**Used for:** `topology-calculator` WASM component, `bom-adapter` WASM component

**Why MoonBit:**
- First-class formal verification via `moon prove` (Z3 SMT backend, built into standard toolchain)
- WASM-native: MoonBit's primary compile target is WebAssembly
- Native WASM Component Model support (WIT bindgen tooling provided)
- "Agentic-first" design: strong typing and explicit contracts help LLMs generate correct proofs
- Smallest WASM binary output of any general-purpose language

**What it formally verifies in AID:**
- Port allocation non-overlap (no logical port allocated twice)
- Allocation completeness (total allocated == total demanded)
- Switch count lower bound (effective_quantity >= ceil(demand/capacity))
- BOM scaling (fleet_count == per_server_count × quantity)
- Mesh constraint (switch count ∈ {2, 3})
- MCLAG even-count

**Key tools:**
- `moon` — build, test, prove
- `wit-bindgen moonbit` — generate WIT bindings
- `wasm-tools` — compose and validate WASM components
- `moon prove` — run Z3 proofs against `.mbtp` predicate files

**Risks to manage:**
- MoonBit 1.0 is targeted for H1 2026 (stdlib still stabilizing)
- Formal verification scope: currently limited to pure functions (recursive/concurrent TBD)
- Small community and ecosystem
- See Phase 7 spike gate in `ROADMAP.md` before committing production code

---

### Rust — Adapters and WASM Components

**Used for:** `hhfab-adapter` WASM component, `netbox-adapter`, plan schema validation

**Why Rust:**
- `serde_yaml`: production-proven YAML serialization — no equivalent in MoonBit ecosystem
- `cargo-component`: first-class WASM Component Model tooling (Rust is the reference implementation language)
- `reqwest`: async HTTP client for NetBox REST API
- Memory safety model is philosophically aligned with MoonBit (no GC, ownership)
- `wasmtime` — the reference WASM runtime — is written in Rust, so Rust is the most natural host
- `clap` if CLI portions are written in Rust

**What Rust handles in AID:**
- Serializing `TopologyIR` to hhfab wiring YAML (Kubernetes CRD format)
- POST/PUT calls to NetBox REST API (Devices, Interfaces, Cables, Modules)
- JSON Schema validation of topology plan YAML files
- Any component where MoonBit's library ecosystem is insufficient

---

### Go — CLI, Orchestration, and Storage

**Used for:** `aid` CLI, plan YAML read/write, SQLite state, WASM component hosting

**Why Go:**
- `cobra` + `viper`: the strongest CLI tooling ecosystem for this type of tool
- Single static binary output: trivial to distribute (`go build` → one binary)
- `wasmtime-go`: official Go bindings for the Wasmtime WASM runtime
- `mattn/go-sqlite3`: SQLite for local generated-state storage
- Fast compile-test loop for orchestration and I/O code
- Net/http is sufficient for NetBox REST calls if a full Rust adapter is overkill

**What Go handles in AID:**
- `aid plan create/validate/diff` — YAML plan I/O
- `aid topology calc/bom/export` — orchestrate WASM components and route output
- `aid publish netbox` — POST topology to NetBox via REST
- Local state database (last IR hash, generation timestamps)
- Configuration management (`~/.aid/config.yaml`)

---

## WASM Component Model

AID uses the WASM Component Model (WIT interfaces, Canonical ABI) for all inter-component
boundaries. This is not an internal implementation detail — it is the primary architectural
boundary mechanism.

**Why WASM Component Model:**
- Language-independent: each component's implementation can change without affecting others
- Formally specified interfaces: WIT is machine-readable and tool-generated
- Enables third-party extension: anyone can implement a new export adapter against the published WIT
- Browser portability: WASM components can run in a browser for a future web UI

**WIT interface files (in `wit/`):**
```
wit/
  world.wit                 # top-level world definition
  topology-calculator.wit   # inputs/outputs for the calculation kernel
  hhfab-adapter.wit         # TopologyIR → wiring YAML
  bom-adapter.wit           # ServerClassBOM[] → CSV/JSON
  netbox-adapter.wit        # TopologyIR + config → NetBox publish result
```

**Component composition:**
The Go CLI uses `wasmtime-go` to host all WASM components. Components do not call each
other directly — the CLI orchestrates all inter-component data flow.

---

## Testing Strategy

| Layer | Language | Framework | Approach |
|-------|----------|-----------|----------|
| Topology kernel | MoonBit | `moon test` + `moon prove` | Unit tests + formal proofs |
| Adapters | Rust | `cargo test` | Unit tests, golden file tests |
| CLI + integration | Go | `go test` | Integration tests using fixture YAML files |
| Behavioral contract | Go | `go test` | Acceptance tests: fixture YAML → expected IR counts |

**Fixture YAML as behavioral contract:**
The topology plan YAML files from the reference architecture catalog
(see `HNP_REFERENCE.md` for source) serve as acceptance test inputs. For each fixture,
AID must produce correct:
- device count, interface count, cable count
- per-fabric switch counts
- BOM totals per server class
- valid wiring YAML (hhfab validate)

These counts are the ground truth derived from the HNP reference implementation and
captured in `tests/fixtures/` alongside the plan YAML.

---

## Development Environment

```bash
# MoonBit
moon --version       # check installation
moon build           # build all targets
moon test            # run tests
moon prove           # run formal proofs

# Rust
cargo build          # build all crates
cargo test           # run tests
cargo component build  # build WASM components

# Go
go build ./...       # build all packages
go test ./...        # run all tests

# Compose final WASM components
wasm-tools compose aid-topology-calculator.wasm hhfab-adapter.wasm -o aid.wasm
```

No containerized runtime is required for AID development — all components run natively
or are hosted by the Go CLI.
