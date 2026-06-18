package main

// Per-subcommand tests for the F7a-retargeted CLI (over internal/design + the
// rebuilt engine). These assert the NEW output shapes; the byte-exact oracle
// reproduction (bom.csv) and the Clos derived counts + wiring live in
// f7a_integration_test.go. Input is the DIET/training bundle + AID optic overlay.

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func execCmd(args ...string) (string, error) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// overAllocFixture is the structurally-valid-but-over-allocating plan (xoc-64 with
// one zone shrunk) — see tests/fixtures/f7a/overalloc-training.yaml.
func overAllocFixture() string {
	return filepath.Join("..", "..", "tests", "fixtures", "f7a", "overalloc-training.yaml")
}

// TestCLI_PlanValidate_Valid: a valid plan reports valid and exits zero.
func TestCLI_PlanValidate_Valid(t *testing.T) {
	training, overlay, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	out, err := execCmd("plan", "validate", training, "--overlay", overlay)
	if err != nil {
		t.Fatalf("valid plan should exit zero: %v\n%s", err, out)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("expected a validity message:\n%s", out)
	}
}

// TestCLI_PlanValidate_InvalidPrintsViolation: an over-allocating plan exits
// non-zero and prints the constraint violation as a human-readable line (the
// calc-errors-as-data surface, note §3.0).
func TestCLI_PlanValidate_InvalidPrintsViolation(t *testing.T) {
	out, err := execCmd("plan", "validate", overAllocFixture())
	if err == nil {
		t.Fatalf("invalid plan must exit non-zero; out=%s", out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected a printed violation (✗ line):\n%s", out)
	}
}

// TestCLI_TopologyCalc: prints the computed switch/server quantities (the new
// CalcOutput shape — the old IR node/edge summary is gone).
func TestCLI_TopologyCalc(t *testing.T) {
	training, overlay, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	out, err := execCmd("topology", "calc", training, "--overlay", overlay)
	if err != nil {
		t.Fatalf("topology calc: %v\n%s", err, out)
	}
	if !strings.Contains(out, "switch quantities") {
		t.Errorf("calc output lacks the switch-quantity summary:\n%s", out)
	}
	if !strings.Contains(out, "soc_storage_scale_out_leaf") {
		t.Errorf("calc output lacks the xoc-64 switch class:\n%s", out)
	}
}

// TestCLI_TopologyBom_JSON: the json view is valid JSON carrying the rows array.
func TestCLI_TopologyBom_JSON(t *testing.T) {
	training, overlay, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	out, err := execCmd("topology", "bom", training, "--overlay", overlay, "--format", "json")
	if err != nil {
		t.Fatalf("topology bom --format json: %v\n%s", err, out)
	}
	if !strings.Contains(out, "\"rows\"") {
		t.Errorf("bom json output lacks the rows array:\n%s", out)
	}
}
