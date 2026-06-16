package design_test

// F7a RED — the internal/design coordinator (note §1). These tests fail now
// because Resolve is a stub (errNotImplemented); GREEN wires it to the rebuilt
// engine. They reproduce committed ORACLE artifacts end-to-end through the
// facade for ≥1 mesh (xoc-64) and ≥1 Clos (xoc-256) composition — the F7a
// acceptance gate — plus the overlay-ordering and calc-errors-as-data contracts.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/afewell-hh/aid/internal/bom"
	"github.com/afewell-hh/aid/internal/design"
	"github.com/afewell-hh/aid/internal/oracle"
)

// inputsFor builds a coordinator request from a composition's committed
// training.yaml + optic overlay (the same two real inputs the oracle harness
// feeds: oracle_test.go ingest + mergeOverlay).
func inputsFor(t *testing.T, c oracle.Composition) design.Inputs {
	t.Helper()
	training, err := os.ReadFile(filepath.Join(c.Dir(), "training.yaml"))
	if err != nil {
		t.Fatalf("read training.yaml (%s): %v", c.Name, err)
	}
	overlay, err := os.ReadFile(c.OverlayPath())
	if err != nil {
		t.Fatalf("read overlay (%s): %v", c.Name, err)
	}
	return design.Inputs{TrainingYAML: training, OverlayYAML: overlay}
}

func composition(t *testing.T, name string) oracle.Composition {
	t.Helper()
	for _, c := range oracle.Compositions() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("composition %q not in oracle table", name)
	return oracle.Composition{}
}

// TestResolve_ReproducesOracleBOM is the headline F7a gate: the coordinator
// reproduces each composition's committed bom.csv BYTE-FOR-BYTE through the
// rebuilt engine, for a mesh AND a Clos plan. bom.csv is the quantity oracle
// (D22), so this reproduces the computed switch/server quantities by construction.
func TestResolve_ReproducesOracleBOM(t *testing.T) {
	for _, name := range []string{"xoc-64-mesh-conv-ro", "xoc-256-2xopg128-clos-ro"} {
		c := composition(t, name)
		t.Run(name, func(t *testing.T) {
			res, err := design.Resolve(inputsFor(t, c))
			if err != nil {
				t.Fatalf("design.Resolve(%s): %v", name, err)
			}
			if !res.Valid() {
				t.Fatalf("%s: expected a valid plan; calc errors=%+v", name, res.Calc.Errors)
			}
			got, err := bom.RenderProjection(res.BOM)
			if err != nil {
				t.Fatalf("RenderProjection(%s): %v", name, err)
			}
			diff, err := oracle.CompareBOMProjection(got, filepath.Join(c.Dir(), "bom.csv"))
			if err != nil {
				t.Fatalf("CompareBOMProjection(%s): %v", name, err)
			}
			if !diff.Equal {
				t.Errorf("%s: coordinator BOM != committed bom.csv: %v", name, diff.Details)
			}
		})
	}
}

// TestResolve_DerivedSwitchQuantities_XOC256 pins the CALCULATED Clos switch
// counts (no override_quantity) the coordinator must surface in Calc — the
// derivation path, exercised only by xoc-256. Counts are read from the committed
// xoc-256 bom.csv (be-rail-leaf=4, be-spine=2, fe-leaf=2, fe-spine=1; compute=32).
func TestResolve_DerivedSwitchQuantities_XOC256(t *testing.T) {
	c := composition(t, "xoc-256-2xopg128-clos-ro")
	res, err := design.Resolve(inputsFor(t, c))
	if err != nil {
		t.Fatalf("design.Resolve(xoc-256): %v", err)
	}
	wantSwitch := map[string]int{"fe-leaf": 2, "fe-spine": 1, "be-rail-leaf": 4, "be-spine": 2}
	gotSwitch := map[string]int{}
	for _, q := range res.Calc.SwitchQuantity {
		gotSwitch[q.ClassID] = q.Quantity
	}
	for id, want := range wantSwitch {
		if gotSwitch[id] != want {
			t.Errorf("xoc-256 switch_quantity[%s] = %d, want %d (full map: %+v)", id, gotSwitch[id], want, gotSwitch)
		}
	}
	var compute int
	for _, q := range res.Calc.ServerQuantity {
		if q.ClassID == "compute" {
			compute = q.Quantity
		}
	}
	if compute != 32 {
		t.Errorf("xoc-256 server_quantity[compute] = %d, want 32", compute)
	}
}

// TestResolve_OverlayDoesNotAffectCalc locks the load-bearing ordering contract
// (note §1.1, devb finding 1): the overlay enriches only the BOM optic columns, so
// the calc result must be IDENTICAL with and without the overlay. If GREEN ever
// merges the overlay before calc, this catches it.
func TestResolve_OverlayDoesNotAffectCalc(t *testing.T) {
	c := composition(t, "xoc-64-mesh-conv-ro")
	in := inputsFor(t, c)

	withOverlay, err := design.Resolve(in)
	if err != nil {
		t.Fatalf("Resolve(with overlay): %v", err)
	}
	noOverlay, err := design.Resolve(design.Inputs{TrainingYAML: in.TrainingYAML}) // OverlayYAML nil
	if err != nil {
		t.Fatalf("Resolve(no overlay): %v", err)
	}

	// FULL calc invariance (devb F7a RED finding 1): the overlay enriches only the
	// BOM optic columns, so the ENTIRE CalcOutput — quantities, per-endpoint
	// allocation IR, transceiver verdicts, AND errors — must be identical with and
	// without it, not just the headline quantities. Anything less would let a
	// GREEN that merged the overlay before calc slip through if it only perturbed
	// endpoints/verdicts.
	if !reflect.DeepEqual(withOverlay.Calc, noOverlay.Calc) {
		t.Errorf("overlay changed the calc output — it must not affect calc at all\n--- with overlay ---\n%s\n--- without overlay ---\n%s",
			mustJSON(t, withOverlay.Calc), mustJSON(t, noOverlay.Calc))
	}
}

// mustJSON renders v as indented JSON for a readable diff in failure messages.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestResolve_CalcErrorsSurfacedAsData: a structurally-valid but over-allocating
// plan returns a non-nil Resolved (NO Go error) with Valid()==false, the
// violations in Calc.Errors, and BOM left nil (note §3.0 / §1).
func TestResolve_CalcErrorsSurfacedAsData(t *testing.T) {
	training, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "f7a", "overalloc-training.yaml"))
	if err != nil {
		t.Fatalf("read overalloc fixture: %v", err)
	}
	res, err := design.Resolve(design.Inputs{TrainingYAML: training})
	if err != nil {
		t.Fatalf("over-alloc is a calc violation (data), not a structural error; Resolve must not fail: %v", err)
	}
	if res.Valid() {
		t.Errorf("over-alloc plan must be invalid (Valid()==false)")
	}
	if res.Calc == nil || len(res.Calc.Errors) == 0 {
		t.Errorf("over-alloc plan must surface Calc.Errors; got %+v", res.Calc)
	}
	if res.BOM != nil {
		t.Errorf("BOM must be nil when calc has errors; got non-nil")
	}
}
