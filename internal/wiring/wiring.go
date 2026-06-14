// Package wiring is AID's F4 hhfab wiring renderer
// (docs/foundation-redesign.md §2.5; docs/foundation/f4-architecture-note.md;
// Issue #60; D22/D23). It is a PURE transform of the F2 IR (calc.CalcOutput) +
// the ingested topology plan + the (overlay-merged) catalog into hhfab wiring
// CRDs (wiring.githedgehog.com/v1beta1 + vpc.githedgehog.com/v1beta1), one
// document per managed fabric (grouped by fabric_name — note §2.1). No new
// topology calc: it consumes F2's per-(switch,zone) endpoints and only pairs the
// mesh-zone ports F2 defers to F4 (note §2.6). D22: wiring only — no
// netbox_inventory.json. No empty `ecmp: {}` (the field is omitted entirely).
//
// F4 RED: this is the approved contract surface; Render is a STUB. GREEN groups
// the IR by managed fabric_name and renders the Switch/Server/Connection +
// namespace CRDs (note §2.2–§2.6). The structural-equivalence bar (note §3B) is
// enforced by internal/oracle.CompareWiringHhfab + the hhfab-validate hard gate.
package wiring

import (
	"errors"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
)

// ErrNotImplemented marks the F4 renderer as not yet implemented (RED). GREEN
// replaces the stub with the real per-fabric render; this sentinel disappears.
var ErrNotImplemented = errors.New("wiring: Render not implemented (F4 GREEN pending)")

// Doc is one managed fabric's wiring YAML — the unit of output. Fabric is the
// managed fabric_name (e.g. "soc-storage-scale-out"); YAML is the multi-document
// CRD stream that file (`wiring-{Fabric}.yaml`) would contain.
type Doc struct {
	Fabric string
	YAML   []byte
}

// Render transforms the F2 IR + plan + catalog into one wiring Doc per managed
// fabric_name (note §2.1). It is a pure function of its inputs — no calc, no
// catalog mutation, no I/O. The catalog is expected to be overlay-merged so the
// switch Item.Model resolves to the hhfab profile (note §2.4).
//
// STUB (RED): returns ErrNotImplemented so the oracle row and the unit fixtures
// fail for the right reason (renderer absent), not skip. GREEN implements it.
func Render(plan *topology.Plan, cat *catalog.Catalog, calcOut *calc.CalcOutput) ([]Doc, error) {
	return nil, ErrNotImplemented
}
