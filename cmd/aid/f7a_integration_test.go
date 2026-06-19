package main

// F7a RED — CLI-level integration over the rebuilt engine (note §3.1 / §5).
//
// These exercise the RETARGETED CLI: the commands must accept a DIET/training
// bundle (HNP's authoring format, D25) + an AID optic overlay via --overlay and
// reproduce the committed oracle artifacts through internal/design. They FAIL now
// because the commands still route through internal/orchestrate + the old plan
// schema (commands.go) and have no --overlay flag — i.e. they fail for the right
// reason (CLI not yet retargeted). GREEN points them at the coordinator.
//
// The existing per-subcommand tests in cli_test.go (old-schema fixtures) stay
// GREEN until F7d retirement — this file adds the new end-state behavior only and
// does not touch them.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/oracle"
)

// oracleArtifacts returns the committed training.yaml path, overlay path, and the
// expected bom.csv text for a composition.
func oracleArtifacts(t *testing.T, name string) (training, overlay, wantBOM string) {
	t.Helper()
	var c oracle.Composition
	for _, cand := range oracle.Compositions() {
		if cand.Name == name {
			c = cand
		}
	}
	if c.Name == "" {
		t.Fatalf("composition %q not in oracle table", name)
	}
	training = filepath.Join(c.Dir(), "training.yaml")
	overlay = c.OverlayPath()
	b, err := os.ReadFile(filepath.Join(c.Dir(), "bom.csv"))
	if err != nil {
		t.Fatalf("read bom.csv (%s): %v", name, err)
	}
	return training, overlay, string(b)
}

// TestCLI_F7a_TopologyBom_ReproducesOracle: `aid topology bom <training.yaml>
// --overlay <overlay> --format csv` prints the committed bom.csv, for a mesh and a
// Clos composition.
func TestCLI_F7a_TopologyBom_ReproducesOracle(t *testing.T) {
	for _, name := range []string{"xoc-64-mesh-conv-ro", "xoc-256-2xopg128-clos-ro"} {
		t.Run(name, func(t *testing.T) {
			training, overlay, wantBOM := oracleArtifacts(t, name)
			out, _ := execCmd("topology", "bom", training, "--overlay", overlay, "--format", "csv")
			// The committed bom.csv has a trailing footer line; compare on
			// trimmed content so whitespace at the edges does not mask a match.
			if strings.TrimSpace(out) != strings.TrimSpace(wantBOM) {
				t.Errorf("%s: `topology bom` not yet reproducing committed bom.csv through the rebuilt engine (CLI not retargeted)\n--- got ---\n%s\n--- want ---\n%s",
					name, out, wantBOM)
			}
		})
	}
}

// TestCLI_F7a_TopologyCalc_ShowsDerivedQuantities: `aid topology calc` over the
// xoc-256 Clos plan prints the CALCULATED switch counts (no override_quantity) —
// the derivation path. The old command printed an IR node/edge summary instead
// (commands.go), so this fails until the calc surface is retargeted (note §3.1:
// drop the IR display for the CalcOutput shape).
func TestCLI_F7a_TopologyCalc_ShowsDerivedQuantities(t *testing.T) {
	training, overlay, _ := oracleArtifacts(t, "xoc-256-2xopg128-clos-ro")
	out, _ := execCmd("topology", "calc", training, "--overlay", overlay)
	// Derived Clos counts from the committed xoc-256 bom.csv.
	for _, want := range []string{"be-rail-leaf", "be-spine", "fe-leaf", "fe-spine"} {
		if !strings.Contains(out, want) {
			t.Errorf("xoc-256 `topology calc` output lacks switch class %q (derived-quantity surface not retargeted)\n%s", want, out)
		}
	}
}

// TestCLI_F7a_ExportWiring_ReproducesFabrics: `aid export wiring` over the xoc-64
// mesh plan emits hhfab wiring CRDs for the managed fabrics through the
// coordinator. (The full hhfab-validate structural gate stays in the oracle
// suite; this is the CLI smoke that the retargeted command produces real CRDs.)
func TestCLI_F7a_ExportWiring_ReproducesFabrics(t *testing.T) {
	training, overlay, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	out, _ := execCmd("export", "wiring", training, "--overlay", overlay)
	if !strings.Contains(out, "wiring.githedgehog.com") {
		t.Errorf("xoc-64 `export wiring` lacks hhfab CRDs through the rebuilt engine (CLI not retargeted)\n%s", out)
	}
}
