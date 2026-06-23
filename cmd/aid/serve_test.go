package main

// httptest handler tests for the REST surface. Plan CRUD + structured-JSON-error
// coverage lives here (model-agnostic — unchanged by F7b). The compute-endpoint
// behavior (calc/bom/wiring over the rebuilt engine, DIET input, overlay
// sub-resource, two-plane validation) lives in f7b_integration_test.go against the
// committed XOC oracles; the pre-F7b compute tests that asserted the retired
// orchestrate shapes were removed in F7b (see the note below the CRUD section).

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/fixtures"
	"github.com/afewell-hh/aid/internal/planstore"
)

// TestAPI_CreatePlan_VendoredOracle_RoundTrips is the documented on-ramp, end to
// end over HTTP: POST a real vendored DIET plan to /api/plans and confirm it is
// accepted and round-trips (listable + gettable by its derived id). This is the
// path a new user / the GUI take. Regression guard for the meta.case_id identity
// contract — before the fix, every vendored plan returned 400 "plan has no id or
// name", and the F7b tests missed it by seeding the store dir directly.
func TestAPI_CreatePlan_VendoredOracle_RoundTrips(t *testing.T) {
	mux, _ := newTestAPI(t)
	for _, tc := range []struct{ name, wantID string }{
		{"xoc-64-mesh-conv-ro", "training_xoc64_1xopg64_mesh_conv_ro"},
		{"xoc-256-2xopg128-clos-ro", "training_xoc256_2xopg128_clos_ro"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trainingPath, _, _ := oracleArtifacts(t, tc.name)
			training, err := os.ReadFile(trainingPath)
			if err != nil {
				t.Fatalf("read training fixture: %v", err)
			}
			rec := do(t, mux, http.MethodPost, "/api/plans", training)
			if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
				t.Fatalf("POST /api/plans = %d, want 2xx; body=%s", rec.Code, rec.Body.String())
			}
			var created struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
				t.Fatalf("decode create response: %v; body=%s", err, rec.Body.String())
			}
			if created.ID != tc.wantID {
				t.Errorf("created id = %q, want %q (from meta.case_id)", created.ID, tc.wantID)
			}
			if created.Name == "" {
				t.Errorf("created name empty, want meta.name resolved")
			}
			if g := do(t, mux, http.MethodGet, "/api/plans/"+created.ID, nil); g.Code != http.StatusOK {
				t.Errorf("GET created plan = %d, want 200; body=%s", g.Code, g.Body.String())
			}
		})
	}
}

// newTestAPI returns a mux backed by a fresh temp-dir plan store, plus the dir.
func newTestAPI(t *testing.T) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := planstore.Open(dir)
	if err != nil {
		t.Fatalf("planstore.Open: %v", err)
	}
	return newServeMux(store), dir
}

// seedPlan writes a fixture plan YAML into the store dir as <id>.yaml so the
// read/calc/bom/wiring routes have something to operate on.
func seedPlan(t *testing.T, dir, id, kind, name string) {
	t.Helper()
	b, err := fixtures.PlanYAML(kind, name)
	if err != nil {
		t.Fatalf("read fixture %s/%s: %v", kind, name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".yaml"), b, 0o644); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func do(t *testing.T, mux http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// assertJSONError asserts a structured JSON error with the wanted status.
func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status: got %d want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type %q is not application/json", ct)
	}
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error == "" {
		t.Errorf("error body is not structured {\"error\":...}: %s", rec.Body.String())
	}
}

// --- GET /api/plans (list) --------------------------------------------------

func TestAPI_ListPlans_Empty(t *testing.T) {
	mux, _ := newTestAPI(t)
	rec := do(t, mux, http.MethodGet, "/api/plans", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Plans []planstore.Plan `json:"plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode list: %v; body=%s", err, rec.Body.String())
	}
	if len(out.Plans) != 0 {
		t.Errorf("empty store: got %d plans want 0", len(out.Plans))
	}
}

func TestAPI_ListPlans_SeededSummaries(t *testing.T) {
	mux, dir := newTestAPI(t)
	seedPlan(t, dir, "clos-small", "valid", "clos-small")
	seedPlan(t, dir, "switch-bom", "valid", "switch-bom")

	rec := do(t, mux, http.MethodGet, "/api/plans", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Plans []planstore.Plan `json:"plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(out.Plans) != 2 {
		t.Fatalf("got %d plans want 2", len(out.Plans))
	}
	for _, p := range out.Plans {
		if p.ID == "" || p.Name == "" {
			t.Errorf("summary missing id/name: %+v", p)
		}
		if p.YAML != "" {
			t.Errorf("list summary should omit yaml, got %q", p.YAML)
		}
	}
}

// --- POST /api/plans (create) -----------------------------------------------

func TestAPI_CreatePlan(t *testing.T) {
	mux, dir := newTestAPI(t)
	body, err := fixtures.PlanYAML("valid", "clos-small")
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, mux, http.MethodPost, "/api/plans", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d want 201; body=%s", rec.Code, rec.Body.String())
	}
	var p planstore.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode created plan: %v", err)
	}
	if p.ID != "clos-small" {
		t.Errorf("created id: got %q want clos-small", p.ID)
	}
	if _, err := os.Stat(filepath.Join(dir, "clos-small.yaml")); err != nil {
		t.Errorf("create did not persist <id>.yaml: %v", err)
	}
}

func TestAPI_CreatePlan_MalformedYAML(t *testing.T) {
	mux, _ := newTestAPI(t)
	rec := do(t, mux, http.MethodPost, "/api/plans", []byte("this: : not: valid: yaml\n  - ["))
	assertJSONError(t, rec, http.StatusBadRequest)
}

// TestAPI_CreatePlan_RejectsTraversalID is the real path-traversal vector: a
// plan whose YAML id escapes the store dir must be rejected, never written.
func TestAPI_CreatePlan_RejectsTraversalID(t *testing.T) {
	mux, dir := newTestAPI(t)
	body := []byte("id: ../../pwned\nname: Evil Plan\nstatus: draft\n")
	rec := do(t, mux, http.MethodPost, "/api/plans", body)
	assertJSONError(t, rec, http.StatusBadRequest)
	if _, err := os.Stat(filepath.Join(dir, "..", "..", "pwned.yaml")); err == nil {
		t.Fatal("path traversal id wrote a file outside the store dir")
	}
}

// --- GET /api/plans/{id} (detail) -------------------------------------------

func TestAPI_GetPlan(t *testing.T) {
	mux, dir := newTestAPI(t)
	seedPlan(t, dir, "clos-small", "valid", "clos-small")
	rec := do(t, mux, http.MethodGet, "/api/plans/clos-small", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	var p planstore.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if p.ID != "clos-small" || p.Name == "" {
		t.Errorf("detail missing id/name: %+v", p)
	}
	if !strings.Contains(p.YAML, "fabric_domains") {
		t.Errorf("detail should include the canonical YAML, got %q", p.YAML)
	}
}

func TestAPI_GetPlan_NotFound(t *testing.T) {
	mux, _ := newTestAPI(t)
	rec := do(t, mux, http.MethodGet, "/api/plans/does-not-exist", nil)
	assertJSONError(t, rec, http.StatusNotFound)
}

// TestAPI_GetPlan_TraversalIDRejected exercises the URL-layer defense in depth:
// an unsafe dot-segment id reaching the item handler must be rejected as a bad
// request, not resolved against the filesystem. The request bypasses ServeMux
// path cleaning by setting URL.Path directly on the item handler, so the id
// arrives as a single (unsafe) segment.
func TestAPI_GetPlan_TraversalIDRejected(t *testing.T) {
	_, dir := newTestAPI(t)
	store, _ := planstore.Open(dir)
	a := &api{store: store}
	req := httptest.NewRequest(http.MethodGet, "/api/plans/placeholder", nil)
	req.URL.Path = "/api/plans/.." // single unsafe id segment reaching getPlan
	rec := httptest.NewRecorder()
	a.routePlanID(rec, req)
	assertJSONError(t, rec, http.StatusBadRequest)
}

// --- PUT /api/plans/{id} (update) -------------------------------------------

func TestAPI_UpdatePlan(t *testing.T) {
	mux, dir := newTestAPI(t)
	seedPlan(t, dir, "clos-small", "valid", "clos-small")
	updated := []byte("id: clos-small\nname: Renamed Clos\nstatus: active\n")
	rec := do(t, mux, http.MethodPut, "/api/plans/clos-small", updated)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}
	var p planstore.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode updated plan: %v", err)
	}
	if p.Name != "Renamed Clos" {
		t.Errorf("update name: got %q want Renamed Clos", p.Name)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "clos-small.yaml"))
	if !strings.Contains(string(got), "Renamed Clos") {
		t.Errorf("update did not persist new YAML: %s", got)
	}
}

func TestAPI_UpdatePlan_NotFound(t *testing.T) {
	mux, _ := newTestAPI(t)
	rec := do(t, mux, http.MethodPut, "/api/plans/missing", []byte("id: missing\nname: X\n"))
	assertJSONError(t, rec, http.StatusNotFound)
}

// --- DELETE /api/plans/{id} -------------------------------------------------

func TestAPI_DeletePlan(t *testing.T) {
	mux, dir := newTestAPI(t)
	seedPlan(t, dir, "clos-small", "valid", "clos-small")
	rec := do(t, mux, http.MethodDelete, "/api/plans/clos-small", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d want 204; body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "clos-small.yaml")); !os.IsNotExist(err) {
		t.Errorf("delete did not remove the file (err=%v)", err)
	}
	// A subsequent GET must 404.
	rec2 := do(t, mux, http.MethodGet, "/api/plans/clos-small", nil)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("get after delete: got %d want 404", rec2.Code)
	}
}

func TestAPI_DeletePlan_NotFound(t *testing.T) {
	mux, _ := newTestAPI(t)
	rec := do(t, mux, http.MethodDelete, "/api/plans/missing", nil)
	assertJSONError(t, rec, http.StatusNotFound)
}

// --- compute endpoints: calc / bom / wiring ---------------------------------
//
// The compute-endpoint behavior — new CalcOutput JSON (is_valid + switch/server
// quantities + endpoints + verdicts), bom rows[] via bom.RenderJSON, wiring Doc
// YAML, DIET/training input, the overlay sub-resource, and two-plane validation —
// is covered by f7b_integration_test.go against the committed XOC oracles. The
// pre-F7b compute tests here asserted the retired orchestrate shapes (IR
// envelope, hierarchical per-unit/fleet BOM, old export_validate codes like
// MCLAG_SWITCH_COUNT — see note §3.0 scope boundary) on old-schema fixtures the
// rebuilt engine does not ingest, so they were removed in F7b. The NotFound
// guards below stay (model-agnostic).

func TestAPI_BOM_NotFound(t *testing.T) {
	mux, _ := newTestAPI(t)
	rec := do(t, mux, http.MethodGet, "/api/plans/missing/bom", nil)
	assertJSONError(t, rec, http.StatusNotFound)
}

func TestAPI_Wiring_NotFound(t *testing.T) {
	mux, _ := newTestAPI(t)
	rec := do(t, mux, http.MethodGet, "/api/plans/missing/wiring/frontend", nil)
	assertJSONError(t, rec, http.StatusNotFound)
}
