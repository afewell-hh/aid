package catalog

import (
	"testing"

	"github.com/afewell-hh/aid/internal/objectmodel"
)

// TestItems_DeterministicEnumeration pins the Items() contract the Library browse
// surface depends on: all items, in a stable order (sorted by pinned ID string),
// with len == Len(). The backing map alone has no deterministic order.
func TestItems_DeterministicEnumeration(t *testing.T) {
	mk := func(n string) Item {
		return Item{ID: objectmodel.ID{Name: n, Version: "1"}, Kind: KindSwitch, Layer: LayerHardwareType}
	}
	c, err := New(mk("zebra"), mk("alpha"), mk("mike"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := c.Items()
	if len(got) != c.Len() {
		t.Fatalf("Items len = %d, want Len() = %d", len(got), c.Len())
	}
	want := []string{"alpha@1", "mike@1", "zebra@1"}
	for i, w := range want {
		if i >= len(got) {
			t.Fatalf("Items()[%d] missing; got %d items", i, len(got))
		}
		if got[i].ID.String() != w {
			t.Errorf("Items()[%d].ID = %q, want %q", i, got[i].ID.String(), w)
		}
	}
}
