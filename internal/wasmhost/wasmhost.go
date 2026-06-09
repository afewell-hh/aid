// Package wasmhost is the single, reusable wasmtime-go host for every AID WASM
// component, driving the D16 JSON-over-linear-memory ABI:
//
//	alloc(len) -> ptr ; dealloc(ptr,len) ; <entry>(ptr,len) -> packed (ptr<<32)|len
//
// where payloads are UTF-8 JSON. All three components (kernel, hhfab, bom) are
// hosted identically through this one path (ARCHITECTURE Layer 4: the CLI is the
// sole orchestrator; components never call each other). The MoonBit kernel and
// the two Rust adapters expose identical wasm signatures, so one host serves
// all three.
package wasmhost

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v45"
)

// Component is a compiled WASM component ready to be invoked. The engine and
// compiled module are created once; a fresh store/instance is used per Call for
// isolation (instantiation is ~2ms — measured in the feasibility spike).
type Component struct {
	name   string
	engine *wasmtime.Engine
	module *wasmtime.Module
}

// New compiles a component from its WASM bytes. The name is used in errors.
func New(name string, wasm []byte) (*Component, error) {
	engine := wasmtime.NewEngine()
	module, err := wasmtime.NewModule(engine, wasm)
	if err != nil {
		return nil, fmt.Errorf("wasmhost %s: compile: %w", name, err)
	}
	return &Component{name: name, engine: engine, module: module}, nil
}

// Name reports the component name (for diagnostics).
func (c *Component) Name() string { return c.name }

// Call runs the entry export with the input JSON and returns the output JSON.
// It allocates input in linear memory, invokes the export, unpacks the returned
// (ptr,len), and copies the output bytes back out. A guest-signalled domain
// error (e.g. {"err":...}) is NOT an error here — it is returned as output JSON;
// only host/trap failures return a non-nil error.
func (c *Component) Call(entry string, input []byte) ([]byte, error) {
	store := wasmtime.NewStore(c.engine)
	instance, err := wasmtime.NewInstance(store, c.module, nil)
	if err != nil {
		return nil, fmt.Errorf("wasmhost %s: instantiate: %w", c.name, err)
	}

	memExport := instance.GetExport(store, "memory")
	if memExport == nil || memExport.Memory() == nil {
		return nil, fmt.Errorf("wasmhost %s: no exported memory", c.name)
	}
	mem := memExport.Memory()

	alloc := instance.GetFunc(store, "alloc")
	entryFn := instance.GetFunc(store, entry)
	if alloc == nil {
		return nil, fmt.Errorf("wasmhost %s: missing export alloc", c.name)
	}
	if entryFn == nil {
		return nil, fmt.Errorf("wasmhost %s: missing export %s", c.name, entry)
	}

	// Allocate input buffer and write the bytes.
	inLen := int32(len(input))
	res, err := alloc.Call(store, inLen)
	if err != nil {
		return nil, fmt.Errorf("wasmhost %s: alloc: %w", c.name, err)
	}
	inPtr, ok := res.(int32)
	if !ok {
		return nil, fmt.Errorf("wasmhost %s: alloc returned %T, want int32", c.name, res)
	}
	// Re-fetch backing memory after alloc (it may have grown) before writing.
	data := mem.UnsafeData(store)
	if int(inPtr)+len(input) > len(data) {
		return nil, fmt.Errorf("wasmhost %s: alloc ptr %d+%d out of bounds (mem %d)", c.name, inPtr, len(input), len(data))
	}
	copy(data[inPtr:int(inPtr)+len(input)], input)

	// Invoke the entry export.
	packedRes, err := entryFn.Call(store, inPtr, inLen)
	if err != nil {
		return nil, fmt.Errorf("wasmhost %s: %s: %w", c.name, entry, err)
	}
	packed, ok := packedRes.(int64)
	if !ok {
		return nil, fmt.Errorf("wasmhost %s: %s returned %T, want int64", c.name, entry, packedRes)
	}
	outPtr := int64(uint64(packed) >> 32)
	outLen := int64(uint64(packed) & 0xFFFFFFFF)

	// Re-fetch memory: the guest allocated its output during the call, which can
	// grow linear memory and invalidate the earlier slice.
	data = mem.UnsafeData(store)
	if outPtr < 0 || outLen < 0 || outPtr+outLen > int64(len(data)) {
		return nil, fmt.Errorf("wasmhost %s: %s output (ptr=%d len=%d) out of bounds (mem %d)", c.name, entry, outPtr, outLen, len(data))
	}
	out := make([]byte, outLen)
	copy(out, data[outPtr:outPtr+outLen])

	// Best-effort free of the output and input buffers if dealloc is present.
	if dealloc := instance.GetFunc(store, "dealloc"); dealloc != nil {
		_, _ = dealloc.Call(store, int32(outPtr), int32(outLen))
		_, _ = dealloc.Call(store, inPtr, inLen)
	}
	return out, nil
}
