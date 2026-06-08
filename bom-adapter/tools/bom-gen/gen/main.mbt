// bom-gen — regenerate vendored device-class-bom[] JSON for the BOM adapter.
//
// Interim test tooling (NOT the production boundary): runs the merged Phase-3
// kernel (`aid/kernel`) `calculate()` on each valid fixture's embedded plan JSON
// and serializes the resulting typed `boms` slice (`Array[DeviceClassBom]`) to
// the snake_case JSON wire shape the BOM adapter consumes. This is the
// single-sourced Layer-1 -> Layer-2 contract documented in ../../BOM_CONTRACT.md
// (mapped field-for-field to wit/types.wit `device-class-bom`). Per the D16
// extension, the Phase-6 kernel encoder must emit these same bytes; this
// hand-rolled encoder is the interim stand-in (no `kernel/` edits — additive).
//
// The adapter RENDERS this data; it does not recompute the BOM or re-derive the
// role-based root rule (the kernel already applied it — server entries omit the
// root SKU, switch entries include it). This tool simply faithfully serializes
// whatever `calculate()` produced.
//
// Output: a single JSON object {fixture-name: [device-class-bom...]} on stdout;
// ../gen-bom.sh splits it into tests/testdata/<fixture>.boms.json.

// --- minimal JSON encoding (full control over integer tokens) ---------------

fn esc(s : String) -> String {
  let mut out = "\""
  for c in s {
    if c == '"' {
      out = out + "\\\""
    } else if c == '\\' {
      out = out + "\\\\"
    } else if c == '\n' {
      out = out + "\\n"
    } else if c == '\r' {
      out = out + "\\r"
    } else if c == '\t' {
      out = out + "\\t"
    } else {
      out = out + c.to_string()
    }
  }
  out + "\""
}

fn opt_str(o : String?) -> String {
  match o {
    Some(s) => esc(s)
    None => "null"
  }
}

fn enc_str_array(parts : Array[String]) -> String {
  let mut out = "["
  for i in 0..<parts.length() {
    if i > 0 {
      out = out + ","
    }
    out = out + esc(parts[i])
  }
  out + "]"
}

fn enc_array(parts : Array[String]) -> String {
  let mut out = "["
  for i in 0..<parts.length() {
    if i > 0 {
      out = out + ","
    }
    out = out + parts[i]
  }
  out + "]"
}

fn enc_summary(s : @kernel.DeviceClassSummary) -> String {
  "{\"id\":" +
  esc(s.id) +
  ",\"name\":" +
  esc(s.name) +
  ",\"slug\":" +
  esc(s.slug) +
  ",\"category\":" +
  esc(s.category) +
  ",\"manufacturer\":" +
  opt_str(s.manufacturer) +
  ",\"part_number\":" +
  opt_str(s.part_number) +
  "}"
}

fn enc_line_item(li : @kernel.BomLineItem) -> String {
  "{\"path\":" +
  enc_str_array(li.path) +
  ",\"device_class\":" +
  enc_summary(li.device_class) +
  ",\"quantity_per_parent\":" +
  li.quantity_per_parent.to_string() +
  ",\"quantity_per_unit\":" +
  li.quantity_per_unit.to_string() +
  ",\"fleet_quantity\":" +
  li.fleet_quantity.to_string() +
  "}"
}

fn enc_bom(b : @kernel.DeviceClassBom) -> String {
  let items = []
  for li in b.line_items {
    items.push(enc_line_item(li))
  }
  "{\"device_class\":" +
  enc_summary(b.device_class) +
  ",\"entry_id\":" +
  esc(b.entry_id) +
  ",\"plan_quantity\":" +
  b.plan_quantity.to_string() +
  ",\"line_items\":" +
  enc_array(items) +
  "}"
}

fn calc_boms(plan : String) -> String {
  match @kernel.calculate(plan) {
    Ok(out) => {
      let boms = []
      for b in out.boms {
        boms.push(enc_bom(b))
      }
      enc_array(boms)
    }
    Err(_) => "null"
  }
}

fn main {
  let s = "{\"clos-small\":" +
    calc_boms(@kernel.clos_small_plan_json) +
    ",\"mesh-two-switch\":" +
    calc_boms(@kernel.mesh_two_switch_plan_json) +
    ",\"switch-bom\":" +
    calc_boms(@kernel.switch_bom_plan_json) +
    "}"
  println(s)
}
