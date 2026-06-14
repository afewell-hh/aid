package oracle

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/bom"
	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
	"github.com/afewell-hh/aid/internal/topology"
)

// repoRoot is the parent of tests/oracle.
func repoRoot() string { return filepath.Dir(filepath.Dir(Root())) }

// --- REAL (pass): the harness is genuinely wired to the committed oracles ------

func TestLayerA_OraclesWired(t *testing.T) {
	dir := LayerADir()

	bom, err := LoadCSV(filepath.Join(dir, "bom.csv"))
	if err != nil || len(bom) < 2 {
		t.Fatalf("bom.csv not loaded: rows=%d err=%v", len(bom), err)
	}
	conn, err := LoadCSV(filepath.Join(dir, "connectivity-map.csv"))
	if err != nil || len(conn) < 2 {
		t.Fatalf("connectivity-map.csv not loaded: rows=%d err=%v", len(conn), err)
	}
	counts, err := LoadNetboxCounts(filepath.Join(dir, "netbox_inventory.json"))
	if err != nil {
		t.Fatalf("netbox counts: %v", err)
	}
	want := NetboxCounts{Cables: 128, Devices: 21, Interfaces: 481, Modules: 259}
	if counts != want {
		t.Errorf("xoc-64 committed counts: got %+v want %+v", counts, want)
	}
}

func TestLayerB_RealServerBOMWired(t *testing.T) {
	path := filepath.Join(repoRoot(), "docs", "requirements", "real-server-bom.csv")
	rows, err := LoadCSV(path)
	if err != nil || len(rows) < 2 {
		t.Fatalf("real-server-bom.csv not loaded: rows=%d err=%v", len(rows), err)
	}
	blob := strings.Join(func() []string {
		var s []string
		for _, r := range rows {
			s = append(s, strings.Join(r, ","))
		}
		return s
	}(), "\n")
	// Non-physical + nested lines the full BOM must reproduce (R3/R5).
	for _, sku := range []string{"EWCSC", "SVC-NVSTDSWSUP-3Y", "MC0037", "OSNBD3", "AOC-CX766003N-SQ0", "GPU-NVDPU-BA3220-C"} {
		if !strings.Contains(blob, sku) {
			t.Errorf("real-server-bom.csv missing expected line %q", sku)
		}
	}
}

// --- REAL (pass): expected.counts row moves SKIP→PASS in F1 -------------------

// TestLayerA_ExpectedCounts_SelfCheck ingests the xoc-64 training form into the
// relational topology model and reproduces the plan's committed expected.counts.
// This Layer A row needs only ingestion (F1), so unlike the device/cable/inventory
// rows it PASSES rather than skipping.
func TestLayerA_ExpectedCounts_SelfCheck(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(LayerADir(), "training.yaml"))
	if err != nil {
		t.Fatalf("read training.yaml: %v", err)
	}
	plan, _, err := topology.IngestBundled(b)
	if err != nil {
		t.Fatalf("IngestBundled(xoc-64): %v", err)
	}
	if plan.Status == nil || plan.Status.Expected == nil {
		t.Fatal("xoc-64 training form must carry expected.counts")
	}
	computed := ExpectedCounts{
		ServerClasses: len(plan.Spec.ServerClasses),
		SwitchClasses: len(plan.Spec.SwitchClasses),
		Connections:   len(plan.Spec.Connections),
	}
	want := ExpectedCounts{
		ServerClasses: plan.Status.Expected.Counts.ServerClasses,
		SwitchClasses: plan.Status.Expected.Counts.SwitchClasses,
		Connections:   plan.Status.Expected.Counts.Connections,
	}
	diff, err := CompareExpectedCounts(computed, want)
	if err != nil {
		t.Fatalf("CompareExpectedCounts: %v", err)
	}
	if !diff.Equal {
		t.Errorf("xoc-64 expected.counts mismatch: %v (computed %+v want %+v)", diff.Details, computed, want)
	}
	if want != (ExpectedCounts{ServerClasses: 5, SwitchClasses: 3, Connections: 21}) {
		t.Errorf("xoc-64 committed expected.counts = %+v, want {5 3 21}", want)
	}
}

// --- F2 RED: the derived-quantities row fails until the F2 calc lands ----------

// TestLayerA_DerivedQuantities is the headline F2 oracle (note §3, D22): for
// xoc-64, AID's COMPUTED switches-per-class and server quantities must equal the
// committed bom.csv. The oracle side (LoadBOMQuantities) is REAL and asserts the
// known target {soc_storage_scale_out_leaf:2, inb_mgmt_leaf:1, oob_leaf:1} +
// servers {8,3,3,2,1}. The COMPUTED side calls the F2 calc, which is a stub in
// RED — so this test FAILS for the right reason (calc not implemented) until
// GREEN. (Full bom.csv reproduction is F3; wiring is F4; netbox is deferred, D22.)
func TestLayerA_DerivedQuantities(t *testing.T) {
	dir := LayerADir()

	oracleQ, err := LoadBOMQuantities(filepath.Join(dir, "bom.csv"))
	if err != nil {
		t.Fatalf("LoadBOMQuantities: %v", err)
	}
	// The committed bom.csv quantities are the F2 target — proves the oracle is
	// wired to real data regardless of the (pending) calc.
	wantSwitch := map[string]int{"soc_storage_scale_out_leaf": 2, "inb_mgmt_leaf": 1, "oob_leaf": 1}
	wantServer := map[string]int{"compute_xpu": 8, "storage_srv": 3, "metadata_srv": 3, "hh_gateway": 2, "hh_controller": 1}
	for c, w := range wantSwitch {
		if got := oracleQ.SwitchPerClass[c]; got != w {
			t.Fatalf("bom.csv switch %s = %d, want %d", c, got, w)
		}
	}
	for c, w := range wantServer {
		if got := oracleQ.ServerPerClass[c]; got != w {
			t.Fatalf("bom.csv server %s = %d, want %d", c, got, w)
		}
	}

	// Ingest the real plan and compute the topology — the F2 calc.
	b, err := os.ReadFile(filepath.Join(dir, "training.yaml"))
	if err != nil {
		t.Fatalf("read training.yaml: %v", err)
	}
	plan, cat, err := topology.IngestBundled(b)
	if err != nil {
		t.Fatalf("IngestBundled(xoc-64): %v", err)
	}

	sw, srv, err := calc.DeriveQuantities(plan, cat)
	if err != nil {
		// RED: the F2 kernel calc is not wired yet. GREEN makes this pass.
		t.Fatalf("F2 RED — derived-quantities calc not implemented: %v", err)
	}
	computed := DerivedQuantities{SwitchPerClass: sw, ServerPerClass: srv}
	diff, err := CompareDerivedQuantities(computed, oracleQ)
	if err != nil {
		t.Fatalf("CompareDerivedQuantities: %v", err)
	}
	if !diff.Equal {
		t.Errorf("xoc-64 derived quantities mismatch: %v", diff.Details)
	}
}

// --- F3 RED: the BOM oracle rows move pending(skip) → executing(fail) ----------

// TestLayerA_BOMProjection is the headline F3 Layer-A oracle (note §5): AID's BOM
// PROJECTION (internal/bom.RenderProjection) must equal the committed bom.csv
// EXACTLY — all 19 columns, every row, the suppressed-cable-assembly footer. The
// oracle comparator (CompareBOMProjection) is REAL; the COMPUTED side calls the F3
// reducer, a stub in RED — so this FAILS for the right reason until GREEN.
func TestLayerA_BOMProjection(t *testing.T) {
	dir := LayerADir()
	b, err := os.ReadFile(filepath.Join(dir, "training.yaml"))
	if err != nil {
		t.Fatalf("read training.yaml: %v", err)
	}
	plan, cat, err := topology.IngestBundled(b)
	if err != nil {
		t.Fatalf("IngestBundled(xoc-64): %v", err)
	}
	calcOut, err := calc.Compute(plan, cat)
	if err != nil {
		t.Fatalf("calc.Compute(xoc-64): %v", err)
	}
	overlay, err := catalog.Load(filepath.Join(repoRoot(), "tests", "fixtures", "f3", "optic-overlay.yaml"))
	if err != nil {
		t.Fatalf("load optic overlay: %v", err)
	}
	cat.Merge(overlay)

	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		// RED: the F3 reducer is not wired yet. GREEN makes this pass.
		t.Fatalf("F3 RED — BOM projection reducer not implemented: %v", err)
	}
	got, err := bom.RenderProjection(model)
	if err != nil {
		t.Fatalf("RenderProjection: %v", err)
	}
	diff, err := CompareBOMProjection(got, filepath.Join(dir, "bom.csv"))
	if err != nil {
		t.Fatalf("CompareBOMProjection: %v", err)
	}
	if !diff.Equal {
		t.Errorf("xoc-64 BOM projection != bom.csv: %v", diff.Details)
	}
}

// TestLayerB_Scaling is the F3 Layer-B oracle (note §5): AID's FULL purchasable
// BOM (internal/bom.RenderFullBOM) must equal real-server-bom.csv exactly at 1×,
// and scale linearly at 2×. The comparator is REAL; the COMPUTED side calls the F3
// reducer, a stub in RED — so this FAILS for the right reason until GREEN.
// (Replaces the former TestLayerB_Scaling_Pending skip: pending → executing.)
func TestLayerB_Scaling(t *testing.T) {
	oraclePath := filepath.Join(repoRoot(), "docs", "requirements", "real-server-bom.csv")
	b200 := filepath.Join(repoRoot(), "tests", "fixtures", "f3", "b200-server.yaml")
	cat, err := catalog.Load(b200)
	if err != nil {
		t.Fatalf("load b200 fixture: %v", err)
	}
	for _, scale := range []int{1, 2} {
		plan := &topology.Plan{Spec: topology.Spec{
			ServerClasses: []topology.ServerClassUse{{
				ServerClassID: "smc-b200-8gpu",
				ClassRef:      objectmodel.Ref{ID: objectmodel.ID{Name: "smc-b200-8gpu", Version: "1"}},
				Quantity:      scale,
			}},
		}}
		calcOut := &calc.CalcOutput{ServerQuantity: []calc.ClassQty{{ClassID: "smc-b200-8gpu", Quantity: scale}}}

		model, err := bom.Resolve(plan, cat, calcOut)
		if err != nil {
			t.Fatalf("F3 RED — full-BOM reducer not implemented (scale=%d): %v", scale, err)
		}
		got, err := bom.RenderFullBOM(model)
		if err != nil {
			t.Fatalf("RenderFullBOM(scale=%d): %v", scale, err)
		}
		diff, err := CompareFullBOM(got, oraclePath, scale)
		if err != nil {
			t.Fatalf("CompareFullBOM(scale=%d): %v", scale, err)
		}
		if !diff.Equal {
			t.Errorf("B200 full BOM %d× != real-server-bom.csv: %v", scale, diff.Details)
		}
	}
}

// --- PENDING (skip, not red): comparisons need calc (F2+) ---------------------

func TestLayerA_Comparisons_Pending(t *testing.T) {
	dir := LayerADir()
	oracleCounts, err := LoadNetboxCounts(filepath.Join(dir, "netbox_inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	// AID has no computed counts yet (no calc). The harness reports the
	// comparison unimplemented → pending, not a red failure.
	if _, err := CompareCounts(NetboxCounts{}, oracleCounts); errors.Is(err, ErrNotImplemented) {
		t.Skip("Layer A counts comparison pending — needs calc (F2+)")
	} else {
		t.Fatalf("unexpected: comparison resolved before calc: %v", err)
	}
}
