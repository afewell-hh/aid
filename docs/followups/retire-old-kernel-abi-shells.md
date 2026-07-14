# Follow-up: retire the old kernel ABI shells — RESOLVED (superseded by #84 / #85)

**Status:** fully resolved. The two dead ABI-shell exports were **removed in #84**,
and the remaining retirement — the invented WIT contract, its `types.mbt` mirror,
the legacy kernel cluster, and the `#38` drift guard — was **completed in #85**
(Option A; **DECISIONS D28**). The live kernel contract is now the F2/F3 JSON shapes
plus executable golden tests (`internal/wasmhost/golden_boundary_test.go`). This
file is kept only as a historical pointer.

## What has happened since

- **#84 (quarantine, merged/branch `deva/issue-84-quarantine-legacy-wit`)** removed
  the two old boundary exports from `kernel/wasm/abi.mbt` and
  `kernel/wasm/moon.pkg.json`:
  - `export_calculate(ptr,len)` → `@src.encode_calc_result(@src.calculate(...))`
  - `export_validate(ptr,len)`  → `@src.encode_validation_result(@src.validate(...))`

  The live host boundary is now **only** `export_f2_calculate` / `export_f3_bom`.
  The pure `@src.calculate` / `@src.validate` functions and their encoders remain
  compiled (via kernel unit tests) but are **quarantined legacy** — carrying
  `LEGACY / NON-LIVE` banners — and the D16 amendment in `DECISIONS.md` records
  the change.

- The reason removal was originally deferred (option 1 below "orphans" the shared
  `@src.calculate`/`encode` surface that the kept **F2** path reuses) still holds
  for a *full* prune: `kernel/src/decode.mbt` and `encode.mbt` share JSON
  primitives (`d_obj`/`d_field`/…, `j_esc`/`j_arr`) with F2/F3, so they cannot be
  deleted wholesale. #84 therefore removed only the dead exports and quarantined
  the rest.

## The remaining follow-up → **#85**

Deciding whether to (A) fully retire the invented WIT contract + `#38` drift guard
(after extracting the shared JSON primitives into their own module) or (B)
reconcile the WIT to the live F2/F3 boundary and retarget the guard is an
architecture decision, tracked in **#85**. See that issue for the full
option analysis; the #84 audit note captures the shared-surface entanglement.
