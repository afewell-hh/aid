package planedit_test

// Tests for the structured-editing core (D26, #67): the projection shape, the
// yaml.Node field-patch round-trip fidelity (untouched regions survive, edits
// land), and the re-validate-before-return safety invariant.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/planedit"
	"github.com/afewell-hh/aid/internal/topology"
)

func meshPlan(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "tests", "oracle", "xoc-64-mesh-conv-ro", "training.yaml"))
	if err != nil {
		t.Fatalf("read xoc-64 training.yaml: %v", err)
	}
	return b
}

func serverClass(p *planedit.Projection, id string) *planedit.ServerClass {
	for i := range p.ServerClasses {
		if p.ServerClasses[i].ID == id {
			return &p.ServerClasses[i]
		}
	}
	return nil
}

func switchClass(p *planedit.Projection, id string) *planedit.SwitchClass {
	for i := range p.SwitchClasses {
		if p.SwitchClasses[i].ID == id {
			return &p.SwitchClasses[i]
		}
	}
	return nil
}

func TestProject_EditableFieldsAndCatalog(t *testing.T) {
	p, err := planedit.Project(meshPlan(t))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	cx := serverClass(p, "compute_xpu")
	if cx == nil {
		t.Fatal("compute_xpu server class missing from projection")
	}
	if cx.Quantity != 8 || cx.GpusPerServer != 8 {
		t.Errorf("compute_xpu: got quantity=%d gpus=%d want 8/8", cx.Quantity, cx.GpusPerServer)
	}
	// NICs are joined from the top-level server_nics list.
	if len(cx.Nics) == 0 {
		t.Fatal("compute_xpu has no joined NICs")
	}
	foundSO := false
	for _, n := range cx.Nics {
		if n.NicID == "scale_out" && n.ModuleType == "nic_xpu_scale_out_8x400" {
			foundSO = true
		}
	}
	if !foundSO {
		t.Errorf("compute_xpu scale_out NIC not projected; got %+v", cx.Nics)
	}
	// Switch topology mode (the mesh|clos field) is surfaced.
	sw := switchClass(p, "soc_storage_scale_out_leaf")
	if sw == nil || sw.TopologyMode != "mesh" {
		t.Errorf("soc_storage_scale_out_leaf topology_mode: got %+v want mesh", sw)
	}
	if sw.OverrideQuantity == nil || *sw.OverrideQuantity != 2 {
		t.Errorf("soc_storage_scale_out_leaf override_quantity: got %v want 2", sw.OverrideQuantity)
	}
	// Dropdown id lists are data-derived from reference_data.
	if !contains(p.Catalog.ModuleTypes, "nic_dual_200g") {
		t.Errorf("catalog.module_types missing nic_dual_200g: %v", p.Catalog.ModuleTypes)
	}
	if !contains(p.Catalog.DeviceTypeExtensions, "sw_ds5000_soc_storage_scale_out_ext") {
		t.Errorf("catalog.device_type_extensions missing the DS5000 ext: %v", p.Catalog.DeviceTypeExtensions)
	}
	if !contains(p.Catalog.DeviceTypes, "srv_xpu_generic_dt") {
		t.Errorf("catalog.device_types missing srv_xpu_generic_dt: %v", p.Catalog.DeviceTypes)
	}
}

func TestApply_SetServerQuantity_RoundTrips(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "set_server_field", ServerClass: "compute_xpu", Field: "quantity", Value: "16"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	p, _ := planedit.Project(out)
	if cx := serverClass(p, "compute_xpu"); cx == nil || cx.Quantity != 16 {
		t.Errorf("quantity not applied: %+v", cx)
	}
	// Untouched regions survive faithfully (content, not just presence).
	assertUntouchedFaithful(t, src, out)
}

func TestApply_FlipMeshToClos(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "set_switch_field", SwitchClass: "soc_storage_scale_out_leaf", Field: "topology_mode", Value: "clos"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	p, _ := planedit.Project(out)
	if sw := switchClass(p, "soc_storage_scale_out_leaf"); sw == nil || sw.TopologyMode != "clos" {
		t.Errorf("topology_mode not flipped: %+v", sw)
	}
	assertUntouchedFaithful(t, src, out)
}

func TestApply_SetNicModuleType(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "set_nic_module_type", ServerClass: "storage_srv", NicID: "soc_storage", Value: "nic_dual_25g"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	p, _ := planedit.Project(out)
	cx := serverClass(p, "storage_srv")
	ok := false
	for _, n := range cx.Nics {
		if n.NicID == "soc_storage" && n.ModuleType == "nic_dual_25g" {
			ok = true
		}
	}
	if !ok {
		t.Errorf("nic module_type not applied: %+v", cx.Nics)
	}
}

func TestApply_AddServerClass(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "add_server_class", ServerClass: "extra_compute", Quantity: "4", GpusPerServer: "8", ServerDeviceType: "srv_xpu_generic_dt"},
	})
	if err != nil {
		t.Fatalf("Apply(add): %v", err)
	}
	p, _ := planedit.Project(out)
	nc := serverClass(p, "extra_compute")
	if nc == nil || nc.Quantity != 4 || nc.GpusPerServer != 8 || nc.ServerDeviceType != "srv_xpu_generic_dt" {
		t.Errorf("added class not projected correctly: %+v", nc)
	}
}

func TestApply_SetServerDeviceType(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "set_server_field", ServerClass: "compute_xpu", Field: "server_device_type", Value: "srv_storage_generic_dt"},
	})
	if err != nil {
		t.Fatalf("Apply(set device type): %v", err)
	}
	p, _ := planedit.Project(out)
	if cx := serverClass(p, "compute_xpu"); cx == nil || cx.ServerDeviceType != "srv_storage_generic_dt" {
		t.Errorf("server_device_type not applied: %+v", cx)
	}
}

// TestApply_DeviceTypeRequired: a blank/unknown server_device_type must be
// rejected — on both add and set — so a semantically incomplete class is never
// stored (devb #67 finding 1/2).
func TestApply_DeviceTypeRequired(t *testing.T) {
	src := meshPlan(t)
	cases := []struct {
		name string
		op   planedit.Op
	}{
		{"add blank", planedit.Op{Op: "add_server_class", ServerClass: "x1", Quantity: "1", ServerDeviceType: ""}},
		{"add unknown", planedit.Op{Op: "add_server_class", ServerClass: "x2", Quantity: "1", ServerDeviceType: "no_such_dt"}},
		{"set blank", planedit.Op{Op: "set_server_field", ServerClass: "compute_xpu", Field: "server_device_type", Value: ""}},
		{"set unknown", planedit.Op{Op: "set_server_field", ServerClass: "compute_xpu", Field: "server_device_type", Value: "no_such_dt"}},
	}
	for _, c := range cases {
		if _, err := planedit.Apply(src, []planedit.Op{c.op}); err == nil {
			t.Errorf("%s: expected rejection of an invalid server_device_type", c.name)
		}
	}
}

// TestApply_InvalidEditRejected: an edit that makes the plan fail ingest must be
// rejected (the D26 guard), not returned for persistence.
func TestApply_InvalidEditRejected(t *testing.T) {
	src := meshPlan(t)
	// A non-numeric quantity breaks ingest (quantity is an int field).
	if _, err := planedit.Apply(src, []planedit.Op{
		{Op: "set_server_field", ServerClass: "compute_xpu", Field: "quantity", Value: "not-a-number"},
	}); err == nil {
		t.Error("expected a non-numeric quantity to be rejected by the re-validate guard")
	}
	// An unknown target id is a structural op error.
	if _, err := planedit.Apply(src, []planedit.Op{
		{Op: "set_server_field", ServerClass: "does_not_exist", Field: "quantity", Value: "2"},
	}); err == nil {
		t.Error("expected an unknown server class to error")
	}
}

// assertUntouchedFaithful checks that regions the edit does not target keep their
// content faithfully across the yaml.Node round-trip — by re-projecting and
// comparing, plus spot-checking that representative untouched blocks survive.
func assertUntouchedFaithful(t *testing.T, src, out []byte) {
	t.Helper()
	// The edited plan must still ingest (Apply already guards this, but assert the
	// structural counts are preserved — nothing dropped).
	op, _, err := topology.IngestBundled(src)
	if err != nil {
		t.Fatalf("baseline ingest: %v", err)
	}
	np, _, err := topology.IngestBundled(out)
	if err != nil {
		t.Fatalf("edited ingest: %v", err)
	}
	if len(op.Spec.Connections) != len(np.Spec.Connections) {
		t.Errorf("server_connections count changed: %d -> %d", len(op.Spec.Connections), len(np.Spec.Connections))
	}
	if len(op.Spec.PortZones) != len(np.Spec.PortZones) {
		t.Errorf("switch_port_zones count changed: %d -> %d", len(op.Spec.PortZones), len(np.Spec.PortZones))
	}
	// A representative untouched reference_data id must still be present verbatim.
	if !strings.Contains(string(out), "transceiver_module_type: osfp_400g_dr4") {
		t.Errorf("an untouched switch_port_zones transceiver line did not survive the round-trip")
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// --- connections (P1.1b, #69) ------------------------------------------------

func conn(p *planedit.Projection, scID, connID string) *planedit.Connection {
	for i := range p.ServerClasses {
		if p.ServerClasses[i].ID != scID {
			continue
		}
		for j := range p.ServerClasses[i].Connections {
			if p.ServerClasses[i].Connections[j].ConnectionID == connID {
				return &p.ServerClasses[i].Connections[j]
			}
		}
	}
	return nil
}

func TestProject_ConnectionsAndTargetZones(t *testing.T) {
	p, err := planedit.Project(meshPlan(t))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	c := conn(p, "compute_xpu", "scale-out-rail-0")
	if c == nil {
		t.Fatal("compute_xpu connection scale-out-rail-0 missing")
	}
	if c.TargetZone != "soc_storage_scale_out_leaf/scale_out_server_2x400" {
		t.Errorf("target_zone: got %q", c.TargetZone)
	}
	if c.NIC != "scale_out" || c.Speed != 400 {
		t.Errorf("connection fields off: %+v", c)
	}
	if !contains(p.Catalog.TargetZones, "soc_storage_scale_out_leaf/scale_out_server_2x400") {
		t.Errorf("catalog.target_zones missing the scale-out zone: %v", p.Catalog.TargetZones)
	}
	if !contains(p.Catalog.TargetZones, "inb_mgmt_leaf/inb_mgmt_server_25g") {
		t.Errorf("catalog.target_zones missing the inb-mgmt zone: %v", p.Catalog.TargetZones)
	}
}

func TestApply_SetConnectionTargetZone(t *testing.T) {
	src := meshPlan(t)
	p0, _ := planedit.Project(src)
	idx := conn(p0, "compute_xpu", "scale-out-rail-0").Index
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "set_connection_field", ConnIndex: idx, Field: "target_zone", Value: "soc_storage_scale_out_leaf/soc_storage_server_4x200"},
	})
	if err != nil {
		t.Fatalf("Apply(set target_zone): %v", err)
	}
	p, _ := planedit.Project(out)
	if c := conn(p, "compute_xpu", "scale-out-rail-0"); c == nil || c.TargetZone != "soc_storage_scale_out_leaf/soc_storage_server_4x200" {
		t.Errorf("target_zone not applied: %+v", c)
	}
	assertUntouchedFaithful(t, src, out)
}

func TestApply_RemoveConnection(t *testing.T) {
	src := meshPlan(t)
	p0, _ := planedit.Project(src)
	before := len(p0.ServerClasses[0].Connections)
	idx := conn(p0, "compute_xpu", "scale-out-rail-7").Index
	out, err := planedit.Apply(src, []planedit.Op{{Op: "remove_connection", ConnIndex: idx}})
	if err != nil {
		t.Fatalf("Apply(remove): %v", err)
	}
	p, _ := planedit.Project(out)
	if conn(p, "compute_xpu", "scale-out-rail-7") != nil {
		t.Error("connection still present after remove")
	}
	if got := len(p.ServerClasses[0].Connections); got != before-1 {
		t.Errorf("connection count: got %d want %d", got, before-1)
	}
}

func TestApply_AddConnection(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "add_connection", ServerClass: "hh_controller", ConnectionID: "extra-inb-0", Fields: map[string]string{
			"target_zone": "inb_mgmt_leaf/inb_mgmt_server_25g", "nic": "inb_mgmt", "speed": "25", "ports_per_connection": "1", "distribution": "same-switch",
		}},
	})
	if err != nil {
		t.Fatalf("Apply(add connection): %v", err)
	}
	p, _ := planedit.Project(out)
	if c := conn(p, "hh_controller", "extra-inb-0"); c == nil || c.TargetZone != "inb_mgmt_leaf/inb_mgmt_server_25g" {
		t.Errorf("added connection not projected: %+v", c)
	}
}

func TestApply_ConnectionInvalidRejected(t *testing.T) {
	src := meshPlan(t)
	p0, _ := planedit.Project(src)
	idx := conn(p0, "compute_xpu", "scale-out-rail-0").Index
	cases := []struct {
		name string
		op   planedit.Op
	}{
		{"set blank target_zone", planedit.Op{Op: "set_connection_field", ConnIndex: idx, Field: "target_zone", Value: ""}},
		{"set unknown target_zone", planedit.Op{Op: "set_connection_field", ConnIndex: idx, Field: "target_zone", Value: "no_such_class/no_zone"}},
		{"add unknown target_zone", planedit.Op{Op: "add_connection", ServerClass: "hh_controller", ConnectionID: "x9", Fields: map[string]string{"target_zone": "bad/zone", "nic": "inb_mgmt"}}},
		{"remove out of range", planedit.Op{Op: "remove_connection", ConnIndex: 9999}},
		{"set out of range", planedit.Op{Op: "set_connection_field", ConnIndex: 9999, Field: "speed", Value: "100"}},
	}
	for _, c := range cases {
		if _, err := planedit.Apply(src, []planedit.Op{c.op}); err == nil {
			t.Errorf("%s: expected rejection", c.name)
		}
	}
}
