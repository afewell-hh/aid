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
	"errors"
	"fmt"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
)

// ErrNotImplemented marks an F0 RED stub.
var ErrNotImplemented = errors.New("topology: not implemented (F0 GREEN)")

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
	Name          string            `json:"name"`
	ServerClasses []ServerClassUse  `json:"server_classes"`
	SwitchClasses []SwitchClassUse  `json:"switch_classes"`
	PortZones     []SwitchPortZone  `json:"switch_port_zones"`
	Connections   []ServerConnection `json:"server_connections"`
	MeshLinks     []MeshLink        `json:"mesh_links,omitempty"`
	MCLAGDomains  []MCLAGDomain     `json:"mclag_domains,omitempty"`
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
	SwitchClassID    string          `json:"switch_class_id"`
	ClassRef         objectmodel.Ref `json:"class_ref"`
	FabricName       string          `json:"fabric_name"`
	OverrideQuantity *int            `json:"override_quantity,omitempty"`
	TopologyMode     string          `json:"topology_mode,omitempty"` // spine-leaf | mesh
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
}

// ServerConnection is connection intent keyed on (nic, port_index) → zone. A
// NIC's multiple ports may target different zones, so the granularity is the
// PORT, not the NIC. The connection-level transceiver (real-format input) is
// resolved into the server class's cage binding on ingest.
type ServerConnection struct {
	ServerClassID      string `json:"server_class"`
	ConnectionID       string `json:"connection_id"`
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
	Counts          Counts         `json:"counts"`
	SwitchQuantity  map[string]int `json:"switch_quantity,omitempty"` // per switch_class_id
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

// IngestBundled parses a real BUNDLED topology-plan.yaml/training_*.yaml
// (reference_data co-located inline) and splits it into (a) an equivalent
// pure-reference Plan and (b) the extracted Catalog. The split must be
// DETERMINISTIC and LOSSLESS — IDs preserved (guardrail 2). F0 RED stub.
func IngestBundled(yamlBytes []byte) (*Plan, *catalog.Catalog, error) {
	return nil, nil, fmt.Errorf("%w: IngestBundled", ErrNotImplemented)
}

// IngestPureReference parses an AID-canonical pure-reference plan against an
// already-loaded catalog. F0 RED stub.
func IngestPureReference(planYAML []byte, cat *catalog.Catalog) (*Plan, error) {
	return nil, fmt.Errorf("%w: IngestPureReference", ErrNotImplemented)
}

// Rebundle re-embeds a pure-reference Plan + Catalog back into the bundled
// shape, used by the round-trip determinism test (guardrail 2). F0 RED stub.
func Rebundle(p *Plan, cat *catalog.Catalog) ([]byte, error) {
	return nil, fmt.Errorf("%w: Rebundle", ErrNotImplemented)
}

// Validate checks a plan against the catalog + objectmodel contracts (refs
// resolve to pinned classes, zones/connections well-formed, per-port bindings
// consistent). F0 RED stub. Does not read Status (guardrail 3).
func Validate(p *Plan, cat *catalog.Catalog, reg *objectmodel.Registry) error {
	return fmt.Errorf("%w: Validate", ErrNotImplemented)
}

// ExpandPorts expands a connection with PortsPerConnection > 1 into deterministic
// per-port cage bindings, validated against the configured class (guardrail 4).
// Defined now (F0) even though it is exercised by calc later. F0 RED stub.
func ExpandPorts(conn ServerConnection, cat *catalog.Catalog) ([]CageBindingRef, error) {
	return nil, fmt.Errorf("%w: ExpandPorts", ErrNotImplemented)
}

// CageBindingRef is one resolved (server class, nic slot, port index) → zone
// binding produced by ExpandPorts.
type CageBindingRef struct {
	ServerClassID string
	NICSlotID     string
	PortIndex     int
	ZoneName      string
}
