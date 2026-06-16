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
//
// Quantity/scaling math (D2/§4.4) is the proved invariant: every qpu/fleet multiply
// routes through the kernel cores @proofs.child_qpu / @proofs.fleet_quantity (I4)
// over the D16 boundary (export_f3_bom). Only catalog resolution, distinct-cage
// aggregation, and CSV rendering are impure Go (note §1.2).
package bom

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/components"
	"github.com/afewell-hh/aid/internal/topology"
)

// opticKeys are the bom.csv transceiver attribute columns 7–19 (note §3.2). They
// are AID-owned public optical facts read from the catalog calc_profile overlay.
var opticKeys = []string{
	"cage_type", "medium", "connector", "standard", "reach_class", "wavelength_nm",
	"host_lane_count", "host_serdes_gbps_per_lane", "optical_lane_pattern",
	"gearbox_present", "cable_assembly_type", "breakout_topology", "is_cable_assembly",
}

// ResolvedLine is one fleet-scaled line in the single resolved graph (the unit
// both renderers view). Membership flags (Projected / InFullBOM) make the two
// views filters of the same list; the per-cage transceiver lines are Projected but
// not InFullBOM (suppressed from the flat full BOM, surfaced by the projection).
type ResolvedLine struct {
	// Identity / classification.
	Kind          string // server|switch|nic|dpu|transceiver|bom_line
	Section       string // projection section: server|switch|nic|server_transceiver|switch_transceiver|""
	HedgehogClass string // projection col 4 (base devices only)
	Manufacturer  string
	Model         string // projection col 2 (module_type_model) / full-BOM identity
	PartNumber    string // SKU (full-BOM "SMC PN")
	Description   string

	// Full-BOM (Layer B) fields.
	Category        string // real-server-bom "Type" column
	Physical        bool
	Seq             int // declared owner order in real-server-bom.csv
	TotalCapacityGB string
	PowerW          string
	TotalPowerW     string

	// Quantity — fleet-scaled (qpu × instance count) via the proved cores.
	FleetQuantity int

	// Membership (one model, two views).
	Projected bool // surfaced in the 19-column projection?
	InFullBOM bool // surfaced in the flat full purchasable BOM?

	// Projection optic columns 7–19 (modules only).
	Optic map[string]string
}

// ResolvedModel is the single resolved object graph (note §2). Both renderers are
// pure functions of this value.
type ResolvedModel struct {
	Lines                        []ResolvedLine
	SuppressedCableAssemblyCount int // the bom.csv footer value (0 for xoc-64)
}

// --- the kernel BOM-scale boundary (every qpu/fleet multiply, D2/§4.4) ---------

type scaleNode struct {
	ParentIndex       int `json:"parent_index"`
	QuantityPerParent int `json:"quantity_per_parent"`
	PlanQuantity      int `json:"plan_quantity"`
}

// scaleViaKernel routes the flattened node tree through export_f3_bom so every
// qpu/fleet multiply is computed by the proven I4 cores (@proofs.child_qpu /
// @proofs.fleet_quantity) — the D2/§4.4 BOM-scaling invariant by construction. The
// kernel returns a compact JSON array of per-node fleet quantities, in node order.
func scaleViaKernel(nodes []scaleNode) ([]int, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	in, err := json.Marshal(struct {
		Nodes []scaleNode `json:"nodes"`
	}{nodes})
	if err != nil {
		return nil, fmt.Errorf("bom: marshal scale plan: %w", err)
	}
	kernel, err := components.Kernel()
	if err != nil {
		return nil, fmt.Errorf("bom: load kernel: %w", err)
	}
	out, err := kernel.Call(components.KernelF3Bom, in)
	if err != nil {
		return nil, fmt.Errorf("bom: kernel f3_bom: %w", err)
	}
	var fleets []int
	if err := json.Unmarshal(out, &fleets); err != nil {
		return nil, fmt.Errorf("bom: decode scaled fleets: %w", err)
	}
	if len(fleets) != len(nodes) {
		return nil, fmt.Errorf("bom: kernel returned %d fleets for %d nodes", len(fleets), len(nodes))
	}
	return fleets, nil
}

// builder accumulates resolved lines alongside the parallel kernel node tree.
type builder struct {
	lines  []ResolvedLine
	nodeOf []int // lines[i] takes its fleet from nodes[nodeOf[i]]; -1 ⇒ fleet preset
	nodes  []scaleNode
}

// node appends a scale node and returns its index (the parent_index children cite).
func (b *builder) node(parent, qpp, planQty int) int {
	b.nodes = append(b.nodes, scaleNode{ParentIndex: parent, QuantityPerParent: qpp, PlanQuantity: planQty})
	return len(b.nodes) - 1
}

// add records a line whose fleet comes from node `nodeIdx` (>=0) or is preset (-1).
func (b *builder) add(line ResolvedLine, nodeIdx int) {
	b.lines = append(b.lines, line)
	b.nodeOf = append(b.nodeOf, nodeIdx)
}

// Resolve is THE single resolver (the anti-drift gate, note §2): it builds one
// resolved object graph from the ingested plan, the catalog (with the AID-owned
// optic/line-template overlay merged in), and the F2 calc-output. Every qpu/fleet
// multiply routes through the kernel cores (scaleViaKernel); switch-side
// transceivers are derived by distinct-physical-cage aggregation over F2 endpoints
// plus plan-time mesh-zone cages (D6 plan-time; note §3.2).
func Resolve(plan *topology.Plan, cat *catalog.Catalog, calcOut *calc.CalcOutput) (*ResolvedModel, error) {
	if plan == nil || cat == nil || calcOut == nil {
		return nil, fmt.Errorf("bom: Resolve needs a plan, catalog and calc-output")
	}
	serverQty := qtyMap(calcOut.ServerQuantity)
	switchQty := qtyMap(calcOut.SwitchQuantity)
	b := &builder{}

	// --- servers: base device (projection) + bom_line_templates + nested slots +
	//     per-cage transceivers. ----------------------------------------------------
	for _, sc := range plan.Spec.ServerClasses {
		item, _ := cat.Get(sc.ClassRef.ID)
		sq := serverQty[sc.ServerClassID]
		if sq == 0 {
			sq = sc.Quantity
		}
		root := b.node(-1, 1, sq)
		b.add(ResolvedLine{
			Kind: catalog.KindServer, Section: "server", HedgehogClass: sc.ServerClassID,
			Manufacturer: item.Manufacturer, Model: item.Model, Projected: true,
		}, root)

		for _, t := range item.BOMLineTemplates {
			b.add(fullBOMLine(t, intAttr(t.Attributes, "seq")), b.node(root, qpiOf(t), sq))
		}

		slotNode := map[string]int{}
		for _, s := range item.ComponentSlots {
			child, _ := cat.Get(s.Target.ID)
			si := b.node(root, s.Quantity, sq)
			slotNode[s.SlotID] = si
			if child.Kind == catalog.KindNIC || child.Kind == catalog.KindDPU {
				b.add(ResolvedLine{
					Kind: child.Kind, Section: "nic", Manufacturer: child.Manufacturer,
					Model: child.Model, Description: child.Description, Projected: true, Optic: opticOf(child),
				}, si)
			}
			seq := intFrom(s.SelectionConstraints, "seq")
			for _, t := range child.BOMLineTemplates {
				b.add(fullBOMLine(t, seq), b.node(si, qpiOf(t), sq))
			}
		}

		// Per-cage transceivers (server side): one per binding, fleet = slotQty × sq.
		for _, cb := range item.CageBindings {
			tx, ok := cat.Get(cb.SelectedTransceiver.ID)
			if !ok {
				tx, _ = cat.ByName(cb.SelectedTransceiver.ID.Name)
			}
			parent, ok := slotNode[cb.NICSlotID]
			if !ok {
				parent = root
			}
			b.add(ResolvedLine{
				Kind: catalog.KindTransceiver, Section: "server_transceiver", Manufacturer: tx.Manufacturer,
				Model: tx.Model, Description: tx.Description, Projected: true, Optic: opticOf(tx),
			}, b.node(parent, 1, sq))
		}
	}

	// --- switches: base device rows (projection). --------------------------------
	for _, sw := range plan.Spec.SwitchClasses {
		item, _ := cat.Get(sw.ClassRef.ID)
		sq := switchQty[sw.SwitchClassID]
		b.add(ResolvedLine{
			Kind: catalog.KindSwitch, Section: "switch", HedgehogClass: sw.SwitchClassID,
			Manufacturer: item.Manufacturer, Model: item.Model, Projected: true,
		}, b.node(-1, 1, sq))
	}

	// --- route every node's qpu/fleet through the proven cores. ------------------
	res, err := scaleViaKernel(b.nodes)
	if err != nil {
		return nil, err
	}
	for i := range b.lines {
		if n := b.nodeOf[i]; n >= 0 {
			b.lines[i].FleetQuantity = res[n]
		}
	}

	// --- switch-side transceivers: distinct physical cages (endpoints) + mesh. ----
	b.lines = append(b.lines, switchTransceiverLines(plan, cat, calcOut, switchQty)...)

	model := &ResolvedModel{Lines: b.lines}
	// Suppress switch_transceiver cable-assembly lines from the projection; preserve
	// the count for the footer (0 for xoc-64 — no cable assemblies).
	for i := range model.Lines {
		l := &model.Lines[i]
		if l.Section == "switch_transceiver" && l.Optic["is_cable_assembly"] == "true" {
			model.SuppressedCableAssemblyCount++
			l.Projected = false
		}
	}
	return model, nil
}

// switchTransceiverLines derives the switch-side optics: one per DISTINCT physical
// switch cage from F2 endpoints (a breakout cage holds one optic, not one per
// logical port — note §3.2), PLUS the mesh-zone cages (mesh ports × switch
// quantity), which F2 does not emit as endpoints (mesh wiring is F4) but are
// plan-time derivable (D6). Aggregated per transceiver SKU; fleet = cage count.
func switchTransceiverLines(plan *topology.Plan, cat *catalog.Catalog, calcOut *calc.CalcOutput, switchQty map[string]int) []ResolvedLine {
	zoneOptic := map[string]string{}
	for _, z := range plan.Spec.PortZones {
		zoneOptic[z.SwitchClassID+"/"+z.ZoneName] = z.Transceiver
	}

	type cage struct {
		sc, zone string
		si, pp   int
	}
	seen := map[cage]bool{}
	for _, e := range calcOut.Endpoints {
		seen[cage{e.SwitchClassID, e.Zone, e.SwitchIndex, e.PortSlot.PhysicalPort}] = true
	}
	countByTx := map[string]int{}
	for c := range seen {
		countByTx[zoneOptic[c.sc+"/"+c.zone]]++
	}
	// Mesh zones: every mesh port on every switch carries an optic.
	for _, z := range plan.Spec.PortZones {
		if z.ZoneType == "mesh" {
			countByTx[z.Transceiver] += countPorts(z.PortSpec) * switchQty[z.SwitchClassID]
		}
	}

	// Clos fabric-link cages (F6 §2.5): every leaf↔spine link carries an optic on
	// BOTH ends. F2 does not emit fabric ports as endpoints (like mesh, they are
	// plan-time derivable). Leaf side: every uplink-zone port on every leaf is
	// populated → switch_qty × uplink-port-count. Spine side (link-derived, so a
	// spare/under-subscribed spine downlink port is NOT counted): the number of
	// links landing in the fabric == the leaf uplink total for that fabric.
	roleByClass := map[string]string{}
	fabricByClass := map[string]string{}
	fabricHasSpine := map[string]bool{}
	for _, sw := range plan.Spec.SwitchClasses {
		roleByClass[sw.SwitchClassID] = sw.HedgehogRole
		fabricByClass[sw.SwitchClassID] = sw.FabricName
		if sw.HedgehogRole == "spine" {
			fabricHasSpine[sw.FabricName] = true
		}
	}
	isLeaf := func(role string) bool { return role == "server-leaf" || role == "border-leaf" }
	leafUplinksByFabric := map[string]int{} // fabric → Σ leaf_qty × uplink ports (== link count)
	for _, z := range plan.Spec.PortZones {
		// Only a Clos fabric (one with a spine) carries leaf↔spine uplink optics; a
		// leaf uplink zone in a spine-less (mesh) fabric is not a fabric link.
		if z.ZoneType != "uplink" || !isLeaf(roleByClass[z.SwitchClassID]) || !fabricHasSpine[fabricByClass[z.SwitchClassID]] {
			continue
		}
		n := countPorts(z.PortSpec) * switchQty[z.SwitchClassID]
		countByTx[z.Transceiver] += n
		leafUplinksByFabric[fabricByClass[z.SwitchClassID]] += n
	}
	// Spine downlink cages: one optic per link landing on the spine's fabric zone.
	spineFabricCounted := map[string]bool{} // one attribution per (spine class) fabric zone SKU
	for _, z := range plan.Spec.PortZones {
		if z.ZoneType != "fabric" || roleByClass[z.SwitchClassID] != "spine" {
			continue
		}
		if spineFabricCounted[z.SwitchClassID] {
			continue
		}
		spineFabricCounted[z.SwitchClassID] = true
		countByTx[z.Transceiver] += leafUplinksByFabric[fabricByClass[z.SwitchClassID]]
	}

	var out []ResolvedLine
	for txID, n := range countByTx {
		if txID == "" || n == 0 {
			continue
		}
		tx, _ := cat.ByName(txID)
		out = append(out, ResolvedLine{
			Kind: catalog.KindTransceiver, Section: "switch_transceiver", Manufacturer: tx.Manufacturer,
			Model: tx.Model, Description: tx.Description, FleetQuantity: n, Projected: true, Optic: opticOf(tx),
		})
	}
	return out
}

// --- RenderFullBOM (Layer B) — pure view of the model -------------------------

// fullBOMColumns is the real-server-bom.csv header.
var fullBOMColumns = []string{"Type", "SMC PN", "Desc", "QTY", "Total Capacity(GB)", "Power(W)", "Total Power(W)"}

// RenderFullBOM renders the full purchasable BOM (Layer B → real-server-bom.csv):
// header + a blank spacer row + every InFullBOM line in declared (Seq) order, with
// per-cage transceiver (kind=transceiver) lines suppressed. Takes ONLY the model
// (the signature is the anti-drift gate).
func RenderFullBOM(m *ResolvedModel) ([][]string, error) {
	if m == nil {
		return nil, fmt.Errorf("bom: RenderFullBOM needs a model")
	}
	var lines []ResolvedLine
	for _, l := range m.Lines {
		if l.InFullBOM {
			lines = append(lines, l)
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Seq < lines[j].Seq })

	rows := [][]string{append([]string(nil), fullBOMColumns...)}
	rows = append(rows, make([]string, len(fullBOMColumns))) // blank spacer row
	for _, l := range lines {
		rows = append(rows, []string{
			l.Category, l.PartNumber, l.Description, strconv.Itoa(l.FleetQuantity),
			l.TotalCapacityGB, l.PowerW, l.TotalPowerW,
		})
	}
	return rows, nil
}

// --- RenderProjection (Layer A) — pure view of the SAME model -----------------

var projectionColumns = []string{
	"section", "module_type_model", "module_type_description", "hedgehog_class", "manufacturer", "quantity",
	"cage_type", "medium", "connector", "standard", "reach_class", "wavelength_nm", "host_lane_count",
	"host_serdes_gbps_per_lane", "optical_lane_pattern", "gearbox_present", "cable_assembly_type",
	"breakout_topology", "is_cable_assembly",
}

// HNP section ordering (bom_export.py:26-27): base devices first, then modules.
var deviceSectionOrder = map[string]int{"server": 0, "switch": 1}
var moduleSectionOrder = map[string]int{"nic": 0, "server_transceiver": 1, "switch_transceiver": 2}

// aggRow accumulates a projection row by its grouping key.
type aggRow struct {
	section, model, description, hedgehogClass, manufacturer string
	quantity                                                 int
	optic                                                    map[string]string
}

// RenderProjection renders the HNP 19-column projection (Layer A → bom.csv): the
// SAME model filtered to HNP-physical sections and regrouped — base device rows
// (server/switch) sorted by (section, hedgehog_class, manufacturer, model), then
// module rows (nic/server_transceiver/switch_transceiver) sorted by (section,
// manufacturer, model) — with the suppressed-cable-assembly footer. Takes ONLY the
// model (the signature is the anti-drift gate).
func RenderProjection(m *ResolvedModel) ([][]string, error) {
	if m == nil {
		return nil, fmt.Errorf("bom: RenderProjection needs a model")
	}
	bases := map[string]*aggRow{}
	mods := map[string]*aggRow{}
	for _, l := range m.Lines {
		if !l.Projected {
			continue
		}
		switch l.Section {
		case "server", "switch":
			k := strings.Join([]string{l.Section, l.HedgehogClass, l.Manufacturer, l.Model}, "\x00")
			acc := bases[k]
			if acc == nil {
				acc = &aggRow{section: l.Section, model: l.Model, hedgehogClass: l.HedgehogClass, manufacturer: l.Manufacturer}
				bases[k] = acc
			}
			acc.quantity += l.FleetQuantity
		case "nic", "server_transceiver", "switch_transceiver":
			k := strings.Join([]string{l.Section, l.Manufacturer, l.Model}, "\x00")
			acc := mods[k]
			if acc == nil {
				acc = &aggRow{section: l.Section, model: l.Model, description: l.Description, manufacturer: l.Manufacturer, optic: l.Optic}
				mods[k] = acc
			}
			acc.quantity += l.FleetQuantity
		}
	}

	baseRows := make([]*aggRow, 0, len(bases))
	for _, r := range bases {
		baseRows = append(baseRows, r)
	}
	sort.SliceStable(baseRows, func(i, j int) bool {
		a, b := baseRows[i], baseRows[j]
		if deviceSectionOrder[a.section] != deviceSectionOrder[b.section] {
			return deviceSectionOrder[a.section] < deviceSectionOrder[b.section]
		}
		if a.hedgehogClass != b.hedgehogClass {
			return a.hedgehogClass < b.hedgehogClass
		}
		if a.manufacturer != b.manufacturer {
			return a.manufacturer < b.manufacturer
		}
		return a.model < b.model
	})

	modRows := make([]*aggRow, 0, len(mods))
	for _, r := range mods {
		modRows = append(modRows, r)
	}
	sort.SliceStable(modRows, func(i, j int) bool {
		a, b := modRows[i], modRows[j]
		if moduleSectionOrder[a.section] != moduleSectionOrder[b.section] {
			return moduleSectionOrder[a.section] < moduleSectionOrder[b.section]
		}
		if a.manufacturer != b.manufacturer {
			return a.manufacturer < b.manufacturer
		}
		return a.model < b.model
	})

	rows := [][]string{append([]string(nil), projectionColumns...)}
	for _, r := range baseRows {
		row := make([]string, len(projectionColumns))
		row[0], row[1], row[2], row[3], row[4], row[5] = r.section, r.model, "", r.hedgehogClass, r.manufacturer, strconv.Itoa(r.quantity)
		for i := 6; i < len(row); i++ {
			row[i] = ""
		}
		row[len(row)-1] = "false" // is_cable_assembly
		rows = append(rows, row)
	}
	for _, r := range modRows {
		row := make([]string, len(projectionColumns))
		row[0], row[1], row[2], row[3], row[4], row[5] = r.section, r.model, r.description, "", r.manufacturer, strconv.Itoa(r.quantity)
		for i, k := range opticKeys {
			row[6+i] = r.optic[k]
		}
		rows = append(rows, row)
	}
	rows = append(rows, []string{"# suppressed_switch_cable_assembly_count", strconv.Itoa(m.SuppressedCableAssemblyCount)})
	return rows, nil
}

// --- helpers ------------------------------------------------------------------

func qtyMap(qs []calc.ClassQty) map[string]int {
	m := make(map[string]int, len(qs))
	for _, q := range qs {
		m[q.ClassID] = q.Quantity
	}
	return m
}

func qpiOf(t catalog.BOMLineTemplate) int {
	if t.QuantityPerInstance < 1 {
		return 1
	}
	return t.QuantityPerInstance
}

// fullBOMLine builds a full-BOM line from a bom_line_template; seq orders it in the
// owner CSV (nested component lines inherit their slot's seq).
func fullBOMLine(t catalog.BOMLineTemplate, seq int) ResolvedLine {
	sku := t.InlineSKU
	return ResolvedLine{
		Kind:            "bom_line",
		Category:        t.Category,
		PartNumber:      sku,
		Description:     strAttr(t.Attributes, "description"),
		Physical:        t.Physical,
		Seq:             seq,
		TotalCapacityGB: strAttr(t.Attributes, "total_capacity_gb"),
		PowerW:          strAttr(t.Attributes, "power_w"),
		TotalPowerW:     strAttr(t.Attributes, "total_power_w"),
		InFullBOM:       true,
	}
}

// opticOf extracts the 13 optic columns (cols 7–19) from an item's calc_profile.
func opticOf(it catalog.Item) map[string]string {
	out := make(map[string]string, len(opticKeys))
	for _, k := range opticKeys {
		if v, ok := it.CalcProfile[k]; ok {
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

func strAttr(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[k]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func intAttr(m map[string]any, k string) int {
	if m == nil {
		return 0
	}
	return asInt(m[k])
}

func intFrom(m map[string]any, k string) int {
	if m == nil {
		return 0
	}
	return asInt(m[k])
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

// countPorts counts the physical ports a port_spec names: comma-separated single
// ports or `A-B` / `A-B:step` ranges (the diet zone grammar).
func countPorts(spec string) int {
	n := 0
	for _, tok := range strings.Split(spec, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		rng := tok
		step := 1
		if i := strings.IndexByte(tok, ':'); i >= 0 {
			rng = tok[:i]
			if s, err := strconv.Atoi(tok[i+1:]); err == nil && s > 0 {
				step = s
			}
		}
		if i := strings.IndexByte(rng, '-'); i >= 0 {
			a, e1 := strconv.Atoi(strings.TrimSpace(rng[:i]))
			b, e2 := strconv.Atoi(strings.TrimSpace(rng[i+1:]))
			if e1 == nil && e2 == nil && b >= a {
				n += (b-a)/step + 1
			}
			continue
		}
		if _, err := strconv.Atoi(rng); err == nil {
			n++
		}
	}
	return n
}
