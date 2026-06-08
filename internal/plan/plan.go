// Package plan handles plan I/O: reading the canonical plan YAML (D9) and
// converting it to the snake_case plan JSON the kernel decoder expects. The CLI
// does the I/O; the kernel stays pure.
//
// RED phase: YAMLToJSON is unimplemented.
package plan

import "errors"

// ErrNotImplemented is returned by the RED-phase stub.
var ErrNotImplemented = errors.New("plan: not implemented (RED)")

// YAMLToJSON converts plan YAML bytes to the snake_case plan JSON the kernel
// expects. Plan YAML keys already use snake_case, so this is a YAML->JSON
// re-encode (no key remapping).
func YAMLToJSON(yamlBytes []byte) ([]byte, error) {
	return nil, ErrNotImplemented
}
