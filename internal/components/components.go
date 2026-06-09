// Package components wires the embedded WASM artifacts to wasmhost.Component
// instances and names the D16 entry-point exports for each component. Each
// component is compiled once and cached.
package components

import (
	"sync"

	"github.com/afewell-hh/aid/embed"
	"github.com/afewell-hh/aid/internal/wasmhost"
)

// Entry-point export names (the JSON-over-memory `(ptr,len)->packed` functions).
const (
	KernelCalculate = "export_calculate"
	KernelValidate  = "export_validate"
	HhfabExport     = "export_wiring"
	BomExport       = "export_bom"
)

type cached struct {
	once sync.Once
	comp *wasmhost.Component
	err  error
}

func (c *cached) get(name string, wasm []byte) (*wasmhost.Component, error) {
	c.once.Do(func() { c.comp, c.err = wasmhost.New(name, wasm) })
	return c.comp, c.err
}

var (
	kernelC cached
	hhfabC  cached
	bomC    cached
)

// Kernel returns the topology calculator component.
func Kernel() (*wasmhost.Component, error) { return kernelC.get("kernel", embed.Kernel) }

// Hhfab returns the hhfab wiring export adapter component.
func Hhfab() (*wasmhost.Component, error) { return hhfabC.get("hhfab", embed.Hhfab) }

// Bom returns the BOM export adapter component.
func Bom() (*wasmhost.Component, error) { return bomC.get("bom", embed.Bom) }
