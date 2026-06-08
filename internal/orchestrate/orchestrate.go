// Package orchestrate is the CLI's sole coordinator over the three WASM
// components (ARCHITECTURE Layer 4). It runs the kernel, then routes the IR to
// the hhfab adapter and the BOMs to the bom adapter, building the D16 JSON
// envelopes. Components stay pure; orchestrate does no calculation itself.
package orchestrate

import (
	"encoding/json"
	"fmt"

	"github.com/afewell-hh/aid/internal/components"
)

// Calculate runs the kernel on plan JSON and returns the parsed calc-output
// envelope. A kernel-level decode failure surfaces as a non-nil CalcResult.Err
// (not a Go error / trap).
func Calculate(planJSON []byte) (*CalcResult, error) {
	kernel, err := components.Kernel()
	if err != nil {
		return nil, err
	}
	out, err := kernel.Call(components.KernelCalculate, planJSON)
	if err != nil {
		return nil, fmt.Errorf("kernel calculate: %w", err)
	}
	var res CalcResult
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("parse calc-output: %w", err)
	}
	return &res, nil
}

// calcOK runs the kernel and returns the calc-output, turning a kernel-level
// {"err":...} into a Go error (used by the export paths).
func calcOK(planJSON []byte) (*CalcOutput, error) {
	res, err := Calculate(planJSON)
	if err != nil {
		return nil, err
	}
	if res.Ok == nil {
		return nil, fmt.Errorf("kernel rejected plan: %s", string(res.Err))
	}
	return res.Ok, nil
}

// Validate runs the kernel's validate entry point on plan JSON.
func Validate(planJSON []byte) (*ValidationResult, error) {
	kernel, err := components.Kernel()
	if err != nil {
		return nil, err
	}
	out, err := kernel.Call(components.KernelValidate, planJSON)
	if err != nil {
		return nil, fmt.Errorf("kernel validate: %w", err)
	}
	var res struct {
		Ok  *ValidationResult `json:"ok"`
		Err json.RawMessage   `json:"err"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("parse validation-result: %w", err)
	}
	if res.Ok == nil {
		return nil, fmt.Errorf("kernel rejected plan: %s", string(res.Err))
	}
	return res.Ok, nil
}

// ExportWiring runs the kernel then the hhfab adapter, returning one wiring
// document per fabric. fabric == "" exports all fabrics.
func ExportWiring(planJSON []byte, fabric string) ([]WiringDocument, error) {
	out, err := calcOK(planJSON)
	if err != nil {
		return nil, err
	}
	irJSON, err := json.Marshal(out.IR)
	if err != nil {
		return nil, err
	}
	var fabricOpt any
	if fabric != "" {
		fabricOpt = fabric
	}
	input, err := json.Marshal(map[string]any{
		"ir": json.RawMessage(irJSON),
		"options": map[string]any{
			"fabric":          fabricOpt,
			"split_by_fabric": false,
		},
	})
	if err != nil {
		return nil, err
	}
	hhfab, err := components.Hhfab()
	if err != nil {
		return nil, err
	}
	res, err := hhfab.Call(components.HhfabExport, input)
	if err != nil {
		return nil, fmt.Errorf("hhfab export: %w", err)
	}
	var parsed hhfabResult
	if err := json.Unmarshal(res, &parsed); err != nil {
		return nil, fmt.Errorf("parse hhfab-output: %w", err)
	}
	if parsed.Ok == nil {
		return nil, fmt.Errorf("hhfab adapter error: %s", string(parsed.Err))
	}
	return parsed.Ok.Documents, nil
}

// ExportBOM runs the kernel then the bom adapter. format is "csv" or "json".
func ExportBOM(planJSON []byte, format string) (*BomOutput, error) {
	out, err := calcOK(planJSON)
	if err != nil {
		return nil, err
	}
	bomsJSON, err := json.Marshal(out.Boms)
	if err != nil {
		return nil, err
	}
	input, err := json.Marshal(map[string]any{
		"boms": json.RawMessage(bomsJSON),
		"options": map[string]any{
			"format":               format,
			"include_fleet_totals": true,
		},
	})
	if err != nil {
		return nil, err
	}
	bom, err := components.Bom()
	if err != nil {
		return nil, err
	}
	res, err := bom.Call(components.BomExport, input)
	if err != nil {
		return nil, fmt.Errorf("bom export: %w", err)
	}
	var parsed bomResult
	if err := json.Unmarshal(res, &parsed); err != nil {
		return nil, fmt.Errorf("parse bom-output: %w", err)
	}
	if parsed.Ok == nil {
		return nil, fmt.Errorf("bom adapter error: %s", string(parsed.Err))
	}
	return parsed.Ok, nil
}
