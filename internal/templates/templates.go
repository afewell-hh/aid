// Package templates carries the starter plan templates served to the GUI's
// "New from template" flow (Issue #65 P0.2). Each template is a vendored XOC
// oracle composition — a real DIET/training.yaml plus its AID optic/identity
// overlay — embedded into the aid binary (D4: single static binary; air-gapped,
// no filesystem dependency on tests/oracle at runtime).
//
// The files under data/ are COPIES of the committed oracle starters
// (tests/oracle/<comp>/training.yaml and the matching optic overlay). They are
// kept in sync by a guard test (templates_test) that diffs each embedded copy
// against its source-of-truth, so a starter can never silently drift from the
// oracle it claims to be. Embedding (rather than reading tests/oracle at run
// time) is required because Go embed cannot reach paths outside the module
// package and the binary must run without the repo checkout present.
package templates

import (
	"embed"
	"io/fs"
	"sort"
)

//go:embed all:data
var dataFS embed.FS

// Template is a starter plan: a stable id, a human name, the topology kind, and
// the embedded training/overlay YAML. The overlay may be empty (no overlay), but
// every shipped starter currently carries one so a template-created plan yields
// a FULL BOM (populated optic identity), not blank optic columns.
type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Topology    string `json:"topology"`
	Description string `json:"description"`
	// dir is the embedded subdirectory under data/.
	dir string
}

// catalog is the ordered template table. Each entry maps to an embedded
// data/<dir>/ holding training.yaml (+ optional optic-overlay.yaml). The ids are
// the GUI-facing template ids (distinct from the derived plan id a created plan
// gets from meta.case_id).
var catalog = []Template{
	{
		ID:          "xoc-64-mesh",
		Name:        "XOC-64 · 1× OPG-64 Mesh (Converged)",
		Topology:    "mesh",
		Description: "Single OPG-64 mesh-converged building block; shared SoC/storage/scale-out DS5000 mesh pair. Smallest reference fabric.",
		dir:         "xoc-64-mesh-conv-ro",
	},
	{
		ID:          "xoc-128-mesh",
		Name:        "XOC-128 · 2× OPG-64 Mesh (Converged)",
		Topology:    "mesh",
		Description: "Two OPG-64 mesh blocks scaled out (scale-out-a/b, soc-storage-a/b). Mid-size mesh reference.",
		dir:         "xoc-128-2xopg64-mesh-conv-ro",
	},
	{
		ID:          "xoc-256-clos",
		Name:        "XOC-256 · 2× OPG-128 Clos",
		Topology:    "clos",
		Description: "Two OPG-128 blocks in a frontend/backend Clos (fe-leaf/fe-spine/be-rail-leaf/be-spine); switch counts are DERIVED.",
		dir:         "xoc-256-2xopg128-clos-ro",
	},
}

// List returns the template summaries (id/name/topology/description), no YAML.
func List() []Template {
	out := make([]Template, len(catalog))
	copy(out, catalog)
	return out
}

// IDs returns the template ids in catalog order (handy for tests/guards).
func IDs() []string {
	ids := make([]string, len(catalog))
	for i, t := range catalog {
		ids[i] = t.ID
	}
	return ids
}

// Get returns the template summary for id, or (Template{}, false) if unknown.
func Get(id string) (Template, bool) {
	for _, t := range catalog {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Training returns the embedded training.yaml bytes for the template id.
func Training(id string) ([]byte, bool) {
	t, ok := Get(id)
	if !ok {
		return nil, false
	}
	b, err := dataFS.ReadFile("data/" + t.dir + "/training.yaml")
	if err != nil {
		return nil, false
	}
	return b, true
}

// Overlay returns the embedded optic-overlay.yaml bytes for the template id, and
// whether one exists. A template with no overlay returns (nil, false) — the
// caller proceeds without attaching an overlay.
func Overlay(id string) ([]byte, bool) {
	t, ok := Get(id)
	if !ok {
		return nil, false
	}
	b, err := dataFS.ReadFile("data/" + t.dir + "/optic-overlay.yaml")
	if err != nil {
		return nil, false
	}
	return b, true
}

// dirs lists the embedded composition directories (for the in-sync guard test).
func dirs() []string {
	entries, err := fs.ReadDir(dataFS, "data")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}
