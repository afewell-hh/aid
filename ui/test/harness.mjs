// harness.mjs — a dependency-free DOM + fetch stub for the Node ESM smoke tests.
// It lets the tests drive the REAL compiled MoonBit->JS functions (../static/
// app.js) without a browser: extern "js" calls resolve against these globals.
// No npm packages (air-gapped); uses only Node built-ins via the test files.

export const dom = {}; // id -> stub element
export const fetches = []; // recorded {url, method, body}
export const saved = []; // recorded {filename, content} from save_file (downloads)

// el returns (lazily creating) the stub element for id. Exported so tests can
// pre-create/address a control the renderer references by id (set_html only sets
// an innerHTML string — it does not materialize child stubs).
export function el(id) {
  return (dom[id] ??= {
    id,
    innerHTML: "",
    value: "",
    textContent: "",
    disabled: false,
    _listeners: {},
    addEventListener(ev, cb) {
      (this._listeners[ev] ??= []).push(cb);
    },
    click() {
      (this._listeners.click || []).forEach((f) => f());
    },
    // Set the value and fire any registered change listeners (models a user
    // editing an <input>/<select>/<textarea>): the New-plan template picker wires
    // on_change to prefill the YAML textarea.
    change(v) {
      if (v !== undefined) this.value = v;
      (this._listeners.change || []).forEach((f) => f());
    },
  });
}

// confirm: window.confirm stub for the delete-confirm step. Defaults to true
// (accept); tests can flip confirmResult to model a user clicking Cancel.
export let confirmResult = true;
export function setConfirm(v) {
  confirmResult = v;
}
globalThis.confirm = () => confirmResult;

// responder returns the response for a (url, opts). It may return either a plain
// string (treated as a 200 OK body) or an object {ok?, status?, body} to model an
// HTTP error / non-2xx, or THROW to model a network failure (the FFI's .catch
// path -> ok=false, status=0). Default: 200 with an empty body.
let responder = () => "";

// setResponder controls what the stubbed fetch returns for a given (url, opts).
export function setResponder(fn) {
  responder = fn;
}

// reset clears DOM, recorded fetches/downloads, and the responder between tests.
export function reset() {
  for (const k in dom) delete dom[k];
  fetches.length = 0;
  saved.length = 0;
  responder = () => "";
  confirmResult = true;
}

// flush drains pending promise callbacks (the fetch().then(...) chain).
export function flush() {
  return new Promise((r) => setTimeout(r, 10));
}

// Install the browser-global stubs the FFI surface (ffi.mbt) touches.
globalThis.document = {
  getElementById: el,
  createElement: () => ({ click() {}, set href(_v) {}, set download(_v) {} }),
};
globalThis.Blob = class Blob {
  constructor(parts) {
    this.parts = parts;
    this._text = (parts || []).join("");
  }
};
// save_file (ffi.mbt) builds a Blob from the content; capture it so download
// tests can assert exactly what would be written to disk.
globalThis.URL = {
  createObjectURL: (b) => {
    saved.push({ blob: b, content: b && b._text != null ? b._text : "" });
    return "blob:stub";
  },
  revokeObjectURL() {},
};
// The mock fetch models the real fetch contract the FFI now depends on: a
// resolved Response carries {ok, status, text()}; a thrown responder rejects the
// promise (FFI .catch -> ok=false, status=0). A string responder is sugar for a
// 200 OK body.
globalThis.fetch = (url, opts = {}) => {
  fetches.push({ url, method: opts.method || "GET", body: opts.body });
  let r;
  try {
    r = responder(url, opts);
  } catch (e) {
    return Promise.reject(e); // network failure
  }
  let ok, status, body, delay;
  if (r != null && typeof r === "object") {
    body = r.body ?? "";
    status = r.status ?? (r.ok === false ? 500 : 200);
    ok = r.ok ?? (status >= 200 && status < 300);
    delay = r.delay ?? 0; // optional ms delay → lets tests force out-of-order responses
  } else {
    body = r ?? "";
    status = 200;
    ok = true;
    delay = 0;
  }
  const resp = { ok, status, text: () => Promise.resolve(body) };
  if (delay > 0) {
    return new Promise((res) => setTimeout(() => res(resp), delay));
  }
  return Promise.resolve(resp);
};
