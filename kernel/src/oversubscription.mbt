// Algorithm 7 — oversubscription ratio (ALGORITHMS.md; DECISIONS.md D11).

/// oversubscription_ratio = total_server_bandwidth_gbps / total_spine_bandwidth_gbps.
/// Returns 0.0 when there is no spine tier (total_spine_bw == 0) — there is
/// nothing to oversubscribe against; callers treat that as "not applicable".
pub fn oversubscription_ratio(
  total_server_bw_gbps : Int,
  total_spine_bw_gbps : Int
) -> Double {
  if total_spine_bw_gbps <= 0 {
    0.0
  } else {
    total_server_bw_gbps.to_double() / total_spine_bw_gbps.to_double()
  }
}

/// A ratio > 1.0 is surfaced as a warning (D11), never a blocking error.
pub fn is_oversubscribed(ratio : Double) -> Bool {
  ratio > 1.0
}
