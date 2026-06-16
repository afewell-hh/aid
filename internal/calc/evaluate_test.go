package calc_test

// F7a RED — the non-failing calc.Evaluate accessor (note §1.1, devb finding 2).
//
// Evaluate must return the decoded CalcOutput with a populated Errors WITHOUT a
// Go error, so the surfaces can present calc constraint violations as data.
// Compute must keep FAILING on the same input (its engine-internal callers rely
// on fail-fast). These tests fail now because Evaluate is a stub; GREEN factors
// Compute into Evaluate + the error gate.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/topology"
)

// ingest reads a bundled DIET training.yaml into the relational model.
func ingest(t *testing.T, path string) (*topology.Plan, *catalog.Catalog) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	plan, cat, err := topology.IngestBundled(b)
	if err != nil {
		t.Fatalf("IngestBundled(%s): %v", path, err)
	}
	return plan, cat
}

// repo-root-relative fixture paths (tests run with CWD = internal/calc).
func overAllocFixture() string {
	return filepath.Join("..", "..", "tests", "fixtures", "f7a", "overalloc-training.yaml")
}
func xoc64Training() string {
	return filepath.Join("..", "..", "tests", "oracle", "xoc-64-mesh-conv-ro", "training.yaml")
}

// TestOverAllocFixture_Compute_Errors is the fixture self-check AND the Compute
// fail-fast regression guard. It PASSES today: it proves the over-alloc fixture is
// genuinely rejected at calc time (not at ingest), so the Evaluate test below is
// exercising the real calc-errors path. If this ever fails, the fixture stopped
// over-allocating — fix the fixture, not this test.
func TestOverAllocFixture_Compute_Errors(t *testing.T) {
	plan, cat := ingest(t, overAllocFixture())
	if _, err := calc.Compute(plan, cat); err == nil {
		t.Fatalf("over-alloc fixture must make Compute fail (calc raise); got nil error")
	}
}

// TestEvaluate_SurfacesCalcErrorsAsData is the core RED: Evaluate returns the
// CalcOutput with a populated Errors and NO Go error.
func TestEvaluate_SurfacesCalcErrorsAsData(t *testing.T) {
	plan, cat := ingest(t, overAllocFixture())
	out, err := calc.Evaluate(plan, cat)
	if err != nil {
		t.Fatalf("Evaluate must NOT fail on calc constraint violations (surface as data); got err=%v", err)
	}
	if out == nil || len(out.Errors) == 0 {
		t.Fatalf("Evaluate must surface the over-allocation in CalcOutput.Errors; got %+v", out)
	}
}

// TestEvaluate_HappyPath: on a valid plan Evaluate must equal Compute EXACTLY —
// they share the kernel path and differ only in that Evaluate does not fail on a
// non-empty Errors. devb F7a RED finding 2: assert the equality directly, don't
// just assert Evaluate is error-free.
func TestEvaluate_HappyPath(t *testing.T) {
	plan, cat := ingest(t, xoc64Training())

	out, err := calc.Evaluate(plan, cat)
	if err != nil {
		t.Fatalf("Evaluate(xoc-64): unexpected err=%v", err)
	}
	if out == nil || len(out.Errors) != 0 {
		t.Fatalf("Evaluate(xoc-64) must be error-free; got %+v", out)
	}

	want, err := calc.Compute(plan, cat)
	if err != nil {
		t.Fatalf("Compute(xoc-64) should succeed on a valid plan: %v", err)
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("Evaluate(xoc-64) must equal Compute(xoc-64) on a valid plan\nEvaluate: %+v\nCompute:  %+v", out, want)
	}
}
