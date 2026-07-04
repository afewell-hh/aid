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

// clone_yaml_identity rewrites a plan YAML's identity so a duplicate does not
// collide with its source: it suffixes the `meta.case_id` (with `suffix`, kept
// id-safe) and the `meta.name` (with a human " (copy)"), plus any top-level
// fallback `id:`/`name:` for non-DIET plans. Line-based (no YAML lib in
// MoonBit-JS): it rewrites the FIRST 2-space-indented `case_id:`/`name:` (the
// meta block) and a column-0 `id:`/`name:`. Unknown shapes pass through unchanged
// — the server then derives an id, still avoiding silent data loss.
pub fn clone_yaml_identity(yaml : String, suffix : String) -> String {
  let id_suffix = sanitize_id_suffix(suffix)
  let lines = split_lines(yaml)
  let mut did_case_id = false
  let mut did_meta_name = false
  let mut did_top_id = false
  let mut did_top_name = false
  let mut out = ""
  let mut first = true
  for l in lines {
    let mut rewritten = l
    if !did_case_id && starts_with(l, "  case_id:") {
      rewritten = append_value(l, id_suffix)
      did_case_id = true
    } else if !did_meta_name && starts_with(l, "  name:") {
      rewritten = append_value(l, " (copy)")
      did_meta_name = true
    } else if !did_top_id && starts_with(l, "id:") {
      rewritten = append_value(l, id_suffix)
      did_top_id = true
    } else if !did_top_name && starts_with(l, "name:") {
      rewritten = append_value(l, " (copy)")
      did_top_name = true
    }
    if first {
      out = rewritten
      first = false
    } else {
      out = out + "\n" + rewritten
    }
  }
  out
}

// split_lines splits s on '\n' into owned Strings (avoids the StringView APIs).
fn split_lines(s : String) -> Array[String] {
  let lines : Array[String] = []
  let mut cur = ""
  for c in s {
    if c == '\n' {
      lines.push(cur)
      cur = ""
    } else {
      cur = cur + c.to_string()
    }
  }
  lines.push(cur)
  lines
}

// starts_with reports whether s begins with prefix (char-wise; no slicing).
fn starts_with(s : String, prefix : String) -> Bool {
  let sc = chars(s)
  let pc = chars(prefix)
  if sc.length() < pc.length() {
    return false
  }
  let mut i = 0
  while i < pc.length() {
    if sc[i] != pc[i] {
      return false
    }
    i = i + 1
  }
  true
}

// chars collects s into an Array[Char] (`for c in s` yields Char; String indexing
// yields UInt16, so we materialize once for char-wise work).
fn chars(s : String) -> Array[Char] {
  let cs : Array[Char] = []
  for c in s {
    cs.push(c)
  }
  cs
}

// append_value appends extra to a `key: value` line, keeping a trailing carriage
// return (CRLF inputs) after the inserted text.
fn append_value(line : String, extra : String) -> String {
  let cs = chars(line)
  let n = cs.length()
  if n > 0 && cs[n - 1] == '\r' {
    // Rebuild without the trailing CR, append extra, then restore CR.
    let mut body = ""
    let mut i = 0
    while i < n - 1 {
      body = body + cs[i].to_string()
      i = i + 1
    }
    return body + extra + "\r"
  }
  line + extra
}

// sanitize_id_suffix keeps only id-safe chars ([A-Za-z0-9_-]) from a suffix so the
// cloned case_id still matches the store's id pattern.
fn sanitize_id_suffix(s : String) -> String {
  let mut out = ""
  for c in s {
    out = out +
      (match c {
        '-' | '_' => c.to_string()
        _ =>
          if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
            c.to_string()
          } else {
            ""
          }
      })
  }
  out
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

// validating_html is the subtle "validating…" state shown in the live-validation
// region while a debounced dry-run is in flight (P1.3, #68).
pub fn validating_html() -> String {
  "<div class=\"text-muted small d-flex align-items-center\" aria-busy=\"true\">" +
  "<div class=\"spinner-border spinner-border-sm me-2\" aria-hidden=\"true\"></div>validating…</div>"
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

// status_badge maps a plan status to a Bootstrap badge. The badge text always
// carries the literal status, so the cue never relies on color alone (P2.3, #72).
// `archived` (text-bg-dark) gains a border so it is not low-contrast against the
// dark page background.
fn status_badge(status : String) -> String {
  let cls = match status {
    "active" => "text-bg-success"
    "draft" => "text-bg-secondary"
    "archived" => "text-bg-dark border border-secondary"
    _ => "text-bg-info"
  }
  "<span class=\"badge \{cls}\">\{esc(status)}</span>"
}

// status_cue renders a Bootstrap badge that does NOT rely on color alone (P2.3,
// #72): an icon glyph (aria-hidden — decorative) paired with the literal text, so
// the Valid/Invalid/present/none state is legible to colorblind users and to
// screen readers (which read the text, not the color). `cls` is the bg class,
// `glyph` the leading symbol, `text` the spoken label.
fn status_cue(cls : String, glyph : String, text : String) -> String {
  // The label lives in its own span so its text node is EXACTLY `text` (the glyph
  // is a separate aria-hidden span) — exact-text matchers / screen readers see the
  // word, not "✓ Valid".
  "<span class=\"badge \{cls}\"><span aria-hidden=\"true\">\{glyph} </span><span>\{esc(text)}</span></span>"
}

// facts_summary renders the at-a-glance derived facts (P2.1, #71) — topology /
// GPU count / server total / switch total + a validity badge — from the facts
// object on a list row or the detail. When the plan can't be computed
// (structural ingest failure) NONE of the facts are trustworthy, so every field
// shows "—" and the badge reads "not computable" — we never present a partial,
// misleading "mesh · 0 GPU · 0 servers" surface for an uncomputable plan (#71).
fn facts_summary(f : Json) -> String {
  let computable = bool_at(f, "computable")
  if !computable {
    return "<span class=\"text-muted small me-2\">— · — GPU · — servers · — switches</span>" +
      status_cue("text-bg-warning", "⚠", "not computable")
  }
  let topo = str_at(f, "topology")
  let gpu = int_at(f, "gpu_count").to_string()
  let srv = int_at(f, "server_total").to_string()
  let sw = int_at(f, "switch_total").to_string()
  let validity = if bool_at(f, "is_valid") {
    status_cue("text-bg-success", "✓", "Valid")
  } else {
    status_cue("text-bg-danger", "✗", "Invalid")
  }
  "<span class=\"text-muted small me-2\">\{esc(topo)} · \{gpu} GPU · \{srv} servers · \{sw} switches</span>\{validity}"
}

// --- surfaces ---------------------------------------------------------------

// loading_html: a centered spinner panel shown while a navigation request is in
// flight (P1.5, #66), so the list/detail transitions are not silent latency.
pub fn loading_html(label : String) -> String {
  "<div class=\"text-center text-muted py-5\" role=\"status\" aria-live=\"polite\" aria-busy=\"true\">" +
  "<div class=\"spinner-border\" aria-hidden=\"true\"></div>" +
  "<p class=\"mt-3 mb-0\">\{esc(label)}</p></div>"
}

// empty_state_html: the first-run guidance panel shown when no plans exist
// (P1.5, #66) — explains what AID does and offers the New-plan CTA (same
// new-plan-btn id load_plans wires), instead of a bare "0 plan(s)" table.
fn empty_state_html() -> String {
  "<div class=\"d-flex justify-content-between align-items-center mb-3\">" +
  "<h2 class=\"h4 mb-0\">Topology Plans</h2></div>" +
  "<div id=\"empty-state\" class=\"card text-center\"><div class=\"card-body py-5\">" +
  "<h3 class=\"h5\">No topology plans yet</h3>" +
  "<p class=\"text-muted mx-auto\" style=\"max-width: 36rem;\">AID turns a DIET/training topology plan into computed switch &amp; server quantities, a bill of materials, and per-fabric hhfab wiring. Start from a vendored template or paste your own YAML.</p>" +
  "<button id=\"new-plan-btn\" class=\"btn btn-success\">+ New plan</button>" +
  "</div></div>"
}

// plan_list_html: a NetBox-style table of plan summaries with status badges and
// a per-row View action; a first-run empty-state panel when there are no plans.
pub fn plan_list_html(plans_json : String) -> String {
  let root = try {
    @json.parse(plans_json)
  } catch {
    _ => return warn_box("Could not parse plan list.")
  }
  let plans = arr_at(root, "plans")
  if plans.length() == 0 {
    return empty_state_html()
  }
  let mut rows = ""
  for p in plans {
    let id = str_at(p, "id")
    let name = str_at(p, "name")
    let status = str_at(p, "status")
    let summary = match get(p, "facts") {
      Some(f) => facts_summary(f)
      None => ""
    }
    rows = rows +
      // P2.4 clickable row: the row body opens the plan (the View button remains
      // the keyboard-accessible control; on_click_row ignores button clicks).
      "<tr id=\"row-\{esc(id)}\" style=\"cursor:pointer\" title=\"Open \{esc(name)}\">" +
      "<td>\{esc(name)}</td>" +
      "<td><code>\{esc(id)}</code></td>" +
      "<td>\{status_badge(status)}</td>" +
      "<td>\{summary}</td>" +
      "<td class=\"text-end\">" +
      "<button id=\"view-\{esc(id)}\" class=\"btn btn-sm btn-outline-primary me-1\">View</button>" +
      "<button id=\"dup-\{esc(id)}\" class=\"btn btn-sm btn-outline-secondary me-1\">Duplicate</button>" +
      "<button id=\"del-\{esc(id)}\" class=\"btn btn-sm btn-outline-danger\">Delete</button>" +
      "</td>" +
      "</tr>"
  }
  let count = plans.length().to_string()
  "<div class=\"d-flex justify-content-between align-items-center mb-3\">" +
  "<h2 class=\"h4 mb-0\">Topology Plans</h2>" +
  "<div><span class=\"badge text-bg-secondary me-2\">\{count} plan(s)</span>" +
  "<button id=\"new-plan-btn\" class=\"btn btn-sm btn-success\">+ New plan</button></div></div>" +
  "<div id=\"list-error\" class=\"mb-2\" role=\"alert\" aria-live=\"assertive\"></div>" +
  "<table class=\"table table-hover align-middle\">" +
  "<caption class=\"visually-hidden\">Topology plans — \{count} plan(s), with status, derived facts, and per-plan actions</caption>" +
  "<thead><tr><th scope=\"col\">Name</th><th scope=\"col\">ID</th><th scope=\"col\">Status</th><th scope=\"col\">Summary</th><th scope=\"col\" class=\"text-end\">Actions</th></tr></thead>" +
  "<tbody>\{rows}</tbody></table>"
}

// new_plan_form_html: the "New plan" authoring form (P0.2). A name field, a
// template <select> (populated from GET /api/templates), and a raw-YAML textarea.
// "From template" prefills the textarea (handled in app.mbt on select-change);
// "import/paste" just leaves it blank to paste into. Submit POSTs the YAML.
pub fn new_plan_form_html(templates_json : String) -> String {
  let tpls : Array[Json] = try {
    arr_at(@json.parse(templates_json), "templates")
  } catch {
    _ => []
  }
  let mut opts = "<option value=\"\">— Blank (paste your own YAML) —</option>"
  for t in tpls {
    let id = str_at(t, "id")
    let name = str_at(t, "name")
    let topo = str_at(t, "topology")
    opts = opts +
      "<option value=\"\{esc(id)}\">\{esc(name)} (\{esc(topo)})</option>"
  }
  "<div class=\"d-flex justify-content-between align-items-center mb-3\">" +
  "<h2 class=\"h4 mb-0\">New plan</h2>" +
  "<button id=\"new-cancel-btn\" class=\"btn btn-sm btn-outline-secondary\">Cancel</button></div>" +
  "<div id=\"new-error\" class=\"mb-2\" role=\"alert\" aria-live=\"assertive\"></div>" +
  "<div class=\"card\"><div class=\"card-body\">" +
  "<div class=\"mb-3\"><label class=\"form-label\" for=\"new-name\">Name (optional — used as a label)</label>" +
  "<input id=\"new-name\" class=\"form-control\" type=\"text\" placeholder=\"My fabric plan\"></div>" +
  "<div class=\"mb-3\"><label class=\"form-label\" for=\"new-template\">Start from</label>" +
  "<select id=\"new-template\" class=\"form-select\">\{opts}</select>" +
  "<div class=\"form-text\">Pick a starter to prefill the YAML, or leave Blank to paste your own.</div></div>" +
  "<div class=\"mb-3\"><label class=\"form-label\" for=\"new-yaml\">Plan YAML</label>" +
  "<textarea id=\"new-yaml\" class=\"form-control font-monospace\" rows=\"18\" spellcheck=\"false\" " +
  "placeholder=\"meta:\n  case_id: my-plan\n  name: My Plan\n...\"></textarea></div>" +
  "<button id=\"new-submit-btn\" class=\"btn btn-primary\">Create plan</button>" +
  "</div></div>"
}

// library_html: render the read-only Library browse table from the
// GET /api/catalog JSON ({"items":[{id:{name,version},kind,layer,...}]}). The
// built-in Library is the deduped union of the shipped reference templates'
// catalogs (#80). RED stub — #80 GREEN renders the item table.
pub fn library_html(catalog_json : String) -> String {
  let root = try {
    @json.parse(catalog_json)
  } catch {
    _ => return warn_box("Could not parse the library.")
  }
  let items = arr_at(root, "items")
  let mut rows = ""
  for it in items {
    let name = match get(it, "id") {
      Some(idobj) => str_at(idobj, "name")
      None => ""
    }
    let kind = str_at(it, "kind")
    let layer = str_at(it, "layer")
    let manufacturer = str_at(it, "manufacturer")
    let model = str_at(it, "model")
    let part = str_at(it, "part_number")
    rows = rows +
      "<tr id=\"catalog-item-\{esc(name)}\" style=\"cursor:pointer\" title=\"Show \{esc(name)}\">" +
      "<td><code>\{esc(name)}</code></td>" +
      "<td>\{esc(kind)}</td>" +
      "<td>\{esc(layer)}</td>" +
      "<td>\{esc(manufacturer)}</td>" +
      "<td>\{esc(model)}</td>" +
      "<td>\{esc(part)}</td>" +
      "</tr>"
  }
  let count = items.length().to_string()
  "<div class=\"d-flex justify-content-between align-items-center mb-3\">" +
  "<h2 class=\"h4 mb-0\">Library</h2>" +
  "<span class=\"badge text-bg-secondary\">\{count} item(s)</span></div>" +
  "<p class=\"text-secondary\">Reusable inventory — the built-in component catalog shared across the reference topologies. Read-only.</p>" +
  "<div id=\"library-detail\" class=\"mb-3\"></div>" +
  "<table class=\"table table-hover align-middle\">" +
  "<caption class=\"visually-hidden\">Built-in library — \{count} catalog item(s) with kind, layer, and identity</caption>" +
  "<thead><tr><th scope=\"col\">ID</th><th scope=\"col\">Kind</th><th scope=\"col\">Layer</th><th scope=\"col\">Manufacturer</th><th scope=\"col\">Model</th><th scope=\"col\">Part #</th></tr></thead>" +
  "<tbody>\{rows}</tbody></table>"
}

// library_item_html: the detail panel for one catalog item (GET /api/catalog/
// {name}). Shows identity + kind/layer + the descriptive/optic fields. Read-only.
pub fn library_item_html(item_json : String) -> String {
  let it = try {
    @json.parse(item_json)
  } catch {
    _ => return warn_box("Could not parse the library item.")
  }
  let name = match get(it, "id") {
    Some(idobj) => str_at(idobj, "name")
    None => ""
  }
  let version = match get(it, "id") {
    Some(idobj) => str_at(idobj, "version")
    None => ""
  }
  let kind = str_at(it, "kind")
  let layer = str_at(it, "layer")
  let manufacturer = str_at(it, "manufacturer")
  let model = str_at(it, "model")
  let part = str_at(it, "part_number")
  let desc = str_at(it, "description")
  "<div class=\"card\"><div class=\"card-body\">" +
  "<div class=\"d-flex justify-content-between align-items-center mb-2\">" +
  "<h3 class=\"h5 mb-0\"><code>\{esc(name)}</code> <span class=\"text-secondary\">@\{esc(version)}</span></h3>" +
  "<button id=\"library-detail-close\" class=\"btn btn-sm btn-outline-secondary\">Close</button></div>" +
  "<dl class=\"row mb-0\">" +
  "<dt class=\"col-sm-3\">Kind</dt><dd class=\"col-sm-9\">\{esc(kind)}</dd>" +
  "<dt class=\"col-sm-3\">Layer</dt><dd class=\"col-sm-9\">\{esc(layer)}</dd>" +
  "<dt class=\"col-sm-3\">Manufacturer</dt><dd class=\"col-sm-9\">\{esc(manufacturer)}</dd>" +
  "<dt class=\"col-sm-3\">Model</dt><dd class=\"col-sm-9\">\{esc(model)}</dd>" +
  "<dt class=\"col-sm-3\">Part #</dt><dd class=\"col-sm-9\">\{esc(part)}</dd>" +
  "<dt class=\"col-sm-3\">Description</dt><dd class=\"col-sm-9\">\{esc(desc)}</dd>" +
  "</dl></div></div>"
}

// reference_gallery_html: render the reference-topology gallery from the
// GET /api/templates JSON. Each card carries a "Use as starting point" control
// (id "use-template-<id>") that reuses the existing template clone chain. RED
// stub — #80 GREEN renders the gallery cards.
pub fn reference_gallery_html(templates_json : String) -> String {
  let root = try {
    @json.parse(templates_json)
  } catch {
    _ => return warn_box("Could not parse reference topologies.")
  }
  let tpls = arr_at(root, "templates")
  let mut cards = ""
  for t in tpls {
    let id = str_at(t, "id")
    let name = str_at(t, "name")
    let topo = str_at(t, "topology")
    let desc = str_at(t, "description")
    cards = cards +
      "<div class=\"col-md-6 col-lg-4 mb-3\"><div class=\"card h-100\"><div class=\"card-body d-flex flex-column\">" +
      "<h3 class=\"h6 card-title\">\{esc(name)}</h3>" +
      "<span class=\"badge text-bg-info align-self-start mb-2\">\{esc(topo)}</span>" +
      "<p class=\"card-text small text-secondary flex-grow-1\">\{esc(desc)}</p>" +
      "<button id=\"use-template-\{esc(id)}\" class=\"btn btn-sm btn-primary mt-2\">Use as starting point</button>" +
      "</div></div></div>"
  }
  "<div class=\"d-flex justify-content-between align-items-center mb-3\">" +
  "<h2 class=\"h4 mb-0\">Reference topologies</h2></div>" +
  "<p class=\"text-secondary\">Built-in reference designs. \"Use as starting point\" creates your own editable plan (a detached copy — no link back).</p>" +
  "<div id=\"reference-error\" class=\"mb-2\" role=\"alert\" aria-live=\"assertive\"></div>" +
  "<div class=\"row\">\{cards}</div>"
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
  let header_facts = match get(root, "facts") {
    Some(f) => "<div id=\"detail-facts\" class=\"mb-3\">\{facts_summary(f)}</div>"
    None => ""
  }
  "<nav aria-label=\"breadcrumb\"><ol class=\"breadcrumb\">" +
  "<li class=\"breadcrumb-item\"><button id=\"crumb-plans\" class=\"btn btn-link p-0 align-baseline\">Plans</button></li>" +
  "<li class=\"breadcrumb-item active\" aria-current=\"page\">\{esc(name)}</li>" +
  "</ol></nav>" +
  "<div class=\"d-flex justify-content-between align-items-center mb-2\">" +
  "<h2 class=\"h4 mb-0\">\{esc(name)}</h2>\{status_badge(status)}</div>" +
  header_facts +
  "<div id=\"live-validation\" class=\"mb-3\" role=\"status\" aria-live=\"polite\"></div>" +
  "<div class=\"row g-3\">" +
  "<div class=\"col-md-4\"><div class=\"card\">" +
  "<div class=\"card-header\">Plan</div><div class=\"card-body\">" +
  "<dl class=\"row mb-3\">" +
  "<dt class=\"col-5\">ID</dt><dd class=\"col-7\"><code>\{esc(id)}</code></dd>" +
  "<dt class=\"col-5\">Status</dt><dd class=\"col-7\">\{status_badge(status)}</dd></dl>" +
  // P2.4: the plan actions grouped in one toolbar with a single clear primary
  // (Calculate). Calculate + View BOM read the plan; Delete is separated.
  "<div class=\"btn-toolbar mb-2\" role=\"toolbar\" aria-label=\"Plan actions\">" +
  "<div class=\"btn-group btn-group-sm me-2 mb-1\" role=\"group\" aria-label=\"Compute\">" +
  "<button id=\"calc-btn\" class=\"btn btn-primary\">Calculate</button>" +
  "<button id=\"bom-btn\" class=\"btn btn-outline-secondary\">View BOM</button>" +
  "</div>" +
  "<div class=\"btn-group btn-group-sm mb-1\" role=\"group\" aria-label=\"Manage\">" +
  "<button id=\"detail-del-btn\" class=\"btn btn-outline-danger\">Delete</button>" +
  "</div></div>" +
  "<button id=\"back-btn\" class=\"btn btn-link btn-sm p-0\">&larr; All plans</button>" +
  "</div></div></div>" +
  "<div class=\"col-md-8\"><div class=\"card\">" +
  "<div class=\"card-header d-flex justify-content-between align-items-center\">" +
  "<span>Plan YAML (editable)</span>" +
  "<button id=\"save-btn\" class=\"btn btn-success btn-sm\">Save</button></div>" +
  "<div class=\"card-body\">" +
  "<div id=\"edit-error\" class=\"mb-2\" role=\"alert\" aria-live=\"assertive\"></div>" +
  // Canonical stored plan YAML (immutable carrier): the structured live-validate
  // applies form ops to THIS base, never the editable textarea (which may hold
  // independent raw edits) — devb #68 finding 1.
  "<textarea id=\"stored-plan-yaml\" class=\"d-none\">\{esc(yaml)}</textarea>" +
  "<textarea id=\"edit-yaml\" class=\"form-control font-monospace\" rows=\"22\" spellcheck=\"false\">\{esc(yaml)}</textarea>" +
  "<div class=\"form-text\">Edit the raw plan YAML and Save, then Calculate to see the change reflected.</div>" +
  "</div></div></div></div>" +
  "<div id=\"structure-editor\" class=\"mt-3\"></div>" +
  "<div id=\"overlay-section\" class=\"mt-3\"></div>" +
  "<div id=\"detail-result\" class=\"mt-3\" role=\"status\" aria-live=\"polite\"></div>"
}

// overlay_section_html: the optic/identity overlay editor (P1.4, #70). Presence
// is always visible (BOM identity depends on it) — a present/none badge — with a
// raw-YAML textarea (v1) + Save. present prefills the textarea with the current
// overlay; absent shows an empty box to author one.
pub fn overlay_section_html(present : Bool, overlay_yaml : String) -> String {
  let badge = if present {
    status_cue("text-bg-success", "✓", "present")
  } else {
    status_cue("text-bg-secondary", "○", "none")
  }
  "<div class=\"card\"><div class=\"card-header d-flex justify-content-between align-items-center\">" +
  "<span>Optic / identity overlay \{badge}</span>" +
  "<button id=\"overlay-save-btn\" class=\"btn btn-success btn-sm\">Save overlay</button></div>" +
  "<div class=\"card-body\">" +
  "<div id=\"overlay-error\" class=\"mb-2\" role=\"alert\" aria-live=\"assertive\"></div>" +
  "<div class=\"form-text mb-2\">The overlay supplies optic SKUs/identity — without it the BOM's optic columns are blank.</div>" +
  "<textarea id=\"overlay-yaml\" class=\"form-control font-monospace\" rows=\"14\" spellcheck=\"false\" " +
  "placeholder=\"items:\n  - id: { name: my-optic, version: '1' }\n    ...\">\{esc(overlay_yaml)}</textarea>" +
  "</div></div>"
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
  "<table class=\"table table-sm mb-3\">" +
  "<caption class=\"visually-hidden\">\{esc(title)} per device class</caption>" +
  "<thead><tr><th scope=\"col\">Class</th><th scope=\"col\" class=\"text-end\">Qty</th></tr></thead>" +
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
    status_cue("text-bg-success", "✓", "Valid")
  } else {
    status_cue("text-bg-danger", "✗", "Invalid")
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
      // optic identity (cols 7–19) — blank until the optic overlay is attached
      // (P1.4, #70); surfaces whether the BOM carries real optical SKUs.
      "<td>\{esc(str_at(r, "standard"))}</td>" +
      "</tr>"
  }
  let suppressed = int_at(root, "suppressed_cable_assembly_count").to_string()
  "<div class=\"d-flex justify-content-between align-items-center\">" +
  "<h3 class=\"h5 mb-0\">Bill of Materials</h3>" +
  "<button id=\"bom-csv-btn\" class=\"btn btn-outline-secondary btn-sm\">Download BOM (CSV)</button></div>" +
  "<table class=\"table table-sm mt-2\">" +
  "<caption class=\"visually-hidden\">Line items: one row per device and optic, with section, model, class, manufacturer, and quantity</caption>" +
  "<thead><tr><th scope=\"col\">Section</th><th scope=\"col\">Model</th><th scope=\"col\">Class</th><th scope=\"col\">Manufacturer</th><th scope=\"col\" class=\"text-end\">Qty</th><th scope=\"col\">Optic standard</th></tr></thead>" +
  "<tbody>\{rows}</tbody></table>" +
  "<p class=\"text-muted small mb-0\">Suppressed cable assemblies: \{suppressed}</p>"
}

// --- P1.1 (#67): structured editor for server & switch classes --------------
// Renders data-derived forms from the GET /structure projection (D26). Dropdowns
// are sourced from the projection's catalog id lists, never hardcoded. The raw
// JSON is stashed in a hidden #structure-data input so the save handlers can
// re-read the id list to collect each field's value.

fn jval(j : Json) -> String {
  match j {
    String(s) => s
    _ => ""
  }
}

// json_escape escapes a value for embedding in a JSON string literal (the save
// handlers build the ops body by hand).
pub fn json_escape(s : String) -> String {
  let mut out = ""
  for c in s {
    out = out +
      (match c {
        '\\' => "\\\\"
        '"' => "\\\""
        '\n' => "\\n"
        '\r' => "\\r"
        '\t' => "\\t"
        _ => c.to_string()
      })
  }
  out
}

// select_html renders a <select> over option id strings, marking `selected`. The
// aria_label gives the control a programmatic name (P2.3, #72) — table-embedded
// selects have no associated <label>, so a column header is not enough.
fn select_html(
  id : String,
  options : Array[Json],
  selected : String,
  blank : Bool,
  aria_label : String
) -> String {
  let mut opts = if blank { "<option value=\"\"></option>" } else { "" }
  for o in options {
    let v = jval(o)
    let sel = if v == selected { " selected" } else { "" }
    opts = opts + "<option value=\"\{esc(v)}\"\{sel}>\{esc(v)}</option>"
  }
  "<select id=\"\{esc(id)}\" class=\"form-select form-select-sm\" aria-label=\"\{esc(aria_label)}\">\{opts}</select>"
}

// select_strings renders a <select> over plain string options, marking selected.
fn select_strings(
  id : String,
  options : Array[String],
  selected : String,
  aria_label : String
) -> String {
  let mut opts = ""
  for v in options {
    let sel = if v == selected { " selected" } else { "" }
    opts = opts + "<option value=\"\{esc(v)}\"\{sel}>\{esc(v)}</option>"
  }
  "<select id=\"\{esc(id)}\" class=\"form-select form-select-sm\" aria-label=\"\{esc(aria_label)}\">\{opts}</select>"
}

// topo_select renders the explicit mesh|clos selector (the headline P1.1/P1.2
// capability), preserving the current value.
fn topo_select(id : String, current : String, aria_label : String) -> String {
  let opt = fn(v : String) -> String {
    let sel = if v == current { " selected" } else { "" }
    "<option value=\"\{v}\"\{sel}>\{v}</option>"
  }
  let blank = if current == "" { "<option value=\"\" selected></option>" } else {
      ""
    }
  "<select id=\"\{esc(id)}\" class=\"form-select form-select-sm\" aria-label=\"\{esc(aria_label)}\">\{blank}\{opt("mesh")}\{opt("clos")}</select>"
}

fn override_value(sw : Json) -> String {
  match get(sw, "override_quantity") {
    Some(Number(n, ..)) => n.to_int().to_string()
    _ => ""
  }
}

// structure_editor_html renders the structured editor (server + switch class
// forms + an add-server-class form) from the /structure projection JSON.
pub fn structure_editor_html(structure_json : String) -> String {
  let root = try {
    @json.parse(structure_json)
  } catch {
    _ => return warn_box("Could not parse plan structure.")
  }
  let module_types = match get(root, "catalog") {
    Some(c) => arr_at(c, "module_types")
    None => []
  }
  let device_type_extensions = match get(root, "catalog") {
    Some(c) => arr_at(c, "device_type_extensions")
    None => []
  }
  let device_types = match get(root, "catalog") {
    Some(c) => arr_at(c, "device_types")
    None => []
  }
  let target_zones = match get(root, "catalog") {
    Some(c) => arr_at(c, "target_zones")
    None => []
  }
  let breakout_options = match get(root, "catalog") {
    Some(c) => arr_at(c, "breakout_options")
    None => []
  }
  // Class-id lists (parents for the add-zone / add-nic mini-forms).
  let switch_class_ids : Array[String] = []
  for s in arr_at(root, "switch_classes") {
    switch_class_ids.push(str_at(s, "id"))
  }
  let server_class_ids : Array[String] = []
  for sc in arr_at(root, "server_classes") {
    server_class_ids.push(str_at(sc, "id"))
  }

  // server classes
  let mut srv = ""
  for sc in arr_at(root, "server_classes") {
    let id = str_at(sc, "id")
    let qty = int_at(sc, "quantity").to_string()
    let gpus = int_at(sc, "gpus_per_server").to_string()
    let devtype = str_at(sc, "server_device_type")
    let mut nic_rows = ""
    let nic_ids : Array[String] = []
    for n in arr_at(sc, "nics") {
      let nicid = str_at(n, "nic_id")
      nic_ids.push(nicid)
      let mt = str_at(n, "module_type")
      nic_rows = nic_rows +
        "<div class=\"row g-2 align-items-center mb-1\">" +
        "<div class=\"col-4\"><code>\{esc(nicid)}</code></div>" +
        "<div class=\"col-8\">\{select_html("nic-" + id + "-" + nicid, module_types, mt, false, "Module type for NIC " + nicid + " on " + id)}</div></div>"
    }
    // connections (P1.1b, #69): each row keyed by its server_connections index;
    // target_zone + nic + distribution are data-derived dropdowns.
    let dist_opts = ["same-switch", "alternating", "rail-optimized"]
    let mut conn_rows = ""
    for cn in arr_at(sc, "connections") {
      let ci = int_at(cn, "index").to_string()
      let cid = str_at(cn, "connection_id")
      conn_rows = conn_rows +
        "<tr>" +
        "<td class=\"text-nowrap\"><code>\{esc(cid)}</code></td>" +
        "<td>\{select_html("conn-" + ci + "-target_zone", target_zones, str_at(cn, "target_zone"), false, "Target zone for connection " + cid)}</td>" +
        "<td>\{select_strings("conn-" + ci + "-nic", nic_ids, str_at(cn, "nic"), "NIC for connection " + cid)}</td>" +
        "<td>\{select_strings("conn-" + ci + "-distribution", dist_opts, str_at(cn, "distribution"), "Distribution for connection " + cid)}</td>" +
        "<td><input id=\"conn-\{ci}-speed\" class=\"form-control form-control-sm\" type=\"number\" aria-label=\"Speed for connection \{esc(cid)}\" value=\"\{int_at(cn, "speed").to_string()}\"></td>" +
        "<td><input id=\"conn-\{ci}-ports\" class=\"form-control form-control-sm\" type=\"number\" min=\"1\" aria-label=\"Ports for connection \{esc(cid)}\" value=\"\{int_at(cn, "ports_per_connection").to_string()}\"></td>" +
        "<td><button id=\"conn-rm-\{ci}\" class=\"btn btn-outline-danger btn-sm\" aria-label=\"Remove connection \{esc(cid)}\"><span aria-hidden=\"true\">✕</span></button></td>" +
        "</tr>"
    }
    let conn_table = if arr_at(sc, "connections").length() > 0 {
      "<table class=\"table table-sm align-middle mb-2\">" +
      "<caption class=\"visually-hidden\">Server connections</caption><thead><tr>" +
      "<th scope=\"col\">ID</th><th scope=\"col\">Target zone</th><th scope=\"col\">NIC</th><th scope=\"col\">Distribution</th><th scope=\"col\">Speed</th><th scope=\"col\">Ports</th><th scope=\"col\"><span class=\"visually-hidden\">Actions</span></th>" +
      "</tr></thead><tbody>\{conn_rows}</tbody></table>"
    } else {
      "<div class=\"small text-muted mb-2\">No connections.</div>"
    }
    let add_conn = "<div class=\"row g-2 mb-1 align-items-center\">" +
      "<div class=\"col-3\"><input id=\"addconn-\{esc(id)}-id\" class=\"form-control form-control-sm\" placeholder=\"connection_id\" aria-label=\"New connection id for \{esc(id)}\"></div>" +
      "<div class=\"col-4\">\{select_html("addconn-" + id + "-target_zone", target_zones, "", true, "Target zone for new connection on " + id)}</div>" +
      "<div class=\"col-2\">\{select_strings("addconn-" + id + "-nic", nic_ids, "", "NIC for new connection on " + id)}</div>" +
      "<div class=\"col-2\"><input id=\"addconn-\{esc(id)}-speed\" class=\"form-control form-control-sm\" type=\"number\" placeholder=\"speed\" aria-label=\"Speed for new connection on \{esc(id)}\"></div>" +
      "<div class=\"col-1\"><button id=\"addconn-\{esc(id)}\" class=\"btn btn-outline-success btn-sm\" aria-label=\"Add connection to \{esc(id)}\"><span aria-hidden=\"true\">+</span></button></div>" +
      "</div>"
    srv = srv +
      "<div class=\"card mb-2\"><div class=\"card-body\">" +
      "<div class=\"fw-semibold mb-2\">\{esc(id)}</div>" +
      "<div class=\"row g-2 mb-2\">" +
      "<div class=\"col-6\"><label class=\"form-label mb-0 small\" for=\"srv-\{esc(id)}-qty\">Quantity</label>" +
      "<input id=\"srv-\{esc(id)}-qty\" class=\"form-control form-control-sm\" type=\"number\" min=\"0\" value=\"\{qty}\"></div>" +
      "<div class=\"col-6\"><label class=\"form-label mb-0 small\" for=\"srv-\{esc(id)}-gpus\">GPUs/server</label>" +
      "<input id=\"srv-\{esc(id)}-gpus\" class=\"form-control form-control-sm\" type=\"number\" min=\"0\" value=\"\{gpus}\"></div></div>" +
      "<div class=\"mb-2\"><label class=\"form-label mb-0 small\" for=\"srv-\{esc(id)}-devtype\">Device type</label>" +
      "\{select_html("srv-" + id + "-devtype", device_types, devtype, false, "Device type for " + id)}</div>" +
      "<div class=\"small text-muted\">NICs (module type)</div>\{nic_rows}" +
      "<hr class=\"my-2\"><div class=\"small text-muted\">Connections</div>\{conn_table}\{add_conn}" +
      "</div></div>"
  }

  // switch classes
  let mut sw = ""
  for s in arr_at(root, "switch_classes") {
    let id = str_at(s, "id")
    let topo = str_at(s, "topology_mode")
    let devext = str_at(s, "device_type_extension")
    let ov = override_value(s)
    sw = sw +
      "<div class=\"card mb-2\"><div class=\"card-body\">" +
      "<div class=\"fw-semibold mb-2\">\{esc(id)}</div>" +
      "<div class=\"row g-2\">" +
      "<div class=\"col-4\"><label class=\"form-label mb-0 small\" for=\"sw-\{esc(id)}-topo\">Topology</label>\{topo_select("sw-" + id + "-topo", topo, "Topology mode for " + id)}</div>" +
      "<div class=\"col-5\"><label class=\"form-label mb-0 small\" for=\"sw-\{esc(id)}-devext\">Device type</label>\{select_html("sw-" + id + "-devext", device_type_extensions, devext, false, "Device type for " + id)}</div>" +
      "<div class=\"col-3\"><label class=\"form-label mb-0 small\" for=\"sw-\{esc(id)}-override\">Override qty</label>" +
      "<input id=\"sw-\{esc(id)}-override\" class=\"form-control form-control-sm\" type=\"number\" min=\"1\" placeholder=\"(derived)\" value=\"\{esc(ov)}\"></div>" +
      "</div></div></div>"
  }

  "<div class=\"card\"><div class=\"card-header d-flex justify-content-between align-items-center\">" +
  "<span>Structured editor</span></div><div class=\"card-body\">" +
  "<input type=\"hidden\" id=\"structure-data\" value=\"\{esc(structure_json)}\">" +
  "<div id=\"structure-error\" class=\"mb-2\" role=\"alert\" aria-live=\"assertive\"></div>" +
  "<h3 class=\"h6\">Server classes</h3>\{srv}" +
  "<div class=\"card mb-3\"><div class=\"card-body\">" +
  "<div class=\"small text-muted mb-2\" id=\"add-srv-label\">Add server class</div>" +
  "<div class=\"row g-2\">" +
  "<div class=\"col-3\"><input id=\"add-srv-id\" class=\"form-control form-control-sm\" placeholder=\"new_class_id\" aria-label=\"New server class id\"></div>" +
  "<div class=\"col-2\"><input id=\"add-srv-qty\" class=\"form-control form-control-sm\" type=\"number\" min=\"1\" value=\"1\" aria-label=\"New server class quantity\"></div>" +
  "<div class=\"col-2\"><input id=\"add-srv-gpus\" class=\"form-control form-control-sm\" type=\"number\" min=\"0\" value=\"0\" aria-label=\"New server class GPUs per server\"></div>" +
  "<div class=\"col-3\">\{select_html("add-srv-devtype", device_types, "", true, "New server class device type")}</div>" +
  "<div class=\"col-2\"><button id=\"add-srv-btn\" class=\"btn btn-sm btn-outline-success\">Add</button></div>" +
  "</div></div></div>" +
  "<button id=\"save-srv-btn\" class=\"btn btn-primary btn-sm me-2 mb-3\">Save server classes</button>" +
  "<button id=\"save-conn-btn\" class=\"btn btn-primary btn-sm mb-3\">Save connections</button>" +
  "<h3 class=\"h6\">Switch classes</h3>\{sw}" +
  "\{add_switch_class_form(device_type_extensions)}" +
  "<button id=\"save-sw-btn\" class=\"btn btn-primary btn-sm\">Save switch classes</button>" +
  "<hr class=\"my-3\"><h3 class=\"h6\">Switch port zones</h3>" +
  "\{add_zone_form(switch_class_ids, breakout_options, module_types)}" +
  "<hr class=\"my-3\"><h3 class=\"h6\">NICs</h3>" +
  "\{add_nic_form(server_class_ids, module_types)}" +
  "</div></div>"
}

// add_switch_class_form: the #81 mini-form for a new switch class — id +
// fabric_name inputs, fabric_class / hedgehog_role / device_type_extension /
// topology selectors (data-derived), optional override quantity.
fn add_switch_class_form(device_type_extensions : Array[Json]) -> String {
  let fabric_classes = ["managed", "unmanaged"]
  let roles = ["server-leaf", "border-leaf", "spine", "virtual"]
  "<div class=\"card mb-3\"><div class=\"card-body\">" +
  "<div class=\"small text-muted mb-2\">Add switch class</div>" +
  "<div class=\"row g-2 align-items-end\">" +
  "<div class=\"col-3\"><input id=\"add-swc-id\" class=\"form-control form-control-sm\" placeholder=\"new_switch_class_id\" aria-label=\"New switch class id\"></div>" +
  "<div class=\"col-3\"><input id=\"add-swc-fabric-name\" class=\"form-control form-control-sm\" placeholder=\"fabric_name\" aria-label=\"New switch class fabric name\"></div>" +
  "<div class=\"col-2\">\{select_strings("add-swc-fabric-class", fabric_classes, "managed", "New switch class fabric class")}</div>" +
  "<div class=\"col-2\">\{select_strings("add-swc-role", roles, "server-leaf", "New switch class hedgehog role")}</div>" +
  "</div><div class=\"row g-2 mt-1 align-items-end\">" +
  "<div class=\"col-4\">\{select_html("add-swc-devext", device_type_extensions, "", true, "New switch class device type extension")}</div>" +
  "<div class=\"col-3\">\{topo_select("add-swc-topo", "", "New switch class topology mode")}</div>" +
  "<div class=\"col-3\"><input id=\"add-swc-override\" class=\"form-control form-control-sm\" type=\"number\" min=\"1\" placeholder=\"override qty (opt)\" aria-label=\"New switch class override quantity\"></div>" +
  "<div class=\"col-2\"><button id=\"add-swc-btn\" class=\"btn btn-sm btn-outline-success\">Add</button></div>" +
  "</div></div></div>"
}

// add_zone_form: the #81 mini-form for a new switch port zone — parent
// switch_class + zone_name + zone_type + port_spec, optional breakout /
// transceiver (data-derived) / allocation strategy / priority.
fn add_zone_form(
  switch_class_ids : Array[String],
  breakout_options : Array[Json],
  module_types : Array[Json]
) -> String {
  let zone_types = ["server", "uplink", "mesh", "peer", "session", "oob"]
  let allocs = ["sequential", "interleaved", "custom"]
  "<div class=\"card mb-2\"><div class=\"card-body\">" +
  "<div class=\"small text-muted mb-2\">Add switch port zone</div>" +
  "<div class=\"row g-2 align-items-end\">" +
  "<div class=\"col-3\">\{select_strings("add-zone-swc", switch_class_ids, "", "Parent switch class for new zone")}</div>" +
  "<div class=\"col-3\"><input id=\"add-zone-name\" class=\"form-control form-control-sm\" placeholder=\"zone_name\" aria-label=\"New zone name\"></div>" +
  "<div class=\"col-2\">\{select_strings("add-zone-type", zone_types, "server", "New zone type")}</div>" +
  "<div class=\"col-2\"><input id=\"add-zone-portspec\" class=\"form-control form-control-sm\" placeholder=\"port_spec\" aria-label=\"New zone port spec\"></div>" +
  "</div><div class=\"row g-2 mt-1 align-items-end\">" +
  "<div class=\"col-3\">\{select_html("add-zone-breakout", breakout_options, "", true, "New zone breakout option")}</div>" +
  "<div class=\"col-3\">\{select_html("add-zone-xcvr", module_types, "", true, "New zone transceiver module type")}</div>" +
  "<div class=\"col-2\">\{select_strings("add-zone-alloc", allocs, "sequential", "New zone allocation strategy")}</div>" +
  "<div class=\"col-2\"><input id=\"add-zone-priority\" class=\"form-control form-control-sm\" type=\"number\" placeholder=\"priority\" aria-label=\"New zone priority\"></div>" +
  "<div class=\"col-2\"><button id=\"add-zone-btn\" class=\"btn btn-sm btn-outline-success\">Add</button></div>" +
  "</div></div></div>"
}

// add_nic_form: the #81 mini-form for a new server NIC — parent server_class +
// nic_id + module_type (data-derived).
fn add_nic_form(server_class_ids : Array[String], module_types : Array[Json]) -> String {
  "<div class=\"card mb-2\"><div class=\"card-body\">" +
  "<div class=\"small text-muted mb-2\">Add NIC</div>" +
  "<div class=\"row g-2 align-items-end\">" +
  "<div class=\"col-4\">\{select_strings("add-nic-server", server_class_ids, "", "Parent server class for new NIC")}</div>" +
  "<div class=\"col-3\"><input id=\"add-nic-id\" class=\"form-control form-control-sm\" placeholder=\"nic_id\" aria-label=\"New NIC id\"></div>" +
  "<div class=\"col-3\">\{select_html("add-nic-module", module_types, "", true, "New NIC module type")}</div>" +
  "<div class=\"col-2\"><button id=\"add-nic-btn\" class=\"btn btn-sm btn-outline-success\">Add</button></div>" +
  "</div></div></div>"
}
