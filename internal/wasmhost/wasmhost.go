// Package wasmhost is the single, reusable wasmtime-go host for every AID WASM
// component, driving the D16 JSON-over-linear-memory ABI:
//
//	alloc(len) -> ptr ; dealloc(ptr,len) ; <entry>(ptr,len) -> packed (ptr<<32)|len
//
// where payloads are UTF-8 JSON. All three components (kernel, hhfab, bom) are
// hosted identically through this one path (ARCHITECTURE Layer 4: the CLI is the
// sole orchestrator; components never call each other).
//
// RED phase: the type/API surface is fixed; Call is unimplemented so every
// integration test fails for the right reason (host not built yet).
package wasmhost

import "errors"

// ErrNotImplemented is returned by the RED-phase stub.
var ErrNotImplemented = errors.New("wasmhost: not implemented (RED)")

// Component is a compiled WASM component ready to be invoked. The engine and
// compiled module are created once; a fresh store/instance is used per Call.
type Component struct {
	name string
	wasm []byte
	// GREEN: engine *wasmtime.Engine; module *wasmtime.Module
}

// New compiles a component from its WASM bytes. The name is used in errors.
func New(name string, wasm []byte) (*Component, error) {
	// GREEN: build engine, compile module, validate alloc/dealloc/memory exports.
	return &Component{name: name, wasm: wasm}, nil
}

// Call runs the entry export with the input JSON and returns the output JSON.
// It allocates input in linear memory, invokes the export, unpacks the returned
// (ptr,len), and copies the output bytes back out.
func (c *Component) Call(entry string, input []byte) ([]byte, error) {
	return nil, ErrNotImplemented
}

// Name reports the component name (for diagnostics).
func (c *Component) Name() string { return c.name }
