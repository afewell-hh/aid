// Package planschema is the schema-validation entry point for the rebuilt
// foundation (docs/foundation-redesign.md §4.1, D18). The canonical wire
// contracts are the JSON Schemas under schema/ (topology-plan-v2.json,
// catalog-v1.json) — language-neutral, validating the real diet/XOC files. Go
// types (internal/{topology,catalog,objectmodel}) are the F0 consumer; the
// MoonBit kernel (calc, F2) and Rust reducers consume the same schemas later.
//
// F0 RED: Validate is a stub. F0 GREEN wires a JSON-Schema validator
// (santhosh-tekuri/jsonschema/v5) and a YAML→JSON normalizer so the real
// training_*.yaml validates.
package planschema

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

// ErrNotImplemented marks the F0 RED stub.
var ErrNotImplemented = errors.New("planschema: not implemented (F0 GREEN)")

// Schema names (files under schema/).
const (
	TopologyPlan = "topology-plan-v2.json"
	Catalog      = "catalog-v1.json"
)

// SchemaDir returns the repo's schema/ directory, located from any working dir.
func SchemaDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "schema")
}

// SchemaPath returns the absolute path to a named schema.
func SchemaPath(name string) string { return filepath.Join(SchemaDir(), name) }

// Validate validates a YAML or JSON document against the named schema. F0 RED
// stub — F0 GREEN loads the schema, normalizes YAML→JSON, and validates,
// returning a clear, path-pointed error on mismatch.
func Validate(schemaName string, doc []byte) error {
	return fmt.Errorf("%w: Validate(%s)", ErrNotImplemented, schemaName)
}
