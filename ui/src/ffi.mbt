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

// Register a click handler on a ROW (or any container) that fires ONLY when the
// click did not originate from an interactive control inside it (P2.4 clickable
// rows): clicking the row body opens the plan, but clicking a View/Duplicate/
// Delete button (or a link/input) still does just that — no double-navigation,
// no behavior regression. The target guard tolerates a synthetic call with no
// event (the Node mock harness fires listeners without one).
extern "js" fn on_click_row(id : String, cb : () -> Unit) -> Unit =
  #|(id, cb) => { const e = document.getElementById(id); if (e) e.addEventListener("click", (ev) => { const t = ev && ev.target; if (t && t.closest && t.closest("button, a, input, select, textarea")) return; cb(); }); }

// Register a change handler (e.g. the New-plan template <select>); the MoonBit
// callback runs when the control's value changes.
extern "js" fn on_change(id : String, cb : () -> Unit) -> Unit =
  #|(id, cb) => { const e = document.getElementById(id); if (e) e.addEventListener("change", () => cb()); }

// Register a delegated input+change handler on a CONTAINER: any field edit
// bubbling up from inside it fires cb (P1.3 live validation — one binding covers
// every structured-editor field; also used on the raw-YAML textarea).
extern "js" fn on_change_within(id : String, cb : () -> Unit) -> Unit =
  #|(id, cb) => { const e = document.getElementById(id); if (e) { e.addEventListener("input", () => cb()); e.addEventListener("change", () => cb()); } }

// schedule debounces cb by key: a new call for the same key cancels the pending
// one and restarts the ms timer (P1.3 — coalesce per-keystroke edits into one
// validate). Uses globalThis so it works under both the browser and the Node
// test harness.
extern "js" fn schedule(key : String, ms : Int, cb : () -> Unit) -> Unit =
  #|(key, ms, cb) => { const t = (globalThis.__aid_timers = globalThis.__aid_timers || {}); if (t[key]) clearTimeout(t[key]); t[key] = setTimeout(() => cb(), ms); }

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

// PUT a body to url; deliver (ok, status, body) to the MoonBit callback (same
// failure contract as fetch_get/fetch_post). Used for plan edit (PUT
// .../plans/{id}) and overlay attach (PUT .../plans/{id}/overlay).
extern "js" fn fetch_put(
  url : String,
  body : String,
  cb : (Bool, Int, String) -> Unit
) -> Unit =
  #|(url, body, cb) => { fetch(url, { method: "PUT", headers: { "Content-Type": "application/json" }, body }).then(r => r.text().then(t => cb(r.ok, r.status, t))).catch(() => cb(false, 0, "")); }

// DELETE url; deliver (ok, status, body) to the MoonBit callback. A 204 No Content
// has an empty body; the caller keys off ok/status (same failure contract).
extern "js" fn fetch_delete(url : String, cb : (Bool, Int, String) -> Unit) -> Unit =
  #|(url, cb) => { fetch(url, { method: "DELETE" }).then(r => r.text().then(t => cb(r.ok, r.status, t))).catch(() => cb(false, 0, "")); }

// Read the current value of an <input>/<textarea>/<select> by id ("" if absent).
// The authoring forms (New plan, Edit YAML) collect user input through this.
extern "js" fn get_value(id : String) -> String =
  #|(id) => { const e = document.getElementById(id); return e ? (e.value || "") : ""; }

// Set the value of an <input>/<textarea>/<select> by id (no-op if absent). Used to
// prefill the New-plan textarea from a fetched template.
extern "js" fn set_value(id : String, value : String) -> Unit =
  #|(id, value) => { const e = document.getElementById(id); if (e) e.value = value; }

// window.confirm: a blocking native confirm dialog (true = OK). Backs the
// delete-confirm step. In the Node mock harness this is stubbed to auto-confirm.
extern "js" fn confirm(message : String) -> Bool = "(m) => (typeof confirm === 'function' ? confirm(m) : true)"

// Trigger a client-side file download of text content (wiring YAML export).
extern "js" fn save_file(filename : String, content : String) -> Unit =
  #|(filename, content) => { const b = new Blob([content], {type:"text/yaml"}); const u = URL.createObjectURL(b); const a = document.createElement("a"); a.href = u; a.download = filename; a.click(); URL.revokeObjectURL(u); }
