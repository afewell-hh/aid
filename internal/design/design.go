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
//
// F7a RED: the Inputs/Resolved types are the approved contract; Resolve and
// Wiring are stubs returning errNotImplemented. GREEN wires them to the engine.
package design

import (
	"errors"

	"github.com/afewell-hh/aid/internal/bom"
	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
	"github.com/afewell-hh/aid/internal/wiring"
)

// errNotImplemented marks the F7a RED stubs; GREEN removes it.
var errNotImplemented = errors.New("design: not implemented (F7a RED)")

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

// Resolve runs the rebuilt engine end-to-end over the request (note §1). A
// structural failure (unparseable / unpinned / unresolved / kernel infra) is a Go
// error; calc constraint violations are returned as data on a non-nil Resolved
// (see the type doc). F7a RED: stub.
func Resolve(in Inputs) (*Resolved, error) {
	_ = in
	return nil, errNotImplemented
}

// Wiring renders the hhfab wiring docs on demand (note §1, F4). It refuses with a
// Go error when calc is invalid (BOM/wiring would be unreliable). An empty fabric
// returns all managed-fabric docs; a non-empty fabric filters to that one. F7a
// RED: stub.
func (r *Resolved) Wiring(fabric string) ([]wiring.Doc, error) {
	_ = fabric
	return nil, errNotImplemented
}
