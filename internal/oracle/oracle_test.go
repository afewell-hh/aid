package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
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

func TestLayerB_Scaling_Pending(t *testing.T) {
	path := filepath.Join(repoRoot(), "docs", "requirements", "real-server-bom.csv")
	for _, scale := range []int{1, 2} {
		if _, err := CompareFullBOM(nil, path, scale); errors.Is(err, ErrNotImplemented) {
			t.Skipf("Layer B full-BOM %d× scaling comparison pending — needs calc/reducer (F3)", scale)
		} else {
			t.Fatalf("unexpected: full-BOM comparison resolved before reducer: %v", err)
		}
	}
}
