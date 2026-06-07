# Spike: MoonBit proof + Go runtime viability for the topology kernel

**AID issue:** [#5](https://github.com/afewell-hh/aid/issues/5) (Backlog Step 04, pulls `ROADMAP.md` Phase 7 forward)
**Parent epic:** #1
**Decision under test:** `DECISIONS.md` D2 (MoonBit for the topology calculation kernel), D8 (WASM Component Model boundaries)

## Recommendation: **PASS**

All four gates pass with large margins:

| Gate (`DECISIONS.md` D2 / `ROADMAP.md` Phase 7) | Target | Result | Status |
|---|---|---|---|
| `moon prove` verifies **port non-overlap** | < 30 s | **0.64 s** (2 goals proved) | ✅ |
| `moon test` passes | pass | 5/5 | ✅ |
| WIT bindings generate for `wit/` without repo pollution | success | exit 0, 23 files, all under `/tmp` | ✅ |
| Go calls the MoonBit-produced WASM via Wasmtime | callable | `wasmtime-go` v45 core-module call works | ✅ |
| cold-start | < 500 ms | **~2.0 ms** | ✅ |
| per-call | < 10 ms | **~7.6 µs avg** (≈0.008 ms) | ✅ |

**One material environment finding (resolved, needs doc follow-up):** `moon prove` requires **Why3 + an SMT solver**, which are **not** part of the toolchain recorded as "ready" in `DEVELOPMENT.md`. They were installed during this spike (Why3 1.7.2 via opam, Z3 4.8.12 via apt). The proof gate cannot be reproduced without them. See [Risks & follow-ups](#risks--follow-ups).

MoonBit is viable for the AID topology kernel. Proceed to Phase 3 (kernel) and Phase 8 (proofs), conditional on provisioning the proof toolchain in `DEVELOPMENT.md`.

---

## What was built

A deliberately tiny, pure model of `ALGORITHMS.md` **Algorithm 8 (Port Allocation — zone cursor)**. Not the kernel.

`src/alloc.mbt` exports two functions, both with first-class proof contracts:

- `allocate(capacity, requested) -> count` — the zone-cursor allocation count. Proves: `count <= capacity` (never exceeds zone capacity), `count <= requested` (never over-allocates), `requested <= capacity → count == requested` (completeness), `count >= 0` (well-formed).
- `non_overlap_holds(base, j, k) -> Bool` — the **mandatory port non-overlap invariant**. The j-th connection in a zone gets logical port `base + j`; this proves that any two distinct offsets `j != k` map to distinct ports (`base + j != base + k`), i.e. the cursor map is injective and no logical port is ever allocated twice. The post-condition `result == true` forces the prover to show it holds for *all* inputs (no counterexample).

Together: `allocate` keeps every allocated offset inside `[0, capacity)`, and `non_overlap_holds` proves the offset→port map over that range is injective — the per-zone non-overlap guarantee.

The same MoonBit package compiles to a 147-byte core-wasm module exporting `allocate` and `non_overlap_holds`, which the Go harness (`go/alloc_test.go`) drives through `wasmtime-go`.

---

## Official references used

- MoonBit 0.9 formal-verification release (proof annotations, `.mbtp`, Why3 backend): https://www.moonbitlang.com/blog/moonbit-0-9-release
- MoonBit verified-examples repo (canonical `proof_require`/`proof_ensure`/`proof_assert`/`predicate`/`lemma` syntax, `options("proof-enabled": true)`, Why3 + Z3/Alt-Ergo/CVC5 config): https://github.com/moonbit-community/verified
- MoonBit toolchain — package config (`link.wasm.exports`) and build targets: https://docs.moonbitlang.com/en/latest/toolchain/moon/package.html
- MoonBit Component Model tutorial (core-wasm → componentize with `wasm-tools`): https://docs.moonbitlang.com/en/stable/toolchain/wasm/component-model-tutorial.html
- Bytecode Alliance — building a MoonBit component: https://component-model.bytecodealliance.org/language-support/building-a-simple-component/moonbit.html
- `wasmtime-go`: https://github.com/bytecodealliance/wasmtime-go (module `github.com/bytecodealliance/wasmtime-go/v45`, used version **v45.0.0** — note: `v45.0.1` does not exist on the proxy)
- Why3: https://www.why3.org/ (1.7.2, the version `moon prove` requests)
- AID internal: `DECISIONS.md` (D2, D8), `TECH_STACK.md`, `ROADMAP.md` (Phase 7/Phase 3), `ALGORITHMS.md` (Algorithm 8), `wit/README.md`, `wit/types.wit`, `DEVELOPMENT.md`.

---

## Toolchain

| Tool | Version | Source | Was in `DEVELOPMENT.md`? |
|---|---|---|---|
| `moon` | `0.1.20260529` (moonc `v0.9.3+08f337e2c`) | pre-installed | ✅ |
| `wit-bindgen` | `0.57.1` | pre-installed | ✅ |
| `wasm-tools` | `1.251.0` | pre-installed | ✅ |
| Go | `go1.21.5 linux/amd64` | pre-installed | ✅ |
| `wasmtime-go` | `v45.0.0` | `go get` | n/a (added by spike) |
| **Why3** | **1.7.2** | **`opam install why3.1.7.2`** | ❌ **missing** |
| **Z3** | **4.8.12** | **`apt-get install z3`** | ❌ **missing** |

Proof-toolchain install (Ubuntu 22.04, user-local opam switch + apt for Z3):

```bash
sudo apt-get install -y z3 opam libgmp-dev pkg-config m4
opam init --disable-sandboxing -y --bare
opam switch create why3env ocaml-base-compiler.4.14.2
eval $(opam env --switch=why3env)
opam install why3.1.7.2 -y
why3 config detect          # discovers Z3, writes ~/.why3.conf
```

`moon prove` finds `why3` on `PATH` and generates its own `_build/verif/why3.conf` pointing at the detected Z3.

---

## Commands & results

All MoonBit/Go commands run from `spikes/moonbit-port-proof/` (or `go/`) with `~/.moon/bin` and the opam `why3env` on `PATH`.

### `moon --version`
```
moon 0.1.20260529 (3e1c753 2026-05-29) ~/.moon/bin/moon
moonc v0.9.3+08f337e2c (2026-05-29) ~/.moon/bin/moonc
moonrun 0.1.20260529 (3e1c753 2026-05-29) ~/.moon/bin/moonrun
Feature flags enabled: rr_moon_mod,rr_moon_pkg
```

### `moon test`
```
Total tests: 5, passed: 5, failed: 0.
```

### `time moon prove`
```
aid/spike/port-proof/src
  Succeeded: 2 goals proved

Summary:
  1 of 1 packages proved
  2 goals proved

real    0m0.641s
user    0m0.485s
sys     0m0.109s
```
**Proof runtime: 0.641 s wall (well under the 30 s gate).** The two goals are the combined post-conditions of `allocate` and of `non_overlap_holds`.

> ⚠️ **`moon prove` always exits 0**, even when goals fail. CI must parse stdout for `N of M packages proved` / `Failed goals:` — do **not** gate on `$?`. (See negative control below.)

### `moon build --target wasm --release`
```
Finished. moon: ran 2 tasks, now up to date
```
Produces `_build/wasm/release/build/src/src.wasm` (147 bytes, 0 imports, exports `memory` + `allocate` + `non_overlap_holds`). `./build.sh` copies it to `go/testdata/alloc.wasm`.

### `wit-bindgen moonbit` against `wit/` (from a scratch CWD)
```bash
rm -rf /tmp/aid-witgen-moonbit /tmp/aid-mbt-scratch && mkdir -p /tmp/aid-mbt-scratch
( cd /tmp/aid-mbt-scratch && wit-bindgen moonbit /ABS/PATH/TO/aid/wit --gen-dir /tmp/aid-witgen-moonbit )
```
- Exit **0**.
- **14** files under `--gen-dir` (`/tmp/aid-witgen-moonbit`), **9** files written relative to the CWD (`/tmp/aid-mbt-scratch`) — confirms `wit/README.md`'s note that `wit-bindgen moonbit` writes part of its output relative to CWD regardless of `--gen-dir`. Running from a scratch dir keeps the repo clean. **23 files total, all under `/tmp`, none committed.**
- The generated bindings use the **legacy `moon.mod.json` / `moon.pkg.json`** format (see note below).

### `go test ./...`
```
=== RUN   TestCorrectness
--- PASS: TestCorrectness (0.01s)
=== RUN   TestColdStart
    alloc_test.go: cold-start: 2.005156ms
--- PASS: TestColdStart (0.00s)
=== RUN   TestPerCallLatency
    alloc_test.go: per-call over 1000 calls: min=4.547µs avg=7.599µs max=61.743µs total=7.599987ms
--- PASS: TestPerCallLatency (0.01s)
PASS
ok      aid/spike/port-proof/go 0.031s
```
Benchmark (independent measurement):
```
BenchmarkAllocate-20    260841    4149 ns/op    295 B/op    17 allocs/op
```

### Repo hygiene
```
$ git diff --check
(clean: no whitespace/conflict errors)

$ git status --short
?? spikes/                       # only the intended spike files

$ rg -n "Python|NetBox|TODO|FIXME" spikes/moonbit-port-proof
(no matches)
```

---

## Go runtime timing (gate detail)

Measured on Intel Xeon Platinum 8160 @ 2.10GHz, `wasmtime-go` v45.0.0, core-wasm module.

| Metric | Definition | Result | Gate | Margin |
|---|---|---|---|---|
| Cold-start | `NewEngine` + compile + instantiate + resolve exports (everything to make the first call) | ~2.0 ms | < 500 ms | ~250× |
| Per-call (avg over 1000) | repeated `allocate` calls after instantiation | ~7.6 µs | < 10 ms | ~1300× |
| Per-call (min / max) | — | 4.5 µs / 61.7 µs | — | — |
| Benchmark | `go test -bench` | 4149 ns/op | — | — |

---

## Negative control (proof actually verifies)

A spike that "passes" because the prover is a no-op is worthless, so the prover was confirmed to **reject** a false claim. With `non_overlap_holds` temporarily inverted to `(base + j) == (base + k)` (false for distinct offsets) but keeping the post-condition `result == true`:

```
spike/.../src
  Failed: 1 goals proved, 1 timeout
Summary:
  0 of 1 packages proved
  1 goals proved, 1 timeout
```

The false goal is **not** discharged (`0 of 1 packages proved`). Two prerequisites were also confirmed empirically:
1. Without `"proof-enabled": true` in the package config, `moon prove` silently no-ops (`0.06 s`, nothing proved) — the flag is **mandatory**.
2. Without `why3` on `PATH`, `moon prove` errors: `failed to locate 'why3' required by 'moon prove'; install Why3 1.7.2`.

This is why the spike package sets `"proof-enabled": true` and why the toolchain finding above is load-bearing.

---

## Proof syntax & maintenance guide (for the next agent)

Enough to add a new invariant without prior MoonBit-proof experience.

**1. Opt the package in.** In `src/moon.pkg.json`:
```json
{ "proof-enabled": true }
```
Omit this and `moon prove` does nothing.

**2. Attach contracts to a function** via a `where { ... }` block between the return type and the body. Keywords are **singular**:
```moonbit
pub fn f(x : Int) -> Int where {
  proof_require: x >= 0,                 // precondition
  proof_ensure: result => result >= x,   // postcondition; `result =>` binds the return value
} {
  x + 1
}
```

**3. Syntax rules learned the hard way:**
- Keep each proof expression on **one line**. A line that *starts* with `→` (i.e. wrapping an implication onto the next line) is a parse error.
- Implication is `→`, conjunction `&&`, quantifier `∀ name : Type,` (Unicode; the LSP/editor inserts them). Example inline assertion:
  `proof_assert ∀ j : Int, ∀ k : Int, (0 <= j && j < n && 0 <= k && k < n && j != k) → (base + j != base + k)`
- For richer invariants, define named `predicate`/`lemma` and helper `fn` in a sibling **`.mbtp`** file (same package). See `moonbit-community/verified` (`stack_min`, `sparse_array`) for worked examples; this spike's invariants were simple enough to inline.

**4. Run it:** `moon prove`. Read the `N of M packages proved` summary — **never trust the exit code**. A goal that the solver can't close shows as `timeout`/`Failed`. If a true goal times out, raise the solver time limit or add intermediate `proof_assert` lemmas to guide Z3 (as the verified-repo examples do).

**5. Picking solvers:** this spike used Z3 4.8.12 only and it discharged the linear-arithmetic goals instantly. The upstream verified repo also configures Alt-Ergo and CVC5; harder kernel invariants (recursive BOM scaling, mesh constraints) may benefit from adding them via `why3 config detect`.

---

## Reproduce

```bash
# 0. proof toolchain (see Toolchain section) — Why3 1.7.2 + Z3, why3 on PATH
# 1. MoonBit: tests + proof + wasm artifact
cd spikes/moonbit-port-proof
moon test
moon prove          # expect: "1 of 1 packages proved, 2 goals proved"
./build.sh          # regenerates go/testdata/alloc.wasm

# 2. Go runtime + timing
cd go
go test ./...                                       # gates: cold-start <500ms, per-call <10ms
go test -bench=BenchmarkAllocate -benchmem -run=^$  # optional throughput

# 3. WIT binding exercise (outputs to /tmp, repo stays clean)
rm -rf /tmp/aid-witgen-moonbit /tmp/aid-mbt-scratch && mkdir -p /tmp/aid-mbt-scratch
( cd /tmp/aid-mbt-scratch && wit-bindgen moonbit "$PWD/../../../wit" --gen-dir /tmp/aid-witgen-moonbit )
```

### Why the wasm is committed
`go/testdata/alloc.wasm` (147 bytes) is checked in so `go test ./...` — the runtime gate — runs **without** the MoonBit/Why3 toolchain. It is the compiled artifact under test, not a WIT binding. Regenerate it with `./build.sh` after editing `src/`. (WIT bindings, by contrast, are written only to `/tmp` and are never committed.)

---

## Risks & follow-ups

1. **Proof toolchain not provisioned (highest priority).** Why3 1.7.2 + an SMT solver (Z3) are required by `moon prove` but absent from `DEVELOPMENT.md`'s "Current Local Readiness". **Follow-up:** add Why3 + Z3 (and ideally Alt-Ergo/CVC5) to `DEVELOPMENT.md`'s required toolchain and to any CI/provisioning before Phase 8. Without them the proof gate is unverifiable.
2. **`moon prove` exit code is always 0.** CI must parse stdout (`N of M packages proved`, `Failed goals:`). A naive `moon prove && echo ok` would report success on a failed proof. **Follow-up:** wrap `moon prove` in a CI script that greps for `0 of` / `Failed` and fails the build.
3. **Config-format drift.** `moon 0.1.20260529` scaffolds the new `moon.mod`/`moon.pkg` format (feature flags `rr_moon_mod,rr_moon_pkg`), while `wit-bindgen 0.57.1` and the upstream verified repo still emit/consume the legacy `moon.mod.json`/`moon.pkg.json`. This spike uses legacy JSON throughout (it accepts both `"proof-enabled"` and `link.wasm.exports`, which is what the spike needs) and `moon` reads it fine. The new-format `moon.pkg` link/export syntax could not be determined from docs or stdlib examples. **Follow-up:** pin a config format convention for the project before the kernel grows.
4. **Runtime path is core-wasm, not a Component-Model component.** MoonBit emits a core module; the Go harness calls it via `wasmtime-go`'s stable core-module API. This fully satisfies the D2 runtime gate ("MoonBit component callable from Go via `wasmtime-go`") with huge margins, and is the issue's explicitly-allowed "closest reasonable supported path." The full D8 Component-Model path (`wasm-tools component embed` + `component new` on the **core** `wasm` target — not `wasm-gc`, which `wasm-tools` rejects — then `wasmtime-go`'s newer component API) was **not** exercised here. **Follow-up:** a small Phase 3 spike to wrap the kernel as a real WIT component and call it through the component API, to validate canonical-ABI marshalling of the richer `aid:core` types (records/lists/variants) rather than bare `i32`s.
5. **Solver version.** Verified with Z3 4.8.12 (Ubuntu apt). Upstream uses newer Z3/Alt-Ergo/CVC5. Kernel-scale invariants may need newer/multiple solvers and a committed `why3.conf`. **Follow-up:** decide on a pinned solver set for reproducible proofs.
6. **MoonBit 1.0 stability** (`TECH_STACK.md` risk, `ROADMAP.md` Phase 7 criterion 3). The spike used only `Int` arithmetic and the proof DSL — no stdlib surface likely to churn. Re-verify at MoonBit 1.0; nothing here suggests a blocker.

## Scope honesty
This spike validates *feasibility*, not the kernel. The model is intentionally trivial (linear-arithmetic invariants over `Int`). It does **not** prove anything about the real `topology-plan` types, recursive BOM traversal, breakout expansion, or multi-zone allocation — those are Phase 3/Phase 8 work and may be materially harder for the solver. The evidence here supports *continuing* with MoonBit, not a claim that the full kernel is proven.
