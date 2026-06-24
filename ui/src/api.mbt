// api.mbt — typed wrappers over the Stage-A REST endpoints. Thin layer over the
// ffi fetch primitives (request plumbing is spike-proven and REAL); the UI logic
// that consumes the responses lives in render.mbt / app.mbt.

// GET <api_base><path>, delivering (ok, status, body) to cb. The caller inspects
// ok/status before rendering so an HTTP error or a network failure is handled
// explicitly (P0.4), never mistaken for a success body.
pub fn api_get(path : String, cb : (Bool, Int, String) -> Unit) -> Unit {
  fetch_get(api_base + path, cb)
}

// POST <api_base><path> with a JSON body, delivering (ok, status, body) to cb
// (same failure contract as api_get).
pub fn api_post(
  path : String,
  body : String,
  cb : (Bool, Int, String) -> Unit
) -> Unit {
  fetch_post(api_base + path, body, cb)
}

// PUT <api_base><path> with a body, delivering (ok, status, body) to cb (same
// failure contract as api_get/api_post). Used for plan edit and overlay attach.
pub fn api_put(
  path : String,
  body : String,
  cb : (Bool, Int, String) -> Unit
) -> Unit {
  fetch_put(api_base + path, body, cb)
}

// DELETE <api_base><path>, delivering (ok, status, body) to cb (same failure
// contract). A 204 has an empty body; the caller keys off ok/status.
pub fn api_delete(path : String, cb : (Bool, Int, String) -> Unit) -> Unit {
  fetch_delete(api_base + path, cb)
}

// Endpoint path builders (single source of truth for the route shapes).
pub fn plans_path() -> String {
  "/plans"
}

pub fn overlay_path(id : String) -> String {
  "/plans/" + id + "/overlay"
}

pub fn templates_path() -> String {
  "/templates"
}

pub fn template_path(id : String) -> String {
  "/templates/" + id
}

pub fn plan_path(id : String) -> String {
  "/plans/" + id
}

pub fn structure_path(id : String) -> String {
  "/plans/" + id + "/structure"
}

pub fn calc_path(id : String) -> String {
  "/plans/" + id + "/calc"
}

pub fn bom_path(id : String) -> String {
  "/plans/" + id + "/bom"
}

pub fn wiring_path(id : String, fabric : String) -> String {
  "/plans/" + id + "/wiring/" + fabric
}
