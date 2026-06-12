package catalog

import (
	"errors"
	"testing"

	"github.com/afewell-hh/aid/internal/objectmodel"
)

func ref(name string) objectmodel.Ref { return objectmodel.Ref{ID: objectmodel.ID{Name: name, Version: "1"}} }

// realB200ServerClass builds the owner's real server (docs/requirements/
// real-server-bom.csv) in the catalog model: a configured server CLASS over bare
// hardware TYPES, with the 8× CX-7 as ONE quantity-bearing slot, the BF3 DPU with
// a fixed BMC port + two cages, and the non-physical line items.
func realB200ServerClass() Item {
	return Item{
		ID:           objectmodel.ID{Name: "smc-b200-8gpu", Version: "1"},
		Kind:         KindServer,
		Layer:        LayerClass,
		Manufacturer: "Supermicro",
		Model:        "AS-4126GS-NBR-LCC",
		PartNumber:   "AS-4126GS-NBR-LCC",
		PurchaseProfile: map[string]any{"unit_of_measure": "each"},
		ComponentSlots: []ComponentSlot{
			{SlotID: "chassis", Target: ref("smc-as4126-barebone"), Quantity: 1, Required: true},
			{SlotID: "gpu-board", Target: ref("nv-hgx-b200-8180-lc"), Quantity: 1, Required: true},
			{SlotID: "cpu", Target: ref("amd-turin-9575f"), Quantity: 2, Required: true},
			{SlotID: "memory", Target: ref("mem-dr512l-cl01-er64"), Quantity: 24, Required: true},
			{SlotID: "nic-cx7", Target: ref("nv-cx7-1x400g"), Quantity: 8, Required: true}, // 8× CX-7 = ONE slot
			{SlotID: "dpu-bf3", Target: ref("nv-bf3-2x200g"), Quantity: 1, Required: true},
			// non-physical
			{SlotID: "warranty", Target: ref("ewcsc"), Quantity: 1, Required: true},
			{SlotID: "sw-support", Target: ref("svc-nvstdswsup-3y"), Quantity: 1, Required: true},
			{SlotID: "accessory", Target: ref("cbl-pwex-1174-60"), Quantity: 1, Required: true},
			{SlotID: "assembly", Target: ref("mc0037"), Quantity: 1, Required: true},
			{SlotID: "onsite", Target: ref("osnbd3"), Quantity: 1, Required: true},
		},
		// per-NIC-port transceiver binding (each CX-7's single QSFP112 cage)
		CageBindings: []CageBinding{
			{NICSlotID: "nic-cx7", PortIndex: 0, SelectedTransceiver: ref("osfp-400g-dr4")},
		},
	}
}

// bf3Type is the BF3 DPU bare hardware type: a FIXED 1000BASE-T BMC port + two
// QSFP112 transceiver cages (the fixed-vs-cage distinction HNP cannot express).
func bf3Type() Item {
	return Item{
		ID:         objectmodel.ID{Name: "nv-bf3-2x200g", Version: "1"},
		Kind:       KindDPU,
		Layer:      LayerHardwareType,
		Model:      "GPU-NVDPU-BA3220-C",
		PartNumber: "GPU-NVDPU-BA3220-C",
		PortTemplates: []PortTemplate{
			{Name: "bmc", PortKind: FixedInterface, MaxSpeedGbps: 1, InterfaceType: "1000base-t", RequiresTransceiver: false},
			{Name: "p0", PortKind: TransceiverCage, MaxSpeedGbps: 200, CageType: "QSFP112", RequiresTransceiver: true},
			{Name: "p1", PortKind: TransceiverCage, MaxSpeedGbps: 200, CageType: "QSFP112", RequiresTransceiver: true},
		},
	}
}

// TestModel_ExpressesRealServer (REAL — passes): the catalog type model can
// represent the owner's real server faithfully. This is the F0 "model of record"
// guarantee for R1/R3/R4/R5; calc/BOM reduction is later.
func TestModel_ExpressesRealServer(t *testing.T) {
	server := realB200ServerClass()
	if server.Layer != LayerClass {
		t.Errorf("server should be a configured class")
	}
	// 8× CX-7 as ONE quantity-bearing slot, not a synthetic 8-port NIC.
	var cx7 *ComponentSlot
	nonPhysicalKinds := map[string]bool{"warranty": true, "sw-support": true, "assembly": true, "onsite": true}
	nonPhysicalSeen := 0
	for i := range server.ComponentSlots {
		s := &server.ComponentSlots[i]
		if s.SlotID == "nic-cx7" {
			cx7 = s
		}
		if nonPhysicalKinds[s.SlotID] {
			nonPhysicalSeen++
		}
	}
	if cx7 == nil || cx7.Quantity != 8 {
		t.Fatalf("8× CX-7 must be one slot with quantity 8, got %+v", cx7)
	}
	if nonPhysicalSeen != 4 {
		t.Errorf("expected 4 non-physical slots (warranty/support/assembly/onsite), saw %d", nonPhysicalSeen)
	}

	bf3 := bf3Type()
	if bf3.Layer != LayerHardwareType || len(bf3.CageBindings) != 0 {
		t.Errorf("bare hardware type must declare capability only (no bindings)")
	}
	var fixed, cages int
	for _, p := range bf3.PortTemplates {
		switch p.PortKind {
		case FixedInterface:
			fixed++
		case TransceiverCage:
			cages++
		}
	}
	if fixed != 1 || cages != 2 {
		t.Errorf("BF3 must be 1 fixed BMC + 2 cages, got fixed=%d cages=%d", fixed, cages)
	}

	// The catalog holds both, keyed by pinned ID.
	c, err := New(server, bf3)
	if err != nil {
		t.Fatal(err)
	}
	if c.Len() != 2 {
		t.Errorf("catalog len = %d, want 2", c.Len())
	}
}

// --- RED (F0 GREEN targets): assert intended behavior; fail until implemented --

// TestContracts: the catalog declares objectmodel contracts incl. an acyclic,
// quantity-bearing component_slot relation (the F0 implementation gate).
func TestContracts(t *testing.T) {
	cs, err := Contracts()
	if err != nil {
		t.Fatalf("Contracts (F0 GREEN target): %v", err)
	}
	if len(cs) == 0 {
		t.Fatal("Contracts must declare contracts for the catalog kinds")
	}
	found := false
	for _, c := range cs {
		if rc, ok := c.Relations["component_slot"]; ok && rc.Acyclic && rc.QuantityField != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected an acyclic, quantity-bearing component_slot relation contract")
	}
}

// TestToObjects: the catalog maps onto the general substrate for validation.
func TestToObjects(t *testing.T) {
	c, _ := New(realB200ServerClass(), bf3Type())
	objs, err := c.ToObjects()
	if err != nil {
		t.Fatalf("ToObjects (F0 GREEN target): %v", err)
	}
	if len(objs) != 2 {
		t.Errorf("ToObjects: got %d objects, want 2", len(objs))
	}
}

// TestLoad: parsing a catalog artifact yields a populated catalog. (F0 GREEN
// authors a catalog fixture and implements the parser.)
func TestLoad(t *testing.T) {
	c, err := Load("tests/oracle/xoc-64-mesh-conv-ro/catalog.yaml")
	if err != nil {
		t.Fatalf("Load (F0 GREEN target): %v", err)
	}
	if c == nil || c.Len() == 0 {
		t.Fatal("Load must return a populated catalog")
	}
}

var _ = errors.Is // keep errors import available for GREEN assertions
