package library

import (
	"errors"
	"testing"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
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
