package calc

import (
	"encoding/json"
	"fmt"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/components"
	"github.com/afewell-hh/aid/internal/topology"
)

// Evaluate resolves the plan+catalog into a calc-plan, runs the MoonBit kernel
// over the D16 boundary, and returns the FULL decoded CalcOutput — INCLUDING a
// populated Errors — WITHOUT failing the call. The Go error is reserved for
// genuine infra failures (BuildCalcPlan / kernel load / marshal / decode);
// calc-level constraint violations (over-allocation = ZONE_OVERFLOW, etc.) come
// back as data in CalcOutput.Errors.
//
// This is the accessor the F7 surfaces coordinator (internal/design) uses so the
// REST/CLI "validation as data" contract is reachable (note §1.1 / §3.0, devb
// finding 2). Compute wraps Evaluate and adds the fail-fast gate for its
// engine-internal callers (DeriveQuantities, bom.Resolve, wiring.Render), which
// must not proceed on unreliable quantities.
func Evaluate(plan *topology.Plan, cat *catalog.Catalog) (*CalcOutput, error) {
	cp, err := BuildCalcPlan(plan, cat)
	if err != nil {
		return nil, err
	}
	in, err := json.Marshal(cp)
	if err != nil {
		return nil, fmt.Errorf("calc: marshal calc-plan: %w", err)
	}
	kernel, err := components.Kernel()
	if err != nil {
		return nil, fmt.Errorf("calc: load kernel: %w", err)
	}
	out, err := kernel.Call(components.KernelF2Calculate, in)
	if err != nil {
		return nil, fmt.Errorf("calc: kernel f2_calculate: %w", err)
	}
	var co CalcOutput
	if err := json.Unmarshal(out, &co); err != nil {
		return nil, fmt.Errorf("calc: decode calc-output: %w", err)
	}
	return &co, nil
}
