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
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
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

// CompareBOMProjection compares AID's BOM projection (the HNP 19-column shape,
// produced by internal/bom.RenderProjection) to the committed bom.csv: exact
// row-and-cell equality incl. the `# suppressed_switch_cable_assembly_count`
// footer (F3, note §5). The comparator is REAL; the COMPUTED side is what is
// pending until the F3 reducer lands.
func CompareBOMProjection(computed [][]string, oracleBOMPath string) (Diff, error) {
	want, err := LoadCSV(oracleBOMPath)
	if err != nil {
		return Diff{}, err
	}
	return diffCSV("bom.csv", computed, want, 1, -1), nil
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

// CompareWiringHhfab is the F4 structural-equivalence comparator (note §3B,
// Issue #60): for every committed managed-fabric wiring file in oracleWiringDir
// (`wiring-{fabric}.yaml`), it matches the renderer's computed Doc for that
// fabric and asserts, semantically (not byte-identically):
//
//	(1) CRD-kind counts (Connection / Server / Switch / VLANNamespace / IPv4Namespace),
//	(2) the order-insensitive Connection endpoint set — unbundled (server,switch)
//	    port tuples + mesh {leaf1,leaf2} link pairs,
//	(3) per-switch identity keyed by metadata.name — the
//	    (profile, role, boot.mac) tuple AND the portBreakouts/portSpeeds maps.
//
// The comparator is REAL (it parses the committed oracle); the COMPUTED side is
// `wiring.Render`, a stub in RED — so the F4 oracle row FAILS for the right
// reason (renderer absent) until GREEN, rather than skipping. The hhfab-validate
// hard gate is applied by the caller (oracle_test) via the golden hhfabValidate
// harness. D22: wiring only.
//
// `computed` maps managed fabric_name → rendered wiring YAML. It is passed as raw
// bytes (not internal/wiring.Doc) so this comparator imports neither the renderer
// nor internal/topology — keeping internal/oracle free of the import cycle that
// internal/topology's tests would otherwise form (cf. the local ExpectedCounts).
func CompareWiringHhfab(computed map[string][]byte, oracleWiringDir string) (Diff, error) {
	entries, err := filepath.Glob(filepath.Join(oracleWiringDir, "wiring-*.yaml"))
	if err != nil {
		return Diff{}, err
	}
	if len(entries) == 0 {
		return Diff{}, fmt.Errorf("oracle: no committed wiring-*.yaml in %s", oracleWiringDir)
	}

	var details []string
	for _, path := range entries {
		fabric := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "wiring-"), ".yaml")
		wantBytes, err := os.ReadFile(path)
		if err != nil {
			return Diff{}, err
		}
		want, err := decodeWiringCRDs(wantBytes)
		if err != nil {
			return Diff{}, fmt.Errorf("oracle: parse committed %s: %w", filepath.Base(path), err)
		}
		yamlBytes, ok := computed[fabric]
		if !ok {
			details = append(details, fmt.Sprintf("%s: no computed wiring doc", fabric))
			continue
		}
		got, err := decodeWiringCRDs(yamlBytes)
		if err != nil {
			details = append(details, fmt.Sprintf("%s: parse computed wiring: %v", fabric, err))
			continue
		}
		details = append(details, diffWiring(fabric, got, want)...)
	}
	return Diff{Equal: len(details) == 0, Details: details}, nil
}

// --- §3B structural comparison helpers (REAL oracle infra) -------------------

type wiringCRD struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec map[string]any `yaml:"spec"`
}

// decodeWiringCRDs reads a multi-document wiring YAML stream into CRDs.
func decodeWiringCRDs(y []byte) ([]wiringCRD, error) {
	dec := yaml.NewDecoder(bytes.NewReader(y))
	var out []wiringCRD
	for {
		var c wiringCRD
		err := dec.Decode(&c)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if c.Kind == "" {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

var wiringKinds = []string{"Connection", "Server", "Switch", "VLANNamespace", "IPv4Namespace"}

// diffWiring reports §3B differences between computed (got) and committed (want).
func diffWiring(fabric string, got, want []wiringCRD) []string {
	var d []string
	pfx := func(s string) string { return fabric + ": " + s }

	// (1) CRD-kind counts.
	for _, k := range wiringKinds {
		if g, w := kindCount(got, k), kindCount(want, k); g != w {
			d = append(d, pfx(fmt.Sprintf("%s count = %d, want %d", k, g, w)))
		}
	}

	// (2) Connection endpoint set (order-insensitive).
	gset, wset := connectionSet(got), connectionSet(want)
	for k := range wset {
		if !gset[k] {
			d = append(d, pfx("missing connection "+k))
		}
	}
	for k := range gset {
		if !wset[k] {
			d = append(d, pfx("unexpected connection "+k))
		}
	}

	// (3) per-switch identity tuple + portBreakouts/portSpeeds maps.
	gsw, wsw := switchFacts(got), switchFacts(want)
	for name, wf := range wsw {
		gf, ok := gsw[name]
		if !ok {
			d = append(d, pfx("missing Switch "+name))
			continue
		}
		if gf.profile != wf.profile {
			d = append(d, pfx(fmt.Sprintf("Switch %s profile=%q want %q", name, gf.profile, wf.profile)))
		}
		if gf.role != wf.role {
			d = append(d, pfx(fmt.Sprintf("Switch %s role=%q want %q", name, gf.role, wf.role)))
		}
		if gf.mac != wf.mac {
			d = append(d, pfx(fmt.Sprintf("Switch %s boot.mac=%q want %q", name, gf.mac, wf.mac)))
		}
		d = append(d, diffStrMap(pfx(fmt.Sprintf("Switch %s portBreakouts", name)), gf.breakouts, wf.breakouts)...)
		d = append(d, diffStrMap(pfx(fmt.Sprintf("Switch %s portSpeeds", name)), gf.speeds, wf.speeds)...)
	}
	for name := range gsw {
		if _, ok := wsw[name]; !ok {
			d = append(d, pfx("unexpected Switch "+name))
		}
	}
	sort.Strings(d)
	return d
}

func kindCount(crds []wiringCRD, kind string) int {
	n := 0
	for _, c := range crds {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

// connectionSet is the order-insensitive set of Connection endpoints: unbundled
// links as "U|server|switch", mesh links as "M|portA|portB" (ports sorted).
func connectionSet(crds []wiringCRD) map[string]bool {
	set := map[string]bool{}
	for _, c := range crds {
		if c.Kind != "Connection" {
			continue
		}
		if sp, ok1 := digStr(c.Spec, "unbundled", "link", "server", "port"); ok1 {
			if wp, ok2 := digStr(c.Spec, "unbundled", "link", "switch", "port"); ok2 {
				set["U|"+sp+"|"+wp] = true
			}
		}
		if mesh, ok := c.Spec["mesh"].(map[string]any); ok {
			if links, ok := mesh["links"].([]any); ok {
				for _, l := range links {
					lm, ok := l.(map[string]any)
					if !ok {
						continue
					}
					p1, _ := digStr(lm, "leaf1", "port")
					p2, _ := digStr(lm, "leaf2", "port")
					a, b := p1, p2
					if b < a {
						a, b = b, a
					}
					set["M|"+a+"|"+b] = true
				}
			}
		}
	}
	return set
}

type switchFact struct {
	profile, role, mac string
	breakouts, speeds  map[string]string
}

func switchFacts(crds []wiringCRD) map[string]switchFact {
	out := map[string]switchFact{}
	for _, c := range crds {
		if c.Kind != "Switch" {
			continue
		}
		profile, _ := digStr(c.Spec, "profile")
		role, _ := digStr(c.Spec, "role")
		mac, _ := digStr(c.Spec, "boot", "mac")
		out[c.Metadata.Name] = switchFact{
			profile:   profile,
			role:      role,
			mac:       mac,
			breakouts: stringMap(c.Spec["portBreakouts"]),
			speeds:    stringMap(c.Spec["portSpeeds"]),
		}
	}
	return out
}

// digStr walks nested string-keyed maps and returns the terminal string.
func digStr(m map[string]any, keys ...string) (string, bool) {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = mm[k]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}

func stringMap(v any) map[string]string {
	out := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		for k, vv := range m {
			out[k] = fmt.Sprint(vv)
		}
	}
	return out
}

func diffStrMap(label string, got, want map[string]string) []string {
	var d []string
	for k, w := range want {
		if g, ok := got[k]; !ok {
			d = append(d, fmt.Sprintf("%s: missing %s (want %q)", label, k, w))
		} else if g != w {
			d = append(d, fmt.Sprintf("%s[%s] = %q, want %q", label, k, g, w))
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			d = append(d, fmt.Sprintf("%s: unexpected %s", label, k))
		}
	}
	return d
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

// fullBOMQtyCol is the QTY column of real-server-bom.csv
// (Type,SMC PN,Desc,QTY,Total Capacity(GB),Power(W),Total Power(W)).
const fullBOMQtyCol = 3

// CompareFullBOM compares AID's FULL purchasable BOM (Layer B, produced by
// internal/bom.RenderFullBOM) to real-server-bom.csv at the given server-quantity
// scale (1×, 2×, …), asserting linear scaling: the oracle is 1×, so at scale N the
// QTY column of every numeric data row is expected to be N× the committed value
// (every other cell unchanged). The comparator is REAL; the COMPUTED side is
// pending until the F3 reducer lands.
func CompareFullBOM(computed [][]string, oracleCSVPath string, scale int) (Diff, error) {
	want, err := LoadCSV(oracleCSVPath)
	if err != nil {
		return Diff{}, err
	}
	return diffCSV(fmt.Sprintf("real-server-bom.csv (%d×)", scale), computed, want, scale, fullBOMQtyCol), nil
}

// diffCSV reports an exact row/cell diff of got vs want. When scale > 1 and
// qtyCol >= 0, the want side's qtyCol is multiplied by scale before comparison
// (linear-scaling oracle); a non-numeric qtyCol cell is compared verbatim.
func diffCSV(label string, got, want [][]string, scale, qtyCol int) Diff {
	var d []string
	if len(got) != len(want) {
		d = append(d, fmt.Sprintf("%s: %d rows, want %d", label, len(got), len(want)))
		return Diff{Equal: false, Details: d}
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			d = append(d, fmt.Sprintf("%s row %d: %d cols, want %d", label, i, len(got[i]), len(want[i])))
			continue
		}
		for j := range want[i] {
			exp := want[i][j]
			if scale > 1 && j == qtyCol {
				if n, err := strconv.Atoi(exp); err == nil {
					exp = strconv.Itoa(n * scale)
				}
			}
			if got[i][j] != exp {
				d = append(d, fmt.Sprintf("%s row %d col %d: got %q want %q", label, i, j, got[i][j], exp))
			}
		}
	}
	return Diff{Equal: len(d) == 0, Details: d}
}
