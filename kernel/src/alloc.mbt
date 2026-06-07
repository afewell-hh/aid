// Algorithm 8 — zone-cursor port allocation (ALGORITHMS.md).
//
// RED phase: stubs return placeholders so unit tests fail on assertions.

/// Number of physical ports described by a port-range expression.
///   "1-48"   -> 48        "49-56" -> 8        "33-36" -> 4
///   "1-32:2" -> stride form (handled in GREEN)
pub fn port_range_count(spec : String) -> Int {
  let _ = spec
  -1
}

/// Sequentially allocate `demand` logical ports from a cursor at `base`,
/// returning the assigned port numbers [base, base+demand). The cursor advances
/// monotonically so no port is revisited (port non-overlap).
pub fn allocate_sequential(base : Int, demand : Int) -> Array[Int] {
  let _ = base
  let _ = demand
  []
}

/// Algorithm 8 pre-flight: true iff total_demanded <= total_capacity for a
/// (switch-class, zone) pair. A false result yields a per-zone deficit error.
pub fn zone_capacity_ok(total_demanded : Int, total_capacity : Int) -> Bool {
  let _ = total_demanded
  let _ = total_capacity
  false
}
