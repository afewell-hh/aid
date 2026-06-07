// Algorithm 7 — oversubscription ratio (ALGORITHMS.md; DECISIONS.md D11).
//
// RED phase: stubs return placeholders so unit tests fail on assertions.

/// oversubscription_ratio = total_server_bandwidth_gbps / total_spine_bandwidth_gbps.
/// total_spine_bandwidth_gbps must be > 0.
pub fn oversubscription_ratio(
  total_server_bw_gbps : Int,
  total_spine_bw_gbps : Int
) -> Double {
  let _ = total_server_bw_gbps
  let _ = total_spine_bw_gbps
  -1.0
}

/// A ratio > 1.0 is surfaced as a warning (D11), never a blocking error.
pub fn is_oversubscribed(ratio : Double) -> Bool {
  let _ = ratio
  false
}
