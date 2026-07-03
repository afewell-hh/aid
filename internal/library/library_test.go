package library

import (
	"errors"
	"testing"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
	"github.com/afewell-hh/aid/internal/templates"
	"github.com/afewell-hh/aid/internal/topology"
)

// TestBuiltinCatalog_CoversShippedReferences pins the lead's #79 blocking point:
// the built-in Library must cover the catalogs of ALL shipped reference
// topologies — both a mesh class from xoc-64 AND the Clos classes from xoc-256 —
// not just one mesh snapshot.
func TestBuiltinCatalog_CoversShippedReferences(t *testing.T) {
	cat, err := BuiltinCatalog()
	if err != nil {
		t.Fatalf("BuiltinCatalog: %v", err)
	}
	if cat == nil || cat.Len() == 0 {
		t.Fatalf("BuiltinCatalog returned an empty catalog")
	}
	for _, name := range []string{
		"soc_storage_scale_out_leaf",                       // mesh (xoc-64)
		"fe-leaf", "fe-spine", "be-rail-leaf", "be-spine", // clos (xoc-256)
	} {
		it, ok := cat.ByName(name)
		if !ok {
			t.Errorf("Library missing class %q (coverage gap across shipped references)", name)
			continue
		}
		if it.Layer != catalog.LayerClass {
			t.Errorf("class %q: layer=%q, want %q", name, it.Layer, catalog.LayerClass)
		}
	}
}

// TestBuiltinCatalog_CoversEveryReferencedItem is the FULL coverage guard the #79
// spec requires (not just a handful of names): EVERY catalog item the shipped
// reference templates reference — every server/switch class AND every module_type
// (NIC / DPU / transceiver) AND device type — must resolve in the built-in
// Library, by pinned ID. The independent oracle re-ingests each shipped template
// with the real engine ingest (topology.IngestBundled) and requires the union to
// contain each of its items. This is what prevents a regression to a single-
// composition snapshot (the #79 revision-1 blocking point) at the granularity the
// spec demanded, and it explicitly proves both a mesh and a Clos reference were
// exercised.
func TestBuiltinCatalog_CoversEveryReferencedItem(t *testing.T) {
	cat, err := BuiltinCatalog()
	if err != nil {
		t.Fatalf("BuiltinCatalog: %v", err)
	}
	ids := templates.IDs()
	if len(ids) < 2 {
		t.Fatalf("expected >=2 shipped reference templates, got %d", len(ids))
	}
	meshSwitchClasses, closSwitchClasses, moduleTypes := 0, 0, 0
	for _, tid := range ids {
		training, ok := templates.Training(tid)
		if !ok {
			t.Fatalf("template %s: training missing", tid)
		}
		_, tcat, err := topology.IngestBundled(training)
		if err != nil {
			t.Fatalf("ingest template %s: %v", tid, err)
		}
		items := tcat.Items()
		if len(items) == 0 {
			t.Fatalf("template %s ingested to an empty catalog", tid)
		}
		for _, it := range items {
			if _, ok := cat.Get(it.ID); !ok {
				t.Errorf("Library missing item %s (kind=%s, layer=%s) referenced by template %s",
					it.ID, it.Kind, it.Layer, tid)
			}
			switch {
			case it.Kind == catalog.KindNIC, it.Kind == catalog.KindDPU, it.Kind == catalog.KindTransceiver:
				moduleTypes++
			case it.Layer == catalog.LayerClass && it.Kind == catalog.KindSwitch:
				switch it.ID.Name {
				case "fe-leaf", "fe-spine", "be-rail-leaf", "be-spine":
					closSwitchClasses++
				case "soc_storage_scale_out_leaf", "inb_mgmt_leaf", "oob_leaf":
					meshSwitchClasses++
				}
			}
		}
	}
	// The oracle must actually have exercised module_types and BOTH topologies —
	// otherwise "covers everything" would be vacuously true.
	if moduleTypes == 0 {
		t.Errorf("coverage oracle saw no module_types (NIC/DPU/transceiver) — not exercising module coverage")
	}
	if meshSwitchClasses == 0 {
		t.Errorf("coverage oracle saw no mesh switch classes — mesh reference (xoc-64) not exercised")
	}
	if closSwitchClasses == 0 {
		t.Errorf("coverage oracle saw no Clos switch classes — Clos reference (xoc-256) not exercised")
	}
}

// TestBuiltinCatalog_Deterministic: the union is stable across calls (the API
// must return a deterministic order, #79 §constraints).
func TestBuiltinCatalog_Deterministic(t *testing.T) {
	a, err := BuiltinCatalog()
	if err != nil {
		t.Fatalf("BuiltinCatalog #1: %v", err)
	}
	b, err := BuiltinCatalog()
	if err != nil {
		t.Fatalf("BuiltinCatalog #2: %v", err)
	}
	ai, bi := a.Items(), b.Items()
	if len(ai) != len(bi) {
		t.Fatalf("non-deterministic length: %d vs %d", len(ai), len(bi))
	}
	for i := range ai {
		if ai[i].ID != bi[i].ID {
			t.Errorf("non-deterministic order at %d: %s vs %s", i, ai[i].ID, bi[i].ID)
		}
	}
}

// TestUnion_DedupesIdenticalIDs: the same pinned id appearing in two source
// catalogs with identical content collapses to a single item.
func TestUnion_DedupesIdenticalIDs(t *testing.T) {
	id := objectmodel.ID{Name: "ds5000", Version: "1"}
	a, _ := catalog.New(catalog.Item{ID: id, Kind: catalog.KindSwitch, Layer: catalog.LayerHardwareType})
	b, _ := catalog.New(catalog.Item{ID: id, Kind: catalog.KindSwitch, Layer: catalog.LayerHardwareType})
	u, err := Union(a, b)
	if err != nil {
		t.Fatalf("Union of identical-id catalogs: %v", err)
	}
	if u.Len() != 1 {
		t.Errorf("Union deduped len = %d, want 1", u.Len())
	}
}

// TestUnion_ConflictOnSameIDDifferentDefinition: the same pinned id with
// differing content is a hard error (D21 reproducibility guard), not last-wins.
func TestUnion_ConflictOnSameIDDifferentDefinition(t *testing.T) {
	id := objectmodel.ID{Name: "ds5000", Version: "1"}
	a, _ := catalog.New(catalog.Item{ID: id, Kind: catalog.KindSwitch, Layer: catalog.LayerHardwareType, Model: "A"})
	b, _ := catalog.New(catalog.Item{ID: id, Kind: catalog.KindSwitch, Layer: catalog.LayerHardwareType, Model: "B"})
	if _, err := Union(a, b); !errors.Is(err, ErrConflict) {
		t.Errorf("Union of conflicting defs: err = %v, want ErrConflict", err)
	}
}
