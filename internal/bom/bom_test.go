package bom_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/afewell-hh/aid/internal/bom"
	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
	"github.com/afewell-hh/aid/internal/topology"
)

// repoRoot is the parent of internal/ (internal/bom/bom_test.go → ../../).
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func loadCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // BOM CSVs carry a trailing comment/footer row
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return rows
}

const (
	opticOverlayFixture = "tests/fixtures/f3/optic-overlay.yaml"
	b200Fixture         = "tests/fixtures/f3/b200-server.yaml"
	xoc64Dir            = "tests/oracle/xoc-64-mesh-conv-ro"
)

// --- GREEN: the AID-owned overlay/fixtures are well-formed --------------------

// TestFixtures_Loadable proves the hand-authored AID overlay + B200 line-template
// fixture parse and carry the expected structure (the F3 catalog deliverables,
// note §3). This PASSES independently of the (stubbed) reducer — it is the
// "fixtures are authored correctly" check, separate from "reducer not implemented".
func TestFixtures_Loadable(t *testing.T) {
	root := repoRoot()

	// Optic overlay: the breakout edge case + a 1x optic.
	overlay, err := catalog.Load(filepath.Join(root, opticOverlayFixture))
	if err != nil {
		t.Fatalf("load optic overlay: %v", err)
	}
	vr, ok := overlay.Get(objectmodel.ID{Name: "r4113_a9220_vr", Version: "1"})
	if !ok {
		t.Fatal("overlay missing r4113_a9220_vr")
	}
	if got, _ := vr.CalcProfile["breakout_topology"].(string); got != "2x400g" {
		t.Errorf("r4113_a9220_vr breakout_topology = %q, want 2x400g (the only non-1x)", got)
	}
	dr, ok := overlay.Get(objectmodel.ID{Name: "osfp_400g_dr4", Version: "1"})
	if !ok {
		t.Fatal("overlay missing osfp_400g_dr4")
	}
	if got, _ := dr.CalcProfile["cage_type"].(string); got != "OSFP" {
		t.Errorf("osfp_400g_dr4 cage_type = %q, want OSFP", got)
	}

	// B200 fixture: 8× CX-7 slot + BF3 (1 fixed BMC + 2 cages) + non-physical lines.
	cat, err := catalog.Load(filepath.Join(root, b200Fixture))
	if err != nil {
		t.Fatalf("load b200 fixture: %v", err)
	}
	srv, ok := cat.Get(objectmodel.ID{Name: "smc-b200-8gpu", Version: "1"})
	if !ok {
		t.Fatal("b200 fixture missing smc-b200-8gpu")
	}
	// Two nested component slots: CX-7 ×8 (one quantity-bearing slot) + BF3 ×1.
	var cx7Qty, dpuQty int
	for _, s := range srv.ComponentSlots {
		switch s.SlotID {
		case "nic-scale-out":
			cx7Qty = s.Quantity
		case "dpu":
			dpuQty = s.Quantity
		}
	}
	if cx7Qty != 8 {
		t.Errorf("CX-7 slot quantity = %d, want 8 (one quantity-bearing slot)", cx7Qty)
	}
	if dpuQty != 1 {
		t.Errorf("BF3 DPU slot quantity = %d, want 1", dpuQty)
	}
	// Non-physical lines present and flagged physical:false.
	wantNonPhysical := map[string]bool{"EWCSC": false, "SVC-NVSTDSWSUP-3Y": false, "MC0037": false, "OSNBD3": false}
	seen := map[string]bool{}
	for _, l := range srv.BOMLineTemplates {
		if _, want := wantNonPhysical[l.InlineSKU]; want {
			seen[l.InlineSKU] = true
			if l.Physical {
				t.Errorf("line %s should be physical:false (non-physical)", l.InlineSKU)
			}
		}
	}
	for sku := range wantNonPhysical {
		if !seen[sku] {
			t.Errorf("B200 fixture missing non-physical line %s", sku)
		}
	}

	// BF3 DPU: one fixed_interface (BMC) + two transceiver cages.
	bf3, ok := cat.Get(objectmodel.ID{Name: "bf3_dpu", Version: "1"})
	if !ok {
		t.Fatal("b200 fixture missing bf3_dpu")
	}
	var fixed, cages int
	for _, p := range bf3.PortTemplates {
		switch p.PortKind {
		case catalog.FixedInterface:
			fixed++
		case catalog.TransceiverCage:
			cages++
		}
	}
	if fixed != 1 || cages != 2 {
		t.Errorf("BF3 ports = %d fixed + %d cages, want 1 fixed (BMC) + 2 cages", fixed, cages)
	}
}

// --- helpers to assemble resolver inputs (RED tests fail at Resolve) ----------

// xoc64Inputs ingests the xoc-64 plan, runs the F2 calc (pure extracted catalog),
// then merges the AID optic overlay for rendering — the exact inputs the F3
// projection reducer consumes.
func xoc64Inputs(t *testing.T) (*topology.Plan, *catalog.Catalog, *calc.CalcOutput) {
	t.Helper()
	root := repoRoot()
	b, err := os.ReadFile(filepath.Join(root, xoc64Dir, "training.yaml"))
	if err != nil {
		t.Fatalf("read training.yaml: %v", err)
	}
	plan, cat, err := topology.IngestBundled(b)
	if err != nil {
		t.Fatalf("IngestBundled(xoc-64): %v", err)
	}
	calcOut, err := calc.Compute(plan, cat) // F2 path on the pure extracted catalog
	if err != nil {
		t.Fatalf("calc.Compute(xoc-64): %v", err)
	}
	overlay, err := catalog.Load(filepath.Join(root, opticOverlayFixture))
	if err != nil {
		t.Fatalf("load optic overlay: %v", err)
	}
	cat.Merge(overlay)
	return plan, cat, calcOut
}

// b200Inputs builds a server-only plan of `qty` B200 servers + the matching
// hand-built F2 server quantities (a standalone server BOM has no switch math).
func b200Inputs(t *testing.T, qty int) (*topology.Plan, *catalog.Catalog, *calc.CalcOutput) {
	t.Helper()
	cat, err := catalog.Load(filepath.Join(repoRoot(), b200Fixture))
	if err != nil {
		t.Fatalf("load b200 fixture: %v", err)
	}
	plan := &topology.Plan{Spec: topology.Spec{
		ServerClasses: []topology.ServerClassUse{{
			ServerClassID: "smc-b200-8gpu",
			ClassRef:      objectmodel.Ref{ID: objectmodel.ID{Name: "smc-b200-8gpu", Version: "1"}},
			Quantity:      qty,
		}},
	}}
	calcOut := &calc.CalcOutput{ServerQuantity: []calc.ClassQty{{ClassID: "smc-b200-8gpu", Quantity: qty}}}
	return plan, cat, calcOut
}

// --- RED: the F3 reducer is a stub; every test fails at Resolve ---------------

// 1. The headline projection oracle: RenderProjection == bom.csv exactly
// (19 columns, every row, the suppressed-cable footer).
func TestProjection_XOC64_ExactBOMCsv(t *testing.T) {
	plan, cat, calcOut := xoc64Inputs(t)
	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		t.Fatalf("F3 RED — reducer not implemented: %v", err)
	}
	got, err := bom.RenderProjection(model)
	if err != nil {
		t.Fatalf("RenderProjection: %v", err)
	}
	want := loadCSV(t, filepath.Join(repoRoot(), xoc64Dir, "bom.csv"))
	assertCSVEqual(t, "bom.csv projection", got, want)
}

// 2. The full-BOM oracle: RenderFullBOM == real-server-bom.csv exactly at 1×.
func TestFullBOM_B200_RealServerBom(t *testing.T) {
	plan, cat, calcOut := b200Inputs(t, 1)
	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		t.Fatalf("F3 RED — reducer not implemented: %v", err)
	}
	got, err := bom.RenderFullBOM(model)
	if err != nil {
		t.Fatalf("RenderFullBOM: %v", err)
	}
	want := loadCSV(t, filepath.Join(repoRoot(), "docs", "requirements", "real-server-bom.csv"))
	assertCSVEqual(t, "real-server-bom.csv (1×)", got, want)
}

// 3. Linear scaling: 2× scales every QTY linearly (with a 1× control).
func TestFullBOM_B200_LinearScaling(t *testing.T) {
	const qtyCol = 3 // real-server-bom.csv: Type,SMC PN,Desc,QTY,...
	base := func(t *testing.T) [][]string {
		plan, cat, calcOut := b200Inputs(t, 1)
		model, err := bom.Resolve(plan, cat, calcOut)
		if err != nil {
			t.Fatalf("F3 RED — reducer not implemented: %v", err)
		}
		got, err := bom.RenderFullBOM(model)
		if err != nil {
			t.Fatalf("RenderFullBOM(1×): %v", err)
		}
		return got
	}
	one := base(t)

	plan2, cat2, calc2 := b200Inputs(t, 2)
	model2, err := bom.Resolve(plan2, cat2, calc2)
	if err != nil {
		t.Fatalf("F3 RED — reducer not implemented: %v", err)
	}
	two, err := bom.RenderFullBOM(model2)
	if err != nil {
		t.Fatalf("RenderFullBOM(2×): %v", err)
	}
	if len(one) != len(two) {
		t.Fatalf("2× changed the line count: 1×=%d rows, 2×=%d rows", len(one), len(two))
	}
	for i := range one {
		if len(one[i]) <= qtyCol || len(two[i]) <= qtyCol {
			continue // header / blank / footer row
		}
		q1, err1 := atoiOK(one[i][qtyCol])
		q2, err2 := atoiOK(two[i][qtyCol])
		if !err1 || !err2 {
			continue // non-numeric QTY (header/blank)
		}
		if q2 != q1*2 {
			t.Errorf("row %d %q: QTY 2×=%d, want %d (linear)", i, one[i][0], q2, q1*2)
		}
	}
}

// 4. The anti-drift structural invariant (note §2.3): every physical projection
// row is accounted for by the full BOM — the projection is a subset, never a
// second counted path.
func TestProjection_IsSubsetOfFullBOM(t *testing.T) {
	plan, cat, calcOut := xoc64Inputs(t)
	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		t.Fatalf("F3 RED — reducer not implemented: %v", err)
	}
	// One resolved model; both renders are pure views of it. The full line set
	// must account for every projected line by the APPROVED structural key
	// (kind, model, fleet_qty) — note §2.3 / f3-architecture-note.md:139-142. The
	// kind component is load-bearing: keying on (model, qty) alone would let a
	// GREEN reducer drift on classification (e.g. mis-section a line) while the
	// same model+qty appears elsewhere in the full set; (kind, model, fleet_qty)
	// closes that hole.
	type key struct {
		kind  string
		model string
		qty   int
	}
	full := map[key]bool{}
	var projected []key
	for _, l := range model.Lines {
		full[key{l.Kind, l.Model, l.FleetQuantity}] = true
		if l.Projected {
			projected = append(projected, key{l.Kind, l.Model, l.FleetQuantity})
		}
	}
	if len(projected) == 0 {
		t.Fatal("no projected lines in the resolved model")
	}
	for _, p := range projected {
		if !full[p] {
			t.Errorf("projected line (kind=%q model=%q ×%d) is not present in the full BOM — projection drifted into a second counted path", p.kind, p.model, p.qty)
		}
	}
}

// 5. The switch-transceiver distinct-physical-cage breakout edge case (note §3.2):
// a breakout cage holds ONE optic (not one per logical port). The r4113_a9220_vr
// 2x400g rows must count physical cages, yielding R4113-VR ×11 and OSFP-400G-DR4 ×32.
func TestProjection_SwitchTransceiver_PerPhysicalCage(t *testing.T) {
	plan, cat, calcOut := xoc64Inputs(t)
	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		t.Fatalf("F3 RED — reducer not implemented: %v", err)
	}
	got, err := bom.RenderProjection(model)
	if err != nil {
		t.Fatalf("RenderProjection: %v", err)
	}
	want := map[string]int{"R4113-A9220-VR": 11, "OSFP-400G-DR4": 32}
	seen := map[string]int{}
	for _, r := range got {
		if len(r) < 6 || r[0] != "switch_transceiver" {
			continue
		}
		if q, ok := atoiOK(r[5]); ok {
			seen[r[1]] = q
		}
	}
	for sku, w := range want {
		if seen[sku] != w {
			t.Errorf("switch_transceiver %s = %d, want %d (distinct physical cages, not logical ports)", sku, seen[sku], w)
		}
	}
}

// 6. Non-physical + nested coverage: the full BOM carries the warranty/support/
// assembly/onsite lines and the nested 8× CX-7 + 1× BF3 rows.
func TestFullBOM_NonPhysicalAndNested(t *testing.T) {
	plan, cat, calcOut := b200Inputs(t, 1)
	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		t.Fatalf("F3 RED — reducer not implemented: %v", err)
	}
	got, err := bom.RenderFullBOM(model)
	if err != nil {
		t.Fatalf("RenderFullBOM: %v", err)
	}
	qtyBySKU := map[string]string{}
	for _, r := range got {
		if len(r) >= 4 {
			qtyBySKU[r[1]] = r[3]
		}
	}
	for _, sku := range []string{"EWCSC", "SVC-NVSTDSWSUP-3Y", "MC0037", "OSNBD3"} {
		if _, ok := qtyBySKU[sku]; !ok {
			t.Errorf("full BOM missing non-physical line %s", sku)
		}
	}
	if qtyBySKU["AOC-CX766003N-SQ0"] != "8" {
		t.Errorf("nested CX-7 qty = %q, want 8", qtyBySKU["AOC-CX766003N-SQ0"])
	}
	if qtyBySKU["GPU-NVDPU-BA3220-C"] != "1" {
		t.Errorf("nested BF3 qty = %q, want 1", qtyBySKU["GPU-NVDPU-BA3220-C"])
	}
}

// --- small assertion helpers --------------------------------------------------

func assertCSVEqual(t *testing.T, label string, got, want [][]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d rows, want %d", label, len(got), len(want))
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Errorf("%s row %d: %d cols, want %d\n got=%v\nwant=%v", label, i, len(got[i]), len(want[i]), got[i], want[i])
			continue
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Errorf("%s row %d col %d: got %q want %q", label, i, j, got[i][j], want[i][j])
			}
		}
	}
}

func atoiOK(s string) (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}
