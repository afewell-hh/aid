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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

// ErrNotImplemented marks the F0 RED stub.
var ErrNotImplemented = errors.New("planschema: not implemented (F0 GREEN)")

// ErrInvalid marks a document that failed schema validation.
var ErrInvalid = errors.New("planschema: document does not validate")

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

// Validate validates a YAML or JSON document against the named schema. It loads
// the JSON Schema under schema/, normalizes the YAML/JSON document into the
// JSON-compatible value tree the validator expects, and validates. A schema
// mismatch returns an error wrapping ErrInvalid with the validator's path-pointed
// detail; a valid document returns nil.
func Validate(schemaName string, doc []byte) error {
	schemaBytes, err := os.ReadFile(SchemaPath(schemaName))
	if err != nil {
		return fmt.Errorf("planschema: read schema %s: %w", schemaName, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaName, bytes.NewReader(schemaBytes)); err != nil {
		return fmt.Errorf("planschema: load schema %s: %w", schemaName, err)
	}
	schema, err := compiler.Compile(schemaName)
	if err != nil {
		return fmt.Errorf("planschema: compile schema %s: %w", schemaName, err)
	}

	v, err := normalize(doc)
	if err != nil {
		return fmt.Errorf("planschema: parse document: %w", err)
	}
	if err := schema.Validate(v); err != nil {
		return fmt.Errorf("%w against %s: %v", ErrInvalid, schemaName, err)
	}
	return nil
}

// normalize parses a YAML (or JSON, a YAML subset) document into the
// JSON-compatible value tree the validator consumes: map[string]any keys,
// []any sequences, and scalar leaves. yaml.v3 already produces string-keyed maps,
// but we coerce defensively so a document authored with non-string keys cannot
// slip through.
func normalize(doc []byte) (any, error) {
	var raw any
	if err := yaml.Unmarshal(doc, &raw); err != nil {
		return nil, err
	}
	return coerce(raw), nil
}

func coerce(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = coerce(val)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprintf("%v", k)] = coerce(val)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, val := range t {
			s[i] = coerce(val)
		}
		return s
	default:
		return v
	}
}
