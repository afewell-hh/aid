package oracle

// RED (#82): pins the built-in reference-data reconciliation contract discovered
// during #80 GREEN. The strict internal/library.Union guard (approved in #79/#80)
// fires because three pinned ids resolve to different content across the shipped
// built-in reference templates:
//   - hh_controller@1 / hh_gateway@1 — xoc-64 binds the management-connection
//     transceivers (cage_bindings) but xoc-128 omits the transceiver_module_type
//     lines, so the extracted class differs (D19: a different transceiver
//     selection ⇒ a distinct class).
//   - sw_ds5000_leaf_dt@1 — cosmetic model-string drift ("celestica-ds5000" vs
//     "Celestica DS5000").
//
// These tests fail now (data inconsistent) and pass after the GREEN data
// correction. They also make the xoc-128 BOM rebaseline EXPLICIT so GREEN cannot
// silently drift the projection oracle.

import (
	"reflect"
	"testing"

	"github.com/afewell-hh/aid/internal/bom"
	"github.com/afewell-hh/aid/internal/calc"
	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
	"github.com/afewell-hh/aid/internal/templates"
	"github.com/afewell-hh/aid/internal/topology"
)

// builtinCatalog ingests one shipped reference template exactly as #80's Library
// does — real engine ingest + optic-overlay merge — and returns the catalog.
func ingestTemplate(t *testing.T, id string) *catalog.Catalog {
	t.Helper()
	training, ok := templates.Training(id)
	if !ok {
		t.Fatalf("template %q has no training.yaml", id)
	}
	_, cat, err := topology.IngestBundled(training)
	if err != nil {
		t.Fatalf("ingest template %q: %v", id, err)
	}
	if overlay, ok := templates.Overlay(id); ok && len(overlay) > 0 {
		ov, err := catalog.LoadBytes(overlay)
		if err != nil {
			t.Fatalf("load overlay for %q: %v", id, err)
		}
		cat.Merge(ov)
	}
	return cat
}

// TestBuiltinReferenceIdentity_Consistent is the core reconciliation contract:
// each of the three known-conflicting pinned ids must resolve to IDENTICAL
// content in every shipped reference template that defines it — otherwise the
// strict union guard cannot form the built-in Library. RED: fails for all three.
// (The general "no other conflicts" check is enforced by #80's strict
// library.Union itself once it is re-enabled after this reconciliation.)
func TestBuiltinReferenceIdentity_Consistent(t *testing.T) {
	conflictIDs := []objectmodel.ID{
		{Name: "hh_controller", Version: "1"},
		{Name: "hh_gateway", Version: "1"},
		{Name: "sw_ds5000_leaf_dt", Version: "1"},
	}
	cats := map[string]*catalog.Catalog{}
	for _, id := range templates.IDs() {
		cats[id] = ingestTemplate(t, id)
	}
	for _, cid := range conflictIDs {
		var ref *catalog.Item
		var refTid string
		defined := 0
		for _, tid := range templates.IDs() {
			it, ok := cats[tid].Get(cid)
			if !ok {
				continue
			}
			defined++
			if ref == nil {
				dup := it
				ref, refTid = &dup, tid
				continue
			}
			if !reflect.DeepEqual(*ref, it) {
				t.Errorf("pinned id %s differs across shipped references (%s vs %s): the strict union guard cannot dedup it",
					cid, refTid, tid)
				break
			}
		}
		if defined < 2 {
			t.Logf("note: %s defined in %d shipped reference(s)", cid, defined)
		}
	}
}

// TestXoc128_ManagementConnectionsBindTransceivers pins the exact missing lines
// (not a vague shape): xoc-128's hh_controller and hh_gateway management NIC ports
// must bind the same transceivers xoc-64 binds — inb_mgmt → sfp28_25gbase_sr,
// bmc → rj45_1000base_t. RED: xoc-128 currently omits transceiver_module_type on
// those connections, so the extracted class carries no cage_bindings.
func TestXoc128_ManagementConnectionsBindTransceivers(t *testing.T) {
	cat := ingestTemplate(t, "xoc-128-mesh")
	want := map[string]map[string]string{
		"hh_controller": {"inb_mgmt": "sfp28_25gbase_sr", "bmc": "rj45_1000base_t"},
		"hh_gateway":    {"inb_mgmt": "sfp28_25gbase_sr", "bmc": "rj45_1000base_t"},
	}
	for class, bindings := range want {
		it, ok := cat.ByName(class)
		if !ok {
			t.Errorf("xoc-128 missing class %q", class)
			continue
		}
		got := map[string]string{}
		for _, cb := range it.CageBindings {
			got[cb.NICSlotID] = cb.SelectedTransceiver.Name
		}
		for slot, xcvr := range bindings {
			if got[slot] != xcvr {
				t.Errorf("xoc-128 %s: NIC slot %q binds %q, want %q (missing transceiver_module_type in the management connection)",
					class, slot, got[slot], xcvr)
			}
		}
	}
}

// TestSwDs5000LeafDt_ModelCanonical pins the cosmetic normalization: every shipped
// reference that defines sw_ds5000_leaf_dt must use the canonical AID-facing model
// string "celestica-ds5000" (the slug form used by xoc-64's oracle catalog, the
// optic overlays, and hhfab wiring). RED: xoc-128 uses "Celestica DS5000".
func TestSwDs5000LeafDt_ModelCanonical(t *testing.T) {
	const canonical = "celestica-ds5000"
	id := objectmodel.ID{Name: "sw_ds5000_leaf_dt", Version: "1"}
	for _, tid := range templates.IDs() {
		it, ok := ingestTemplate(t, tid).Get(id)
		if !ok {
			continue // not every composition defines this hardware type
		}
		if it.Model != canonical {
			t.Errorf("%s: sw_ds5000_leaf_dt model = %q, want canonical %q", tid, it.Model, canonical)
		}
	}
}

// TestXoc128_ProjectionNamesManagementTransceivers makes the BOM rebaseline
// EXPLICIT (oracle-awareness): once xoc-128 binds the management transceivers AND
// its overlay carries their optic identity (copied byte-for-byte from xoc-64),
// the projection renders them as TWO NAMED rows — SFP28-25GBASE-SR qty 6 and
// RJ45-1000BASE-T qty 6 (6 mgmt servers each) — exactly as xoc-64 does, NOT an
// anonymous aggregate. The projection goes from 23 (main) to 25 rows. GREEN adds
// the data + overlay identities, rebaselines tests/oracle/xoc-128 bom.csv and the
// composition BOMRows tripwire (23→25).
func TestXoc128_ProjectionNamesManagementTransceivers(t *testing.T) {
	var comp Composition
	found := false
	for _, c := range Compositions() {
		if c.Name == "xoc-128-2xopg64-mesh-conv-ro" {
			comp, found = c, true
		}
	}
	if !found {
		t.Fatal("xoc-128 composition not found")
	}
	plan, cat := ingest(t, comp)
	calcOut, err := calc.Compute(plan, cat)
	if err != nil {
		t.Fatalf("calc.Compute: %v", err)
	}
	mergeOverlay(t, comp, cat)
	model, err := bom.Resolve(plan, cat, calcOut)
	if err != nil {
		t.Fatalf("bom.Resolve: %v", err)
	}
	rows, err := bom.RenderProjection(model)
	if err != nil {
		t.Fatalf("RenderProjection: %v", err)
	}
	if len(rows) != 25 {
		t.Errorf("xoc-128 projection rows = %d, want 25 (two NAMED management transceiver rows)", len(rows))
	}
	// The management optics must render as NAMED rows (identity, not blank), each
	// qty 6 — matching how xoc-64 renders the same pinned ids.
	want := map[string]bool{"SFP28-25GBASE-SR": false, "RJ45-1000BASE-T": false}
	blankAggregate := false
	for _, r := range rows {
		if len(r) < 6 || r[0] != "server_transceiver" {
			continue
		}
		if r[1] == "" && r[5] == "12" {
			blankAggregate = true
		}
		if _, ok := want[r[1]]; ok {
			if r[5] != "6" {
				t.Errorf("management transceiver %q qty = %q, want 6", r[1], r[5])
			}
			want[r[1]] = true
		}
	}
	if blankAggregate {
		t.Errorf("xoc-128 still renders the anonymous server_transceiver aggregate (qty 12) — overlay identity missing")
	}
	for model, seen := range want {
		if !seen {
			t.Errorf("xoc-128 projection missing named management transceiver row %q (qty 6)", model)
		}
	}
}
