// Package objectmodel is the general, extensible object substrate for the
// rebuilt AID foundation (docs/foundation-redesign.md §4.2, D19). Every modelled
// thing — catalog hardware types, configured server/switch classes, topology
// plan entities — is a typed Object carrying an OPEN, NAMESPACED attribute set
// plus ARBITRARY typed nested relationships. New features extend the model by
// adding attribute namespaces, relation kinds, and projections — never by
// re-foundationing.
//
// This package defines the substrate + the validation-CONTRACT surface (the F0
// implementation gate: stable IDs, required-fields-per-projection, quantity
// composition, component-slot acyclicity, clear errors). The contract checks are
// stubbed in F0 RED (ErrNotImplemented) and implemented in F0 GREEN. No
// calculation lives here (calc is F2+).
package objectmodel

import (
	"errors"
	"fmt"
)

// ErrNotImplemented marks an F0 RED stub whose behavior arrives in F0 GREEN.
var ErrNotImplemented = errors.New("objectmodel: not implemented (F0 GREEN)")

// ID is a stable, pinned object identity. Plans and relations reference objects
// by ID+Version (guardrail 1: pin identity + version/digest, never a bare
// mutable friendly name) so old plans and oracle fixtures stay reproducible.
type ID struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (i ID) String() string { return i.Name + "@" + i.Version }

// Ref is a pinned reference to another object: ID+Version, optionally hardened
// with a content Digest (guardrail 1).
type Ref struct {
	ID
	Digest string `json:"digest,omitempty"`
}

// Object is a typed object in the substrate: a kind, a pinned id, an open set of
// namespaced attribute planes (e.g. calc_profile, purchase_profile, and future
// power/lifecycle/cost planes), and typed nested relationships (component_slots,
// port_templates, …). Attributes and Relations are deliberately open so new
// features attach without schema churn.
type Object struct {
	Kind       string                    `json:"kind"`
	ID         ID                         `json:"id"`
	Attributes map[string]map[string]any `json:"attributes,omitempty"` // namespace -> {field: value}
	Relations  []Relation                `json:"relations,omitempty"`
}

// Relation is a typed edge from an Object to another (Target) plus relation-kind
// fields (e.g. a component_slot carries quantity/required/optional; a
// port_template carries port_kind/cage_type/allowed_transceivers).
type Relation struct {
	Kind   string         `json:"kind"`             // e.g. "component_slot", "port_template"
	Target Ref            `json:"target,omitempty"` // pinned ref (empty for inline-only relations)
	Fields map[string]any `json:"fields,omitempty"`
}

// Graph is a set of objects keyed by ID, the unit the contract checks run over.
type Graph struct {
	objects map[ID]Object
}

// NewGraph builds a graph; duplicate IDs are a hard error (stable-ID contract).
func NewGraph(objs ...Object) (*Graph, error) {
	g := &Graph{objects: make(map[ID]Object, len(objs))}
	for _, o := range objs {
		if _, dup := g.objects[o.ID]; dup {
			return nil, fmt.Errorf("%w: duplicate object id %s", ErrInvalidGraph, o.ID)
		}
		g.objects[o.ID] = o
	}
	return g, nil
}

// Get returns the object for id, if present.
func (g *Graph) Get(id ID) (Object, bool) { o, ok := g.objects[id]; return o, ok }

// Len reports the object count.
func (g *Graph) Len() int { return len(g.objects) }

// --- validation contracts (the F0 implementation gate) ----------------------

// ErrInvalidGraph / ErrContract / ErrCycle are the substrate's clear,
// distinguishable validation errors.
var (
	ErrInvalidGraph = errors.New("objectmodel: invalid graph")
	ErrContract     = errors.New("objectmodel: contract violation")
	ErrCycle        = errors.New("objectmodel: relation cycle")
)

// RelationContract declares the rules for one relation kind: whether it must be
// acyclic (e.g. component_slot) and which field carries the quantity used in
// composition down a nesting chain.
type RelationContract struct {
	Kind          string
	Acyclic       bool
	QuantityField string // "" if the relation is not quantity-bearing
}

// Contract declares, for one object Kind, the required attribute paths PER
// PROJECTION (which fields a consumer such as the BOM/HNP-projection/wiring
// demands) and the contracts for its relation kinds. This is what keeps the
// "open attributes" generality from becoming "anything goes".
type Contract struct {
	Kind             string
	RequiredByProjection map[string][]string // projection -> required "namespace.field" paths
	Relations            map[string]RelationContract
}

// Registry holds the contracts for all object kinds.
type Registry struct {
	contracts map[string]Contract
}

// NewRegistry builds a contract registry; duplicate kinds are an error.
func NewRegistry(cs ...Contract) (*Registry, error) {
	r := &Registry{contracts: make(map[string]Contract, len(cs))}
	for _, c := range cs {
		if _, dup := r.contracts[c.Kind]; dup {
			return nil, fmt.Errorf("%w: duplicate contract for kind %q", ErrContract, c.Kind)
		}
		r.contracts[c.Kind] = c
	}
	return r, nil
}

// Contract returns the contract for a kind.
func (r *Registry) Contract(kind string) (Contract, bool) { c, ok := r.contracts[kind]; return c, ok }

// Validate checks every object against its kind's contract: stable/known IDs,
// required-fields-per-projection presence, and relation contracts. F0 RED stub.
func (r *Registry) Validate(g *Graph, projection string) error {
	return fmt.Errorf("%w: Validate", ErrNotImplemented)
}

// CheckAcyclic verifies the given relation kind forms no cycles across the graph
// (component_slots must be acyclic). F0 RED stub.
func (r *Registry) CheckAcyclic(g *Graph, relationKind string) error {
	return fmt.Errorf("%w: CheckAcyclic(%s)", ErrNotImplemented, relationKind)
}

// ComposeQuantity returns the effective quantity of a nested object reached from
// root via a chain of quantity-bearing relations (the per-unit multiply used by
// the BOM reducer in F3). F0 RED stub — defined here so the composition
// semantics have one home; no calc is performed in F0.
func (r *Registry) ComposeQuantity(g *Graph, root ID, path []ID) (int, error) {
	return 0, fmt.Errorf("%w: ComposeQuantity", ErrNotImplemented)
}
