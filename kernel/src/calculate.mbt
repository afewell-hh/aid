// Top-level kernel entry points: plan JSON -> calc-output / validation.
//
// These are the logical realizations of the `topology-calculator` WIT interface
// (`calculate`, `validate`). Per D16 the MVP wasm boundary is JSON-over-memory;
// these typed functions are what the boundary edge (Phase 6) will wrap. The pure
// kernel takes the JSON string, decodes to typed input, and computes — no I/O.
//
// RED phase:
//   - `calculate` returns an empty calc-output (so every fixture count/BOM
//     assertion fails on its own value, mapping each to its seam).
//   - `validate` returns an all-valid result (so invalid-fixture tests fail).

/// Validate and calculate the full topology from a plan JSON string.
pub fn calculate(plan_json : String) -> Result[CalcOutput, CalcError] {
  let _ = plan_json
  Ok(empty_calc_output())
}

/// Validate a plan JSON string without producing the full IR/BOMs.
pub fn validate(plan_json : String) -> Result[ValidationResult, CalcError] {
  let _ = plan_json
  Ok(empty_validation())
}
