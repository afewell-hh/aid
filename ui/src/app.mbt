// app.mbt — exported entry points + event wiring. These are the stable public
// surface (moon.pkg.json `link.js.exports`); the HTML shell and the Node smoke
// tests call only these names. Internal symbols are name-mangled and must not be
// referenced.
//
// The flow: main_entry -> load_plans (#app); each row's View wires to load_detail
// (#app); the detail's Calculate/View-BOM buttons render into #detail-result.
// Everything calls only the Stage-A REST API (no new core behavior).

// render_plan_list: render a plan-list table from JSON into the target element.
pub fn render_plan_list(target : String, plans_json : String) -> Unit {
  set_html(target, plan_list_html(plans_json))
}

// render_plan_detail: render the plan-detail cards from JSON.
pub fn render_plan_detail(target : String, detail_json : String) -> Unit {
  set_html(target, plan_detail_html(detail_json))
}

// render_bom: render the BOM view (per-unit + fleet) from JSON.
pub fn render_bom(target : String, bom_json : String) -> Unit {
  set_html(target, bom_html(bom_json))
}

// load_plans: GET /api/plans, render the list, and wire each row's View button.
// A non-2xx response or a network failure (status==0) renders the shared error
// alert instead of a misleading empty table (P0.4).
pub fn load_plans(target : String) -> Unit {
  api_get(plans_path(), fn(ok, status, body) {
    if (!ok) || body_has_error(body) {
      set_html(target, error_html(status, body))
      return
    }
    set_html(target, plan_list_html(body))
    wire_plan_rows(body)
  })
}

// wire_plan_rows attaches a click handler to each rendered plan row.
fn wire_plan_rows(plans_json : String) -> Unit {
  let root = try {
    @json.parse(plans_json)
  } catch {
    _ => return
  }
  for p in arr_at(root, "plans") {
    let id = str_at(p, "id")
    on_click("view-" + id, fn() { load_detail(id) })
  }
}

// load_detail: GET /api/plans/{id}, render the detail, wire its action buttons.
// A 404 (deleted/unknown plan) or network failure renders the error alert, not a
// ghost detail card with live-but-broken buttons (P0.4).
fn load_detail(id : String) -> Unit {
  api_get(plan_path(id), fn(ok, status, body) {
    if (!ok) || body_has_error(body) {
      set_html("app", error_html(status, body))
      return
    }
    set_html("app", plan_detail_html(body))
    on_click("calc-btn", fn() { trigger_calc("detail-result", id) })
    on_click("bom-btn", fn() { load_bom("detail-result", id) })
  })
}

// trigger_calc: POST /api/plans/{id}/calc, render the validation summary, and
// wire per-fabric wiring-download buttons from the response's managed_fabrics.
// Two-plane contract (P0.4): a structural 4xx {"error":...} / network failure is
// an ERROR (error_html); a 200 is_valid:false is calc-as-DATA (the Invalid badge
// via calc_summary_html), NOT an error.
pub fn trigger_calc(target : String, plan_id : String) -> Unit {
  set_inflight("calc-btn", true, "Calculating…")
  api_post(calc_path(plan_id), "{}", fn(ok, status, body) {
    set_inflight("calc-btn", false, "Calculate")
    // A non-2xx is a structural failure (an error). A 200 body — even an
    // {"error":...} structural body that came back 200 — is handled by
    // calc_summary_html (which already distinguishes error vs calc-data).
    if !ok {
      set_html(target, error_html(status, body))
      return
    }
    set_html(target, calc_summary_html(body))
    wire_wiring_buttons(plan_id, body)
  })
}

// wire_wiring_buttons binds each rendered per-fabric download button (id
// "wiring-<fabric>") to download_wiring(plan_id, fabric). Names come from the
// server-derived managed_fabrics list — never guessed.
fn wire_wiring_buttons(plan_id : String, calc_body : String) -> Unit {
  for fabric in managed_fabrics(calc_body) {
    on_click("wiring-" + fabric, fn() { download_wiring(plan_id, fabric) })
  }
}

// load_bom: GET /api/plans/{id}/bom, render the BOM. A 422 (calc-gated) or other
// non-2xx renders the error alert, not a misleading empty BOM (P0.4).
fn load_bom(target : String, plan_id : String) -> Unit {
  set_inflight("bom-btn", true, "Loading…")
  api_get(bom_path(plan_id), fn(ok, status, body) {
    set_inflight("bom-btn", false, "View BOM")
    if (!ok) || body_has_error(body) {
      set_html(target, error_html(status, body))
      return
    }
    set_html(target, bom_html(body))
  })
}

// download_wiring: GET /api/plans/{id}/wiring/{fabric}, save the YAML locally.
// GUARD (P0.4): a non-OK response (e.g. 404 bad fabric, 422 calc-gated) or an
// error body is NEVER written to a .yaml file — it surfaces the error alert in
// #detail-result instead, so the user never gets a file full of {"error":...}.
pub fn download_wiring(plan_id : String, fabric : String) -> Unit {
  let btn = "wiring-" + fabric
  set_inflight(btn, true, "Downloading…")
  api_get(wiring_path(plan_id, fabric), fn(ok, status, body) {
    set_inflight(btn, false, "Download wiring: " + fabric)
    if (!ok) || body_has_error(body) {
      set_html("detail-result", error_html(status, body))
      return
    }
    save_file(plan_id + "-" + fabric + ".yaml", body)
  })
}

// set_inflight toggles a button's disabled state + label for async feedback
// (P0.4): disabled while a request is outstanding (no double-fire), with a
// transient label so latency is not silent.
fn set_inflight(id : String, busy : Bool, label : String) -> Unit {
  set_disabled(id, busy)
  set_text(id, label)
}

// main_entry: bootstrap the app — load the plan list into #app.
pub fn main_entry() -> Unit {
  console_log("AID UI starting")
  load_plans("app")
}
