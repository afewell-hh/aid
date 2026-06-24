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

// load_plans: GET /api/plans, render the list, and wire each row's actions
// (View / Duplicate / Delete) plus the "New plan" affordance. A non-2xx response
// or a network failure (status==0) renders the shared error alert instead of a
// misleading empty table (P0.4).
pub fn load_plans(target : String) -> Unit {
  set_html(target, loading_html("Loading plans…")) // P1.5: nav not silent
  api_get(plans_path(), fn(ok, status, body) {
    if (!ok) || body_has_error(body) {
      set_html(target, error_html(status, body))
      return
    }
    set_html(target, plan_list_html(body))
    wire_plan_rows(body)
    on_click("new-plan-btn", fn() { open_new_plan_form() })
  })
}

// wire_plan_rows attaches the per-row click handlers: View (detail), Duplicate
// (clone as a new plan), Delete (confirm + DELETE then refresh).
fn wire_plan_rows(plans_json : String) -> Unit {
  let root = try {
    @json.parse(plans_json)
  } catch {
    _ => return
  }
  for p in arr_at(root, "plans") {
    let id = str_at(p, "id")
    let name = str_at(p, "name")
    on_click("view-" + id, fn() { load_detail(id) })
    on_click("dup-" + id, fn() { duplicate_plan(id, name) })
    on_click("del-" + id, fn() { delete_plan(id, name) })
  }
}

// --- P0.2: create (New plan + New from template) ----------------------------

// open_new_plan_form: GET /api/templates, render the New-plan form (name +
// template picker + YAML textarea), and wire it. The template list is best-effort
// — if it fails the form still renders with a Blank option so paste/import works.
fn open_new_plan_form() -> Unit {
  api_get(templates_path(), fn(ok, status, body) {
    let templates_json = if ok && !body_has_error(body) { body } else { "{}" }
    ignore(status)
    set_html("app", new_plan_form_html(templates_json))
    on_click("new-cancel-btn", fn() { load_plans("app") })
    on_click("new-submit-btn", fn() { submit_new_plan() })
    // Prefill the textarea when a template is chosen (and clear it for Blank).
    on_change("new-template", fn() { prefill_from_template(get_value("new-template")) })
  })
}

// prefill_from_template: GET /api/templates/{id} and drop its training YAML into
// the textarea. The selected template id is remembered (via the select value) so
// submit_new_plan can attach the overlay after create. A blank id clears the box.
fn prefill_from_template(template_id : String) -> Unit {
  if template_id == "" {
    set_value("new-yaml", "")
    return
  }
  api_get(template_path(template_id), fn(ok, status, body) {
    if (!ok) || body_has_error(body) {
      set_html("new-error", error_html(status, body))
      return
    }
    let root = try {
      @json.parse(body)
    } catch {
      _ => return
    }
    set_value("new-yaml", str_at(root, "training"))
  })
}

// submit_new_plan: POST the textarea YAML to /api/plans. On success, if a template
// was selected (and carries an overlay), attach the overlay before routing to the
// new plan's detail — so a template-created plan yields a FULL BOM. On a malformed
// body the shared error alert renders in-form and NO ghost plan is created (the
// server rejects it; we never navigated away).
fn submit_new_plan() -> Unit {
  let yaml = get_value("new-yaml")
  let template_id = get_value("new-template")
  set_inflight("new-submit-btn", true, "Creating…")
  api_post(plans_path(), yaml, fn(ok, status, body) {
    set_inflight("new-submit-btn", false, "Create plan")
    if (!ok) || body_has_error(body) {
      set_html("new-error", error_html(status, body))
      return
    }
    let root = try {
      @json.parse(body)
    } catch {
      _ => return
    }
    let new_id = str_at(root, "id")
    if new_id == "" {
      set_html("new-error", error_html(0, body))
      return
    }
    if template_id == "" {
      load_detail(new_id)
    } else {
      // Attach the template's overlay (if any) so the BOM is complete, then route.
      attach_template_overlay(template_id, new_id)
    }
  })
}

// attach_template_overlay: GET the template's overlay and PUT it onto the new
// plan, then route to detail. A missing/empty overlay just routes (the plan still
// calculates; only optic identity columns would be sparse). Overlay-attach is
// best-effort: a failure still lands on the plan (it computes) — the user can
// re-attach via Edit.
fn attach_template_overlay(template_id : String, plan_id : String) -> Unit {
  api_get(template_path(template_id), fn(ok, _status, body) {
    if !ok {
      load_detail(plan_id)
      return
    }
    let root = try {
      @json.parse(body)
    } catch {
      _ => {
        load_detail(plan_id)
        return
      }
    }
    let overlay = str_at(root, "overlay")
    if overlay == "" {
      load_detail(plan_id)
      return
    }
    api_put(overlay_path(plan_id), overlay, fn(_ok2, _s2, _b2) {
      // Whether or not the overlay attached, the plan exists — show it.
      load_detail(plan_id)
    })
  })
}

// --- P0.2: duplicate --------------------------------------------------------

// duplicate_plan: GET the source plan's YAML, mutate its identity (suffix the
// meta.case_id / id so it does not collide), POST it as a new plan, then refresh
// the list. The source overlay is copied too when present, so the clone keeps a
// full BOM.
fn duplicate_plan(id : String, _name : String) -> Unit {
  api_get(plan_path(id), fn(ok, status, body) {
    if (!ok) || body_has_error(body) {
      set_html("list-error", error_html(status, body))
      return
    }
    let root = try {
      @json.parse(body)
    } catch {
      _ => return
    }
    let yaml = str_at(root, "yaml")
    let cloned = clone_yaml_identity(yaml, "-copy")
    api_post(plans_path(), cloned, fn(ok2, status2, body2) {
      if (!ok2) || body_has_error(body2) {
        set_html("list-error", error_html(status2, body2))
        return
      }
      let r2 = try {
        @json.parse(body2)
      } catch {
        _ => {
          load_plans("app")
          return
        }
      }
      let new_id = str_at(r2, "id")
      copy_overlay_then_refresh(id, new_id)
    })
  })
}

// copy_overlay_then_refresh: best-effort copy the source plan's overlay onto the
// clone, then refresh the list. A missing source overlay (404) just refreshes.
fn copy_overlay_then_refresh(src_id : String, new_id : String) -> Unit {
  if new_id == "" {
    load_plans("app")
    return
  }
  api_get(overlay_path(src_id), fn(ok, _s, body) {
    if (!ok) || body == "" || body_has_error(body) {
      load_plans("app")
      return
    }
    api_put(overlay_path(new_id), body, fn(_o, _os, _ob) { load_plans("app") })
  })
}

// --- P0.2: delete -----------------------------------------------------------

// delete_plan: confirm, then DELETE /api/plans/{id} and refresh the list. A
// non-2xx (e.g. already gone) surfaces the error alert above the table.
fn delete_plan(id : String, name : String) -> Unit {
  let label = if name == "" { id } else { name }
  if !confirm("Delete plan \"" + label + "\"? This cannot be undone.") {
    return
  }
  api_delete(plan_path(id), fn(ok, status, body) {
    if (!ok) || body_has_error(body) {
      set_html("list-error", error_html(status, body))
      return
    }
    load_plans("app")
  })
}

// load_detail: GET /api/plans/{id}, render the detail, wire its action buttons.
// A 404 (deleted/unknown plan) or network failure renders the error alert, not a
// ghost detail card with live-but-broken buttons (P0.4).
fn load_detail(id : String) -> Unit {
  set_html("app", loading_html("Loading plan…")) // P1.5: nav not silent
  api_get(plan_path(id), fn(ok, status, body) {
    if (!ok) || body_has_error(body) {
      set_html("app", error_html(status, body))
      return
    }
    set_html("app", plan_detail_html(body))
    on_click("calc-btn", fn() { trigger_calc("detail-result", id) })
    on_click("bom-btn", fn() { load_bom("detail-result", id) })
    on_click("save-btn", fn() { save_plan(id) })
    on_click("back-btn", fn() { load_plans("app") })
    on_click("crumb-plans", fn() { load_plans("app") }) // P1.5 breadcrumb
    on_click("detail-del-btn", fn() {
      delete_plan_from_detail(id, str_at_body(body, "name"))
    })
  })
}

// str_at_body parses a JSON body and reads a top-level string key (helper for the
// detail Delete confirm label). "" on any parse/shape failure.
fn str_at_body(body : String, key : String) -> String {
  let root = try {
    @json.parse(body)
  } catch {
    _ => return ""
  }
  str_at(root, key)
}

// save_plan: PUT the edited YAML from #edit-yaml to /api/plans/{id}. On success
// re-render the detail (so a renamed plan shows its new name) and let the user
// re-Calculate. On a malformed body the error alert renders in #edit-error and the
// edited text is left in place to fix.
fn save_plan(id : String) -> Unit {
  let yaml = get_value("edit-yaml")
  set_inflight("save-btn", true, "Saving…")
  api_put(plan_path(id), yaml, fn(ok, status, body) {
    set_inflight("save-btn", false, "Save")
    if (!ok) || body_has_error(body) {
      set_html("edit-error", error_html(status, body))
      return
    }
    // Re-load the canonical detail so the rendered YAML/name reflect the save.
    load_detail(id)
  })
}

// delete_plan_from_detail: confirm + DELETE, then return to the plan list. The
// error (if any) surfaces in #detail-result since the list is no longer on screen.
fn delete_plan_from_detail(id : String, name : String) -> Unit {
  let label = if name == "" { id } else { name }
  if !confirm("Delete plan \"" + label + "\"? This cannot be undone.") {
    return
  }
  api_delete(plan_path(id), fn(ok, status, body) {
    if (!ok) || body_has_error(body) {
      set_html("detail-result", error_html(status, body))
      return
    }
    load_plans("app")
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

// main_entry: bootstrap the app — wire the SPA navbar brand (P1.5: returns to
// the list without a full reload; the brand lives in the static shell, outside
// #app, so this one-time binding survives every in-app re-render) and load the
// plan list into #app.
pub fn main_entry() -> Unit {
  console_log("AID UI starting")
  on_click("nav-home", fn() { load_plans("app") })
  load_plans("app")
}
