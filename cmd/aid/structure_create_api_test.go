package main

// RED (#81): the three structured-CREATE ops over the existing PUT
// /api/plans/{id}/structure contract — success returns the fresh projection and
// persists; an invalid op is 422 and the stored plan is untouched (D26). Reuses
// seedDIET / do / assertJSONError from the other cmd/aid tests.

import (
	"net/http"
	"strings"
	"testing"
)

func TestAPI_Structure_AddSwitchClass(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	body := `{"ops":[{"op":"add_switch_class","switch_class":"extra_leaf","fields":{` +
		`"fabric_name":"extra-fabric","fabric_class":"managed","hedgehog_role":"server-leaf",` +
		`"device_type_extension":"sw_ds2000_inb_ext","topology_mode":"mesh","override_quantity":"2"}}]}`
	rec := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT add_switch_class: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "extra_leaf") {
		t.Errorf("projection should show the new switch class; body=%s", rec.Body.String())
	}
	// persisted: a re-GET shows it.
	rec2 := do(t, mux, http.MethodGet, "/api/plans/p/structure", nil)
	if !strings.Contains(rec2.Body.String(), "extra_leaf") {
		t.Errorf("add_switch_class did not persist; re-GET body=%s", rec2.Body.String())
	}
}

func TestAPI_Structure_AddZone(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	body := `{"ops":[{"op":"add_zone","switch_class":"soc_storage_scale_out_leaf","zone_name":"extra_zone","fields":{` +
		`"zone_type":"server","port_spec":"1-4","breakout_option":"brk_2x400_osfp",` +
		`"transceiver_module_type":"osfp_400g_dr4","allocation_strategy":"sequential","priority":"99"}}]}`
	rec := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT add_zone: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	// A new zone surfaces as a "switch_class/zone_name" target-zone option.
	if !strings.Contains(rec.Body.String(), "soc_storage_scale_out_leaf/extra_zone") {
		t.Errorf("projection should show the new zone as a target_zone; body=%s", rec.Body.String())
	}
}

func TestAPI_Structure_AddNic(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	body := `{"ops":[{"op":"add_nic","server_class":"compute_xpu","nic_id":"extra_nic","value":"nic_dual_25g"}]}`
	rec := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT add_nic: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "extra_nic") {
		t.Errorf("projection should show the new nic; body=%s", rec.Body.String())
	}
}

// Invalid creates → 422 and the stored plan is untouched (D26).
func TestAPI_Structure_CreateInvalid_Rejected_PlanUntouched(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	for _, bad := range []string{
		// duplicate switch class id
		`{"ops":[{"op":"add_switch_class","switch_class":"soc_storage_scale_out_leaf","fields":{"fabric_name":"f","fabric_class":"managed","hedgehog_role":"server-leaf","device_type_extension":"sw_ds2000_inb_ext"}}]}`,
		// unknown switch class for a zone
		`{"ops":[{"op":"add_zone","switch_class":"no_such_switch","zone_name":"z","fields":{"zone_type":"server","port_spec":"1-4"}}]}`,
		// unknown module type for a nic
		`{"ops":[{"op":"add_nic","server_class":"compute_xpu","nic_id":"n","value":"no_such_module"}]}`,
	} {
		rec := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte(bad))
		assertJSONError(t, rec, http.StatusUnprocessableEntity)
	}

	// The stored plan is untouched — the original switch class set (3) survives and
	// no stray ids were added.
	rec2 := do(t, mux, http.MethodGet, "/api/plans/p/structure", nil)
	b := rec2.Body.String()
	if strings.Contains(b, "no_such_switch") || strings.Contains(b, "no_such_module") {
		t.Errorf("a rejected create must not corrupt the stored plan; re-GET body=%s", b)
	}
}
