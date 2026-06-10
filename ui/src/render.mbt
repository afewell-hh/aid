// render.mbt — pure HTML-string builders (Bootstrap 5, NetBox style) for each
// surface. These take the Stage-A API's JSON response text and return markup
// (dark cards, tables, status badges). They touch no JS — app.mbt sets the
// result into the DOM via ffi. No new core behavior: rendering only.

// --- JSON helpers (over moonbitlang/core/json) ------------------------------
// Navigate by pattern-matching the Json enum (builtin). Missing keys / wrong
// shapes degrade to "", 0, false, or [] — the UI never throws on odd data.

fn get(j : Json, key : String) -> Json? {
  match j {
    Object(obj) => obj.get(key)
    _ => None
  }
}

fn str_at(j : Json, key : String) -> String {
  match get(j, key) {
    Some(String(s)) => s
    _ => ""
  }
}

fn int_at(j : Json, key : String) -> Int {
  match get(j, key) {
    Some(Number(n, ..)) => n.to_int()
    _ => 0
  }
}

fn bool_at(j : Json, key : String) -> Bool {
  match get(j, key) {
    Some(True) => true
    _ => false
  }
}

fn arr_at(j : Json, key : String) -> Array[Json] {
  match get(j, key) {
    Some(Array(a)) => a
    _ => []
  }
}

// count_at returns the length of the array at j[key1][key2] (0 if absent).
fn count_at(j : Json, key1 : String, key2 : String) -> Int {
  match get(j, key1) {
    Some(sub) => arr_at(sub, key2).length()
    None => 0
  }
}

// esc HTML-escapes text destined for element content / attributes.
fn esc(s : String) -> String {
  let mut out = ""
  for c in s {
    out = out +
      (match c {
        '&' => "&amp;"
        '<' => "&lt;"
        '>' => "&gt;"
        '"' => "&quot;"
        '\'' => "&#39;"
        _ => c.to_string()
      })
  }
  out
}

fn repeat(s : String, n : Int) -> String {
  let mut out = ""
  let mut i = 0
  while i < n {
    out = out + s
    i = i + 1
  }
  out
}

fn warn_box(msg : String) -> String {
  "<div class=\"alert alert-warning\">\{esc(msg)}</div>"
}

// status_badge maps a plan status to a Bootstrap badge.
fn status_badge(status : String) -> String {
  let cls = match status {
    "active" => "text-bg-success"
    "draft" => "text-bg-secondary"
    "archived" => "text-bg-dark"
    _ => "text-bg-info"
  }
  "<span class=\"badge \{cls}\">\{esc(status)}</span>"
}

// --- surfaces ---------------------------------------------------------------

// plan_list_html: a NetBox-style table of plan summaries with status badges and
// a per-row View action.
pub fn plan_list_html(plans_json : String) -> String {
  let root = try {
    @json.parse(plans_json)
  } catch {
    _ => return warn_box("Could not parse plan list.")
  }
  let plans = arr_at(root, "plans")
  let mut rows = ""
  for p in plans {
    let id = str_at(p, "id")
    let name = str_at(p, "name")
    let status = str_at(p, "status")
    rows = rows +
      "<tr>" +
      "<td>\{esc(name)}</td>" +
      "<td><code>\{esc(id)}</code></td>" +
      "<td>\{status_badge(status)}</td>" +
      "<td class=\"text-end\"><button id=\"view-\{esc(id)}\" class=\"btn btn-sm btn-outline-primary\">View</button></td>" +
      "</tr>"
  }
  let count = plans.length().to_string()
  "<div class=\"d-flex justify-content-between align-items-center mb-3\">" +
  "<h2 class=\"h4 mb-0\">Topology Plans</h2>" +
  "<span class=\"badge text-bg-secondary\">\{count} plan(s)</span></div>" +
  "<table class=\"table table-hover align-middle\">" +
  "<thead><tr><th>Name</th><th>ID</th><th>Status</th><th class=\"text-end\">Actions</th></tr></thead>" +
  "<tbody>\{rows}</tbody></table>"
}

// plan_detail_html: cards for the plan summary and its topology (fabrics +
// device classes, from the canonical YAML), with calc/BOM actions and a result
// region the calc/BOM views render into.
pub fn plan_detail_html(detail_json : String) -> String {
  let root = try {
    @json.parse(detail_json)
  } catch {
    _ => return warn_box("Could not parse plan detail.")
  }
  let id = str_at(root, "id")
  let name = str_at(root, "name")
  let status = str_at(root, "status")
  let yaml = str_at(root, "yaml")
  "<div class=\"d-flex justify-content-between align-items-center mb-3\">" +
  "<h2 class=\"h4 mb-0\">\{esc(name)}</h2>\{status_badge(status)}</div>" +
  "<div class=\"row g-3\">" +
  "<div class=\"col-md-4\"><div class=\"card\">" +
  "<div class=\"card-header\">Plan</div><div class=\"card-body\">" +
  "<dl class=\"row mb-3\">" +
  "<dt class=\"col-5\">ID</dt><dd class=\"col-7\"><code>\{esc(id)}</code></dd>" +
  "<dt class=\"col-5\">Status</dt><dd class=\"col-7\">\{status_badge(status)}</dd></dl>" +
  "<button id=\"calc-btn\" class=\"btn btn-primary btn-sm me-2\">Calculate</button>" +
  "<button id=\"bom-btn\" class=\"btn btn-outline-secondary btn-sm\">View BOM</button>" +
  "</div></div></div>" +
  "<div class=\"col-md-8\"><div class=\"card\">" +
  "<div class=\"card-header\">Topology (fabrics &amp; device classes)</div>" +
  "<div class=\"card-body\"><pre class=\"mb-0\"><code>\{esc(yaml)}</code></pre></div>" +
  "</div></div></div>" +
  "<div id=\"detail-result\" class=\"mt-3\"></div>"
}

// calc_summary_html: the validation result (a green/red badge + any error codes)
// and an IR summary (nodes/edges/fabrics) — validation surfaced as data.
pub fn calc_summary_html(calc_json : String) -> String {
  let root = try {
    @json.parse(calc_json)
  } catch {
    _ => return warn_box("Could not parse calc result.")
  }
  let nodes = count_at(root, "ir", "nodes").to_string()
  let edges = count_at(root, "ir", "edges").to_string()
  let fabrics = count_at(root, "ir", "fabrics").to_string()
  let is_valid = match get(root, "validation") {
    Some(v) => bool_at(v, "is_valid")
    None => false
  }
  let errors = match get(root, "validation") {
    Some(v) => arr_at(v, "errors")
    None => []
  }
  let badge = if is_valid {
    "<span class=\"badge text-bg-success\">Valid</span>"
  } else {
    "<span class=\"badge text-bg-danger\">Invalid</span>"
  }
  let mut err_rows = ""
  for e in errors {
    err_rows = err_rows +
      "<li class=\"list-group-item list-group-item-danger\"><code>\{esc(str_at(e, "code"))}</code> \{esc(str_at(e, "message"))}</li>"
  }
  let err_block = if errors.length() > 0 {
    "<ul class=\"list-group list-group-flush\">\{err_rows}</ul>"
  } else {
    ""
  }
  "<div class=\"card\">" +
  "<div class=\"card-header d-flex justify-content-between align-items-center\"><span>Validation</span>\{badge}</div>" +
  "<div class=\"card-body\"><p class=\"mb-0\">Topology: \{nodes} nodes, \{edges} edges, \{fabrics} fabric(s).</p></div>" +
  "\{err_block}</div>"
}

// bom_html: a hierarchical BOM per device class, showing BOTH per-unit and fleet
// quantities for every line item (the exit-gate multi-level requirement).
pub fn bom_html(bom_json : String) -> String {
  let root = try {
    @json.parse(bom_json)
  } catch {
    _ => return warn_box("Could not parse BOM.")
  }
  let boms = arr_at(root, "boms")
  let mut sections = ""
  for b in boms {
    let dc_name = match get(b, "device_class") {
      Some(dc) => str_at(dc, "name")
      None => ""
    }
    let qty = int_at(b, "plan_quantity").to_string()
    let items = arr_at(b, "line_items")
    let mut rows = ""
    for it in items {
      let level = int_at(it, "level")
      let name = str_at(it, "name")
      let per = int_at(it, "quantity_per_unit").to_string()
      let fleet = int_at(it, "fleet_quantity").to_string()
      let pad = repeat("&nbsp;&nbsp;&nbsp;&nbsp;", level)
      rows = rows +
        "<tr><td>\{pad}\{esc(name)}</td>" +
        "<td class=\"text-end\">\{per}</td>" +
        "<td class=\"text-end\">\{fleet}</td></tr>"
    }
    sections = sections +
      "<div class=\"card mb-3\"><div class=\"card-header\">\{esc(dc_name)} <span class=\"badge text-bg-secondary\">x\{qty}</span></div>" +
      "<div class=\"card-body p-0\"><table class=\"table table-sm mb-0\">" +
      "<thead><tr><th>Component</th><th class=\"text-end\">Per-Unit</th><th class=\"text-end\">Fleet</th></tr></thead>" +
      "<tbody>\{rows}</tbody></table></div></div>"
  }
  "<h3 class=\"h5\">Bill of Materials</h3>\{sections}"
}
