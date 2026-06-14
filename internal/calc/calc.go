// Package calc is the Go side of the F2 calculation boundary
// (docs/foundation-redesign.md §5 F2; docs/foundation/f2-architecture-note.md §1.1;
// Issue #52). It resolves an ingested topology plan + catalog into the FLAT,
// NUMERIC, catalog-resolved calc-plan, hands it to the MoonBit kernel over the
// existing D16 wasmhost JSON boundary, and decodes the calc-output (computed
// switch/server quantities + per-realized-endpoint allocation IR).
//
// The split (D16): Go resolves every catalog-dependent scalar (notably
// breakout_option → logical_ports and the optic attribute_data for both ends);
// the kernel parses the raw port_spec string and stays catalog-free. The boundary
// preserves full per-connection identity in and emits one record per realized
// endpoint out (devb review finding 1). Mesh-link pairing is deferred to F4
// (note §2.6); only the 2-or-3 structural gate stays in F2.
//
// F2 RED: the wire types are the approved contract; the producer is a stub. GREEN
// builds the calc-plan, calls the kernel, and decodes the result.
package calc

import (
	"errors"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
)

// ErrNotImplemented marks the F2 calc as not yet wired (RED). GREEN removes it.
var ErrNotImplemented = errors.New("calc: F2 kernel calculation not implemented (RED)")

// --- calc-plan (Go → kernel): the resolved, numeric input -------------------

// XcvrAttrs is the optic attribute_data Go resolves from the catalog and passes
// to the kernel so it can verdict transceivers WITHOUT touching the catalog (note
// §1.1, §2.5). The fields are exactly the keys the verdict rules read: Medium
// (mismatch BLOCKS), CageType / Connector (mismatch → needs_review). A nil
// *XcvrAttrs means the end carries no optic intent. (Cable-assembly far_end_*
// attrs extend this in GREEN if a fixture needs them.)
type XcvrAttrs struct {
	Medium    string `json:"medium"`
	CageType  string `json:"cage_type"`
	Connector string `json:"connector"`
}

// ZoneIn is a port-allocation zone. PortSpec is the RAW string the kernel parses
// (comma-list + range + start-end:step); BreakoutLogicalPorts is resolved by Go
// from the catalog breakout_option. TransceiverAttrs is the zone-side (switch-end)
// optic, resolved by Go (note §1.1).
type ZoneIn struct {
	ZoneName             string     `json:"zone_name"`
	ZoneType             string     `json:"zone_type"`
	PortSpec             string     `json:"port_spec"`
	BreakoutLogicalPorts int        `json:"breakout_logical_ports"`
	AllocationStrategy   string     `json:"allocation_strategy"`
	TransceiverAttrs     *XcvrAttrs `json:"transceiver_attrs,omitempty"`
}

// SwitchClassIn carries the derivation inputs. OverrideQuantity is nil on the
// derived path; Redundancy ∈ {none,mclag,eslag}; TopologyMode ∈ {clos,mesh}.
type SwitchClassIn struct {
	SwitchClassID    string   `json:"switch_class_id"`
	OverrideQuantity *int     `json:"override_quantity,omitempty"`
	Redundancy       string   `json:"redundancy"`
	TopologyMode     string   `json:"topology_mode"`
	Zones            []ZoneIn `json:"zones"`
}

type ServerClassIn struct {
	ServerClassID string `json:"server_class_id"`
	Quantity      int    `json:"quantity"`
}

// ConnIn carries the FULL per-connection identity (devb finding 1): ConnectionID,
// NICSlotID, PortIndex and Speed are preserved so alternating (which keys on
// port_index) and per-instance fan-out are faithfully re-derivable. Rail is nil
// unless Distribution == "rail-optimized". ServerTransceiverAttrs is the server-end
// optic Go resolved from the catalog; it is paired against the target zone's
// TransceiverAttrs to produce a transceiver verdict (§2.5).
type ConnIn struct {
	ConnectionID           string     `json:"connection_id"`
	ServerClassID          string     `json:"server_class_id"`
	ServerQuantity         int        `json:"server_quantity"`
	NICSlotID              string     `json:"nic_slot_id"`
	PortIndex              int        `json:"port_index"`
	PortsPerConnection     int        `json:"ports_per_connection"`
	Speed                  int        `json:"speed"`
	Distribution           string     `json:"distribution"`
	Rail                   *int       `json:"rail,omitempty"`
	TargetSwitchClass      string     `json:"target_switch_class"`
	TargetZone             string     `json:"target_zone"`
	ServerTransceiverAttrs *XcvrAttrs `json:"server_transceiver_attrs,omitempty"`
}

// CalcPlan is the full kernel input.
type CalcPlan struct {
	SwitchClasses []SwitchClassIn `json:"switch_classes"`
	ServerClasses []ServerClassIn `json:"server_classes"`
	Connections   []ConnIn        `json:"connections"`
}

// --- calc-output (kernel → Go): quantities + per-endpoint IR ----------------

// PortSlot is one allocated switch port. BreakoutIndex is nil for a non-breakout
// port (Name "E1/{port}") or the lane 1..N for a breakout port ("E1/{port}/{lane}").
type PortSlot struct {
	PhysicalPort  int    `json:"physical_port"`
	BreakoutIndex *int   `json:"breakout_index,omitempty"`
	Name          string `json:"name"`
}

// Endpoint names which server INSTANCE got which switch INSTANCE and port slot.
type Endpoint struct {
	ServerClassID string   `json:"server_class_id"`
	ServerIndex   int      `json:"server_index"`
	ConnectionID  string   `json:"connection_id"`
	NICSlotID     string   `json:"nic_slot_id"`
	PortIndex     int      `json:"port_index"`
	SwitchClassID string   `json:"switch_class_id"`
	SwitchIndex   int      `json:"switch_index"`
	Zone          string   `json:"zone"`
	PortSlot      PortSlot `json:"port_slot"`
}

type ClassQty struct {
	ClassID  string `json:"class_id"`
	Quantity int    `json:"quantity"`
}

type Verdict struct {
	ConnectionID string `json:"connection_id"`
	Outcome      string `json:"outcome"`
	ReasonCode   string `json:"reason_code"`
}

// CalcOutput is the full kernel result.
type CalcOutput struct {
	SwitchQuantity      []ClassQty `json:"switch_quantity"`
	ServerQuantity      []ClassQty `json:"server_quantity"`
	Endpoints           []Endpoint `json:"endpoints"`
	TransceiverVerdicts []Verdict  `json:"transceiver_verdicts"`
}

// DeriveQuantities resolves the plan+catalog into a calc-plan, runs the kernel,
// and returns the computed per-class switch and server quantities (keyed by class
// id) — the F2 derived-quantities oracle target. F2 RED: stub. GREEN builds the
// calc-plan (BuildCalcPlan), calls components.Kernel via wasmhost, and projects
// CalcOutput.SwitchQuantity/ServerQuantity into the maps.
func DeriveQuantities(plan *topology.Plan, cat *catalog.Catalog) (switchQty, serverQty map[string]int, err error) {
	return nil, nil, ErrNotImplemented
}

// BuildCalcPlan resolves the ingested model + catalog into the flat numeric
// calc-plan. F2 RED: stub. GREEN performs the catalog resolution (breakout →
// logical_ports, optic attribute_data) described in note §1.1.
func BuildCalcPlan(plan *topology.Plan, cat *catalog.Catalog) (CalcPlan, error) {
	return CalcPlan{}, ErrNotImplemented
}
