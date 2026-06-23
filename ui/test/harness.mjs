// harness.mjs — a dependency-free DOM + fetch stub for the Node ESM smoke tests.
// It lets the tests drive the REAL compiled MoonBit->JS functions (../static/
// app.js) without a browser: extern "js" calls resolve against these globals.
// No npm packages (air-gapped); uses only Node built-ins via the test files.

export const dom = {}; // id -> stub element
export const fetches = []; // recorded {url, method, body}
export const saved = []; // recorded {filename, content} from save_file (downloads)

function el(id) {
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
  });
}

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
  let ok, status, body;
  if (r != null && typeof r === "object") {
    body = r.body ?? "";
    status = r.status ?? (r.ok === false ? 500 : 200);
    ok = r.ok ?? (status >= 200 && status < 300);
  } else {
    body = r ?? "";
    status = 200;
    ok = true;
  }
  return Promise.resolve({ ok, status, text: () => Promise.resolve(body) });
};
