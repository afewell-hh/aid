// harness.mjs — a dependency-free DOM + fetch stub for the Node ESM smoke tests.
// It lets the tests drive the REAL compiled MoonBit->JS functions (../static/
// app.js) without a browser: extern "js" calls resolve against these globals.
// No npm packages (air-gapped); uses only Node built-ins via the test files.

export const dom = {}; // id -> stub element
export const fetches = []; // recorded {url, method, body}

function el(id) {
  return (dom[id] ??= {
    id,
    innerHTML: "",
    value: "",
    _listeners: {},
    addEventListener(ev, cb) {
      (this._listeners[ev] ??= []).push(cb);
    },
    click() {
      (this._listeners.click || []).forEach((f) => f());
    },
  });
}

let responder = () => "";

// setResponder controls what the stubbed fetch returns for a given (url, opts).
export function setResponder(fn) {
  responder = fn;
}

// reset clears DOM, recorded fetches, and the responder between tests.
export function reset() {
  for (const k in dom) delete dom[k];
  fetches.length = 0;
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
  }
};
globalThis.URL = { createObjectURL: () => "blob:stub", revokeObjectURL() {} };
globalThis.fetch = (url, opts = {}) => {
  fetches.push({ url, method: opts.method || "GET", body: opts.body });
  return Promise.resolve({ text: () => Promise.resolve(responder(url, opts)) });
};
