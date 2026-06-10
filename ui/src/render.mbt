// render.mbt — pure HTML-string builders (Bootstrap 5 markup) for each surface.
// These take the API's JSON response text and return NetBox-style markup
// (cards, tables, badges). They touch no JS — app.mbt sets the result into the
// DOM via ffi.
//
// RED phase (issue #11, Stage B): every builder returns "" so the surfaces are
// blank. GREEN implements the markup; the Node smoke tests assert the produced
// markup and so currently fail.

// Plan list: a Bootstrap table of plan summaries with status badges.
pub fn plan_list_html(plans_json : String) -> String {
  ignore(plans_json)
  "" // RED stub
}

// Plan detail: cards for fabrics, device classes, port zones, BOM summary, and
// validation results.
pub fn plan_detail_html(detail_json : String) -> String {
  ignore(detail_json)
  "" // RED stub
}

// Calc summary: validation result (is_valid + any error codes) after a calc.
pub fn calc_summary_html(calc_json : String) -> String {
  ignore(calc_json)
  "" // RED stub
}

// BOM view: hierarchical line items with per-unit AND fleet quantities.
pub fn bom_html(bom_json : String) -> String {
  ignore(bom_json)
  "" // RED stub
}
