package main

// httptest coverage for the structured-editing endpoints (D26 / #67):
// GET /api/plans/{id}/structure (projection) and PUT .../structure (field-patch
// with re-validate-before-persist). Reuses the DIET seed + do() helpers from the
// other cmd/aid tests; seeds a real oracle plan via the store.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAPI_Structure_Projection(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	rec := do(t, mux, http.MethodGet, "/api/plans/p/structure", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET structure: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		ServerClasses []struct {
			ID            string `json:"id"`
			Quantity      int    `json:"quantity"`
			GpusPerServer int    `json:"gpus_per_server"`
			Nics          []struct {
				NicID      string `json:"nic_id"`
				ModuleType string `json:"module_type"`
			} `json:"nics"`
		} `json:"server_classes"`
		SwitchClasses []struct {
			ID           string `json:"id"`
			TopologyMode string `json:"topology_mode"`
		} `json:"switch_classes"`
		Catalog struct {
			ModuleTypes          []string `json:"module_types"`
			DeviceTypeExtensions []string `json:"device_type_extensions"`
		} `json:"catalog"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode projection: %v; body=%s", err, rec.Body.String())
	}
	if len(got.ServerClasses) == 0 || len(got.SwitchClasses) == 0 {
		t.Fatalf("projection missing classes: %+v", got)
	}
	if len(got.Catalog.ModuleTypes) == 0 || len(got.Catalog.DeviceTypeExtensions) == 0 {
		t.Errorf("projection catalog dropdown lists are empty")
	}
}

func TestAPI_Structure_Patch_SetQuantity_Persists(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	body := `{"ops":[{"op":"set_server_field","server_class":"compute_xpu","field":"quantity","value":"16"}]}`
	rec := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT structure: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	// The returned projection reflects the edit.
	if !strings.Contains(rec.Body.String(), "\"quantity\": 16") {
		t.Errorf("patched projection should show quantity 16; body=%s", rec.Body.String())
	}
	// And it persisted: a re-GET of the structure shows 16.
	rec2 := do(t, mux, http.MethodGet, "/api/plans/p/structure", nil)
	if !strings.Contains(rec2.Body.String(), "\"quantity\": 16") {
		t.Errorf("edit did not persist; re-GET body=%s", rec2.Body.String())
	}
}

// An edit that fails ingest must be rejected (422) and MUST NOT corrupt the
// stored plan (the D26 guard): a subsequent GET still returns the original.
func TestAPI_Structure_Patch_InvalidRejected_PlanUntouched(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	body := `{"ops":[{"op":"set_server_field","server_class":"compute_xpu","field":"quantity","value":"not-a-number"}]}`
	rec := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte(body))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid edit: got %d want 422; body=%s", rec.Code, rec.Body.String())
	}
	assertJSONError(t, rec, http.StatusUnprocessableEntity)
	// The stored plan is untouched — original quantity (8) survives.
	rec2 := do(t, mux, http.MethodGet, "/api/plans/p/structure", nil)
	if !strings.Contains(rec2.Body.String(), "\"quantity\": 8") {
		t.Errorf("a rejected edit must not corrupt the stored plan; re-GET body=%s", rec2.Body.String())
	}
}

func TestAPI_Structure_Patch_MalformedBody_400(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)
	rec := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte("{not json"))
	assertJSONError(t, rec, http.StatusBadRequest)
}

// connections (P1.1b, #69): the projection exposes connections + target_zone
// options, and a connection target_zone edit round-trips via PUT .../structure.
func TestAPI_Structure_Connections(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	// projection carries connections + the target_zone dropdown options.
	get := do(t, mux, http.MethodGet, "/api/plans/p/structure", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("GET structure: %d; %s", get.Code, get.Body.String())
	}
	body := get.Body.String()
	if !strings.Contains(body, "\"connections\"") || !strings.Contains(body, "scale-out-rail-0") {
		t.Errorf("projection missing connections: %s", body)
	}
	if !strings.Contains(body, "soc_storage_scale_out_leaf/scale_out_server_2x400") {
		t.Errorf("projection missing target_zone options")
	}

	// retarget connection index 0's zone -> persists.
	patch := `{"ops":[{"op":"set_connection_field","conn_index":0,"field":"target_zone","value":"soc_storage_scale_out_leaf/soc_storage_server_4x200"}]}`
	rec := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte(patch))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT connection edit: %d; %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "soc_storage_server_4x200") {
		t.Errorf("edited target_zone not reflected: %s", rec.Body.String())
	}

	// an invalid target_zone is rejected (422) and the plan is untouched.
	bad := `{"ops":[{"op":"set_connection_field","conn_index":0,"field":"target_zone","value":"bad/zone"}]}`
	rec2 := do(t, mux, http.MethodPut, "/api/plans/p/structure", []byte(bad))
	assertJSONError(t, rec2, http.StatusUnprocessableEntity)
}

// P2.1 (#71): the list + detail responses carry derived facts (topology / gpu /
// totals / validity), engine-derived for both a mesh and a Clos plan.
func TestAPI_DerivedFacts(t *testing.T) {
	mesh, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	clos, _, _ := oracleArtifacts(t, "xoc-256-2xopg128-clos-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "mesh64", mesh)
	seedDIET(t, dir, "clos256", clos)

	rec := do(t, mux, http.MethodGet, "/api/plans", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var out struct {
		Plans []struct {
			ID    string    `json:"id"`
			Facts planFacts `json:"facts"`
		} `json:"plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v; %s", err, rec.Body.String())
	}
	byID := map[string]planFacts{}
	for _, p := range out.Plans {
		byID[p.ID] = p.Facts
	}
	// mesh xoc-64: topology mesh, compute 8 servers × 8 gpus = 64 gpus, valid.
	m := byID["mesh64"]
	if m.Topology != "mesh" || !m.IsValid || !m.Computable {
		t.Errorf("mesh facts off: %+v", m)
	}
	if m.GpuCount != 64 || m.SwitchTotal == 0 {
		t.Errorf("mesh gpu/switch off: %+v", m)
	}
	// Clos xoc-256: topology Clos (spine tier), derived switch total 9, valid.
	c := byID["clos256"]
	if c.Topology != "Clos" || !c.IsValid {
		t.Errorf("clos facts off: %+v", c)
	}
	if c.SwitchTotal != 9 { // be-rail-leaf 4 + be-spine 2 + fe-leaf 2 + fe-spine 1
		t.Errorf("clos switch total: got %d want 9", c.SwitchTotal)
	}

	// detail carries facts too.
	det := do(t, mux, http.MethodGet, "/api/plans/mesh64", nil)
	if !strings.Contains(det.Body.String(), "\"facts\"") || !strings.Contains(det.Body.String(), "\"topology\": \"mesh\"") {
		t.Errorf("detail missing facts: %s", det.Body.String())
	}
}
