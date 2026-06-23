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

// Endpoint path builders (single source of truth for the route shapes).
pub fn plans_path() -> String {
  "/plans"
}

pub fn plan_path(id : String) -> String {
  "/plans/" + id
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
