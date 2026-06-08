// Package orchestrate is the CLI's sole coordinator over the three WASM
// components (ARCHITECTURE Layer 4). It runs the kernel, then routes the IR to
// the hhfab adapter and the BOMs to the bom adapter, building the D16 JSON
// envelopes. Components stay pure; orchestrate does no calculation itself.
//
// RED phase: every entry point is unimplemented so the integration tests fail
// for the right reason.
package orchestrate

import "errors"

// ErrNotImplemented is returned by the RED-phase stubs.
var ErrNotImplemented = errors.New("orchestrate: not implemented (RED)")

// Calculate runs the kernel on plan JSON and returns the parsed calc-output.
// A kernel-level decode failure surfaces as a non-nil CalcResult.Err (not a Go
// error / trap).
func Calculate(planJSON []byte) (*CalcResult, error) {
	return nil, ErrNotImplemented
}

// Validate runs the kernel's validate entry point on plan JSON.
func Validate(planJSON []byte) (*ValidationResult, error) {
	return nil, ErrNotImplemented
}

// ExportWiring runs the kernel then the hhfab adapter, returning one wiring
// document per fabric. fabric == "" exports all fabrics.
func ExportWiring(planJSON []byte, fabric string) ([]WiringDocument, error) {
	return nil, ErrNotImplemented
}

// ExportBOM runs the kernel then the bom adapter. format is "csv" or "json".
func ExportBOM(planJSON []byte, format string) (*BomOutput, error) {
	return nil, ErrNotImplemented
}
