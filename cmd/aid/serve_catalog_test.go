package main

// RED (#80): API tests for the read-only Library browse endpoints. These fail
// against the listCatalog/getCatalogItem stubs (501) until #80 GREEN wires
// internal/library. The method-guard (405) path is real scaffolding and passes.

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAPI_Catalog_List_OK: GET /api/catalog returns 200 with the built-in
// Library union and a deterministic body.
func TestAPI_Catalog_List_OK(t *testing.T) {
	mux, _ := newTestAPI(t)
	rec := do(t, mux, http.MethodGet, "/api/catalog", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/catalog = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Items []struct {
			ID    struct{ Name, Version string } `json:"id"`
			Kind  string                         `json:"kind"`
			Layer string                         `json:"layer"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode /api/catalog: %v; body=%s", err, rec.Body.String())
	}
	if len(resp.Items) == 0 {
		t.Errorf("GET /api/catalog returned 0 items, want the built-in library union")
	}
	rec2 := do(t, mux, http.MethodGet, "/api/catalog", nil)
	if rec.Body.String() != rec2.Body.String() {
		t.Errorf("GET /api/catalog is not deterministic across calls")
	}
}

// TestAPI_Catalog_Item_KnownAndUnknown: a class present in the shipped references
// (xoc-256 clos fe-leaf) returns 200; an unknown id returns 404.
func TestAPI_Catalog_Item_KnownAndUnknown(t *testing.T) {
	mux, _ := newTestAPI(t)
	if rec := do(t, mux, http.MethodGet, "/api/catalog/fe-leaf", nil); rec.Code != http.StatusOK {
		t.Errorf("GET /api/catalog/fe-leaf = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if rec := do(t, mux, http.MethodGet, "/api/catalog/does-not-exist", nil); rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/catalog/does-not-exist = %d, want 404", rec.Code)
	}
}

// TestAPI_Catalog_NonGET_405: the Library is read-only.
func TestAPI_Catalog_NonGET_405(t *testing.T) {
	mux, _ := newTestAPI(t)
	if rec := do(t, mux, http.MethodPost, "/api/catalog", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/catalog = %d, want 405", rec.Code)
	}
}
