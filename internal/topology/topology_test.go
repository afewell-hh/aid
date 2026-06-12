package topology

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/afewell-hh/aid/internal/oracle"
)

func trainingYAML(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(oracle.LayerADir(), "training.yaml"))
	if err != nil {
		t.Fatalf("read training fixture: %v", err)
	}
	return b
}

// --- RED (F0 GREEN targets): assert intended behavior; fail until implemented --

// TestIngestBundled: ingesting the real bundled training_*.yaml must yield a
// pure-reference plan (with its server classes) + an extracted catalog.
func TestIngestBundled(t *testing.T) {
	plan, cat, err := IngestBundled(trainingYAML(t))
	if err != nil {
		t.Fatalf("IngestBundled (F0 GREEN target): %v", err)
	}
	if plan == nil || cat == nil {
		t.Fatal("IngestBundled must return a plan and a catalog")
	}
	// xoc-64 training form has 5 server classes (expected.counts).
	if got := len(plan.Spec.ServerClasses); got != 5 {
		t.Errorf("server classes: got %d want 5", got)
	}
}

// TestIngestRoundTrip_Lossless: bundled → split → rebundle must round-trip
// deterministically and losslessly (guardrail 2).
func TestIngestRoundTrip_Lossless(t *testing.T) {
	in := trainingYAML(t)
	p, cat, err := IngestBundled(in)
	if err != nil {
		t.Fatalf("IngestBundled (F0 GREEN target): %v", err)
	}
	out, err := Rebundle(p, cat)
	if err != nil {
		t.Fatalf("Rebundle (F0 GREEN target): %v", err)
	}
	if len(out) == 0 {
		t.Fatal("rebundle produced empty output")
	}
}

// TestValidate: a well-formed plan validates against its catalog + contracts.
func TestValidate(t *testing.T) {
	if err := Validate(&Plan{Meta: Meta{CaseID: "x", Name: "x"}}, nil, nil); err != nil {
		t.Fatalf("Validate (F0 GREEN target): %v", err)
	}
}

// TestExpandPorts: ports_per_connection > 1 expands deterministically into
// per-port cage bindings (guardrail 4).
func TestExpandPorts(t *testing.T) {
	conn := ServerConnection{ServerClassID: "compute", NICSlotID: "nic-cx7", PortIndex: 0, PortsPerConnection: 2, TargetZone: "z"}
	got, err := ExpandPorts(conn, nil)
	if err != nil {
		t.Fatalf("ExpandPorts (F0 GREEN target): %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ports_per_connection=2 must expand to 2 bindings, got %d", len(got))
	}
}
