// Package topology is the relational topology-plan model of record
// (docs/foundation-redesign.md §4.1, §4.3, D18/D21). A plan is POINTERS + INTENT:
// it references catalog server/switch CLASSES by pinned ID and carries the
// plan-specific parts — quantity per class and per-NIC-PORT connection intent.
// It does NOT embed inventory detail (that lives in the separate catalog).
//
// The plan schema is the real diet/XOC 9-section shape PLUS a spec/status plane
// (Kubernetes-style): `Spec` is authored input; `Status`/`Expected` is computed
// values populated after running. A plan with inputs only is a valid input; the
// same plan with expected values populated is a self-checking test oracle.
// Status/expected NEVER drives production calculation (guardrail 3) — it is read
// only in an explicit self-check mode.
//
// F0 builds the types + ingest stubs; no calculation (calc is F2+).
package topology

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
)

// classVersion is the version AID pins for classes synthesized from an external
// bundled plan (which carries friendly ids but no explicit version). Pinning a
// concrete version satisfies guardrail 1: every ref the ingest emits is
// reproducible (id+version), never a bare mutable name.
const classVersion = "1"

// Distinguishable validation errors (the F0 contract). GREEN returns these; the
// RED tests assert them so a trivial implementation cannot pass.
var (
	// ErrUnpinnedRef: a catalog reference lacks a pinned id+version (guardrail 1).
	ErrUnpinnedRef = errors.New("topology: catalog ref is not pinned (id+version required)")
	// ErrUnresolvedRef: a catalog reference does not resolve to a catalog object.
	ErrUnresolvedRef = errors.New("topology: catalog ref does not resolve")
	// ErrInvalidPlan: the plan is structurally invalid.
	ErrInvalidPlan = errors.New("topology: invalid plan")
	// ErrInsufficientPorts: a connection needs more ports than the class provides (guardrail 4).
	ErrInsufficientPorts = errors.New("topology: connection requires more ports than the configured class provides")
	// ErrSelfCheckMismatch: computed counts diverge from the plan's expected counts (D21 self-check).
	ErrSelfCheckMismatch = errors.New("topology: self-check failed (computed counts != expected)")
	// ErrNoExpected: a self-check was requested but the plan carries no expected block.
	ErrNoExpected = errors.New("topology: plan has no expected counts to self-check against")
)

// Plan is the top-level document: spec (inputs) + optional status/expected.
type Plan struct {
	Meta   Meta    `json:"meta"`
	Spec   Spec    `json:"spec"`
	Status *Status `json:"status,omitempty"` // computed/expected; self-check mode only
}

// Meta identifies the plan.
type Meta struct {
	CaseID  string `json:"case_id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// Spec is the authored input: class references + topology intent.
type Spec struct {
	Name          string             `json:"name"`
	ServerClasses []ServerClassUse   `json:"server_classes"`
	SwitchClasses []SwitchClassUse   `json:"switch_classes"`
	PortZones     []SwitchPortZone   `json:"switch_port_zones"`
	Connections   []ServerConnection `json:"server_connections"`
	MeshLinks     []MeshLink         `json:"mesh_links,omitempty"`
	MCLAGDomains  []MCLAGDomain      `json:"mclag_domains,omitempty"`
}

// ServerClassUse references a catalog server CLASS (by pinned ID) and gives the
// plan-specific quantity. The class itself (build, NICs, baked transceivers)
// lives in the catalog.
type ServerClassUse struct {
	ServerClassID string          `json:"server_class_id"`
	ClassRef      objectmodel.Ref `json:"class_ref"` // pinned identity + version/digest
	Quantity      int             `json:"quantity"`
}

// SwitchClassUse references a catalog switch CLASS. Switch quantity is DERIVED
// later (F2); override_quantity is the plan-level override.
type SwitchClassUse struct {
	SwitchClassID string          `json:"switch_class_id"`
	ClassRef      objectmodel.Ref `json:"class_ref"`
	FabricName    string          `json:"fabric_name"`
	// FabricClass (managed|unmanaged) gates which fabrics F4 renders as hhfab
	// wiring; HedgehogRole is the Switch CRD spec.role. Both are read-only
	// plan-intent the F4 renderer keys on (note §2.1.1) — model-correct sources
	// rather than xoc-64-inferred constants. Re-emitted by Rebundle.
	FabricClass      string `json:"fabric_class,omitempty"`
	HedgehogRole     string `json:"hedgehog_role,omitempty"`
	OverrideQuantity *int   `json:"override_quantity,omitempty"`
	TopologyMode     string `json:"topology_mode,omitempty"` // spine-leaf | mesh
}

// SwitchPortZone is a port-allocation zone on a switch class.
type SwitchPortZone struct {
	SwitchClassID string `json:"switch_class"`
	ZoneName      string `json:"zone_name"`
	ZoneType      string `json:"zone_type"` // server|uplink|mclag|peer|session|oob|fabric|mesh
	PortSpec      string `json:"port_spec"` // ranges + comma-lists + breakout steps
	BreakoutID    string `json:"breakout_option,omitempty"`
	Allocation    string `json:"allocation_strategy,omitempty"`
	Priority      int    `json:"priority,omitempty"`
	// Transceiver is the zone-level optic (real-format transceiver_module_type),
	// resolved against the catalog on ingest. Kept on the zone (not a class
	// binding) because it is a switch-side, per-zone selection.
	Transceiver string `json:"transceiver_module_type,omitempty"`
}

// ServerConnection is connection intent keyed on (nic, port_index) → zone. A
// NIC's multiple ports may target different zones, so the granularity is the
// PORT, not the NIC. The connection-level transceiver (real-format input) is
// resolved into the server class's cage binding on ingest.
type ServerConnection struct {
	ServerClassID      string `json:"server_class"`
	ConnectionID       string `json:"connection_id"`
	ConnectionName     string `json:"connection_name,omitempty"`
	NICSlotID          string `json:"nic"`
	PortIndex          int    `json:"port_index"`
	PortsPerConnection int    `json:"ports_per_connection"`
	ConnType           string `json:"hedgehog_conn_type,omitempty"` // unbundled|bundled|mclag|eslag
	Distribution       string `json:"distribution,omitempty"`       // same-switch|alternating|rail-optimized
	TargetSwitchClass  string `json:"target_switch_class"`          // from target_zone "class/zone"
	TargetZone         string `json:"target_zone_name"`
	Speed              int    `json:"speed"`
	Rail               *int   `json:"rail,omitempty"`
	PortType           string `json:"port_type,omitempty"`
	// TransceiverID is the real-format connection-level optic selection; ingest
	// resolves it into the class CageBinding (it is not authoritative on the plan).
	TransceiverID string `json:"transceiver_module_type,omitempty"`
}

// MeshLink is a point-to-point mesh link between two switch classes.
type MeshLink struct {
	FabricName   string `json:"fabric_name"`
	SwitchClassA string `json:"switch_class_a"`
	SwitchClassB string `json:"switch_class_b"`
	LinkIndex    int    `json:"link_index"`
}

// MCLAGDomain is an MCLAG/ESLAG redundancy domain.
type MCLAGDomain struct {
	DomainID        string `json:"domain_id"`
	SwitchClassID   string `json:"switch_class"`
	SwitchGroupName string `json:"switch_group_name"`
	RedundancyType  string `json:"redundancy_type"` // mclag|eslag
}

// Status holds computed/expected values (Kubernetes-style status). Read only in
// self-check mode (guardrail 3); never an input to production calculation.
type Status struct {
	Expected *Expected `json:"expected,omitempty"`
	Computed *Computed `json:"computed,omitempty"`
}

// Expected is the author-supplied oracle for self-check (generalizes the real
// format's expected.counts).
type Expected struct {
	Counts         Counts         `json:"counts"`
	SwitchQuantity map[string]int `json:"switch_quantity,omitempty"` // per switch_class_id
}

// Computed is what AID populated (F2+ fills it; empty in F0).
type Computed struct {
	Counts         Counts         `json:"counts"`
	SwitchQuantity map[string]int `json:"switch_quantity,omitempty"`
}

// Counts mirrors the real format's expected.counts.
type Counts struct {
	ServerClasses int `json:"server_classes"`
	SwitchClasses int `json:"switch_classes"`
	Connections   int `json:"connections"`
}

// --- ingest (deterministic, lossless) ---------------------------------------

// bundledDoc is the parsed external diet/XOC document. reference_data and
// server_nics are retained verbatim (as generic trees) so the split is lossless;
// the load-bearing sections are decoded into typed views.
type bundledDoc struct {
	Meta          map[string]any   `json:"meta"`
	Plan          map[string]any   `json:"plan"`
	ReferenceData map[string]any   `json:"reference_data"`
	SwitchClasses []rawSwitchClass `json:"switch_classes"`
	PortZones     []SwitchPortZone `json:"switch_port_zones"`
	ServerClasses []rawServerClass `json:"server_classes"`
	ServerNics    []any            `json:"server_nics"`
	Connections   []rawConnection  `json:"server_connections"`
	Expected      *rawExpected     `json:"expected"`
}

type rawServerClass struct {
	ServerClassID    string `json:"server_class_id"`
	Quantity         int    `json:"quantity"`
	ServerDeviceType string `json:"server_device_type"`
}

// rawRefData is the typed view of the bundled reference_data block needed to
// extract the hardware-type catalog layer (device/NIC/transceiver types). The
// block is also retained verbatim (doc.ReferenceData) for lossless rebundle; this
// typed view is read-only and never re-emitted.
type rawRefData struct {
	DeviceTypes          []rawHardwareType `json:"device_types"`
	ModuleTypes          []rawHardwareType `json:"module_types"`
	DeviceTypeExtensions []struct {
		ID         string `json:"id"`
		DeviceType string `json:"device_type"`
	} `json:"device_type_extensions"`
}

// rawHardwareType is a device_type or module_type entry. A module_type WITH
// interface_templates is a NIC; without, a transceiver (§4.2). A device_type is a
// chassis (server/switch) hardware type.
type rawHardwareType struct {
	ID                 string             `json:"id"`
	Manufacturer       string             `json:"manufacturer"`
	Model              string             `json:"model"`
	InterfaceTemplates []rawIfaceTemplate `json:"interface_templates"`
}

type rawIfaceTemplate struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// rawServerNic is one row of the server_nics join: a NIC module bound to a named
// slot on a server class.
type rawServerNic struct {
	ServerClass string `json:"server_class"`
	NicID       string `json:"nic_id"`
	ModuleType  string `json:"module_type"`
}

type rawSwitchClass struct {
	SwitchClassID    string `json:"switch_class_id"`
	FabricName       string `json:"fabric_name"`
	FabricClass      string `json:"fabric_class"`
	HedgehogRole     string `json:"hedgehog_role"`
	OverrideQuantity *int   `json:"override_quantity"`
	TopologyMode     string `json:"topology_mode"`
}

// rawConnection mirrors the on-the-wire connection (target_zone is the single
// "class/zone" string the real format uses); ingest splits it into the model's
// TargetSwitchClass/TargetZone and rebundle rejoins it.
type rawConnection struct {
	ServerClass        string `json:"server_class"`
	ConnectionID       string `json:"connection_id"`
	ConnectionName     string `json:"connection_name,omitempty"`
	Nic                string `json:"nic"`
	PortIndex          int    `json:"port_index"`
	PortsPerConnection int    `json:"ports_per_connection"`
	ConnType           string `json:"hedgehog_conn_type,omitempty"`
	Distribution       string `json:"distribution,omitempty"`
	TargetZone         string `json:"target_zone"`
	Speed              int    `json:"speed"`
	Rail               *int   `json:"rail,omitempty"`
	PortType           string `json:"port_type,omitempty"`
	Transceiver        string `json:"transceiver_module_type,omitempty"`
}

type rawExpected struct {
	Counts Counts `json:"counts"`
}

// IngestBundled parses a real BUNDLED topology-plan.yaml/training_*.yaml
// (reference_data co-located inline) and splits it into (a) an equivalent
// pure-reference Plan and (b) the extracted Catalog. The split is DETERMINISTIC
// and LOSSLESS: every server class becomes a PINNED, resolvable catalog ref
// (guardrail 1), and the reference_data/server_nics are extracted into the
// catalog verbatim so the bundle round-trips (guardrail 2).
func IngestBundled(yamlBytes []byte) (*Plan, *catalog.Catalog, error) {
	var doc bundledDoc
	if err := decodeYAML(yamlBytes, &doc); err != nil {
		return nil, nil, fmt.Errorf("%w: parse bundled plan: %v", ErrInvalidPlan, err)
	}

	// --- typed views of the bundled hardware layer + joins (read-only; the
	// verbatim blocks are still retained for lossless rebundle below).
	var rd rawRefData
	if err := remarshal(doc.ReferenceData, &rd); err != nil {
		return nil, nil, fmt.Errorf("%w: decode reference_data: %v", ErrInvalidPlan, err)
	}
	var nics []rawServerNic
	if err := remarshal(doc.ServerNics, &nics); err != nil {
		return nil, nil, fmt.Errorf("%w: decode server_nics: %v", ErrInvalidPlan, err)
	}

	// --- catalog layer 1: bare hardware TYPES extracted from reference_data.
	//   - module_types WITH interface_templates → NIC types (cages = templates);
	//     WITHOUT → transceiver types (§4.2 capability layer).
	//   - device_types → server/switch chassis hardware types (classified by the
	//     server_classes / device_type_extensions references).
	var items []catalog.Item
	for _, mt := range rd.ModuleTypes {
		if len(mt.InterfaceTemplates) > 0 {
			items = append(items, nicHardwareType(mt))
		} else {
			items = append(items, catalog.Item{
				ID:           objectmodel.ID{Name: mt.ID, Version: classVersion},
				Kind:         catalog.KindTransceiver,
				Layer:        catalog.LayerHardwareType,
				Manufacturer: mt.Manufacturer,
				Model:        mt.Model,
			})
		}
	}
	switchDT := map[string]bool{}
	for _, ext := range rd.DeviceTypeExtensions {
		switchDT[ext.DeviceType] = true
	}
	serverDT := map[string]bool{}
	for _, sc := range doc.ServerClasses {
		if sc.ServerDeviceType != "" {
			serverDT[sc.ServerDeviceType] = true
		}
	}
	for _, dt := range rd.DeviceTypes {
		kind := catalog.KindComponent
		switch {
		case switchDT[dt.ID]:
			kind = catalog.KindSwitch
		case serverDT[dt.ID]:
			kind = catalog.KindServer
		}
		it := catalog.Item{
			ID:           objectmodel.ID{Name: dt.ID, Version: classVersion},
			Kind:         kind,
			Layer:        catalog.LayerHardwareType,
			Manufacturer: dt.Manufacturer,
			Model:        dt.Model,
		}
		for _, tmpl := range dt.InterfaceTemplates {
			it.PortTemplates = append(it.PortTemplates, portTemplate(tmpl))
		}
		items = append(items, it)
	}

	// --- catalog layer 2: configured server/switch CLASSES, pinned by id+version.
	// Server classes carry the server_nics join (→ NIC component_slots) and the
	// per-NIC-port connection transceivers resolved INTO the class (→ cage
	// bindings, ports_per_connection expanded).
	nicsByClass := map[string][]rawServerNic{}
	for _, n := range nics {
		nicsByClass[n.ServerClass] = append(nicsByClass[n.ServerClass], n)
	}
	connsByClass := map[string][]rawConnection{}
	for _, c := range doc.Connections {
		connsByClass[c.ServerClass] = append(connsByClass[c.ServerClass], c)
	}
	for _, sc := range doc.ServerClasses {
		it := catalog.Item{
			ID:    objectmodel.ID{Name: sc.ServerClassID, Version: classVersion},
			Kind:  catalog.KindServer,
			Layer: catalog.LayerClass,
		}
		for _, n := range nicsByClass[sc.ServerClassID] {
			it.ComponentSlots = append(it.ComponentSlots, catalog.ComponentSlot{
				SlotID:   n.NicID,
				Target:   objectmodel.Ref{ID: objectmodel.ID{Name: n.ModuleType, Version: classVersion}},
				Quantity: 1,
				Required: true,
			})
		}
		for _, c := range connsByClass[sc.ServerClassID] {
			if c.Transceiver == "" {
				continue
			}
			ppc := c.PortsPerConnection
			if ppc < 1 {
				ppc = 1
			}
			for i := 0; i < ppc; i++ {
				it.CageBindings = append(it.CageBindings, catalog.CageBinding{
					NICSlotID:           c.Nic,
					PortIndex:           c.PortIndex + i,
					SelectedTransceiver: objectmodel.Ref{ID: objectmodel.ID{Name: c.Transceiver, Version: classVersion}},
				})
			}
		}
		items = append(items, it)
	}
	for _, sw := range doc.SwitchClasses {
		items = append(items, catalog.Item{
			ID:    objectmodel.ID{Name: sw.SwitchClassID, Version: classVersion},
			Kind:  catalog.KindSwitch,
			Layer: catalog.LayerClass,
		})
	}
	cat, err := catalog.New(items...)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: build extracted catalog: %v", ErrInvalidPlan, err)
	}
	cat.SetExtracted(doc.ReferenceData, doc.ServerNics)

	// --- plan: pure-reference intent that points at the extracted catalog.
	plan := &Plan{
		Meta: Meta{
			CaseID:  asString(doc.Meta["case_id"]),
			Name:    asString(doc.Meta["name"]),
			Version: asInt(doc.Meta["version"]),
		},
		Spec: Spec{Name: asString(doc.Meta["name"])},
	}
	for _, sc := range doc.ServerClasses {
		plan.Spec.ServerClasses = append(plan.Spec.ServerClasses, ServerClassUse{
			ServerClassID: sc.ServerClassID,
			ClassRef:      objectmodel.Ref{ID: objectmodel.ID{Name: sc.ServerClassID, Version: classVersion}},
			Quantity:      sc.Quantity,
		})
	}
	for _, sw := range doc.SwitchClasses {
		plan.Spec.SwitchClasses = append(plan.Spec.SwitchClasses, SwitchClassUse{
			SwitchClassID:    sw.SwitchClassID,
			ClassRef:         objectmodel.Ref{ID: objectmodel.ID{Name: sw.SwitchClassID, Version: classVersion}},
			FabricName:       sw.FabricName,
			FabricClass:      sw.FabricClass,
			HedgehogRole:     sw.HedgehogRole,
			OverrideQuantity: sw.OverrideQuantity,
			TopologyMode:     sw.TopologyMode,
		})
	}
	plan.Spec.PortZones = doc.PortZones
	for _, c := range doc.Connections {
		plan.Spec.Connections = append(plan.Spec.Connections, c.toModel())
	}
	if doc.Expected != nil {
		plan.Status = &Status{Expected: &Expected{Counts: doc.Expected.Counts}}
	}
	return plan, cat, nil
}

// toModel converts a wire connection into the model's connection, splitting the
// "class/zone" target into its two pinned components.
func (c rawConnection) toModel() ServerConnection {
	sc := ServerConnection{
		ServerClassID:      c.ServerClass,
		ConnectionID:       c.ConnectionID,
		ConnectionName:     c.ConnectionName,
		NICSlotID:          c.Nic,
		PortIndex:          c.PortIndex,
		PortsPerConnection: c.PortsPerConnection,
		ConnType:           c.ConnType,
		Distribution:       c.Distribution,
		Speed:              c.Speed,
		Rail:               c.Rail,
		PortType:           c.PortType,
		TransceiverID:      c.Transceiver,
	}
	if cls, zone, ok := splitZone(c.TargetZone); ok {
		sc.TargetSwitchClass = cls
		sc.TargetZone = zone
	}
	return sc
}

// fromModel rebuilds the wire connection from the model (rejoining target_zone),
// the inverse of toModel.
func (sc ServerConnection) fromModel() map[string]any {
	m := map[string]any{
		"server_class":         sc.ServerClassID,
		"connection_id":        sc.ConnectionID,
		"nic":                  sc.NICSlotID,
		"port_index":           sc.PortIndex,
		"ports_per_connection": sc.PortsPerConnection,
		"target_zone":          sc.TargetSwitchClass + "/" + sc.TargetZone,
		"speed":                sc.Speed,
	}
	putIf(m, "connection_name", sc.ConnectionName)
	putIf(m, "hedgehog_conn_type", sc.ConnType)
	putIf(m, "distribution", sc.Distribution)
	putIf(m, "port_type", sc.PortType)
	putIf(m, "transceiver_module_type", sc.TransceiverID)
	if sc.Rail != nil {
		m["rail"] = *sc.Rail
	}
	return m
}

// IngestPureReference parses an AID-canonical pure-reference plan against an
// already-loaded catalog. Every class ref must be PINNED (id+version) —
// guardrail 1 — or it is rejected with ErrUnpinnedRef.
func IngestPureReference(planYAML []byte, cat *catalog.Catalog) (*Plan, error) {
	var p Plan
	if err := decodeYAML(planYAML, &p); err != nil {
		return nil, fmt.Errorf("%w: parse pure-reference plan: %v", ErrInvalidPlan, err)
	}
	for _, sc := range p.Spec.ServerClasses {
		if !isPinned(sc.ClassRef) {
			return nil, fmt.Errorf("%w: server class %q ref %+v", ErrUnpinnedRef, sc.ServerClassID, sc.ClassRef)
		}
	}
	for _, sw := range p.Spec.SwitchClasses {
		if !isPinned(sw.ClassRef) {
			return nil, fmt.Errorf("%w: switch class %q ref %+v", ErrUnpinnedRef, sw.SwitchClassID, sw.ClassRef)
		}
	}
	return &p, nil
}

// Rebundle re-embeds a pure-reference Plan + Catalog back into the bundled shape,
// used by the round-trip determinism test (guardrail 2). It is the inverse of
// IngestBundled: the retained reference_data/server_nics and the modeled
// connections/expected are emitted so the bundle round-trips losslessly.
func Rebundle(p *Plan, cat *catalog.Catalog) ([]byte, error) {
	if p == nil || cat == nil {
		return nil, fmt.Errorf("%w: Rebundle needs a plan and a catalog", ErrInvalidPlan)
	}
	doc := map[string]any{
		"meta": map[string]any{
			"case_id": p.Meta.CaseID,
			"name":    p.Meta.Name,
			"version": p.Meta.Version,
		},
		"plan": map[string]any{"name": p.Spec.Name},
	}
	if rd := cat.ReferenceData(); rd != nil {
		doc["reference_data"] = rd
	}

	var serverClasses []any
	for _, sc := range p.Spec.ServerClasses {
		serverClasses = append(serverClasses, map[string]any{
			"server_class_id": sc.ServerClassID,
			"quantity":        sc.Quantity,
		})
	}
	doc["server_classes"] = serverClasses

	var switchClasses []any
	for _, sw := range p.Spec.SwitchClasses {
		entry := map[string]any{"switch_class_id": sw.SwitchClassID, "fabric_name": sw.FabricName}
		putIf(entry, "fabric_class", sw.FabricClass)
		putIf(entry, "hedgehog_role", sw.HedgehogRole)
		putIf(entry, "topology_mode", sw.TopologyMode)
		if sw.OverrideQuantity != nil {
			entry["override_quantity"] = *sw.OverrideQuantity
		}
		switchClasses = append(switchClasses, entry)
	}
	doc["switch_classes"] = switchClasses

	if p.Spec.PortZones != nil {
		doc["switch_port_zones"] = p.Spec.PortZones
	}
	if sn := cat.ServerNics(); sn != nil {
		doc["server_nics"] = sn
	}

	var conns []any
	for _, c := range p.Spec.Connections {
		conns = append(conns, c.fromModel())
	}
	doc["server_connections"] = conns

	if p.Status != nil && p.Status.Expected != nil {
		doc["expected"] = map[string]any{"counts": map[string]any{
			"server_classes": p.Status.Expected.Counts.ServerClasses,
			"switch_classes": p.Status.Expected.Counts.SwitchClasses,
			"connections":    p.Status.Expected.Counts.Connections,
		}}
	}
	return yaml.Marshal(doc)
}

// Validate checks a plan against the catalog: every class ref must be PINNED
// (guardrail 1) and RESOLVE to a catalog item. It NEVER reads Status/Expected
// (guardrail 3) — that plane is consumed only by an explicit self-check mode, so
// a status that conflicts with spec cannot affect ordinary validation. The
// objectmodel registry is accepted for substrate-contract checks in later phases.
func Validate(p *Plan, cat *catalog.Catalog, reg *objectmodel.Registry) error {
	if p == nil || cat == nil {
		return fmt.Errorf("%w: Validate needs a plan and a catalog", ErrInvalidPlan)
	}
	for _, sc := range p.Spec.ServerClasses {
		if !isPinned(sc.ClassRef) {
			return fmt.Errorf("%w: server class %q", ErrUnpinnedRef, sc.ServerClassID)
		}
		if _, ok := cat.Get(sc.ClassRef.ID); !ok {
			return fmt.Errorf("%w: server class ref %s", ErrUnresolvedRef, sc.ClassRef.ID)
		}
	}
	for _, sw := range p.Spec.SwitchClasses {
		if !isPinned(sw.ClassRef) {
			return fmt.Errorf("%w: switch class %q", ErrUnpinnedRef, sw.SwitchClassID)
		}
		if _, ok := cat.Get(sw.ClassRef.ID); !ok {
			return fmt.Errorf("%w: switch class ref %s", ErrUnresolvedRef, sw.ClassRef.ID)
		}
	}
	return nil
}

// ExpandPorts expands a connection with PortsPerConnection > 1 into deterministic
// per-port cage bindings, validated against the configured class (guardrail 4):
// the expansion occupies cages [PortIndex, PortIndex+PortsPerConnection) on the
// connection's NIC slot, in ascending port order, all targeting the connection's
// zone. If the resolved NIC type lacks enough cages it is rejected with
// ErrInsufficientPorts. Defined now (F0); exercised by calc later.
func ExpandPorts(conn ServerConnection, cat *catalog.Catalog) ([]CageBindingRef, error) {
	if conn.PortsPerConnection < 1 {
		return nil, fmt.Errorf("%w: ports_per_connection must be >= 1", ErrInvalidPlan)
	}
	server, ok := cat.ByName(conn.ServerClassID)
	if !ok {
		return nil, fmt.Errorf("%w: server class %q", ErrUnresolvedRef, conn.ServerClassID)
	}
	slot, ok := findSlot(server, conn.NICSlotID)
	if !ok {
		return nil, fmt.Errorf("%w: server class %q has no nic slot %q", ErrInvalidPlan, conn.ServerClassID, conn.NICSlotID)
	}
	nic, ok := cat.Get(slot.Target.ID)
	if !ok {
		return nil, fmt.Errorf("%w: nic type %s for slot %q", ErrUnresolvedRef, slot.Target.ID, conn.NICSlotID)
	}
	cages := countCages(nic)
	if conn.PortIndex+conn.PortsPerConnection > cages {
		return nil, fmt.Errorf("%w: slot %q needs ports [%d,%d) but nic %s has %d cage(s)",
			ErrInsufficientPorts, conn.NICSlotID, conn.PortIndex, conn.PortIndex+conn.PortsPerConnection, nic.ID, cages)
	}
	out := make([]CageBindingRef, 0, conn.PortsPerConnection)
	for i := 0; i < conn.PortsPerConnection; i++ {
		out = append(out, CageBindingRef{
			ServerClassID: conn.ServerClassID,
			NICSlotID:     conn.NICSlotID,
			PortIndex:     conn.PortIndex + i,
			ZoneName:      conn.TargetZone,
		})
	}
	return out, nil
}

// --- helpers -----------------------------------------------------------------

// decodeYAML decodes YAML through a YAML→JSON bridge so the structs' `json` field
// tags (the wire contract) drive decoding.
func decodeYAML(b []byte, v any) error {
	var raw any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return err
	}
	j, err := json.Marshal(jsonify(raw))
	if err != nil {
		return err
	}
	return json.Unmarshal(j, v)
}

func jsonify(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = jsonify(val)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprintf("%v", k)] = jsonify(val)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, val := range t {
			s[i] = jsonify(val)
		}
		return s
	default:
		return v
	}
}

// remarshal converts an already-JSON-friendly generic tree (a map/slice produced
// by decodeYAML's jsonify bridge) into a typed view via a JSON round-trip, so the
// typed struct's json tags drive decoding. Used to read the verbatim
// reference_data/server_nics into their typed extraction views.
func remarshal(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// nicHardwareType builds a NIC hardware type from a module_type: each
// interface_template becomes a transceiver cage (capability only; the selected
// optic is bound per NIC-port on the configured class).
func nicHardwareType(mt rawHardwareType) catalog.Item {
	it := catalog.Item{
		ID:           objectmodel.ID{Name: mt.ID, Version: classVersion},
		Kind:         catalog.KindNIC,
		Layer:        catalog.LayerHardwareType,
		Manufacturer: mt.Manufacturer,
		Model:        mt.Model,
	}
	for _, tmpl := range mt.InterfaceTemplates {
		t := portTemplate(tmpl)
		t.RequiresTransceiver = true
		it.PortTemplates = append(it.PortTemplates, t)
	}
	return it
}

// portTemplate maps an interface_template to a transceiver cage port template.
func portTemplate(tmpl rawIfaceTemplate) catalog.PortTemplate {
	return catalog.PortTemplate{
		Name:          tmpl.Name,
		PortKind:      catalog.TransceiverCage,
		InterfaceType: tmpl.Type,
	}
}

func isPinned(r objectmodel.Ref) bool { return r.Name != "" && r.Version != "" }

func splitZone(s string) (cls, zone string, ok bool) {
	i := strings.IndexByte(s, '/')
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

func findSlot(it catalog.Item, slotID string) (catalog.ComponentSlot, bool) {
	for _, s := range it.ComponentSlots {
		if s.SlotID == slotID {
			return s, true
		}
	}
	return catalog.ComponentSlot{}, false
}

func countCages(nic catalog.Item) int {
	n := 0
	for _, p := range nic.PortTemplates {
		if p.PortKind == catalog.TransceiverCage {
			n++
		}
	}
	return n
}

func putIf(m map[string]any, k, v string) {
	if v != "" {
		m[k] = v
	}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// ResolvePlan verifies every catalog reference the ingested plan carries
// resolves against cat: each server/switch class use, each server_nics-join NIC
// type, each connection's (server class, nic slot, NIC type, target switch
// class + zone, transceiver), and each zone transceiver. It returns
// ErrUnresolvedRef naming the first dangling reference. This is the F1
// "catalog-ref resolution" gate over the relational model.
func ResolvePlan(p *Plan, cat *catalog.Catalog) error {
	if p == nil || cat == nil {
		return fmt.Errorf("%w: ResolvePlan needs a plan and a catalog", ErrInvalidPlan)
	}
	// Server class uses: pinned, resolvable, and every NIC component_slot resolves.
	// Index the resolved configured class by its plan-level id so the connection
	// loop can resolve each connection's (server class, nic slot, NIC type) path.
	serverClassItem := map[string]catalog.Item{}
	for _, sc := range p.Spec.ServerClasses {
		if !isPinned(sc.ClassRef) {
			return fmt.Errorf("%w: server class %q", ErrUnpinnedRef, sc.ServerClassID)
		}
		item, ok := cat.Get(sc.ClassRef.ID)
		if !ok {
			return fmt.Errorf("%w: server class ref %s", ErrUnresolvedRef, sc.ClassRef.ID)
		}
		for _, slot := range item.ComponentSlots {
			if _, ok := cat.Get(slot.Target.ID); !ok {
				return fmt.Errorf("%w: server class %q nic slot %q target %s", ErrUnresolvedRef, sc.ServerClassID, slot.SlotID, slot.Target.ID)
			}
		}
		serverClassItem[sc.ServerClassID] = item
	}
	// Switch class uses: pinned + resolvable.
	for _, sw := range p.Spec.SwitchClasses {
		if !isPinned(sw.ClassRef) {
			return fmt.Errorf("%w: switch class %q", ErrUnpinnedRef, sw.SwitchClassID)
		}
		if _, ok := cat.Get(sw.ClassRef.ID); !ok {
			return fmt.Errorf("%w: switch class ref %s", ErrUnresolvedRef, sw.ClassRef.ID)
		}
	}
	// Zone-level transceivers resolve against the catalog.
	for _, z := range p.Spec.PortZones {
		if z.Transceiver == "" {
			continue
		}
		if _, ok := cat.ByName(z.Transceiver); !ok {
			return fmt.Errorf("%w: zone %s/%s transceiver %q", ErrUnresolvedRef, z.SwitchClassID, z.ZoneName, z.Transceiver)
		}
	}
	// Connections: the (server class, nic slot, NIC type) path resolves against the
	// configured class, the connection-level transceiver resolves against the
	// catalog, and the (switch class, zone) target resolves to an ingested port zone.
	for _, c := range p.Spec.Connections {
		item, ok := serverClassItem[c.ServerClassID]
		if !ok {
			return fmt.Errorf("%w: connection %q server class %q", ErrUnresolvedRef, c.ConnectionID, c.ServerClassID)
		}
		slot, ok := findSlot(item, c.NICSlotID)
		if !ok {
			return fmt.Errorf("%w: connection %q nic slot %q on server class %q", ErrUnresolvedRef, c.ConnectionID, c.NICSlotID, c.ServerClassID)
		}
		if _, ok := cat.Get(slot.Target.ID); !ok {
			return fmt.Errorf("%w: connection %q nic type %s for slot %q", ErrUnresolvedRef, c.ConnectionID, slot.Target.ID, c.NICSlotID)
		}
		if c.TransceiverID != "" {
			if _, ok := cat.ByName(c.TransceiverID); !ok {
				return fmt.Errorf("%w: connection %q transceiver %q", ErrUnresolvedRef, c.ConnectionID, c.TransceiverID)
			}
		}
		if !zoneExists(p, c.TargetSwitchClass, c.TargetZone) {
			return fmt.Errorf("%w: connection %q target zone %s/%s", ErrUnresolvedRef, c.ConnectionID, c.TargetSwitchClass, c.TargetZone)
		}
	}
	return nil
}

// zoneExists reports whether the plan carries a port zone (switchClass, zoneName).
func zoneExists(p *Plan, switchClass, zoneName string) bool {
	for _, z := range p.Spec.PortZones {
		if z.SwitchClassID == switchClass && z.ZoneName == zoneName {
			return true
		}
	}
	return false
}

// SelfCheck is the explicit expected.counts self-check mode (D21, guardrail 3):
// the ONLY place Status/Expected is read. It derives counts from the ingested
// relational model and compares them to the plan's Expected counts, populating
// Status.Computed. Returns ErrNoExpected if the plan carries no expected block
// and ErrSelfCheckMismatch on divergence. In F1 the computed counts are the
// ingested counts (no calc yet).
func SelfCheck(p *Plan) (Counts, error) {
	if p == nil || p.Status == nil || p.Status.Expected == nil {
		return Counts{}, ErrNoExpected
	}
	got := Counts{
		ServerClasses: len(p.Spec.ServerClasses),
		SwitchClasses: len(p.Spec.SwitchClasses),
		Connections:   len(p.Spec.Connections),
	}
	if p.Status.Computed == nil {
		p.Status.Computed = &Computed{}
	}
	p.Status.Computed.Counts = got
	if got != p.Status.Expected.Counts {
		return got, fmt.Errorf("%w: computed %+v != expected %+v", ErrSelfCheckMismatch, got, p.Status.Expected.Counts)
	}
	return got, nil
}

// CageBindingRef is one resolved (server class, nic slot, port index) → zone
// binding produced by ExpandPorts.
type CageBindingRef struct {
	ServerClassID string
	NICSlotID     string
	PortIndex     int
	ZoneName      string
}
