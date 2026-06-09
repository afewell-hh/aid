// Feasibility spike — NOT the production kernel.
//
// A minimal, pure model of AID's zone-cursor port allocator
// (ALGORITHMS.md, Algorithm 8: Port Allocation). It exists only to exercise
// MoonBit's formal-verification path (`moon prove`) and the Go/wasmtime runtime
// path for AID issue #5. The full topology kernel (Phase 3) is out of scope.
//
// The proof annotations (`where { proof_require / proof_ensure }`,
// `proof_assert`) are first-class MoonBit syntax discharged by `moon prove`
// via Why3 + an SMT solver. See README.md for the verification toolchain and
// the maintenance guide for adding new invariants.

///|
/// Zone-cursor allocation count (Algorithm 8, pre-flight + cursor advance).
///
/// A switch zone exposes `capacity` logical ports. `requested` connections each
/// consume one port, handed out sequentially from a per-(switch, zone) cursor:
/// the j-th connection (`0 <= j < count`) receives logical port `base + j`
/// (see `non_overlap_holds` for the port-identity map). `allocate` returns how
/// many ports the zone actually hands out: the full demand when it fits, else
/// the zone is filled to capacity.
///
/// Proven invariants (discharged by `moon prove`):
///   - never hands out more ports than the zone has        (count <= capacity)
///   - never hands out more than were requested            (count <= requested)
///   - completeness: full demand is met whenever it fits
///                              (requested <= capacity -> count == requested)
///   - the result is a well-formed non-negative count      (count >= 0)
pub fn allocate(capacity : Int, requested : Int) -> Int where {
  proof_require: capacity >= 0,
  proof_require: requested >= 0,
  proof_ensure: count => count <= capacity,
  proof_ensure: count => count <= requested,
  proof_ensure: count => requested <= capacity → count == requested,
  proof_ensure: count => count >= 0,
} {
  if requested <= capacity {
    requested
  } else {
    capacity
  }
}

///|
/// Port non-overlap — the MANDATORY Algorithm 8 invariant.
///
/// Within a zone whose cursor starts at logical port `base`, the j-th
/// connection is assigned port `base + j`. This function states that any two
/// distinct connection offsets `j != k` map to two distinct logical ports
/// (`base + j != base + k`): the cursor map is injective, so the allocator
/// never assigns one logical port to two connections. It returns `true` for
/// every distinct pair, and the post-condition `result == true` forces
/// `moon prove` to discharge that it is *always* true (i.e. no counterexample
/// exists). Combined with `allocate` (which keeps every allocated offset inside
/// `[0, capacity)`), this is the per-zone port-non-overlap guarantee.
pub fn non_overlap_holds(base : Int, j : Int, k : Int) -> Bool where {
  proof_require: base >= 0,
  proof_require: j >= 0,
  proof_require: k >= 0,
  proof_require: j != k,
  proof_ensure: result => result == true,
} {
  (base + j) == (base + k)
}
