// F3 BOM-scale kernel boundary (docs/foundation/f3-architecture-note.md §1.1, §4;
// Issue #56; D2/D6). The Go reducer (internal/bom) resolves the catalog + topology
// + F2 calc-output into a FLAT node tree and hands the kernel a "bom-scale plan"
// (each node: parent index + quantity_per_parent + the owning entry's
// plan_quantity). The kernel computes every node's quantity_per_unit and
// fleet_quantity by ROUTING THROUGH THE PROVEN I4 CORES @proofs.child_qpu /
// @proofs.fleet_quantity — so the D2/§4.4 BOM-scaling invariant
// (`fleet = qpu × plan_quantity`, `qpu = parent_qpu × qpp`) holds for the real
// model BY CONSTRUCTION. Go then renders the two CSV views; attributes never cross
// the boundary (only the scaling arithmetic does — note §1.2).
//
// The cores are already proved and CI-gated (kernel/proofs/cores.mbt; I4); F3 adds
// NO new proof goals and does NOT weaken them.
//
// F3 RED: the wire types + the proved-core scaling primitive `f3_scale` are
// present and unit-tested (it routes through the cores); the full
// decode→scale→encode pipeline `f3_bom` is a STUB returning an empty, well-formed
// output. GREEN fills the decode/encode and the tree walk.

// ----------------------------------------------------------------------
// bom-scale plan (Go -> kernel) / scaled lines (kernel -> Go)
// ----------------------------------------------------------------------

/// One node of the flattened resolve tree. `parent_index` is the index of the
/// parent node in the plan's node array, or -1 for a root (an instantiated
/// server/switch line). `quantity_per_parent` is this node's multiplier within its
/// parent (e.g. the 8× CX-7 slot, the 24× memory line); `plan_quantity` is the
/// owning entry's instance count (the F2 server/switch fleet quantity).
pub(all) struct F3Node {
  node_id : String
  parent_index : Int
  quantity_per_parent : Int
  plan_quantity : Int
} derive(Eq, Show)

pub(all) struct F3ScalePlan {
  nodes : Array[F3Node]
} derive(Eq, Show)

/// One scaled line: the node's `quantity_per_unit` (product of qpp from root) and
/// `fleet_quantity` (qpu × plan_quantity), both via the proven cores.
pub(all) struct F3ScaledLine {
  node_id : String
  quantity_per_unit : Int
  fleet_quantity : Int
} derive(Eq, Show)

pub(all) struct F3BomOutput {
  lines : Array[F3ScaledLine]
} derive(Eq, Show)

// ----------------------------------------------------------------------
// the proved-core scaling primitive
// ----------------------------------------------------------------------

/// Scale one node THROUGH THE PROVEN I4 CORES. `parent_qpu` is the parent's
/// already-computed quantity_per_unit (1 for a root). Returns
/// `(quantity_per_unit, fleet_quantity)` where:
///   - quantity_per_unit = @proofs.child_qpu(parent_qpu, quantity_per_parent)
///   - fleet_quantity    = @proofs.fleet_quantity(quantity_per_unit, plan_quantity)
/// This IS the kernel's BOM-scaling rule — the same identity proved in
/// kernel/proofs/cores.mbt (I4). The GREEN tree walk calls this per node in
/// parent-before-child order; RED unit-tests it directly.
pub fn f3_scale(
  parent_qpu : Int,
  quantity_per_parent : Int,
  plan_quantity : Int
) -> (Int, Int) {
  let qpu = @proofs.child_qpu(parent_qpu, quantity_per_parent)
  let fleet = @proofs.fleet_quantity(qpu, plan_quantity)
  (qpu, fleet)
}

// ----------------------------------------------------------------------
// boundary entry (D16; wired to export_f3_bom)
// ----------------------------------------------------------------------

/// F3 bom-scale plan JSON -> F3 scaled-lines JSON. RED STUB: returns an empty,
/// well-formed output so the Go host always decodes a valid result. GREEN decodes
/// the plan, walks the nodes parent-before-child calling `f3_scale` (routing every
/// multiply through the proven cores), and encodes the scaled lines.
pub fn f3_bom(_plan_json : String) -> String {
  "{\"lines\":[]}"
}
