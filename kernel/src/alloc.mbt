// Algorithm 8 — zone-cursor port allocation (ALGORITHMS.md).

/// Number of physical ports described by a port-range expression.
///   "1-48"   -> 48        "49-56" -> 8        "33-36" -> 4
///   "1-32:2" -> base range "1-32" (the ":n" breakout/stride suffix is parsed
///               into logical ports elsewhere; here we count physical ports).
pub fn port_range_count(spec : String) -> Int {
  let mut low = 0
  let mut high = 0
  let mut seen_dash = false
  let mut stop = false
  for c in spec {
    if stop {
      // ignore the ":stride" suffix
      ()
    } else if c == ':' {
      stop = true
    } else if c == '-' {
      seen_dash = true
    } else if c >= '0' && c <= '9' {
      let d = c.to_int() - '0'.to_int()
      if seen_dash {
        high = high * 10 + d
      } else {
        low = low * 10 + d
      }
    }
  }
  if seen_dash {
    if high >= low {
      high - low + 1
    } else {
      0
    }
  } else if low > 0 {
    1
  } else {
    0
  }
}

/// Sequentially allocate `demand` logical ports from a cursor at `base`,
/// returning the assigned port numbers [base, base+demand). The cursor advances
/// monotonically so no port is revisited (port non-overlap).
pub fn allocate_sequential(base : Int, demand : Int) -> Array[Int] {
  let ports = []
  for i in 0..<demand {
    // The port for offset `i` is `@proofs.port_at(base, i)` (= base + i), the
    // cursor map proven injective by `@proofs.ports_distinct` (Issue #7 I1).
    ports.push(@proofs.port_at(base, i))
  }
  ports
}

/// Algorithm 8 pre-flight: true iff total_demanded <= total_capacity for a
/// (switch-class, zone) pair. A false result yields a per-zone deficit error.
pub fn zone_capacity_ok(total_demanded : Int, total_capacity : Int) -> Bool {
  total_demanded <= total_capacity
}
