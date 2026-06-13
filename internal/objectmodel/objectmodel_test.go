package objectmodel

import (
	"errors"
	"testing"
)

// --- REAL (pass): the substrate's structural guarantees -----------------------

func TestNewGraph_RejectsDuplicateID(t *testing.T) {
	id := ID{Name: "x", Version: "1"}
	_, err := NewGraph(Object{Kind: "a", ID: id}, Object{Kind: "b", ID: id})
	if !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("duplicate id: want ErrInvalidGraph, got %v", err)
	}
}

func TestNewRegistry_RejectsDuplicateKind(t *testing.T) {
	_, err := NewRegistry(Contract{Kind: "server"}, Contract{Kind: "server"})
	if !errors.Is(err, ErrContract) {
		t.Fatalf("duplicate kind: want ErrContract, got %v", err)
	}
}

// --- RED (F0 GREEN targets): the validation contracts -------------------------
// These assert the intended contract behavior; they currently fail on the
// ErrNotImplemented stubs. F0 GREEN implements the contracts and turns them green.

// A server composing a quantity-bearing NIC slot, used by the contract tests.
func contractFixture(t *testing.T) (*Graph, *Registry) {
	t.Helper()
	nic := Object{Kind: "nic", ID: ID{"cx7", "1"}}
	server := Object{
		Kind: "server", ID: ID{"b200", "1"},
		Relations: []Relation{{
			Kind:   "component_slot",
			Target: Ref{ID: ID{"cx7", "1"}},
			Fields: map[string]any{"quantity": 8},
		}},
	}
	g, err := NewGraph(nic, server)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := NewRegistry(
		Contract{Kind: "nic"},
		Contract{
			Kind: "server",
			RequiredByProjection: map[string][]string{
				"bom": {"purchase_profile.part_number"},
			},
			Relations: map[string]RelationContract{
				"component_slot": {Kind: "component_slot", Acyclic: true, QuantityField: "quantity"},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return g, reg
}

func TestValidate_RequiredFieldsPerProjection(t *testing.T) {
	g, reg := contractFixture(t)
	// The server lacks purchase_profile.part_number that the "bom" projection
	// requires → Validate must report a clear ErrContract. (RED: ErrNotImplemented.)
	err := reg.Validate(g, "bom")
	if !errors.Is(err, ErrContract) {
		t.Fatalf("required-field contract: want ErrContract, got %v", err)
	}
}

func TestCheckAcyclic_RejectsCycle(t *testing.T) {
	// a → b → a via component_slot must be rejected with ErrCycle. (RED.)
	a := Object{Kind: "x", ID: ID{"a", "1"}, Relations: []Relation{{Kind: "component_slot", Target: Ref{ID: ID{"b", "1"}}}}}
	b := Object{Kind: "x", ID: ID{"b", "1"}, Relations: []Relation{{Kind: "component_slot", Target: Ref{ID: ID{"a", "1"}}}}}
	g, err := NewGraph(a, b)
	if err != nil {
		t.Fatal(err)
	}
	reg, _ := NewRegistry(Contract{Kind: "x", Relations: map[string]RelationContract{"component_slot": {Kind: "component_slot", Acyclic: true}}})
	if err := reg.CheckAcyclic(g, "component_slot"); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle detection: want ErrCycle, got %v", err)
	}
}

func TestComposeQuantity_MultipliesDownChain(t *testing.T) {
	g, reg := contractFixture(t)
	// 8× CX-7 under one server → effective quantity 8. (RED: ErrNotImplemented.)
	got, err := reg.ComposeQuantity(g, ID{"b200", "1"}, []ID{{"cx7", "1"}})
	if err != nil {
		t.Fatalf("ComposeQuantity: %v", err)
	}
	if got != 8 {
		t.Fatalf("ComposeQuantity: want 8, got %d", got)
	}
}
