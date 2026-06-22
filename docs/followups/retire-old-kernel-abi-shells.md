# Follow-up: retire the old kernel ABI shells (deferred from F7d)

**Status:** deferred (intentionally, per the F7d guardrail). Tracked here because
`gh` is unavailable in the deva environment — **lead to file as a GitHub issue**
and link from #64 / #35.

## What was deferred

`kernel/wasm/abi.mbt` still exports the two old boundary shells:

- `export_calculate(ptr,len)` → `@src.encode_calc_result(@src.calculate(...))`
- `export_validate(ptr,len)`  → `@src.encode_validation_result(@src.validate(...))`

F7d removed everything that *called* them — `internal/orchestrate`,
`components.Hhfab/Bom`, and the `KernelCalculate`/`KernelValidate` entry
constants — so these exports are now **unreachable from the rebuilt engine**
(the Go boundary uses only `export_f2_calculate` / `export_f3_bom`).

## Why not removed in F7d

The F7d standing guardrail is "never risk the proved kernel for cleanup; remove
the abi shells only if cleanly separable with moon prove green + kernel.wasm
rebuilding clean + the F2 path unaffected — otherwise leave them dead and file a
follow-up."

Removing only the two `abi.mbt` exports is mechanically trivial, but it **orphans**
`@src.calculate` / `@src.validate` / `@src.encode_calc_result` /
`@src.encode_validation_result` in `kernel/src`, and a *full* retirement of those
touches the shared encoder/type surface that the kept **F2** path
(`@src.f2_calculate`) also uses. That is real risk to the proved kernel for a
purely cosmetic gain (dead, unreachable exports), so F7d defers it.

F7d evidence (kernel untouched this phase):
- `make wasm` rebuilds `embed/kernel.wasm` **byte-identical** to committed.
- `scripts/moon-prove-gate.sh spikes/moonbit-port-proof kernel/proofs` → all
  packages proved (port-proof 2 goals, kernel/proofs 8 goals).

## The follow-up

Audit `kernel/src` for the calc/validate/encode surface shared between the old
(`calculate`/`validate`) path and the kept F2/F3 path. Then either:

1. remove the two `abi.mbt` exports **and** the now-dead `@src.*` old-path
   functions/encoders that are exclusively theirs, leaving the F2/F3 path + the
   proof cores untouched; or
2. if the surface is too entangled to separate safely, leave the shells dead and
   record that decision.

**Acceptance:** `moon prove` green (port-proof + kernel/proofs), `kernel.wasm`
rebuilds clean, the full Go suite + oracle (mesh + Clos, real hhfab validate)
stay green, and `KernelF2Calculate` / `KernelF3Bom` are byte-for-byte unaffected.
