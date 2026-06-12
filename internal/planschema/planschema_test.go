package planschema

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func oracleDir() string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // repo root
	return filepath.Join(root, "tests", "oracle", "xoc-64-mesh-conv-ro")
}

// --- REAL (pass): the schema artifacts exist and are valid JSON ---------------

func TestSchemaFiles_AreValidJSON(t *testing.T) {
	for _, name := range []string{TopologyPlan, Catalog} {
		b, err := os.ReadFile(SchemaPath(name))
		if err != nil {
			t.Fatalf("read schema %s: %v", name, err)
		}
		var doc any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("schema %s is not valid JSON: %v", name, err)
		}
	}
}

// --- RED (F0 GREEN targets): the new schema validates the REAL files ----------

func TestTopologyPlanSchema_ValidatesTrainingYAML_RED(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(oracleDir(), "training.yaml"))
	if err != nil {
		t.Fatalf("read training fixture: %v", err)
	}
	// The new schema must validate the real diet/XOC training file. RED until
	// F0 GREEN wires the validator + YAML→JSON normalizer.
	if err := Validate(TopologyPlan, b); err != nil {
		if errors.Is(err, ErrNotImplemented) {
			t.Fatalf("schema validation pending (F0 GREEN): %v", err)
		}
		t.Fatalf("training.yaml must validate against %s: %v", TopologyPlan, err)
	}
}

func TestTopologyPlanSchema_ValidatesAuthoredTopologyPlan_RED(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(oracleDir(), "topology-plan.yaml"))
	if err != nil {
		t.Fatalf("read authored topology-plan fixture: %v", err)
	}
	if err := Validate(TopologyPlan, b); err != nil {
		if errors.Is(err, ErrNotImplemented) {
			t.Fatalf("schema validation pending (F0 GREEN): %v", err)
		}
		t.Fatalf("topology-plan.yaml must validate against %s: %v", TopologyPlan, err)
	}
}
