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

First captured for Issue #2 (foundation readiness); the four missing tools were then
provisioned under Issue #15 (Backlog Step 01a). These reflect one developer workstation,
not a CI baseline — re-run the verify commands in your own environment. All four newly
provisioned tools are **user-local** installs (no system-wide changes, no sudo).

| Tool | Status | Reported version |
|------|--------|------------------|
| Go | ✅ present | `go1.21.5 linux/amd64` |
| Rust / Cargo | ✅ present | `cargo 1.90.0` |
| cargo-component | ✅ present | `cargo-component-component 0.21.1` |
| MoonBit (`moon`) | ✅ present | `moon 0.1.20260529` (moonc `v0.9.3`) |
| wasm-tools | ✅ present | `wasm-tools 1.251.0` |
| wit-bindgen | ✅ present | `wit-bindgen-cli 0.57.1` |
| hhfab | ✅ present | `v0.43.1` (fabric API `v0.96.2`) |

### Exact commands and results

```text
$ moon --version
moon 0.1.20260529 (3e1c753 2026-05-29) ~/.moon/bin/moon

$ wasm-tools --version
wasm-tools 1.251.0

$ wit-bindgen --version
wit-bindgen-cli 0.57.1

$ cargo component --version
cargo-component-component 0.21.1

$ go version
go version go1.21.5 linux/amd64

$ cargo --version
cargo 1.90.0 (840b83a10 2025-07-30)

$ hhfab versions
INF Hedgehog Fabricator version=v0.43.1
INF No configuration found file=fab.yaml action="Showing release versions"
# fabric API/agent/ctl: v0.96.2 (full output elided)
```

Smoke checks (`moon help`, `wasm-tools --help`, `wit-bindgen --help`,
`cargo component --help`) all exit `0`. `wit-bindgen --help` lists the `moonbit`
subcommand and `wasm-tools --help` lists `validate` — the two commands the Phase 1
exit gate depends on.

### Installation method (Issue #15)

All four tools were installed with official upstream commands, user-local, no sudo:

```bash
# MoonBit toolchain → installs to ~/.moon/bin (requires git)
curl -fsSL https://cli.moonbitlang.com/install/unix.sh | bash

# Bytecode Alliance tools → installs to ~/.cargo/bin (needs C toolchain + OpenSSL)
cargo install --locked wasm-tools
cargo install wit-bindgen-cli
cargo install cargo-component --locked
```

PATH persistence for non-interactive shells (so future agents/CI resolve the tools):
- `~/.cargo/env` is already sourced from both `~/.bashrc` and `~/.profile` (rustup default),
  so `~/.cargo/bin` is on PATH for login and non-login shells.
- The MoonBit installer adds `~/.moon/bin` to `~/.bashrc` only, which Ubuntu's `~/.bashrc`
  skips for non-interactive shells. Issue #15 therefore also appends
  `export PATH="$HOME/.moon/bin:$PATH"` to `~/.profile` so login shells resolve `moon`.

Reproducibility note: `wit-bindgen-cli` is installed without `--locked` because that is the
official upstream command, and upstream states the CLI is not yet stable — pin the recorded
version (`0.57.1`) when reproducing. A future CI/workstation-setup ticket can convert these
steps into a provisioning script.

---

## Toolchain Coverage by Phase

As of Issue #15, the validation toolchain is fully provisioned locally. Each tool and the
work it enables:

- **wasm-tools** (`1.251.0`) — `wasm-tools validate` for the Phase 1 WIT exit gate; WASM
  component compose/validate in Phases 3 and 5.

- **wit-bindgen** (`0.57.1`) — `wit-bindgen moonbit` scaffolding for the Phase 1 exit gate;
  bindings for Phases 3, 5, and 9. Upstream marks the CLI unstable — pin the version.

- **MoonBit (`moon` `0.1.20260529`, moonc `v0.9.3`)** — the pulled-forward feasibility spike
  (Backlog Step 04 / roadmap Phase 7), the topology kernel (Phase 3), BOM adapter (Phase 4),
  JS frontend (Phase 6b), `moon prove` proofs (Phase 8), and the MoonBit half of the Phase 1
  exit gate.

- **cargo-component** (`0.21.1`) — building Rust crates as WASM components for Phase 5 (hhfab
  adapter) and Phase 9 (if the NetBox adapter is Rust).

> Tooling is necessary but not sufficient. Having these tools installed does **not** make any
> phase ready to start out of order — backlog dependency order (contracts → fixtures → kernel
> → adapters → CLI/API → frontend) still gates the work.

### Phase 1 readiness summary

Phase 1 (WIT interface design) is now **toolchain-unblocked**: WIT authoring can proceed,
and the Phase 1 exit gate (`wasm-tools validate` plus `wit-bindgen moonbit` scaffolding) can
be fully satisfied with the locally installed tools. No tooling gap remains for Phase 1.

Later backlog steps remain gated by their own dependencies, not by tooling: CLI/API,
frontend, and export work stay blocked until their contract, fixture, kernel, and adapter
prerequisites are approved.
