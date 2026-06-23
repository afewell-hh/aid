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

fn warn_box(msg : String) -> String {
  "<div class=\"alert alert-warning\">\{esc(msg)}</div>"
}

// error_html is the shared HTTP/network error alert (P0.4). It renders a clear,
// human danger alert — never success/empty markup — for a non-2xx response or a
// network failure (status==0). When the body carries a structured {"error": ...}
// message it is shown; otherwise a status-derived fallback is used. This is
// distinct from calc-as-data (is_valid:false), which stays the calc-summary path.
pub fn error_html(status : Int, body : String) -> String {
  // Prefer the server's structured error message when present.
  let detail = error_message(body)
  let headline = if status == 0 {
    "Network error"
  } else {
    "Request failed (HTTP \{status})"
  }
  let msg = if detail != "" {
    detail
  } else if status == 0 {
    "Could not reach the server. Check that it is running and try again."
  } else {
    "The server returned an error."
  }
  "<div class=\"alert alert-danger\" role=\"alert\">" +
  "<strong>\{esc(headline)}:</strong> \{esc(msg)}</div>"
}

// error_message extracts a top-level {"error": "..."} string from a JSON body, or
// "" when the body is not JSON / has no error key. Used both to surface a server
// error message and to detect an error body on an otherwise 2xx-shaped response.
fn error_message(body : String) -> String {
  let root = try {
    @json.parse(body)
  } catch {
    _ => return ""
  }
  str_at(root, "error")
}

// body_has_error reports whether a (2xx) body is actually a structured error.
pub fn body_has_error(body : String) -> Bool {
  error_message(body) != ""
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

// quantity_table renders a class_id→quantity table for a CalcOutput quantity
// array (switch_quantity / server_quantity); "" when the array is empty.
fn quantity_table(root : Json, key : String, title : String) -> String {
  let items = arr_at(root, key)
  if items.length() == 0 {
    return ""
  }
  let mut rows = ""
  for q in items {
    rows = rows +
      "<tr><td>\{esc(str_at(q, "class_id"))}</td><td class=\"text-end\">\{int_at(q, "quantity").to_string()}</td></tr>"
  }
  "<h4 class=\"h6\">\{title}</h4>" +
  "<table class=\"table table-sm mb-3\"><thead><tr><th>Class</th><th class=\"text-end\">Qty</th></tr></thead>" +
  "<tbody>\{rows}</tbody></table>"
}

// calc_summary_html: the F7b calc result, rendered as the two-plane validation
// (note §3.0). A STRUCTURAL failure arrives as a 4xx {"error": ...} body and
// renders as a distinct danger alert (no Valid/Invalid badge). Otherwise the calc
// result is DATA: an is_valid badge, the computed switch/server quantities, an
// endpoint/verdict summary, and any calc constraint violations listed as errors.
pub fn calc_summary_html(calc_json : String) -> String {
  let root = try {
    @json.parse(calc_json)
  } catch {
    _ => return warn_box("Could not parse calc result.")
  }
  // Structural plane: a 4xx {"error": ...} body, distinct from calc-as-data.
  match get(root, "error") {
    Some(String(msg)) =>
      return "<div class=\"alert alert-danger\"><strong>Cannot resolve plan:</strong> \{esc(msg)}</div>"
    _ => ()
  }
  let is_valid = bool_at(root, "is_valid")
  let badge = if is_valid {
    "<span class=\"badge text-bg-success\">Valid</span>"
  } else {
    "<span class=\"badge text-bg-danger\">Invalid</span>"
  }
  let endpoints = arr_at(root, "endpoints").length().to_string()
  let verdicts = arr_at(root, "transceiver_verdicts").length().to_string()
  let errors = arr_at(root, "errors")
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
  // Per-fabric wiring download (P0.3): rendered only on a VALID calc, populated
  // from the server-derived managed_fabrics list (never guessed). app.mbt wires
  // each button (id "wiring-<fabric>") to download_wiring(plan_id, fabric).
  let wiring_block = if is_valid {
    wiring_download_html(arr_at(root, "managed_fabrics"))
  } else {
    ""
  }
  "<div class=\"card\">" +
  "<div class=\"card-header d-flex justify-content-between align-items-center\"><span>Validation</span>\{badge}</div>" +
  "<div class=\"card-body\">" +
  quantity_table(root, "switch_quantity", "Switch quantities") +
  quantity_table(root, "server_quantity", "Server quantities") +
  "<p class=\"mb-0 text-muted small\">\{endpoints} endpoint(s) · \{verdicts} transceiver verdict(s)</p>" +
  wiring_block +
  "</div>\{err_block}</div>"
}

// managed_fabrics extracts the managed-fabric name strings from a calc-result
// JSON body (the server-derived list). Empty when absent or not an array.
pub fn managed_fabrics(calc_json : String) -> Array[String] {
  let root = try {
    @json.parse(calc_json)
  } catch {
    _ => return []
  }
  let names : Array[String] = []
  for f in arr_at(root, "managed_fabrics") {
    match f {
      String(s) => names.push(s)
      _ => ()
    }
  }
  names
}

// wiring_download_html renders one "Download wiring" button per managed fabric,
// each with a stable id "wiring-<fabric>" the event wiring binds. "" when there
// are no managed fabrics (nothing to offer).
fn wiring_download_html(fabrics : Array[Json]) -> String {
  let names : Array[String] = []
  for f in fabrics {
    match f {
      String(s) => names.push(s)
      _ => ()
    }
  }
  if names.length() == 0 {
    return ""
  }
  let mut btns = ""
  for name in names {
    btns = btns +
      "<button id=\"wiring-\{esc(name)}\" class=\"btn btn-outline-success btn-sm me-2 mb-2\" " +
      "data-fabric=\"\{esc(name)}\">Download wiring: \{esc(name)}</button>"
  }
  "<hr><h4 class=\"h6\">Validated wiring (hhfab)</h4>" +
  "<div class=\"d-flex flex-wrap\">\{btns}</div>"
}

// bom_html: the F7b BOM — a flat table of rows[] (the projection's string cells:
// section / model / hedgehog class / manufacturer / quantity) plus the
// suppressed-cable-assembly footer. Replaces the old hierarchical per-device-class
// per-unit/fleet view.
pub fn bom_html(bom_json : String) -> String {
  let root = try {
    @json.parse(bom_json)
  } catch {
    _ => return warn_box("Could not parse BOM.")
  }
  let items = arr_at(root, "rows")
  let mut rows = ""
  for r in items {
    rows = rows +
      "<tr>" +
      "<td>\{esc(str_at(r, "section"))}</td>" +
      "<td>\{esc(str_at(r, "module_type_model"))}</td>" +
      "<td>\{esc(str_at(r, "hedgehog_class"))}</td>" +
      "<td>\{esc(str_at(r, "manufacturer"))}</td>" +
      "<td class=\"text-end\">\{esc(str_at(r, "quantity"))}</td>" +
      "</tr>"
  }
  let suppressed = int_at(root, "suppressed_cable_assembly_count").to_string()
  "<h3 class=\"h5\">Bill of Materials</h3>" +
  "<table class=\"table table-sm\">" +
  "<thead><tr><th>Section</th><th>Model</th><th>Class</th><th>Manufacturer</th><th class=\"text-end\">Qty</th></tr></thead>" +
  "<tbody>\{rows}</tbody></table>" +
  "<p class=\"text-muted small mb-0\">Suppressed cable assemblies: \{suppressed}</p>"
}
