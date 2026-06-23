// ffi.mbt — the entire `extern "js"` surface for the AID frontend. Nothing else
// in the module touches JS directly; every browser primitive crosses one of
// these explicit, reviewable lines (TECH_STACK: MoonBit-JS has no npm interop).
//
// This boundary is the part proven by the Phase-6b spike (DOM render, fetch
// GET/POST, event registration, MoonBit closures across the FFI).

// Log to the browser console.
extern "js" fn console_log(msg : String) -> Unit = "(m) => console.log(m)"

// Set the innerHTML of the element with the given id (no-op if absent).
extern "js" fn set_html(id : String, html : String) -> Unit =
  #|(id, html) => { const e = document.getElementById(id); if (e) e.innerHTML = html; }

// Register a click handler; the MoonBit callback runs from the JS event loop.
extern "js" fn on_click(id : String, cb : () -> Unit) -> Unit =
  #|(id, cb) => { const e = document.getElementById(id); if (e) e.addEventListener("click", () => cb()); }

// Enable/disable a button (in-flight feedback; no-op if absent). A disabled
// button cannot be re-clicked, so an async action does not double-fire.
extern "js" fn set_disabled(id : String, disabled : Bool) -> Unit =
  #|(id, disabled) => { const e = document.getElementById(id); if (e) e.disabled = disabled; }

// Replace a button's visible label (e.g. "Calculate" -> "Calculating…") so
// latency is not silent. Restored to the original label on completion.
extern "js" fn set_text(id : String, text : String) -> Unit =
  #|(id, text) => { const e = document.getElementById(id); if (e) e.textContent = text; }

// GET url; deliver (ok, status, body) to the MoonBit callback so the caller can
// distinguish success from an HTTP error (non-2xx) or a network failure. A thrown
// fetch (offline / connection refused / CORS) is surfaced as ok=false, status=0,
// body="" via .catch — never a silent hang and never a dropped rejection.
extern "js" fn fetch_get(
  url : String,
  cb : (Bool, Int, String) -> Unit
) -> Unit =
  #|(url, cb) => { fetch(url).then(r => r.text().then(t => cb(r.ok, r.status, t))).catch(() => cb(false, 0, "")); }

// POST json body to url; deliver (ok, status, body) to the MoonBit callback (same
// failure contract as fetch_get).
extern "js" fn fetch_post(
  url : String,
  body : String,
  cb : (Bool, Int, String) -> Unit
) -> Unit =
  #|(url, body, cb) => { fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body }).then(r => r.text().then(t => cb(r.ok, r.status, t))).catch(() => cb(false, 0, "")); }

// Trigger a client-side file download of text content (wiring YAML export).
extern "js" fn save_file(filename : String, content : String) -> Unit =
  #|(filename, content) => { const b = new Blob([content], {type:"text/yaml"}); const u = URL.createObjectURL(b); const a = document.createElement("a"); a.href = u; a.download = filename; a.click(); URL.revokeObjectURL(u); }
