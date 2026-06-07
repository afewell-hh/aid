# AID Development Environment

This document records the toolchain required to build, test, and verify AID across its
implementation phases, and the **current local readiness** of each tool. It exists so a
phase agent can tell, before starting, whether the tools that phase depends on are present.

AID is implemented in **MoonBit, Rust, and Go** using the WASM Component Model. There is
no Python (see `DECISIONS.md` D1) — do not add Python tooling to this project.

For the per-phase dependency map, see `ROADMAP.md`. For why each language/tool was chosen,
see `TECH_STACK.md`.

---

## Required Toolchain

| Tool | Used for | Verify with | Phases that need it |
|------|----------|-------------|---------------------|
| Go | CLI, `aid serve` API, WASM hosting (`wasmtime-go`), SQLite state | `go version` | 6, 6b, 9 |
| Rust / Cargo | hhfab + NetBox adapters, plan-schema validation | `cargo --version` | 5, 9 |
| cargo-component | Build Rust crates as WASM components | `cargo component --version` | 5, 9 (if Rust) |
| MoonBit (`moon`) | Topology kernel, BOM adapter, JS frontend, `moon prove` proofs | `moon --version` | 1 (bindgen), 3, 4, 6b, 7, 8 |
| wasm-tools | Validate/compose WASM components and WIT | `wasm-tools --version` | 1, 3, 5 |
| wit-bindgen | Generate language bindings from WIT interfaces | `wit-bindgen --version` | 1, 3, 5, 9 |
| hhfab | Behavioral contract: `hhfab validate` on generated wiring YAML | `hhfab versions` | 3, 5, 6 |

> The local `hhfab` build may not support `hhfab version`; use `hhfab versions` instead.

---

## Current Local Readiness

Captured on this machine for Issue #2 (foundation readiness). These reflect one developer
workstation, not a CI baseline — re-run the verify commands in your own environment.

| Tool | Status | Reported version |
|------|--------|------------------|
| Go | ✅ present | `go1.21.5 linux/amd64` |
| Rust / Cargo | ✅ present | `cargo 1.90.0` |
| cargo-component | ❌ missing | `error: no such command: component` |
| MoonBit (`moon`) | ❌ missing | `moon: command not found` |
| wasm-tools | ❌ missing | `wasm-tools: command not found` |
| wit-bindgen | ❌ missing | `wit-bindgen: command not found` |
| hhfab | ✅ present | `v0.43.1` (fabric API `v0.96.2`) |

### Exact commands and results

```text
$ go version
go version go1.21.5 linux/amd64

$ cargo --version
cargo 1.90.0 (840b83a10 2025-07-30)

$ cargo component --version
error: no such command: `component`

$ moon --version
moon: command not found

$ wasm-tools --version
wasm-tools: command not found

$ wit-bindgen --version
wit-bindgen: command not found

$ hhfab versions
INF Hedgehog Fabricator version=v0.43.1
INF No configuration found file=fab.yaml action="Showing release versions"
# fabric API/agent/ctl: v0.96.2 (full output elided)
```

---

## Toolchain Gaps and the Phases They Block

Per project policy, missing tools are **documented, not installed** — installation requires
explicit project-lead approval. Each gap below is mapped to the earliest phase it blocks.

- **wasm-tools — blocks Phase 1 (WIT Interface Design).**
  Phase 1's exit gate requires `wasm-tools validate` on every WIT file. WIT files can be
  *authored* without it, but the Phase 1 verification step cannot complete until it is
  installed. Also needed to compose/validate components in Phases 3 and 5.

- **wit-bindgen — blocks Phase 1 (WIT Interface Design).**
  Phase 1's exit gate requires `wit-bindgen moonbit` to generate valid scaffolding from the
  WIT files. Required again for bindings in Phases 3, 5, and 9.

- **MoonBit (`moon`) — blocks Phase 3 (kernel) first, and Phases 4, 6b, 7, 8.**
  Also participates in the Phase 1 exit gate (MoonBit bindgen scaffolding). The kernel,
  BOM adapter (if MoonBit), JS frontend, and all `moon prove` formal-verification work
  cannot proceed without it.

- **cargo-component — blocks Phase 5 (hhfab adapter), and Phase 9 if NetBox adapter is Rust.**
  Rust source can be written and unit-tested with the present Cargo, but building the
  adapters as WASM components requires `cargo-component`.

### Phase 1 readiness summary

Phase 1 (the immediate next phase) is **partially unblocked**: WIT interface design and
authoring can begin now using the already-settled `DOMAIN_MODEL.md` types. However, the
**Phase 1 exit gate cannot be fully satisfied** until `wasm-tools`, `wit-bindgen`, and
`moon` are installed, because the gate requires WIT validation and MoonBit bindgen
scaffolding. Recommend installing those three before Phase 1 review, or splitting Phase 1
into an authoring sub-step (unblocked) and a validation sub-step (blocked on tooling).

Phases that are tooling-ready today: anything depending only on **Go** (6/6b orchestration
scaffolding) and the **hhfab** behavioral-contract check are runnable now.
