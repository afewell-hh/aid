package planedit_test

// RED (#81): the three missing structured-CREATE ops — add_switch_class,
// add_zone, add_nic (spec #79 Ticket B). Happy paths fail now because applyOne
// returns "unknown op"; reject paths assert the SPECIFIC validator message the
// GREEN impl must produce, so "unknown op" does not spuriously satisfy them.
// Server-class / connection create flows are out of scope and untouched.

import (
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/planedit"
)

// mustReject asserts Apply(op) fails AND the error mentions want (the owned
// validator message) — so an unimplemented op ("unknown op") is NOT a pass.
func mustReject(t *testing.T, src []byte, op planedit.Op, want string) {
	t.Helper()
	_, err := planedit.Apply(src, []planedit.Op{op})
	if err == nil {
		t.Errorf("expected rejection (%s), got nil", want)
		return
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected error containing %q, got %q", want, err.Error())
	}
}

// --- add_switch_class -------------------------------------------------------

func TestApply_AddSwitchClass_HappyRoundTrips(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "add_switch_class", SwitchClass: "extra_leaf", Fields: map[string]string{
			"fabric_name":           "extra-fabric",
			"fabric_class":          "managed",
			"hedgehog_role":         "server-leaf",
			"device_type_extension": "sw_ds2000_inb_ext",
			"topology_mode":         "mesh",
			"override_quantity":     "2",
		}},
	})
	if err != nil {
		t.Fatalf("Apply(add_switch_class): %v", err)
	}
	p, _ := planedit.Project(out)
	sw := switchClass(p, "extra_leaf")
	if sw == nil || sw.TopologyMode != "mesh" || sw.DeviceTypeExtension != "sw_ds2000_inb_ext" {
		t.Errorf("added switch class not projected correctly: %+v", sw)
	}
	if sw != nil && (sw.OverrideQuantity == nil || *sw.OverrideQuantity != 2) {
		t.Errorf("override_quantity not applied: %+v", sw)
	}
}

func TestApply_AddSwitchClass_Rejected(t *testing.T) {
	src := meshPlan(t)
	base := func(overrides map[string]string) map[string]string {
		f := map[string]string{
			"fabric_name": "extra-fabric", "fabric_class": "managed",
			"hedgehog_role": "server-leaf", "device_type_extension": "sw_ds2000_inb_ext",
		}
		for k, v := range overrides {
			if v == "" {
				delete(f, k)
			} else {
				f[k] = v
			}
		}
		return f
	}
	// duplicate switch_class_id
	mustReject(t, src, planedit.Op{Op: "add_switch_class", SwitchClass: "soc_storage_scale_out_leaf", Fields: base(nil)}, "already exists")
	// unknown device_type_extension
	mustReject(t, src, planedit.Op{Op: "add_switch_class", SwitchClass: "x1", Fields: base(map[string]string{"device_type_extension": "no_such_ext"})}, "device_type_extension")
	// missing required fabric_name
	mustReject(t, src, planedit.Op{Op: "add_switch_class", SwitchClass: "x2", Fields: base(map[string]string{"fabric_name": ""})}, "fabric_name")
	// missing required hedgehog_role
	mustReject(t, src, planedit.Op{Op: "add_switch_class", SwitchClass: "x3", Fields: base(map[string]string{"hedgehog_role": ""})}, "hedgehog_role")
	// bad fabric_class
	mustReject(t, src, planedit.Op{Op: "add_switch_class", SwitchClass: "x4", Fields: base(map[string]string{"fabric_class": "bogus"})}, "fabric_class")
	// missing id
	mustReject(t, src, planedit.Op{Op: "add_switch_class", SwitchClass: "", Fields: base(nil)}, "switch_class")
}

// --- add_zone ---------------------------------------------------------------

func TestApply_AddZone_HappyRoundTrips(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "add_zone", SwitchClass: "soc_storage_scale_out_leaf", ZoneName: "extra_zone", Fields: map[string]string{
			"zone_type":               "server",
			"port_spec":               "1-4",
			"breakout_option":         "brk_2x400_osfp",
			"transceiver_module_type": "osfp_400g_dr4",
			"allocation_strategy":     "sequential",
			"priority":                "99",
		}},
	})
	if err != nil {
		t.Fatalf("Apply(add_zone): %v", err)
	}
	// A zone lands as a new "switch_class/zone_name" target-zone option.
	p, _ := planedit.Project(out)
	want := "soc_storage_scale_out_leaf/extra_zone"
	found := false
	for _, tz := range p.Catalog.TargetZones {
		if tz == want {
			found = true
		}
	}
	if !found {
		t.Errorf("added zone not present in target zones: %v", p.Catalog.TargetZones)
	}
}

func TestApply_AddZone_Rejected(t *testing.T) {
	src := meshPlan(t)
	base := func(overrides map[string]string) map[string]string {
		f := map[string]string{"zone_type": "server", "port_spec": "1-4"}
		for k, v := range overrides {
			if v == "" {
				delete(f, k)
			} else {
				f[k] = v
			}
		}
		return f
	}
	// unknown switch_class
	mustReject(t, src, planedit.Op{Op: "add_zone", SwitchClass: "no_such_switch", ZoneName: "z1", Fields: base(nil)}, "switch_class")
	// duplicate zone_name within class (soc_storage_scale_out_uplink_800 already exists)
	mustReject(t, src, planedit.Op{Op: "add_zone", SwitchClass: "soc_storage_scale_out_leaf", ZoneName: "soc_storage_scale_out_uplink_800", Fields: base(nil)}, "already exists")
	// bad breakout_option ref
	mustReject(t, src, planedit.Op{Op: "add_zone", SwitchClass: "soc_storage_scale_out_leaf", ZoneName: "z2", Fields: base(map[string]string{"breakout_option": "no_such_brk"})}, "breakout_option")
	// bad transceiver_module_type ref
	mustReject(t, src, planedit.Op{Op: "add_zone", SwitchClass: "soc_storage_scale_out_leaf", ZoneName: "z3", Fields: base(map[string]string{"transceiver_module_type": "no_such_xcvr"})}, "transceiver_module_type")
	// missing zone_name
	mustReject(t, src, planedit.Op{Op: "add_zone", SwitchClass: "soc_storage_scale_out_leaf", ZoneName: "", Fields: base(nil)}, "zone_name")
	// missing required port_spec
	mustReject(t, src, planedit.Op{Op: "add_zone", SwitchClass: "soc_storage_scale_out_leaf", ZoneName: "z4", Fields: base(map[string]string{"port_spec": ""})}, "port_spec")
}

// --- add_nic ----------------------------------------------------------------

func TestApply_AddNic_HappyRoundTrips(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "add_nic", ServerClass: "compute_xpu", NicID: "extra_nic", Value: "nic_dual_25g"},
	})
	if err != nil {
		t.Fatalf("Apply(add_nic): %v", err)
	}
	p, _ := planedit.Project(out)
	sc := serverClass(p, "compute_xpu")
	if sc == nil {
		t.Fatal("compute_xpu missing")
	}
	found := false
	for _, n := range sc.Nics {
		if n.NicID == "extra_nic" && n.ModuleType == "nic_dual_25g" {
			found = true
		}
	}
	if !found {
		t.Errorf("added nic not projected: %+v", sc.Nics)
	}
}

func TestApply_AddNic_Rejected(t *testing.T) {
	src := meshPlan(t)
	// unknown server_class
	mustReject(t, src, planedit.Op{Op: "add_nic", ServerClass: "no_such_server", NicID: "n1", Value: "nic_dual_25g"}, "server_class")
	// duplicate nic_id within class (compute_xpu already has scale_out)
	mustReject(t, src, planedit.Op{Op: "add_nic", ServerClass: "compute_xpu", NicID: "scale_out", Value: "nic_dual_25g"}, "already exists")
	// unknown module_type
	mustReject(t, src, planedit.Op{Op: "add_nic", ServerClass: "compute_xpu", NicID: "n2", Value: "no_such_module"}, "module_type")
	// missing module_type
	mustReject(t, src, planedit.Op{Op: "add_nic", ServerClass: "compute_xpu", NicID: "n3", Value: ""}, "module_type")
}

// --- untouched-region fidelity (D26) ----------------------------------------

// TestApply_CreateOps_PreserveUntouchedRegions: each create appends to its own
// list and leaves other regions faithful (same standard as the existing ops).
func TestApply_CreateOps_PreserveUntouchedRegions(t *testing.T) {
	src := meshPlan(t)
	out, err := planedit.Apply(src, []planedit.Op{
		{Op: "add_nic", ServerClass: "compute_xpu", NicID: "extra_nic", Value: "nic_dual_25g"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertUntouchedFaithful(t, src, out)
}
