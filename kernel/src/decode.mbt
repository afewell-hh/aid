// JSON -> TopologyPlan decoder (the kernel ingress edge, DECISIONS.md D16).
//
// The kernel is pure: this takes a JSON *string* (never a file path) in the
// user-facing plan-schema shape (snake_case) and returns a typed plan, or
// `calc-error::invalid-plan` on malformed/structurally-unusable input. Schema
// validation (ajv against schema/topology-plan-v1.json) stays OUTSIDE the kernel
// at the ingress/test layer; this decoder only fails closed on input it cannot
// turn into a well-formed TopologyPlan.
//
// RED phase: stub returns invalid-plan so the decode unit test fails.

/// Decode a plan JSON string into a typed TopologyPlan.
pub fn decode_plan(plan_json : String) -> Result[TopologyPlan, CalcError] {
  let _ = plan_json
  Err(Internal("decode_plan not implemented (RED)"))
}
