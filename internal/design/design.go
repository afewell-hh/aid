// Package design is the F7 surfaces coordinator: the single facade the CLI/REST/
// GUI call to run a plan through the rebuilt engine end-to-end —
//
//	topology.IngestBundled → calc.Evaluate (base catalog) → catalog.Merge(overlay)
//	→ bom.Resolve / wiring.Render
//
// It replaces internal/orchestrate (retired in F7d). The MoonBit proved kernel is
// untouched: calc.Evaluate routes through components.Kernel() exactly as
// calc.Compute does. See docs/foundation/f7-architecture-note.md §1 / §1.1 / §3.0.
//
// ⚠️ Ordering is load-bearing (note §1.1, devb finding 1): calc runs on the BASE
// extracted catalog; the overlay merges AFTER calc and BEFORE bom/wiring (the
// overlay only enriches bom.csv optic cols 7–19, which calc never reads). Proven
// by the oracle harness (oracle_test.go:250,326).
package design

import (
	"fmt"
	"sort"

	"github.com/afewell-hh/aid/internal/bom"
	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
	"github.com/afewell-hh/aid/internal/wiring"
)

// Inputs is one self-contained design request (note §1).
type Inputs struct {
	TrainingYAML []byte // the DIET/training bundle (HNP's authoring format, D25)
	OverlayYAML  []byte // optional AID optic/identity overlay (note §2); nil ⇒ base catalog only
}

// Resolved is the fully-computed model every surface renders from (note §1).
//
// A non-nil Resolved with a NON-EMPTY Calc.Errors is a VALID return: the plan
// ingested and resolved, but the kernel reported calc-level constraint violations
// (e.g. over-allocation). In that case BOM is nil and Wiring() refuses —
// quantities are unreliable, so a silently-wrong BOM/wiring is never produced.
// This preserves calc.Compute's fail-fast guarantee for its engine-internal
// callers while still surfacing the violations as data to the surfaces (§3.0).
type Resolved struct {
	Plan    *topology.Plan
	Catalog *catalog.Catalog
	Calc    *calc.CalcOutput   // switch/server quantities, endpoints, verdicts, Errors
	BOM     *bom.ResolvedModel // nil iff len(Calc.Errors) > 0
}

// Valid reports whether calc produced no constraint violations (the is_valid
// boolean the surfaces expose, note §3.0).
func (r *Resolved) Valid() bool {
	return r != nil && r.Calc != nil && len(r.Calc.Errors) == 0
}

// Resolve runs the rebuilt engine end-to-end over the request (note §1):
//
//	IngestBundled → calc.Evaluate (BASE catalog) → catalog.Merge(overlay) → bom.Resolve
//
// ⚠️ Ordering: calc runs on the base extracted catalog; the overlay merges only
// AFTER calc and before bom (it enriches bom.csv optic cols, which calc never
// reads — note §1.1). A structural failure (unparseable / unpinned / unresolved /
// kernel infra) is a Go error; calc constraint violations are returned as data on
// a non-nil Resolved with BOM left nil (see the type doc).
func Resolve(in Inputs) (*Resolved, error) {
	plan, cat, err := topology.IngestBundled(in.TrainingYAML)
	if err != nil {
		return nil, err
	}
	calcOut, err := calc.Evaluate(plan, cat)
	if err != nil {
		return nil, err
	}
	res := &Resolved{Plan: plan, Catalog: cat, Calc: calcOut}
	if len(calcOut.Errors) > 0 {
		// Valid ingest, but the kernel rejected the allocation: surface the
		// violations as data, leave BOM nil (quantities are unreliable).
		return res, nil
	}
	if len(in.OverlayYAML) > 0 {
		overlay, err := catalog.LoadBytes(in.OverlayYAML)
		if err != nil {
			return nil, err
		}
		cat.Merge(overlay)
	}
	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		return nil, err
	}
	res.BOM = model
	return res, nil
}

// Wiring renders the hhfab wiring docs on demand (note §1, F4). It refuses with a
// Go error when calc is invalid (BOM/wiring would be unreliable). An empty fabric
// returns all managed-fabric docs; a non-empty fabric filters to that one.
func (r *Resolved) Wiring(fabric string) ([]wiring.Doc, error) {
	if !r.Valid() {
		n := 0
		if r != nil && r.Calc != nil {
			n = len(r.Calc.Errors)
		}
		return nil, fmt.Errorf("design: cannot render wiring: calc has %d error(s)", n)
	}
	docs, err := wiring.Render(r.Plan, r.Catalog, r.Calc)
	if err != nil {
		return nil, err
	}
	if fabric == "" {
		return docs, nil
	}
	var filtered []wiring.Doc
	for _, d := range docs {
		if d.Fabric == fabric {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

// ManagedFabrics returns the distinct managed-fabric names of the plan — the
// fabric_name of every switch class whose fabric_class == "managed" — sorted for
// stable output. These are exactly the {fabric} values GET .../wiring/{fabric}
// accepts; the surfaces use them to populate per-fabric download buttons (no
// guessing) and to list the valid choices in a bad-fabric error. It is derived
// from the plan model directly (no calc/render dependency), so it is available
// even before a calc has run.
func (r *Resolved) ManagedFabrics() []string {
	if r == nil || r.Plan == nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, sw := range r.Plan.Spec.SwitchClasses {
		if sw.FabricClass != "managed" || sw.FabricName == "" {
			continue
		}
		if !seen[sw.FabricName] {
			seen[sw.FabricName] = true
			names = append(names, sw.FabricName)
		}
	}
	sort.Strings(names)
	return names
}
