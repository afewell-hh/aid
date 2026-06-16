package calc

import (
	"errors"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
)

// errNotImplemented marks the F7a RED stub; GREEN removes it.
var errNotImplemented = errors.New("calc: Evaluate not implemented (F7a RED)")

// Evaluate runs the SAME F2 kernel path as Compute but returns the decoded
// CalcOutput WITHOUT failing when CalcOutput.Errors is non-empty. The Go error is
// reserved for genuine infra failures (BuildCalcPlan / kernel load / marshal /
// decode); calc-level constraint violations (over-allocation, etc.) come back as
// data in the returned CalcOutput.Errors.
//
// This is the accessor the F7 surfaces coordinator (internal/design) uses so the
// REST/CLI "validation as data" contract is reachable (note §1.1 / §3.0, devb
// finding 2). Compute keeps failing on a non-empty Errors for its engine-internal
// callers (DeriveQuantities, bom.Resolve, wiring.Render), which must not proceed
// on unreliable quantities.
//
// F7a RED: stub. GREEN factors Compute into Evaluate + the error gate so the two
// share one kernel-call path with no behavior change for existing callers.
func Evaluate(plan *topology.Plan, cat *catalog.Catalog) (*CalcOutput, error) {
	_, _ = plan, cat
	return nil, errNotImplemented
}
