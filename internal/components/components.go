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
// F7d retired the old adapter/orchestrate entries (export_calculate /
// export_validate / export_wiring / export_bom); the rebuilt engine uses the F2
// calc + F3 BOM kernel entries below (#64/#35).
const (
	KernelF2Calculate = "export_f2_calculate"
	// KernelF3Bom routes the F3 BOM-scale plan through the proven I4 cores
	// (@proofs.child_qpu/fleet_quantity) and returns the fleet-scaled lines.
	KernelF3Bom = "export_f3_bom"
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

var kernelC cached

// Kernel returns the proved MoonBit topology calculator component.
func Kernel() (*wasmhost.Component, error) { return kernelC.get("kernel", embed.Kernel) }
