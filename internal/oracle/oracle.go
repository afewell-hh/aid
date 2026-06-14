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
	"strconv"
)

// ErrNotImplemented marks a comparison that needs calc (F2+) — pending, not a
// failure.
var ErrNotImplemented = errors.New("oracle: comparison not implemented (needs calc, F2+)")

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
func CompareExpectedCounts(computed, oracle ExpectedCounts) (Diff, error) {
	var details []string
	if computed.ServerClasses != oracle.ServerClasses {
		details = append(details, fmt.Sprintf("server_classes: computed %d != expected %d", computed.ServerClasses, oracle.ServerClasses))
	}
	if computed.SwitchClasses != oracle.SwitchClasses {
		details = append(details, fmt.Sprintf("switch_classes: computed %d != expected %d", computed.SwitchClasses, oracle.SwitchClasses))
	}
	if computed.Connections != oracle.Connections {
		details = append(details, fmt.Sprintf("connections: computed %d != expected %d", computed.Connections, oracle.Connections))
	}
	return Diff{Equal: len(details) == 0, Details: details}, nil
}

// CompareWiringHhfab generates wiring CRDs and runs the existing hhfab validate
// harness against them. F0: pending.
func CompareWiringHhfab(computedWiringDir, oracleWiringDir string) (Diff, error) {
	return Diff{}, fmt.Errorf("%w: CompareWiringHhfab", ErrNotImplemented)
}

// --- F2 derived-quantities row (the headline F2 oracle, D22) -----------------

// DerivedQuantities is the computed-quantity oracle: switches per class and
// server instances per class, both keyed by hedgehog class id. For F2 this is
// validated against the committed bom.csv (NOT netbox_inventory.json, deferred
// per D22). The full bom.csv reproduction is F3; this row only checks the
// per-class QUANTITIES.
type DerivedQuantities struct {
	SwitchPerClass map[string]int
	ServerPerClass map[string]int
}

// LoadBOMQuantities reads the committed bom.csv and projects the switch/server
// per-class quantities (the F2 oracle target). bom.csv columns:
//
//	0=section, 3=hedgehog_class, 5=quantity
//
// `section` ∈ {switch, server} selects the rows; `*_transceiver`/cable rows are
// ignored (they are F3's full-BOM concern). This loader is REAL — it proves the
// F2 oracle is wired to the committed reference, independent of any calc.
func LoadBOMQuantities(bomPath string) (DerivedQuantities, error) {
	rows, err := LoadCSV(bomPath)
	if err != nil {
		return DerivedQuantities{}, err
	}
	out := DerivedQuantities{SwitchPerClass: map[string]int{}, ServerPerClass: map[string]int{}}
	for i, r := range rows {
		if i == 0 || len(r) < 6 {
			continue // header or short/comment row
		}
		class := r[3]
		if class == "" {
			continue
		}
		qty, err := strconv.Atoi(r[5])
		if err != nil {
			continue
		}
		switch r[0] {
		case "switch":
			out.SwitchPerClass[class] = qty
		case "server":
			out.ServerPerClass[class] = qty
		}
	}
	return out, nil
}

// CompareDerivedQuantities compares AID's computed per-class quantities to the
// bom.csv-derived oracle (set-equality over both class→qty maps). Implemented
// here (the comparison is real); it is the COMPUTED side that is pending until
// the F2 calc lands.
func CompareDerivedQuantities(computed, oracle DerivedQuantities) (Diff, error) {
	var details []string
	cmp := func(kind string, got, want map[string]int) {
		for class, w := range want {
			if g, ok := got[class]; !ok {
				details = append(details, fmt.Sprintf("%s %s: missing (want %d)", kind, class, w))
			} else if g != w {
				details = append(details, fmt.Sprintf("%s %s: computed %d != want %d", kind, class, g, w))
			}
		}
		for class := range got {
			if _, ok := want[class]; !ok {
				details = append(details, fmt.Sprintf("%s %s: computed but not in oracle", kind, class))
			}
		}
	}
	cmp("switch", computed.SwitchPerClass, oracle.SwitchPerClass)
	cmp("server", computed.ServerPerClass, oracle.ServerPerClass)
	return Diff{Equal: len(details) == 0, Details: details}, nil
}

// CompareFullBOM compares AID's FULL purchasable BOM (Layer B) to
// real-server-bom.csv at the given server-quantity scale (1×, 2×, …),
// asserting linear scaling. F0: pending.
func CompareFullBOM(computed [][]string, oracleCSVPath string, scale int) (Diff, error) {
	return Diff{}, fmt.Errorf("%w: CompareFullBOM(scale=%d)", ErrNotImplemented, scale)
}
