// Package catalog is AID's NetBox-independent component catalog
// (docs/foundation-redesign.md §4.2, D18/D19/D21). It is a SEPARATE, versioned,
// AID-owned artifact of independent objects (CRD-style) that topology plans
// reference by pinned ID. HNP delegated this to NetBox DCIM
// (reference_data.py:142-158); AID has no NetBox, so it owns the catalog.
//
// Two layers:
//   - bare hardware TYPES (chassis, NIC, DPU, transceiver, component) —
//     CAPABILITY only (port_templates/cages declare allowed transceivers;
//     calc_profile/purchase_profile; bom_line_templates). Reusable; no
//     per-design selection baked in.
//   - configured server/switch CLASSES — composites that reference hardware
//     types via component_slots and BIND specific transceivers into specific
//     NIC-PORT cages, yielding a fully self-describing object with a complete,
//     context-free BOM. Reusable inventory; a different optic ⇒ a distinct class.
//
// F0 builds the model + Load stub; no calculation (calc is F2+).
package catalog

import (
	"errors"
	"fmt"

	"github.com/afewell-hh/aid/internal/objectmodel"
)

// ErrNotImplemented marks an F0 RED stub.
var ErrNotImplemented = errors.New("catalog: not implemented (F0 GREEN)")

// Layer distinguishes a bare hardware type from a configured class.
type Layer string

const (
	LayerHardwareType Layer = "hardware_type"
	LayerClass        Layer = "class"
)

// Kind values. The set is open (the substrate allows new kinds); these are the
// ones F0 must model, including the owner's non-physical kinds
// (real-server-bom.csv:4,12-15).
const (
	KindServer          = "server"
	KindSwitch          = "switch"
	KindNIC             = "nic"
	KindDPU             = "dpu"
	KindTransceiver     = "transceiver"
	KindComponent       = "component"
	KindAccessory       = "accessory"
	KindWarranty        = "warranty"
	KindSoftwareSupport = "software_support"
	KindAssembly        = "assembly"
	KindOnsiteService   = "onsite_service"
)

// PortKind distinguishes a soldered fixed interface from a pluggable cage (R4).
type PortKind string

const (
	// FixedInterface needs no optic (e.g. the BF3 1000BASE-T BMC port).
	FixedInterface PortKind = "fixed_interface"
	// TransceiverCage requires a compatible selected transceiver.
	TransceiverCage PortKind = "transceiver_cage"
)

// PortTemplate is a port/cage on a bare hardware type. It declares CAPABILITY
// only — the selected transceiver is bound on the class, per NIC port (the
// devb capability-vs-binding gate), except a captive/fixed SKU optic.
type PortTemplate struct {
	Name                string           `json:"name"`
	PortKind            PortKind         `json:"port_kind"`
	MaxSpeedGbps        int              `json:"max_speed_gbps"`
	InterfaceType       string           `json:"interface_type"`
	CageType            string           `json:"cage_type,omitempty"`
	RequiresTransceiver bool             `json:"requires_transceiver"`
	AllowedTransceivers []objectmodel.Ref `json:"allowed_transceivers,omitempty"`
}

// ComponentSlot is a nested purchasable part. The 8× CX-7 are ONE slot with
// Quantity:8 over the faithful one-cage CX-7 type (the quantity-bearing NIC slot
// — not a synthetic 8-port NIC). Slots may reference non-physical kinds too
// (warranty/support/assembly/onsite).
type ComponentSlot struct {
	SlotID               string            `json:"slot_id"`
	Target               objectmodel.Ref   `json:"target"`
	Quantity             int               `json:"quantity"`
	Required             bool              `json:"required"`
	SelectionConstraints map[string]any    `json:"selection_constraints,omitempty"`
}

// BOMLineTemplate is an arbitrary row a catalog item contributes to the
// purchasable BOM — physical (chassis, GPU board, CPU, memory, drives) or
// NON-physical (warranty, support, accessory, assembly, onsite). Scales linearly
// with the owning item's instance count (the F3 reducer).
type BOMLineTemplate struct {
	Category          string         `json:"category"`
	TargetRef         *objectmodel.Ref `json:"target_ref,omitempty"`
	InlineSKU         string         `json:"inline_sku,omitempty"`
	QuantityPerInstance int          `json:"quantity_per_instance"`
	Physical          bool           `json:"physical"`
	Attributes        map[string]any `json:"attributes,omitempty"`
}

// CageBinding is a class-level selection: which transceiver populates which cage
// on which NIC slot at which port index. Selection granularity is the NIC PORT
// (a NIC's ports may attach to different zones), per the owner model + HNP
// (PlanServerConnection.nic + port_index, topology_plans.py:673,680).
type CageBinding struct {
	NICSlotID            string          `json:"nic_slot_id"`
	PortIndex            int             `json:"port_index"`
	SelectedTransceiver  objectmodel.Ref `json:"selected_transceiver"`
}

// Item is a catalog object — either a bare hardware type or a configured class.
type Item struct {
	ID           objectmodel.ID `json:"id"`
	Kind         string         `json:"kind"`
	Layer        Layer          `json:"layer"`
	Manufacturer string         `json:"manufacturer,omitempty"`
	Model        string         `json:"model,omitempty"`
	PartNumber   string         `json:"part_number,omitempty"` // SKU
	Description  string         `json:"description,omitempty"`
	Orderable    bool           `json:"orderable"`

	// Attribute namespaces (open/extensible — future: power/lifecycle/cost/…).
	CalcProfile     map[string]any `json:"calc_profile,omitempty"`
	PurchaseProfile map[string]any `json:"purchase_profile,omitempty"`

	// Relations.
	PortTemplates    []PortTemplate    `json:"port_templates,omitempty"`    // hardware types
	ComponentSlots   []ComponentSlot   `json:"component_slots,omitempty"`   // nested parts
	BOMLineTemplates []BOMLineTemplate `json:"bom_line_templates,omitempty"`

	// Class-only: per-NIC-port transceiver bindings (empty on bare types).
	CageBindings []CageBinding `json:"cage_bindings,omitempty"`
}

// Catalog is a set of items keyed by pinned ID.
type Catalog struct {
	items map[objectmodel.ID]Item
}

// New builds a catalog; duplicate IDs are a hard error.
func New(items ...Item) (*Catalog, error) {
	c := &Catalog{items: make(map[objectmodel.ID]Item, len(items))}
	for _, it := range items {
		if _, dup := c.items[it.ID]; dup {
			return nil, fmt.Errorf("catalog: duplicate item id %s", it.ID)
		}
		c.items[it.ID] = it
	}
	return c, nil
}

// Get returns the item for id.
func (c *Catalog) Get(id objectmodel.ID) (Item, bool) { it, ok := c.items[id]; return it, ok }

// Len reports the item count.
func (c *Catalog) Len() int { return len(c.items) }

// Load reads a catalog artifact (the AID-owned catalog YAML/JSON) from path.
// F0 RED stub (F0 GREEN parses the real catalog).
func Load(path string) (*Catalog, error) {
	return nil, fmt.Errorf("%w: Load(%s)", ErrNotImplemented, path)
}

// ToObjects maps catalog items onto the general substrate so the objectmodel
// contracts can validate them. F0 RED stub.
func (c *Catalog) ToObjects() ([]objectmodel.Object, error) {
	return nil, fmt.Errorf("%w: ToObjects", ErrNotImplemented)
}

// Contracts returns the objectmodel contracts for the catalog kinds (required
// fields per projection, component_slot acyclicity + quantity composition).
// F0 RED stub.
func Contracts() ([]objectmodel.Contract, error) {
	return nil, fmt.Errorf("%w: Contracts", ErrNotImplemented)
}
