// Package bom is AID's deterministic, plan-time BOM reducer
// (docs/foundation-redesign.md §4.4; docs/foundation/f3-architecture-note.md;
// Issue #56; D2/D6/D22). It resolves the catalog + topology + F2 calc-output into
// ONE resolved object graph and renders TWO views of it:
//
//   - the full purchasable BOM (Layer B → docs/requirements/real-server-bom.csv),
//   - the HNP 19-column projection (Layer A → tests/oracle/.../bom.csv).
//
// THE ANTI-DRIFT GATE (note §2, load-bearing). There is exactly ONE resolver,
// Resolve(...) → *ResolvedModel, and the two renderers are PURE functions of that
// model and NOTHING else: RenderFullBOM(*ResolvedModel) and
// RenderProjection(*ResolvedModel) take only the model, so by their signature they
// cannot re-count from the plan. The projection is a FILTER + REGROUP of the same
// []ResolvedLine the full BOM renders — never a second independently-counted path.
// (A subset-invariant test enforces this structurally.)
//
// Quantity/scaling math (D2/§4.4) is the proved invariant: every qpu/fleet
// multiply routes through the kernel cores @proofs.child_qpu / @proofs.fleet_quantity
// (I4) over the D16 boundary (export_f3_bom). Only catalog resolution and CSV/JSON
// rendering are impure Go (note §1.2).
//
// F3 RED: types + stubs only; Resolve/RenderFullBOM/RenderProjection return
// ErrNotImplemented. GREEN fills the bodies. No production reducer logic here yet.
package bom

import (
	"errors"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
)

// ErrNotImplemented marks an F3 RED stub (the reducer body lands in GREEN).
var ErrNotImplemented = errors.New("bom: reducer not implemented (F3 GREEN)")

// OpticAttrs are the transceiver attribute columns 7–19 of the HNP bom.csv
// (note §3.2). They are AID-owned public optical facts carried in the catalog
// calc_profile overlay (NOT learned from bom.csv, NOT imported from HNP — D1/D12);
// the projection renderer reads them. Only transceiver lines carry them.
type OpticAttrs struct {
	CageType           string // OSFP, QSFP112, SFP28, RJ45, …
	Medium             string // SMF, MMF, Copper
	Connector          string // MPO-12, LC, RJ45, Dual MPO-12
	Standard           string // 400GBASE-DR4, 200GBASE-SR2, …
	ReachClass         string // DR, SR, VR, Copper
	WavelengthNm       string // 1310, 850, "" (copper)
	HostLaneCount      string // 4, 2, 1, 8
	HostSerdesGbps     string // 100, 25, 1
	OpticalLanePattern string // DR4, SR2, VR4, Unknown (NICs), "" (copper)
	GearboxPresent     string // false / true (string to round-trip the CSV literal)
	CableAssemblyType  string // none
	BreakoutTopology   string // 1x, 2x400g (the only non-1x is r4113_a9220_vr)
	IsCableAssembly    string // false / true
}

// ResolvedLine is one fleet-scaled line in the single resolved graph (the unit
// both renderers view). The full BOM renders every line in the owner shape; the
// projection filters to Section ∈ {server,switch,nic,server_transceiver,
// switch_transceiver} and regroups into the 19-column shape.
type ResolvedLine struct {
	// Catalog identity / classification.
	Kind          string // catalog kind: server|switch|nic|dpu|transceiver|accessory|warranty|…
	Section       string // HNP projection section ("" ⇒ not in the projection)
	HedgehogClass string // projection col 4 (compute_xpu, soc_storage_scale_out_leaf, …)
	Manufacturer  string
	Model         string // projection col 2 (module_type_model) / full-BOM identity
	PartNumber    string // SKU
	Description   string

	// Full-BOM (Layer B) fields.
	Category        string // real-server-bom "Type" column (Barebone, EWCSC, GPU Board, …)
	Physical        bool   // false ⇒ warranty/support/assembly/onsite
	TotalCapacityGB string
	PowerW          string
	TotalPowerW     string

	// Quantity — fleet-scaled (qpu × instance count) via the proved cores.
	FleetQuantity int

	// Projection (Layer A) extras.
	Projected bool        // surfaced in the 19-column projection?
	Optic     *OpticAttrs // transceiver lines only
}

// ResolvedModel is the single resolved object graph (note §2). Both renderers are
// pure functions of this value.
type ResolvedModel struct {
	Lines                        []ResolvedLine
	SuppressedCableAssemblyCount int // the bom.csv footer value (0 for xoc-64)
}

// Resolve is THE single resolver (the anti-drift gate, note §2). It builds one
// resolved object graph from the ingested plan, the catalog (incl. the AID-owned
// optic/line-template overlay merged into cat), and the F2 calc-output:
//   - server/switch instances scaled by F2 quantities;
//   - each server's component_slots expanded recursively (the 8× CX-7 / 1× BF3
//     quantity-bearing slots) and bom_line_templates (physical + non-physical);
//   - the selected transceiver per populated cage (server cage_bindings);
//   - the switch-side transceiver per DISTINCT physical cage from calcOut.Endpoints
//     (breakout cage = one optic, not one-per-logical-port — note §3.2).
//
// Every quantity multiply routes through the kernel cores (export_f3_bom).
//
// RED: stub — returns ErrNotImplemented.
func Resolve(plan *topology.Plan, cat *catalog.Catalog, calcOut *calc.CalcOutput) (*ResolvedModel, error) {
	return nil, ErrNotImplemented
}

// RenderFullBOM renders the full purchasable BOM (Layer B → real-server-bom.csv):
// every line in the owner CSV shape, incl. non-physical and nested lines, with
// per-cage transceiver (kind=transceiver) lines SUPPRESSED from the flat CSV.
// Takes ONLY the model (the signature is the anti-drift gate).
//
// RED: stub — returns ErrNotImplemented.
func RenderFullBOM(m *ResolvedModel) ([][]string, error) {
	return nil, ErrNotImplemented
}

// RenderProjection renders the HNP 19-column projection (Layer A → bom.csv): the
// SAME model filtered to HNP-physical sections and regrouped into the 19-column
// shape with HNP's section ordering and the suppressed-cable-assembly footer.
// Takes ONLY the model (the signature is the anti-drift gate).
//
// RED: stub — returns ErrNotImplemented.
func RenderProjection(m *ResolvedModel) ([][]string, error) {
	return nil, ErrNotImplemented
}
