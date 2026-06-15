package oracle

import "path/filepath"

// Composition is one vendored XOC oracle snapshot (the mesh-conv-ro family for
// F5, Issue #62 / D24). It makes the Layer-A oracle suite PARAMETRIC over
// composition: every comparison target is derived from the vendored artifacts of
// the composition's Dir(); only the headline totals below are pinned in code, as
// tripwires that catch silent corruption of a snapshot.
//
// The pinned tripwire fields are NOT the comparison oracle — they are a small set
// of provenance-pinned numbers, each verified against the vendored artifact in
// TestLayerA_Tripwires. If a tripwire fails, the snapshot (or this number) is
// wrong; investigate the snapshot, do not edit the number to pass.
type Composition struct {
	Name    string // dir name under tests/oracle
	Overlay string // AID optic/identity overlay, path relative to repo root (§3.3)

	// --- tripwires (pinned; verified against the vendored artifact) ---
	ServerClasses int      // == expected.counts.server_classes == len(spec.server_classes)
	SwitchClasses int      // == expected.counts.switch_classes == len(spec.switch_classes)
	Connections   int      // == expected.counts.connections == len(spec.server_connections)
	TotalServers  int      // == Σ spec.server_classes[].quantity
	BOMRows       int      // == len(LoadCSV(bom.csv)) (header + data + footer)
	Managed       []string // sorted managed fabric_names (fabric_class == managed)
}

// Dir is the composition's vendored oracle directory.
func (c Composition) Dir() string { return filepath.Join(Root(), c.Name) }

// OverlayPath resolves the per-composition overlay against the repo root.
func (c Composition) OverlayPath() string { return filepath.Join(repoRootDir(), c.Overlay) }

// repoRootDir is the parent of tests/oracle (mirrors oracle_test.repoRoot for
// non-test callers).
func repoRootDir() string { return filepath.Dir(filepath.Dir(Root())) }

// Compositions is the parametric oracle table. xoc-64 is the established
// mesh-conv-ro baseline; xoc-128 is the F5 2×OPG-64 mesh scale-out (override-only;
// no derivation, D24). Adding another mesh composition later is one row + one
// vendored snapshot — no Go changes.
func Compositions() []Composition {
	return []Composition{
		{
			Name:          "xoc-64-mesh-conv-ro",
			Overlay:       filepath.Join("tests", "fixtures", "f3", "optic-overlay.yaml"),
			ServerClasses: 5, SwitchClasses: 3, Connections: 21,
			TotalServers: 17, BOMRows: 23,
			Managed: []string{"inb-mgmt", "soc-storage-scale-out"},
		},
		{
			Name:          "xoc-128-2xopg64-mesh-conv-ro",
			Overlay:       filepath.Join("tests", "oracle", "xoc-128-2xopg64-mesh-conv-ro", "optic-overlay.yaml"),
			ServerClasses: 8, SwitchClasses: 6, Connections: 38,
			TotalServers: 34, BOMRows: 23,
			Managed: []string{"inb-mgmt", "scale-out-a", "scale-out-b", "soc-storage-a", "soc-storage-b"},
		},
	}
}

// XOC64 returns the baseline mesh-conv-ro composition (the per-package engine
// tests anchor on it via LayerADir).
func XOC64() Composition { return Compositions()[0] }
