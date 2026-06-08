// Package components wires the embedded WASM artifacts to wasmhost.Component
// instances and names the D16 entry-point exports for each component.
package components

import (
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

// Kernel returns the topology calculator component.
func Kernel() (*wasmhost.Component, error) {
	return wasmhost.New("kernel", embed.Kernel)
}

// Hhfab returns the hhfab wiring export adapter component.
func Hhfab() (*wasmhost.Component, error) {
	return wasmhost.New("hhfab", embed.Hhfab)
}

// Bom returns the BOM export adapter component.
func Bom() (*wasmhost.Component, error) {
	return wasmhost.New("bom", embed.Bom)
}
