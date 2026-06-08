// Package plan handles plan I/O: reading the canonical plan YAML (D9) and
// converting it to the snake_case plan JSON the kernel decoder expects. The CLI
// does the I/O; the kernel stays pure.
package plan

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// YAMLToJSON converts plan YAML bytes to the snake_case plan JSON the kernel
// expects. Plan YAML keys already use snake_case, so this is a YAML->JSON
// re-encode (no key remapping). yaml.v3 decodes mappings to map[string]any, so
// the result marshals directly to JSON.
func YAMLToJSON(yamlBytes []byte) ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}
