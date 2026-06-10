// app.mbt — exported entry points + event wiring. These are the stable public
// surface (moon.pkg.json `link.js.exports`); the HTML shell and the Node smoke
// tests call only these names. Internal symbols are name-mangled and must not be
// referenced.
//
// RED phase (issue #11, Stage B): the render builders (render.mbt) return "",
// so the render_* entry points and the fetch-then-render flows produce no
// markup; download_wiring issues no request. The Node smoke tests therefore
// fail (no markup / no request) — the UI is not implemented yet. The ffi +
// request plumbing below is real (spike-proven); GREEN fills in render.mbt.

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

// load_plans: GET /api/plans, then render the list into target.
pub fn load_plans(target : String) -> Unit {
  api_get(plans_path(), fn(body) { set_html(target, plan_list_html(body)) })
}

// trigger_calc: POST /api/plans/{id}/calc, then render the validation summary.
pub fn trigger_calc(target : String, plan_id : String) -> Unit {
  api_post(calc_path(plan_id), "{}", fn(body) {
    set_html(target, calc_summary_html(body))
  })
}

// download_wiring: GET /api/plans/{id}/wiring/{fabric}, then save the YAML.
pub fn download_wiring(plan_id : String, fabric : String) -> Unit {
  // RED stub: not wired yet. GREEN issues the request and calls save_file.
  ignore(api_base + wiring_path(plan_id, fabric))
  ignore(save_file)
}

// main_entry: bootstrap the app (GREEN: render the navbar and load the plan
// list into #app).
pub fn main_entry() -> Unit {
  console_log("AID UI starting")
  // RED stub: does not load anything yet.
  ignore(get_value)
  ignore(on_click)
}
