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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"

	"github.com/afewell-hh/aid/internal/objectmodel"
)

// ErrNotImplemented marks an F0 RED stub.
var ErrNotImplemented = errors.New("catalog: not implemented (F0 GREEN)")

// RelComponentSlot / RelPortTemplate are the substrate relation kinds catalog
// items project onto (objectmodel.Relation.Kind values).
const (
	RelComponentSlot = "component_slot"
	RelPortTemplate  = "port_template"
)

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
	Name                string            `json:"name"`
	PortKind            PortKind          `json:"port_kind"`
	MaxSpeedGbps        int               `json:"max_speed_gbps"`
	InterfaceType       string            `json:"interface_type"`
	CageType            string            `json:"cage_type,omitempty"`
	RequiresTransceiver bool              `json:"requires_transceiver"`
	AllowedTransceivers []objectmodel.Ref `json:"allowed_transceivers,omitempty"`
}

// ComponentSlot is a nested purchasable part. The 8× CX-7 are ONE slot with
// Quantity:8 over the faithful one-cage CX-7 type (the quantity-bearing NIC slot
// — not a synthetic 8-port NIC). Slots may reference non-physical kinds too
// (warranty/support/assembly/onsite).
type ComponentSlot struct {
	SlotID               string          `json:"slot_id"`
	Target               objectmodel.Ref `json:"target"`
	Quantity             int             `json:"quantity"`
	Required             bool            `json:"required"`
	SelectionConstraints map[string]any  `json:"selection_constraints,omitempty"`
}

// BOMLineTemplate is an arbitrary row a catalog item contributes to the
// purchasable BOM — physical (chassis, GPU board, CPU, memory, drives) or
// NON-physical (warranty, support, accessory, assembly, onsite). Scales linearly
// with the owning item's instance count (the F3 reducer).
type BOMLineTemplate struct {
	Category            string           `json:"category"`
	TargetRef           *objectmodel.Ref `json:"target_ref,omitempty"`
	InlineSKU           string           `json:"inline_sku,omitempty"`
	QuantityPerInstance int              `json:"quantity_per_instance"`
	Physical            bool             `json:"physical"`
	Attributes          map[string]any   `json:"attributes,omitempty"`
}

// CageBinding is a class-level selection: which transceiver populates which cage
// on which NIC slot at which port index. Selection granularity is the NIC PORT
// (a NIC's ports may attach to different zones), per the owner model + HNP
// (PlanServerConnection.nic + port_index, topology_plans.py:673,680).
type CageBinding struct {
	NICSlotID           string          `json:"nic_slot_id"`
	PortIndex           int             `json:"port_index"`
	SelectedTransceiver objectmodel.Ref `json:"selected_transceiver"`
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
	PortTemplates    []PortTemplate    `json:"port_templates,omitempty"`  // hardware types
	ComponentSlots   []ComponentSlot   `json:"component_slots,omitempty"` // nested parts
	BOMLineTemplates []BOMLineTemplate `json:"bom_line_templates,omitempty"`

	// Class-only: per-NIC-port transceiver bindings (empty on bare types).
	CageBindings []CageBinding `json:"cage_bindings,omitempty"`
}

// Catalog is a set of items keyed by pinned ID. When a catalog is produced by
// extracting a bundled plan (topology.IngestBundled), it also RETAINS the
// extracted reference_data block and server_nics verbatim so the extraction
// round-trips losslessly (deliverable 6, guardrail 2); these carriers are empty
// for hand-authored catalogs.
type Catalog struct {
	items      map[objectmodel.ID]Item
	refData    map[string]any // extracted reference_data, retained for lossless rebundle
	serverNics []any          // extracted server_nics, retained for lossless rebundle
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

// ByName returns the (first) item whose pinned ID has the given Name, regardless
// of version. Used where only a plan-level class id (not a full pinned ref) is in
// hand, e.g. resolving a connection's server_class for port expansion.
func (c *Catalog) ByName(name string) (Item, bool) {
	for id, it := range c.items {
		if id.Name == name {
			return it, true
		}
	}
	return Item{}, false
}

// Len reports the item count.
func (c *Catalog) Len() int { return len(c.items) }

// SetExtracted attaches the reference_data block and server_nics extracted from a
// bundled plan so they can be re-embedded losslessly (deliverable 6). Called by
// topology.IngestBundled; a no-op carrier for hand-authored catalogs.
func (c *Catalog) SetExtracted(refData map[string]any, serverNics []any) {
	c.refData = refData
	c.serverNics = serverNics
}

// ReferenceData returns the retained extracted reference_data block (nil if the
// catalog was not produced from a bundled plan).
func (c *Catalog) ReferenceData() map[string]any { return c.refData }

// ServerNics returns the retained extracted server_nics (nil if not from a bundle).
func (c *Catalog) ServerNics() []any { return c.serverNics }

// Merge applies an AID-owned OVERLAY catalog onto this one (F3, note §3.2): for
// each overlay item, if an item with the same pinned ID already exists its
// descriptive identity and attribute namespaces are ENRICHED from the overlay
// (overlay wins on non-empty descriptive fields; calc_profile/purchase_profile
// keys are merged, overlay overriding; bom_line_templates/cage_bindings appended
// if the base had none); otherwise the overlay item is added. This is how the
// hand-authored optic/description plane (bom.csv cols 7–19 + manufacturer/desc)
// joins the catalog extracted from a bundled plan, keyed by id — without reading
// bom.csv or importing HNP (D1/D12).
func (c *Catalog) Merge(overlay *Catalog) {
	if overlay == nil {
		return
	}
	for id, ov := range overlay.items {
		base, ok := c.items[id]
		if !ok {
			c.items[id] = ov
			continue
		}
		if ov.Manufacturer != "" {
			base.Manufacturer = ov.Manufacturer
		}
		if ov.Model != "" {
			base.Model = ov.Model
		}
		if ov.PartNumber != "" {
			base.PartNumber = ov.PartNumber
		}
		if ov.Description != "" {
			base.Description = ov.Description
		}
		base.CalcProfile = mergeAttrs(base.CalcProfile, ov.CalcProfile)
		base.PurchaseProfile = mergeAttrs(base.PurchaseProfile, ov.PurchaseProfile)
		if len(base.BOMLineTemplates) == 0 {
			base.BOMLineTemplates = ov.BOMLineTemplates
		}
		if len(base.CageBindings) == 0 {
			base.CageBindings = ov.CageBindings
		}
		c.items[id] = base
	}
}

// mergeAttrs returns base with overlay's keys applied (overlay wins). A nil base
// is initialized from overlay; nil overlay leaves base untouched.
func mergeAttrs(base, overlay map[string]any) map[string]any {
	if len(overlay) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]any, len(overlay))
	}
	for k, v := range overlay {
		base[k] = v
	}
	return base
}

// catalogFile is the on-disk shape of the AID-owned catalog artifact: a list of
// items. Parsed via a YAML→JSON bridge so the items' `json` field tags (the wire
// contract shared with the JSON Schema) drive decoding.
type catalogFile struct {
	Items []Item `json:"items"`
}

// Load reads a catalog artifact (the AID-owned catalog YAML/JSON) from path and
// returns a populated catalog. Relative paths are resolved against the current
// working directory first, then against the repo root, so callers can name the
// vendored fixture by its repo-relative path from any package's test cwd.
func Load(path string) (*Catalog, error) {
	b, err := readResolved(path)
	if err != nil {
		return nil, err
	}
	// YAML→generic→JSON→typed so the json field tags apply (yaml.v3 alone would
	// lowercase struct field names and miss snake_case tags).
	var raw any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("catalog: parse %s: %w", path, err)
	}
	jsonBytes, err := json.Marshal(jsonify(raw))
	if err != nil {
		return nil, fmt.Errorf("catalog: normalize %s: %w", path, err)
	}
	var cf catalogFile
	if err := json.Unmarshal(jsonBytes, &cf); err != nil {
		return nil, fmt.Errorf("catalog: decode %s: %w", path, err)
	}
	return New(cf.Items...)
}

// readResolved reads path, falling back to repo-root resolution for relative
// paths that do not exist relative to the working directory.
func readResolved(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	} else if filepath.IsAbs(path) {
		return nil, err
	}
	rooted := filepath.Join(repoRoot(), path)
	b, err := os.ReadFile(rooted)
	if err != nil {
		return nil, fmt.Errorf("catalog: read %s (also tried %s): %w", path, rooted, err)
	}
	return b, nil
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// jsonify coerces yaml.v3's value tree (which may carry map[any]any) into a
// JSON-marshalable tree (string-keyed maps).
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

// ToObjects maps catalog items onto the general substrate so the objectmodel
// contracts can validate them. Each item becomes a typed Object: its
// calc_profile/purchase_profile become attribute namespaces, its component_slots
// and port_templates become typed relations (component_slots carry the quantity
// used by ComposeQuantity).
func (c *Catalog) ToObjects() ([]objectmodel.Object, error) {
	objs := make([]objectmodel.Object, 0, len(c.items))
	for _, it := range c.items {
		o := objectmodel.Object{Kind: it.Kind, ID: it.ID}
		o.Attributes = map[string]map[string]any{}
		if len(it.CalcProfile) > 0 {
			o.Attributes["calc_profile"] = it.CalcProfile
		}
		if len(it.PurchaseProfile) > 0 {
			o.Attributes["purchase_profile"] = it.PurchaseProfile
		}
		// Surface the SKU into purchase_profile so the bom projection contract can
		// require it without reaching into struct internals.
		if it.PartNumber != "" {
			if o.Attributes["purchase_profile"] == nil {
				o.Attributes["purchase_profile"] = map[string]any{}
			}
			if _, ok := o.Attributes["purchase_profile"]["part_number"]; !ok {
				o.Attributes["purchase_profile"]["part_number"] = it.PartNumber
			}
		}
		if len(o.Attributes) == 0 {
			o.Attributes = nil
		}
		for _, s := range it.ComponentSlots {
			o.Relations = append(o.Relations, objectmodel.Relation{
				Kind:   RelComponentSlot,
				Target: s.Target,
				Fields: map[string]any{"quantity": s.Quantity, "required": s.Required, "slot_id": s.SlotID},
			})
		}
		for _, p := range it.PortTemplates {
			o.Relations = append(o.Relations, objectmodel.Relation{
				Kind:   RelPortTemplate,
				Fields: map[string]any{"name": p.Name, "port_kind": string(p.PortKind)},
			})
		}
		objs = append(objs, o)
	}
	return objs, nil
}

// Contracts returns the objectmodel contracts for the catalog kinds. The
// composite kinds (server, switch) declare an ACYCLIC, QUANTITY-BEARING
// component_slot relation (the 8× CX-7 multiply) and require their purchasable
// SKU under the "bom" projection — the F0 implementation gate ("open attributes"
// is not "anything goes").
func Contracts() ([]objectmodel.Contract, error) {
	slotRel := map[string]objectmodel.RelationContract{
		RelComponentSlot: {Kind: RelComponentSlot, Acyclic: true, QuantityField: "quantity"},
		RelPortTemplate:  {Kind: RelPortTemplate},
	}
	composite := func(kind string) objectmodel.Contract {
		return objectmodel.Contract{
			Kind:                 kind,
			RequiredByProjection: map[string][]string{"bom": {"purchase_profile.part_number"}},
			Relations:            slotRel,
		}
	}
	hwType := func(kind string) objectmodel.Contract {
		return objectmodel.Contract{Kind: kind, Relations: map[string]objectmodel.RelationContract{
			RelPortTemplate: {Kind: RelPortTemplate},
		}}
	}
	return []objectmodel.Contract{
		composite(KindServer),
		composite(KindSwitch),
		hwType(KindNIC),
		hwType(KindDPU),
		hwType(KindTransceiver),
		hwType(KindComponent),
	}, nil
}
