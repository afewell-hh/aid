// Package oracle is the F0 oracle harness (docs/foundation-redesign.md §4.5,
// D20). It wires the two validation layers against the committed reference
// artifacts:
//
//   - Layer A — the XOC physical/topology subset. For a composition (xoc-64
//     first) AID's outputs are compared to the committed bom.csv (the §4.4
//     projection), connectivity-map.csv, netbox_inventory.json .metadata.counts,
//     wiring/*.yaml (hhfab validate), and expected.counts.
//   - Layer B — the owner's FULL purchasable BOM (docs/requirements/
//     real-server-bom.csv), incl. 1×/2× linear-scaling.
//
// F0 wires the harness and LOADS the real oracles (proving it is genuinely
// wired); the COMPARISONS need computed output from calc (F2+) and are therefore
// reported unimplemented (ErrNotImplemented) — their tests are pending/skipped so
// CI stays green, not red.
package oracle

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ErrNotImplemented marks a comparison that needs calc (F2+) — pending, not a
// failure.
var ErrNotImplemented = errors.New("oracle: comparison not implemented (needs calc, F2+)")

// ErrNotImplementedIngest marks an INGESTION-level comparison still stubbed in
// RED. It is distinct from ErrNotImplemented because such a row needs only F1
// ingestion (no calc) and must move SKIP→PASS in F1 — so the phase boundary is
// not misstated as "needs calc, F2+".
var ErrNotImplementedIngest = errors.New("oracle: comparison not implemented (F1 GREEN — ingestion only, no calc)")

// Root returns the vendored oracle directory (tests/oracle), located from any
// working dir.
func Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(file))), "tests", "oracle")
}

// LayerADir returns the xoc-64 mesh-conv-ro composition dir.
func LayerADir() string { return filepath.Join(Root(), "xoc-64-mesh-conv-ro") }

// --- loaders (REAL — prove the harness is wired) ----------------------------

// LoadCSV reads a committed CSV oracle into rows (header + data). Used for
// bom.csv / connectivity-map.csv / real-server-bom.csv.
func LoadCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // BOM csv has a trailing comment row
	return r.ReadAll()
}

// NetboxCounts is the committed inventory count oracle (.metadata.counts).
type NetboxCounts struct {
	Cables     int `json:"cables"`
	Devices    int `json:"devices"`
	Interfaces int `json:"interfaces"`
	Modules    int `json:"modules"`
}

// LoadNetboxCounts reads netbox_inventory.json .metadata.counts.
func LoadNetboxCounts(path string) (NetboxCounts, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return NetboxCounts{}, err
	}
	var doc struct {
		Metadata struct {
			Counts NetboxCounts `json:"counts"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return NetboxCounts{}, err
	}
	return doc.Metadata.Counts, nil
}

// --- comparisons (PENDING — need calc, F2+) ---------------------------------

// Diff is a structured comparison result (populated in later phases).
type Diff struct {
	Equal   bool
	Details []string
}

// CompareBOMProjection compares AID's BOM projection (the HNP 19-column shape)
// to the committed bom.csv. F0: pending (no calc).
func CompareBOMProjection(computedCSV []byte, oracleBOMPath string) (Diff, error) {
	return Diff{}, fmt.Errorf("%w: CompareBOMProjection", ErrNotImplemented)
}

// CompareConnectivityMap compares AID's connectivity map to connectivity-map.csv
// (set-equality of cable endpoint tuples). F0: pending.
func CompareConnectivityMap(computedCSV []byte, oraclePath string) (Diff, error) {
	return Diff{}, fmt.Errorf("%w: CompareConnectivityMap", ErrNotImplemented)
}

// CompareCounts compares AID's inventory counts to the committed counts. F0:
// pending.
func CompareCounts(computed NetboxCounts, oracle NetboxCounts) (Diff, error) {
	return Diff{}, fmt.Errorf("%w: CompareCounts", ErrNotImplemented)
}

// ExpectedCounts is the plan's self-check oracle (the real format's
// expected.counts). A local type avoids an import cycle with internal/topology.
type ExpectedCounts struct {
	ServerClasses int `json:"server_classes"`
	SwitchClasses int `json:"switch_classes"`
	Connections   int `json:"connections"`
}

// CompareExpectedCounts compares the counts AID derived from the ingested
// relational model to the plan's committed expected.counts (Layer A's
// expected.counts row, D20/D21). Unlike the other Layer A comparisons it needs
// only ingestion (F1), so it is IMPLEMENTED here and PASSES for xoc-64.
//
// F1 RED stub: implemented in GREEN (ingestion only — NOT gated on calc).
func CompareExpectedCounts(computed, oracle ExpectedCounts) (Diff, error) {
	return Diff{}, fmt.Errorf("%w: CompareExpectedCounts", ErrNotImplementedIngest)
}

// CompareWiringHhfab generates wiring CRDs and runs the existing hhfab validate
// harness against them. F0: pending.
func CompareWiringHhfab(computedWiringDir, oracleWiringDir string) (Diff, error) {
	return Diff{}, fmt.Errorf("%w: CompareWiringHhfab", ErrNotImplemented)
}

// CompareFullBOM compares AID's FULL purchasable BOM (Layer B) to
// real-server-bom.csv at the given server-quantity scale (1×, 2×, …),
// asserting linear scaling. F0: pending.
func CompareFullBOM(computed [][]string, oracleCSVPath string, scale int) (Diff, error) {
	return Diff{}, fmt.Errorf("%w: CompareFullBOM(scale=%d)", ErrNotImplemented, scale)
}
