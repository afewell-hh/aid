package main

// Per-subcommand RED tests. The four real subcommands fail (RunE stubs);
// `serve` is a documented stub whose mux already returns 501 (locked here).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/fixtures"
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

func TestCLI_PlanValidate_InvalidPrintsHumanError(t *testing.T) {
	out, _ := execCmd("plan", "validate", fixtures.PlanYAMLPath("invalid", "mclag-odd-count"))
	// Non-zero exit is expected for an invalid plan; the human-readable message
	// must name the violation.
	if !strings.Contains(strings.ToLower(out), "mclag") {
		t.Errorf("plan validate output lacks the MCLAG violation:\n%s", out)
	}
}

func TestCLI_TopologyCalc(t *testing.T) {
	out, err := execCmd("topology", "calc", fixtures.PlanYAMLPath("valid", "clos-small"))
	if err != nil {
		t.Fatalf("topology calc: %v", err)
	}
	if !strings.Contains(out, "node") {
		t.Errorf("calc output lacks an IR summary:\n%s", out)
	}
}

func TestCLI_TopologyBom_JSON(t *testing.T) {
	out, err := execCmd("topology", "bom", fixtures.PlanYAMLPath("valid", "clos-small"), "--format", "json")
	if err != nil {
		t.Fatalf("topology bom: %v", err)
	}
	if !strings.Contains(out, "{") {
		t.Errorf("bom json output empty:\n%s", out)
	}
}

func TestCLI_ExportWiring(t *testing.T) {
	out, err := execCmd("export", "wiring", fixtures.PlanYAMLPath("valid", "clos-small"), "--fabric", "frontend")
	if err != nil {
		t.Fatalf("export wiring: %v", err)
	}
	if !strings.Contains(out, "wiring.githedgehog.com") {
		t.Errorf("wiring output lacks hhfab CRDs:\n%s", out)
	}
}
