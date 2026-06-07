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
| Why3 | Proof platform driving `moon prove` (requires an SMT solver) | `why3 --version` | 7 (spike), 8 (proofs) |
| Z3 (SMT solver) | Discharges `moon prove` goals via Why3 | `z3 --version` | 7 (spike), 8 (proofs) |
| wasm-tools | Validate/compose WASM components and WIT | `wasm-tools --version` | 1, 3, 5 |
| wit-bindgen | Generate language bindings from WIT interfaces | `wit-bindgen --version` | 1, 3, 5, 9 |
| hhfab | Behavioral contract: `hhfab validate` on generated wiring YAML | `hhfab versions` | 3, 5, 6 |

> The local `hhfab` build may not support `hhfab version`; use `hhfab versions` instead.

> **`moon prove` requires Why3 + an SMT solver.** `moon prove` shells out to `why3`
> (version 1.7.2), which in turn drives an SMT solver (Z3). Neither ships with the
> MoonBit toolchain — both must be installed separately (see install steps below).
> Without `why3` on `PATH`, `moon prove` fails with
> `failed to locate 'why3' required by 'moon prove'`. **Do not gate CI on `moon prove`'s
> exit code:** it exits `0` even when goals fail. Parse stdout for the
> `N of M packages proved` / `Failed goals:` summary instead. If `moon prove` produces
> no output and proves nothing, confirm the package opts in with `"proof-enabled": true`
> in its `moon.pkg.json`. (All three caveats were established by the issue #5 spike;
> see `spikes/moonbit-port-proof/README.md`.)

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
| Why3 | ✅ present | `Why3 platform, version 1.7.2` |
| Z3 (SMT solver) | ✅ present | `Z3 version 4.8.12 - 64 bit` |
| wasm-tools | ✅ present | `wasm-tools 1.251.0` |
| wit-bindgen | ✅ present | `wit-bindgen-cli 0.57.1` |
| hhfab | ✅ present | `v0.43.1` (fabric API `v0.96.2`) |

> Why3 + Z3 were provisioned during the issue #5 / Phase 7 spike (Backlog Step 04);
> they are user-local installs (opam switch + apt), no system-wide MoonBit changes.

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

$ why3 --version
Why3 platform, version 1.7.2

$ z3 --version
Z3 version 4.8.12 - 64 bit

# End-to-end proof gate from a fresh login shell (no manual `eval $(opam env)`):
$ bash -lc 'cd spikes/moonbit-port-proof && why3 --version && z3 --version && moon prove'
Why3 platform, version 1.7.2
Z3 version 4.8.12 - 64 bit
aid/spike/port-proof/src
  Succeeded: 2 goals proved
Summary:
  1 of 1 packages proved
  2 goals proved
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

### Proof toolchain installation (Issue #5 / Phase 7 spike)

`moon prove` needs Why3 1.7.2 + an SMT solver. Installed user-local on Ubuntu 22.04
(opam switch for Why3, apt for Z3 — no MoonBit-side changes):

```bash
sudo apt-get install -y z3 opam libgmp-dev pkg-config m4   # Z3 4.8.12 + opam build deps
opam init --disable-sandboxing -y --bare
opam switch create why3env ocaml-base-compiler.4.14.2      # dedicated OCaml switch
eval $(opam env --switch=why3env)
opam install why3.1.7.2 -y
why3 config detect                                         # detects Z3, writes ~/.why3.conf
```

`moon prove` finds `why3` on `PATH`, generates its own `_build/verif/why3.conf` pointing
at the detected Z3, and runs the solver. Reproducibility note: pin Why3 `1.7.2` and Z3
`4.8.12` when reproducing; a future CI/workstation-setup ticket should fold these into a
provisioning script alongside the Issue #15 tools (tracked for **before Phase 8**).

PATH persistence for non-interactive shells (so future agents/CI resolve the tools):
- `~/.cargo/env` is already sourced from both `~/.bashrc` and `~/.profile` (rustup default),
  so `~/.cargo/bin` is on PATH for login and non-login shells.
- The MoonBit installer adds `~/.moon/bin` to `~/.bashrc` only, which Ubuntu's `~/.bashrc`
  skips for non-interactive shells. Issue #15 therefore also appends
  `export PATH="$HOME/.moon/bin:$PATH"` to `~/.profile` so login shells resolve `moon`.
- Why3 lives in the `why3env` opam switch (`~/.opam/why3env/bin`), which is not on `PATH`
  by default for new shells. Issue #5 therefore also appends
  `export PATH="$HOME/.opam/why3env/bin:$PATH"` to `~/.profile` so login/non-interactive
  shells resolve `why3` (and thus `moon prove`) **without** manually running
  `eval $(opam env --switch=why3env)`. Z3 is at `/usr/bin/z3` (apt), already on `PATH`.
  Verified: `bash -lc 'cd spikes/moonbit-port-proof && why3 --version && z3 --version && moon prove'`
  succeeds from a fresh login shell.

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

- **Why3 (`1.7.2`) + Z3 (`4.8.12`)** — the SMT proof backend `moon prove` shells out to.
  Required by the pulled-forward feasibility spike (Backlog Step 04 / Phase 7) and the full
  formal-verification work (Phase 8). Must be provisioned (and added to CI) before Phase 8
  proofs can be enforced as a build gate.

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
