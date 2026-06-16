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
	"encoding/json"
	"fmt"
	"sort"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/components"
	"github.com/afewell-hh/aid/internal/topology"
)

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
// FabricName + Role are the Clos spine-derivation keys (F6, Issue #63): the
// kernel groups leaves (Role ∈ {server-leaf,border-leaf}) under each spine
// (Role == "spine") sharing the same FabricName. Both are "" for mesh plans
// that do not derive spine counts.
type SwitchClassIn struct {
	SwitchClassID    string   `json:"switch_class_id"`
	FabricName       string   `json:"fabric_name"`
	Role             string   `json:"role"`
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

// CalcIssue is a calc-level failure surfaced by the kernel (the analogue of
// HNP's allocator raising — port_allocator.py:57-67). A non-empty Errors makes
// DeriveQuantities fail the calc rather than return a silently-wrong result.
type CalcIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CalcOutput is the full kernel result.
type CalcOutput struct {
	SwitchQuantity      []ClassQty  `json:"switch_quantity"`
	ServerQuantity      []ClassQty  `json:"server_quantity"`
	Endpoints           []Endpoint  `json:"endpoints"`
	TransceiverVerdicts []Verdict   `json:"transceiver_verdicts"`
	Errors              []CalcIssue `json:"errors"`
}

// Compute resolves the plan+catalog into a calc-plan, runs the kernel, and
// returns the FULL decoded calc-output (per-class switch/server quantities, the
// per-realized-endpoint allocation IR, and transceiver verdicts). It is the
// shared kernel-call path: DeriveQuantities projects its quantities, and the F3
// BOM reducer (internal/bom) consumes its Endpoints for the switch-transceiver
// projection. This is an additive accessor over the SAME F2 boundary/output — no
// F2 boundary change (note §7.4). A kernel-reported calc error fails the call,
// mirroring HNP's allocator raise.
func Compute(plan *topology.Plan, cat *catalog.Catalog) (*CalcOutput, error) {
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
	// The kernel surfaces over-allocation (and a malformed plan) as calc errors
	// rather than wrapping/reusing ports; fail the calc, mirroring HNP's raise.
	if len(co.Errors) > 0 {
		return nil, fmt.Errorf("calc: kernel reported %d error(s): [%s] %s",
			len(co.Errors), co.Errors[0].Code, co.Errors[0].Message)
	}
	return &co, nil
}

// DeriveQuantities resolves the plan+catalog into a calc-plan, runs the kernel,
// and returns the computed per-class switch and server quantities (keyed by class
// id) — the F2 derived-quantities oracle target. It projects Compute's
// CalcOutput.SwitchQuantity/ServerQuantity into the maps.
func DeriveQuantities(plan *topology.Plan, cat *catalog.Catalog) (switchQty, serverQty map[string]int, err error) {
	co, err := Compute(plan, cat)
	if err != nil {
		return nil, nil, err
	}
	switchQty = make(map[string]int, len(co.SwitchQuantity))
	for _, q := range co.SwitchQuantity {
		switchQty[q.ClassID] = q.Quantity
	}
	serverQty = make(map[string]int, len(co.ServerQuantity))
	for _, q := range co.ServerQuantity {
		serverQty[q.ClassID] = q.Quantity
	}
	return switchQty, serverQty, nil
}

// BuildCalcPlan resolves the ingested model + catalog into the flat, numeric
// calc-plan: Go performs every catalog-dependent resolution (note §1.1) —
// breakout_option → logical_ports, the redundancy mode, and the optic
// attribute_data for both ends — so the kernel consumes typed numbers plus the
// raw port_spec string it parses, and never touches the catalog (D16).
func BuildCalcPlan(plan *topology.Plan, cat *catalog.Catalog) (CalcPlan, error) {
	if plan == nil || cat == nil {
		return CalcPlan{}, fmt.Errorf("calc: BuildCalcPlan needs a plan and a catalog")
	}

	// server_classes (and the per-class quantity used for connection fan-out).
	serverQtyByClass := make(map[string]int, len(plan.Spec.ServerClasses))
	cp := CalcPlan{}
	for _, sc := range plan.Spec.ServerClasses {
		serverQtyByClass[sc.ServerClassID] = sc.Quantity
		cp.ServerClasses = append(cp.ServerClasses, ServerClassIn{
			ServerClassID: sc.ServerClassID,
			Quantity:      sc.Quantity,
		})
	}

	// Redundancy domains keyed by switch class (MCLAG/ESLAG); absent ⇒ "none".
	redundancyByClass := map[string]string{}
	for _, d := range plan.Spec.MCLAGDomains {
		if d.RedundancyType != "" {
			redundancyByClass[d.SwitchClassID] = d.RedundancyType
		}
	}

	// Zones grouped by switch class, with the catalog-resolved scalars.
	zonesByClass := map[string][]ZoneIn{}
	for _, z := range plan.Spec.PortZones {
		zonesByClass[z.SwitchClassID] = append(zonesByClass[z.SwitchClassID], ZoneIn{
			ZoneName:             z.ZoneName,
			ZoneType:             z.ZoneType,
			PortSpec:             z.PortSpec,
			BreakoutLogicalPorts: breakoutLogicalPorts(cat, z.BreakoutID),
			AllocationStrategy:   z.Allocation,
			TransceiverAttrs:     resolveXcvr(cat, z.Transceiver),
		})
	}

	// switch_classes. Redundancy prefers the inline class field (the real
	// training form carries redundancy_type directly on the switch class — xoc-256
	// clos-ro, Issue #63); the separate mclag_domains section is the fallback for
	// older fixtures.
	for _, sw := range plan.Spec.SwitchClasses {
		redundancy := sw.RedundancyType
		if redundancy == "" {
			redundancy = redundancyByClass[sw.SwitchClassID]
		}
		if redundancy == "" {
			redundancy = "none"
		}
		mode := sw.TopologyMode
		if mode == "" {
			mode = "clos"
		}
		cp.SwitchClasses = append(cp.SwitchClasses, SwitchClassIn{
			SwitchClassID:    sw.SwitchClassID,
			FabricName:       sw.FabricName,
			Role:             sw.HedgehogRole,
			OverrideQuantity: sw.OverrideQuantity,
			Redundancy:       redundancy,
			TopologyMode:     mode,
			Zones:            zonesByClass[sw.SwitchClassID],
		})
	}

	// connections — full per-connection identity, plus the server-end optic.
	for _, c := range plan.Spec.Connections {
		cp.Connections = append(cp.Connections, ConnIn{
			ConnectionID:           c.ConnectionID,
			ServerClassID:          c.ServerClassID,
			ServerQuantity:         serverQtyByClass[c.ServerClassID],
			NICSlotID:              c.NICSlotID,
			PortIndex:              c.PortIndex,
			PortsPerConnection:     c.PortsPerConnection,
			Speed:                  c.Speed,
			Distribution:           c.Distribution,
			Rail:                   c.Rail,
			TargetSwitchClass:      c.TargetSwitchClass,
			TargetZone:             c.TargetZone,
			ServerTransceiverAttrs: resolveXcvr(cat, c.TransceiverID),
		})
	}

	// Allocation-order fidelity (Issue #61, fix 1): the kernel's per-(switch,zone)
	// port cursor consumes server classes in HNP's order —
	// PlanServerClass.Meta.ordering = ['plan','server_class_id'], walked by
	// device_generator._create_connections. Reproduce it by STABLE-sorting the
	// connections (and the server-class echo) by server_class_id; the stable sort
	// preserves each class's intra-class connection order (rail-0..rail-N). This is
	// feed-order only — quantities and counts are unaffected (the kernel
	// reorders consumption to server-outer/rail-inner to match HNP, Issue #61 fix 2).
	sort.SliceStable(cp.ServerClasses, func(i, j int) bool {
		return cp.ServerClasses[i].ServerClassID < cp.ServerClasses[j].ServerClassID
	})
	sort.SliceStable(cp.Connections, func(i, j int) bool {
		return cp.Connections[i].ServerClassID < cp.Connections[j].ServerClassID
	})

	return cp, nil
}

// breakoutLogicalPorts resolves a breakout_option id to its logical_ports count
// from the catalog's retained reference_data block (the breakout_options are
// reference data, not extracted into typed catalog items). An empty id or an
// unknown breakout resolves to 1 (a non-breakout 1:1 port), matching the HNP
// default of one logical port per physical port.
func breakoutLogicalPorts(cat *catalog.Catalog, breakoutID string) int {
	if breakoutID == "" {
		return 1
	}
	rd := cat.ReferenceData()
	opts, ok := rd["breakout_options"].([]any)
	if !ok {
		return 1
	}
	for _, o := range opts {
		m, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := m["id"].(string); id == breakoutID {
			if lp, ok := asInt(m["logical_ports"]); ok && lp > 0 {
				return lp
			}
			return 1
		}
	}
	return 1
}

// resolveXcvr resolves an optic (transceiver) selection to the kernel's
// catalog-free XcvrAttrs — the medium/cage_type/connector the §2.5 verdict rules
// read. The attribute keys are read from the catalog item's calc_profile if
// present; an unresolved optic or one carrying no optic attribute_data yields nil
// (no optic intent), which the kernel verdicts as a match. For xoc-64 the
// module_types carry no such attributes, so both ends resolve to nil.
func resolveXcvr(cat *catalog.Catalog, transceiverID string) *XcvrAttrs {
	if transceiverID == "" {
		return nil
	}
	item, ok := cat.ByName(transceiverID)
	if !ok {
		return nil
	}
	medium, _ := item.CalcProfile["medium"].(string)
	cage, _ := item.CalcProfile["cage_type"].(string)
	connector, _ := item.CalcProfile["connector"].(string)
	if medium == "" && cage == "" && connector == "" {
		return nil
	}
	return &XcvrAttrs{Medium: medium, CageType: cage, Connector: connector}
}

// asInt coerces a JSON/YAML-bridged numeric (float64/int/int64) to an int.
func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
