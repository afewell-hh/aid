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
pub fn load_plans(target : String) -> Unit {
  api_get(plans_path(), fn(body) {
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
fn load_detail(id : String) -> Unit {
  api_get(plan_path(id), fn(body) {
    set_html("app", plan_detail_html(body))
    on_click("calc-btn", fn() { trigger_calc("detail-result", id) })
    on_click("bom-btn", fn() { load_bom("detail-result", id) })
  })
}

// trigger_calc: POST /api/plans/{id}/calc, render the validation summary.
pub fn trigger_calc(target : String, plan_id : String) -> Unit {
  api_post(calc_path(plan_id), "{}", fn(body) {
    set_html(target, calc_summary_html(body))
  })
}

// load_bom: GET /api/plans/{id}/bom, render the per-unit/fleet BOM.
fn load_bom(target : String, plan_id : String) -> Unit {
  api_get(bom_path(plan_id), fn(body) { set_html(target, bom_html(body)) })
}

// download_wiring: GET /api/plans/{id}/wiring/{fabric}, save the YAML locally.
pub fn download_wiring(plan_id : String, fabric : String) -> Unit {
  api_get(wiring_path(plan_id, fabric), fn(body) {
    save_file(plan_id + "-" + fabric + ".yaml", body)
  })
}

// main_entry: bootstrap the app — load the plan list into #app.
pub fn main_entry() -> Unit {
  console_log("AID UI starting")
  load_plans("app")
}
