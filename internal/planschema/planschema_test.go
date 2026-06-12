package planschema

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func oracleDir() string    { return filepath.Join(repoRoot(), "tests", "oracle", "xoc-64-mesh-conv-ro") }
func canonicalDir() string { return filepath.Join(repoRoot(), "tests", "oracle", "canonical") }

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

// expectValid asserts a document validates against the schema. RED until the
// validator is wired (F0 GREEN); a trivial impl that rejects everything fails it.
func expectValid(t *testing.T, doc []byte, what string) {
	t.Helper()
	if err := Validate(TopologyPlan, doc); err != nil {
		if errors.Is(err, ErrNotImplemented) {
			t.Fatalf("%s: schema validation pending (F0 GREEN): %v", what, err)
		}
		t.Fatalf("%s must validate against %s: %v", what, TopologyPlan, err)
	}
}

// expectRejected asserts a document is REJECTED with a real validation error
// (not nil, not ErrNotImplemented). A trivial impl that returns nil fails it.
func expectRejected(t *testing.T, doc []byte, what string) {
	t.Helper()
	err := Validate(TopologyPlan, doc)
	if err == nil {
		t.Fatalf("%s must be rejected by %s, but validation passed", what, TopologyPlan)
	}
	if errors.Is(err, ErrNotImplemented) {
		t.Fatalf("%s: schema validation pending (F0 GREEN): %v", what, err)
	}
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

// --- RED (F0 GREEN targets): the schema validates the right docs --------------

// The real EXTERNAL diet/XOC files must validate (the adopted external shape).
func TestTopologyPlanSchema_ValidatesTrainingYAML(t *testing.T) {
	expectValid(t, mustRead(t, filepath.Join(oracleDir(), "training.yaml")), "training.yaml (external)")
}

func TestTopologyPlanSchema_ValidatesAuthoredTopologyPlan(t *testing.T) {
	expectValid(t, mustRead(t, filepath.Join(oracleDir(), "topology-plan.yaml")), "topology-plan.yaml (external)")
}

// The AID-canonical pure-reference shape must validate, inputs-only and with a
// populated expected oracle (D21).
func TestTopologyPlanSchema_ValidatesCanonicalInputOnly(t *testing.T) {
	expectValid(t, mustRead(t, filepath.Join(canonicalDir(), "plan-input-only.yaml")), "canonical input-only")
}

func TestTopologyPlanSchema_ValidatesCanonicalWithExpected(t *testing.T) {
	expectValid(t, mustRead(t, filepath.Join(canonicalDir(), "plan-with-expected.yaml")), "canonical input+expected")
}

// A malformed canonical doc (class_ref missing the pinned version) matches
// neither oneOf branch and must be REJECTED.
func TestTopologyPlanSchema_RejectsMalformedSpec(t *testing.T) {
	expectRejected(t, mustRead(t, filepath.Join(canonicalDir(), "plan-malformed-spec.yaml")), "malformed canonical spec")
}
