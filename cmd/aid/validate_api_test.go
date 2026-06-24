package main

// httptest coverage for the stateless dry-run validate endpoint (P1.3 / #68):
// POST /api/validate returns the calc summary for a draft plan WITHOUT persisting
// anything; same two-plane contract as calc.

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func validateBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal validate body: %v", err)
	}
	return b
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestAPI_Validate_Valid(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)

	body := validateBody(t, map[string]any{"yaml": readFile(t, training)})
	rec := do(t, mux, http.MethodPost, "/api/validate", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate(valid): got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"is_valid\": true") {
		t.Errorf("expected is_valid:true; body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"switch_quantity\"") {
		t.Errorf("expected computed quantities; body=%s", rec.Body.String())
	}
	// Persists NOTHING: the store dir stays empty.
	assertStoreEmpty(t, dir)
}

func TestAPI_Validate_OverAlloc_IsValidFalse(t *testing.T) {
	mux, dir := newTestAPI(t)
	body := validateBody(t, map[string]any{
		"yaml": readFile(t, overAllocFixture()),
	})
	rec := do(t, mux, http.MethodPost, "/api/validate", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate(over-alloc) must be 200 (calc errors as data), got %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"is_valid\": false") {
		t.Errorf("over-alloc must be is_valid:false; body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ZONE_OVERFLOW") {
		t.Errorf("over-alloc must surface ZONE_OVERFLOW; body=%s", rec.Body.String())
	}
	assertStoreEmpty(t, dir)
}

func TestAPI_Validate_Malformed_4xx(t *testing.T) {
	mux, dir := newTestAPI(t)
	body := validateBody(t, map[string]any{"yaml": "not: : valid: yaml\n  - ["})
	rec := do(t, mux, http.MethodPost, "/api/validate", body)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("malformed draft must be a 4xx (cannot compute), got %d; body=%s", rec.Code, rec.Body.String())
	}
	assertJSONError(t, rec, rec.Code)
	assertStoreEmpty(t, dir)
}

// ops path: a draft = stored plan YAML + structured ops, validated without
// persisting. A valid edit → 200; a structurally-bad edit → 4xx.
func TestAPI_Validate_WithOps(t *testing.T) {
	training, _, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	plan := readFile(t, training)
	mux, dir := newTestAPI(t)

	ok := validateBody(t, map[string]any{
		"yaml": plan,
		"ops":  []map[string]string{{"op": "set_server_field", "server_class": "hh_controller", "field": "quantity", "value": "2"}},
	})
	rec := do(t, mux, http.MethodPost, "/api/validate", ok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "\"is_valid\": true") {
		t.Errorf("valid ops edit should be 200 is_valid:true; got %d body=%s", rec.Code, rec.Body.String())
	}

	bad := validateBody(t, map[string]any{
		"yaml": plan,
		"ops":  []map[string]string{{"op": "set_server_field", "server_class": "hh_controller", "field": "quantity", "value": "not-a-number"}},
	})
	rec2 := do(t, mux, http.MethodPost, "/api/validate", bad)
	if rec2.Code < 400 || rec2.Code >= 500 {
		t.Errorf("structurally-bad ops edit should be 4xx; got %d", rec2.Code)
	}
	assertStoreEmpty(t, dir)
}

func assertStoreEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read store dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			t.Errorf("dry-run validate must persist nothing, but found %s in the store", e.Name())
		}
	}
}
