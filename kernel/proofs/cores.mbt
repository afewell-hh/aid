// AID kernel proof cores — Phase 8, Issue #7 (DECISIONS.md D2; ARCHITECTURE Layer 1).
//
// Pure Int/Bool functions carrying machine-checked contracts discharged by
// `moon prove` (Why3 + Z3). The production kernel (`aid/kernel/src`) routes its
// REAL computation through these cores — they are the production code path, not
// a parallel copy. See the spike maintenance guide
// (`spikes/moonbit-port-proof/README.md`) for the `where { proof_require /
// proof_ensure }` syntax and the `moon prove` gate.
//
// VERIFICATION SCOPE ("[bridge]"). A `moon prove` logic body cannot reference
// `Array` methods (`.length()`), struct fields, or enum equality — only Int/Bool
// arithmetic and calls to pure/contracted functions. So each invariant is proven
// at its pure-arithmetic core here; the surrounding Array / whole-plan wiring
// (loop bounds, recursive traversal, validation iteration) is covered by the
// kernel's fixture tests, not by `moon prove`. This split is documented
// per-invariant in the Phase 8 report and in ARCHITECTURE.md Layer 1.

// --------------------------------------------------------------------------
// I1 — Port non-overlap (Algorithm 8 — zone-cursor allocation)
// --------------------------------------------------------------------------

/// The logical port assigned to connection offset `i` within a zone whose cursor
/// starts at `base`: the cursor map. `allocate_sequential` hands offset `i` this
/// exact port, so this function IS the kernel's port-assignment rule.
pub fn port_at(base : Int, offset : Int) -> Int {
  base + offset
}

/// Port non-overlap — the mandatory Algorithm 8 invariant. The cursor map is
/// INJECTIVE: two distinct connection offsets never receive the same logical
/// port. Faithful (not a tautology): for `j != k`, `port_at(base,j) !=
/// port_at(base,k)`; `proof_ensure: result == true` forces the prover to show no
/// counterexample exists for ANY inputs. Together with `allocate_sequential`
/// covering the offsets `[0, demand)`, this is the per-zone non-overlap
/// guarantee. (I2 completeness is the totality face of this same map — the
/// allocated ports are exactly the `demand` distinct ports `port_at(base, 0..demand)`.)
pub fn ports_distinct(base : Int, j : Int, k : Int) -> Bool where {
  proof_require: j != k,
  proof_ensure: result => result == true,
} {
  (base + j) != (base + k)
}

// --------------------------------------------------------------------------
// I3 — Switch-count lower bound (Algorithms 1 & 2)
// --------------------------------------------------------------------------

/// Integer ceiling division for a positive divisor — the lower-bound core. For
/// `n >= 0`, `d > 0`:
///   - `q * d >= n`              : `q` switches cover at least `n` logical ports
///   - `(q - 1) * d < n` (n>=1)  : `q` is MINIMAL — no smaller count covers `n`
///   - `q >= 0`                  : well-formed count
/// `spine_count` and the base of `leaf_count` both delegate here.
pub fn ceil_div_pos(numerator : Int, denominator : Int) -> Int where {
  proof_require: numerator >= 0,
  proof_require: denominator > 0,
  proof_ensure: q => q * denominator >= numerator,
  proof_ensure: q => (q - 1) * denominator < numerator || numerator == 0,
  proof_ensure: q => q >= 0,
} {
  (numerator + denominator - 1) / denominator
}

/// Leaf-count adjustment for the NON-ESLAG paths: the Alternating "at least 2"
/// bump and the MCLAG even-rounding. Two proven properties:
///   - MONOTONE (`result >= base`): the adjustments never lower the count, so the
///     `ceil_div_pos` lower bound survives — this is how `leaf_count >=
///     ceil(demand/usable)` is established for the non-ESLAG paths (I3).
///   - MCLAG output is EVEN and `>= 2` (I6): the kernel never emits an invalid
///     MCLAG leaf count by construction.
/// `force_two` is `distribution == Alternating`; `mclag` is the MCLAG branch.
pub fn leaf_adjust_non_eslag(base : Int, force_two : Bool, mclag : Bool) -> Int where {
  proof_ensure: result => result >= base,
  proof_ensure: result => force_two → result >= 2,
  proof_ensure: result => mclag → result >= 2 && result % 2 == 0,
} {
  let mut count = base
  if force_two && count < 2 {
    count = 2
  }
  if mclag {
    if count < 2 {
      count = 2
    }
    if count % 2 != 0 {
      count = count + 1
    }
  }
  count
}

/// ESLAG leaf count is hard-clamped to `[2, 4]` — a deliberate cap, NOT the
/// ceil-div lower bound (the clamp can legitimately drop below `ceil(demand/
/// usable)`; the runtime hazard of that is tracked in #41, the doc was fixed in
/// #40). This proves ESLAG's own invariant: `2 <= result <= 4`.
pub fn leaf_clamp_eslag(base : Int) -> Int where {
  proof_ensure: result => result >= 2,
  proof_ensure: result => result <= 4,
} {
  let mut count = base
  if count < 2 {
    count = 2
  }
  if count > 4 {
    count = 4
  }
  count
}

// --------------------------------------------------------------------------
// I4 — BOM scaling (Algorithm 6 — recursive DeviceClass traversal)
// --------------------------------------------------------------------------

/// quantity_per_unit of a child node = parent's qpu × quantity_per_parent.
/// Proven: `result == parent_qpu * quantity_per_parent` and non-negative.
/// `bom_traverse` computes every node's qpu through this core, so the scaling
/// identity holds at EVERY tree level by construction (the recursion itself is
/// over `Array[SubComponent]`, hence [bridge]: traversal wiring is test-covered).
pub fn child_qpu(parent_qpu : Int, quantity_per_parent : Int) -> Int where {
  proof_require: parent_qpu >= 0,
  proof_require: quantity_per_parent >= 0,
  proof_ensure: r => r == parent_qpu * quantity_per_parent,
  proof_ensure: r => r >= 0,
} {
  parent_qpu * quantity_per_parent
}

/// fleet_quantity of a node = quantity_per_unit × plan_entry.quantity — THE D2
/// BOM-scaling identity. Proven: `result == quantity_per_unit * plan_quantity`
/// and non-negative, at every tree level (`bom_traverse`/`bom_for_entry` route
/// every fleet computation through this core).
pub fn fleet_quantity(quantity_per_unit : Int, plan_quantity : Int) -> Int where {
  proof_require: quantity_per_unit >= 0,
  proof_require: plan_quantity >= 0,
  proof_ensure: r => r == quantity_per_unit * plan_quantity,
  proof_ensure: r => r >= 0,
} {
  quantity_per_unit * plan_quantity
}

// --------------------------------------------------------------------------
// I5 — Mesh inter-switch link count (Algorithm 5)
// --------------------------------------------------------------------------

/// Mesh cables per switch pair (Algorithm 5). Proven WELL-FORMED under the mesh
/// switch-count guard (`switch_count ∈ {2,3}`, enforced by `validate_plan`):
/// non-negative and never exceeding the per-switch mesh port budget. The `{2,3}`
/// membership itself is a validation predicate over the plan ([bridge],
/// test-covered); this core proves the cable arithmetic is sound under it.
pub fn mesh_cable_count(switch_count : Int, logical_mesh_ports_per_switch : Int) -> Int where {
  proof_require: logical_mesh_ports_per_switch >= 0,
  proof_require: switch_count == 2 || switch_count == 3,
  proof_ensure: r => r >= 0,
  proof_ensure: r => r <= logical_mesh_ports_per_switch,
} {
  if switch_count == 3 {
    logical_mesh_ports_per_switch / 2
  } else {
    logical_mesh_ports_per_switch
  }
}
