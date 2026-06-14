// F3 BOM-scale kernel boundary (docs/foundation/f3-architecture-note.md §1.1, §4;
// Issue #56; D2/D6). The Go reducer (internal/bom) resolves the catalog + topology
// + F2 calc-output into a FLAT, topologically-ordered node tree and hands the
// kernel a "bom-scale plan" — each node: its parent's index (or -1 for a root
// server/switch line), its quantity_per_parent, and the owning entry's
// plan_quantity. The kernel computes every node's quantity_per_unit and
// fleet_quantity by ROUTING THROUGH THE PROVEN I4 CORES @proofs.child_qpu /
// @proofs.fleet_quantity — so the D2/§4.4 BOM-scaling invariant
// (`fleet = qpu × plan_quantity`, `qpu = parent_qpu × qpp`) holds for the real
// model BY CONSTRUCTION at every nesting level. Go then renders the two CSV views;
// only the scaling arithmetic crosses the boundary (note §1.2).
//
// The cores are already proved and CI-gated (kernel/proofs/cores.mbt; I4); F3 adds
// NO new proof goals and does NOT weaken them.

// ----------------------------------------------------------------------
// bom-scale plan (Go -> kernel) / scaled lines (kernel -> Go)
// ----------------------------------------------------------------------

/// One node of the flattened resolve tree. `parent_index` is the index of the
/// parent node in the plan's node array, or -1 for a root (an instantiated
/// server/switch line). Nodes are GUARANTEED parent-before-child by the Go host.
/// `quantity_per_parent` is this node's multiplier within its parent (e.g. the 8×
/// CX-7 slot, the 24× memory line); `plan_quantity` is the owning entry's instance
/// count (the F2 server/switch fleet quantity).
pub(all) struct F3Node {
  parent_index : Int
  quantity_per_parent : Int
  plan_quantity : Int
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
/// kernel/proofs/cores.mbt (I4). `f3_bom` calls it per node in parent-before-child
/// order; the unit tests exercise it directly.
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

/// F3 bom-scale plan JSON -> a COMPACT JSON array of per-node fleet quantities,
/// in node order: `[f0,f1,...]`. Decodes the node array, walks it in order (parents
/// first), and for each node routes the qpu/fleet multiply through `f3_scale` (the
/// proven cores). The Go host consumes only the fleet quantities (quantity_per_unit
/// is an internal of the recursion), so a flat int array keeps the boundary output
/// small. A malformed plan yields `[]` so the host always decodes a valid result.
pub fn f3_bom(plan_json : String) -> String {
  let nodes = try f3_decode_nodes(plan_json) catch {
    _ => []
  }
  let qpus : Array[Int] = []
  let mut body = ""
  let mut first = true
  for node in nodes {
    let parent_qpu = if node.parent_index < 0 || node.parent_index >= qpus.length() {
      1
    } else {
      qpus[node.parent_index]
    }
    let (qpu, fleet) = f3_scale(parent_qpu, node.quantity_per_parent, node.plan_quantity)
    qpus.push(qpu)
    if first {
      first = false
    } else {
      body = body + ","
    }
    body = body + fleet.to_string()
  }
  "[" + body + "]"
}

fn f3_decode_nodes(plan_json : String) -> Array[F3Node] raise {
  let j = @json.parse(plan_json)
  let m = d_obj(j)
  let out = []
  for el in d_arr_or_empty(m, "nodes") {
    let om = d_obj(el)
    out.push({
      parent_index: d_field_int(om, "parent_index"),
      quantity_per_parent: d_field_int(om, "quantity_per_parent"),
      plan_quantity: d_field_int(om, "plan_quantity"),
    })
  }
  out
}
