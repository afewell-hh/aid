package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/bom"
	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
)

// Negative controls for the F5 parametric oracle (Issue #62, lead ruling): for a
// fixtures/coverage phase with no engine-RED, the COMMITTED guard against a
// VACUOUS oracle is the RED. Each case drives a deliberate mismatch through one of
// the three comparators and asserts it is caught (err != nil or diff.Equal ==
// false) — proving the green suite in oracle_test.go has teeth, not that the
// comparators trivially agree.
//
// Scope: oracle harness only. No engine/derivation/normalization/Clos changes
// (D24); nothing in the positive suite is weakened.

// compByName looks up a composition by dir name (no reliance on slice order).
func compByName(t *testing.T, name string) Composition {
	t.Helper()
	for _, c := range Compositions() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("composition %q not in table", name)
	return Composition{}
}

// TestNegative_OverlayIsLoadBearing is the must-have control: rendering xoc-128
// with the WRONG (xoc-64) overlay must FAIL the byte-exact BOM projection. This
// commits the teeth shown transiently in the RED report — if the per-composition
// overlay (osfp_200g_dr4 + the _a/_b class identities) were not load-bearing, the
// projection would still match and this test would (wrongly) pass.
func TestNegative_OverlayIsLoadBearing(t *testing.T) {
	c128 := compByName(t, "xoc-128-2xopg64-mesh-conv-ro")
	c64 := compByName(t, "xoc-64-mesh-conv-ro")

	plan, cat := ingest(t, c128)
	calcOut, err := calc.Compute(plan, cat)
	if err != nil {
		t.Fatalf("calc.Compute(xoc-128): %v", err)
	}
	// Deliberately merge the xoc-64 overlay onto the xoc-128 catalog.
	wrong, err := catalog.Load(c64.OverlayPath())
	if err != nil {
		t.Fatalf("load xoc-64 overlay: %v", err)
	}
	cat.Merge(wrong)

	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		t.Fatalf("bom.Resolve: %v", err)
	}
	got, err := bom.RenderProjection(model)
	if err != nil {
		t.Fatalf("RenderProjection: %v", err)
	}
	diff, err := CompareBOMProjection(got, filepath.Join(c128.Dir(), "bom.csv"))
	if err != nil {
		t.Fatalf("CompareBOMProjection: %v", err)
	}
	if diff.Equal {
		t.Fatal("negative control FAILED: xoc-128 BOM projection matched the committed bom.csv with the WRONG (xoc-64) overlay — the overlay is not load-bearing or the comparator is vacuous")
	}
}

// TestNegative_WiringComparatorNonVacuous proves CompareWiringHhfab does not
// vacuously pass: (a) an empty oracle wiring dir (zero wiring-*.yaml matched) is
// an error, not a silent pass; (b) when committed fabrics exist but a computed
// fabric is dropped, the comparison is not Equal.
func TestNegative_WiringComparatorNonVacuous(t *testing.T) {
	c128 := compByName(t, "xoc-128-2xopg64-mesh-conv-ro")
	wiringDir := filepath.Join(c128.Dir(), "wiring")

	// (a) empty oracle dir → error (guards a vacuous glob).
	if _, err := CompareWiringHhfab(map[string][]byte{}, t.TempDir()); err == nil {
		t.Error("negative control FAILED: CompareWiringHhfab returned no error for an oracle dir with zero wiring-*.yaml")
	}

	// Build a computed map from the committed wiring (fabric → bytes), keyed the
	// same way the comparator extracts fabric from wiring-<fabric>.yaml.
	entries, err := filepath.Glob(filepath.Join(wiringDir, "wiring-*.yaml"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("glob committed wiring: entries=%d err=%v", len(entries), err)
	}
	computed := map[string][]byte{}
	for _, p := range entries {
		fabric := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "wiring-"), ".yaml")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		computed[fabric] = b
	}

	// Baseline sanity: the full committed set compares Equal (so the failure below
	// is attributable to the dropped fabric, not an unrelated parse mismatch).
	if diff, err := CompareWiringHhfab(computed, wiringDir); err != nil {
		t.Fatalf("baseline CompareWiringHhfab: %v", err)
	} else if !diff.Equal {
		t.Fatalf("baseline CompareWiringHhfab not Equal (committed vs itself): %v", diff.Details)
	}

	// (b) drop one fabric → must not be Equal.
	var dropped string
	for f := range computed {
		dropped = f
		delete(computed, f)
		break
	}
	diff, err := CompareWiringHhfab(computed, wiringDir)
	if err != nil {
		t.Fatalf("CompareWiringHhfab (dropped %s): %v", dropped, err)
	}
	if diff.Equal {
		t.Fatalf("negative control FAILED: CompareWiringHhfab passed with fabric %q dropped from the computed set", dropped)
	}
}

// TestNegative_BOMComparatorNonVacuous proves CompareBOMProjection catches a
// single-cell difference: load the committed bom.csv, mutate exactly one cell
// in-memory (no committed-corrupt file), and confirm the projection no longer
// matches.
func TestNegative_BOMComparatorNonVacuous(t *testing.T) {
	c128 := compByName(t, "xoc-128-2xopg64-mesh-conv-ro")
	bomPath := filepath.Join(c128.Dir(), "bom.csv")

	rows, err := LoadCSV(bomPath)
	if err != nil {
		t.Fatalf("LoadCSV: %v", err)
	}

	// Baseline: the committed file compared against itself is Equal.
	if diff, err := CompareBOMProjection(deepCopyRows(rows), bomPath); err != nil {
		t.Fatalf("baseline CompareBOMProjection: %v", err)
	} else if !diff.Equal {
		t.Fatalf("baseline CompareBOMProjection not Equal (committed vs itself): %v", diff.Details)
	}

	// Mutate exactly one cell on the first data row.
	mutated := deepCopyRows(rows)
	if len(mutated) < 2 || len(mutated[1]) == 0 {
		t.Fatalf("bom.csv shape unexpected: %d rows", len(mutated))
	}
	mutated[1][len(mutated[1])-1] = mutated[1][len(mutated[1])-1] + "_X"

	diff, err := CompareBOMProjection(mutated, bomPath)
	if err != nil {
		t.Fatalf("CompareBOMProjection: %v", err)
	}
	if diff.Equal {
		t.Fatal("negative control FAILED: CompareBOMProjection passed after a one-cell mutation of the projection")
	}
}

func deepCopyRows(in [][]string) [][]string {
	out := make([][]string, len(in))
	for i, r := range in {
		row := make([]string, len(r))
		copy(row, r)
		out[i] = row
	}
	return out
}
