package templates

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoRoot walks up from this package to the module root (the dir holding go.mod).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root (go.mod)")
		}
		dir = parent
	}
}

// TestList_Catalog asserts every shipped template has the required summary fields
// and a stable, unique id.
func TestList_Catalog(t *testing.T) {
	seen := map[string]bool{}
	for _, tpl := range List() {
		if tpl.ID == "" || tpl.Name == "" || tpl.Topology == "" {
			t.Errorf("template missing id/name/topology: %+v", tpl)
		}
		if seen[tpl.ID] {
			t.Errorf("duplicate template id %q", tpl.ID)
		}
		seen[tpl.ID] = true
	}
	if len(List()) == 0 {
		t.Fatal("template catalog is empty")
	}
}

// TestTraining_ExistsAndParses asserts each template's embedded training.yaml is
// present, non-empty, valid YAML, and carries a derivable identity
// (meta.case_id / meta.name) — i.e. a created plan really resolves to an id.
func TestTraining_ExistsAndParses(t *testing.T) {
	for _, tpl := range List() {
		t.Run(tpl.ID, func(t *testing.T) {
			b, ok := Training(tpl.ID)
			if !ok || len(b) == 0 {
				t.Fatalf("training.yaml missing/empty for %q", tpl.ID)
			}
			var doc struct {
				Meta struct {
					CaseID string `yaml:"case_id"`
					Name   string `yaml:"name"`
				} `yaml:"meta"`
			}
			if err := yaml.Unmarshal(b, &doc); err != nil {
				t.Fatalf("training.yaml for %q is not valid YAML: %v", tpl.ID, err)
			}
			if doc.Meta.CaseID == "" {
				t.Errorf("training.yaml for %q has no meta.case_id (plan id would not derive)", tpl.ID)
			}
		})
	}
}

// TestOverlay_PresentForAll asserts every shipped starter carries an optic
// overlay (so a template-created plan yields a FULL BOM, not blank optic
// columns). If a future template ships without one, relax this — but the P0.2
// promise is "New from template -> full BOM".
func TestOverlay_PresentForAll(t *testing.T) {
	for _, tpl := range List() {
		if b, ok := Overlay(tpl.ID); !ok || len(b) == 0 {
			t.Errorf("template %q has no optic overlay; created plan would have blank optic identity", tpl.ID)
		}
	}
}

// TestEmbeddedCopies_MatchOracleSource is the in-sync guard: each embedded
// template file must be BYTE-IDENTICAL to its committed source of truth, so a
// starter can never silently drift from the oracle it claims to be.
func TestEmbeddedCopies_MatchOracleSource(t *testing.T) {
	root := repoRoot(t)
	// embeddedDir -> {training source, overlay source} relative to repo root.
	sources := map[string]struct{ training, overlay string }{
		"xoc-64-mesh-conv-ro": {
			training: "tests/oracle/xoc-64-mesh-conv-ro/training.yaml",
			overlay:  "tests/fixtures/f3/optic-overlay.yaml",
		},
		"xoc-128-2xopg64-mesh-conv-ro": {
			training: "tests/oracle/xoc-128-2xopg64-mesh-conv-ro/training.yaml",
			overlay:  "tests/oracle/xoc-128-2xopg64-mesh-conv-ro/optic-overlay.yaml",
		},
		"xoc-256-2xopg128-clos-ro": {
			training: "tests/oracle/xoc-256-2xopg128-clos-ro/training.yaml",
			overlay:  "tests/oracle/xoc-256-2xopg128-clos-ro/optic-overlay.yaml",
		},
	}
	for _, d := range dirs() {
		src, ok := sources[d]
		if !ok {
			t.Errorf("embedded template dir %q has no declared oracle source", d)
			continue
		}
		mustMatch(t, root, "data/"+d+"/training.yaml", src.training)
		mustMatch(t, root, "data/"+d+"/optic-overlay.yaml", src.overlay)
	}
}

func mustMatch(t *testing.T, root, embedded, srcRel string) {
	t.Helper()
	got, err := dataFS.ReadFile(embedded)
	if err != nil {
		t.Errorf("read embedded %s: %v", embedded, err)
		return
	}
	want, err := os.ReadFile(filepath.Join(root, srcRel))
	if err != nil {
		t.Errorf("read oracle source %s: %v", srcRel, err)
		return
	}
	if !bytes.Equal(got, want) {
		t.Errorf("embedded %s drifted from %s (re-copy the oracle starter)", embedded, srcRel)
	}
}
