package main

// F7b RED — httptest API tests for `aid serve` retargeted onto internal/design
// (the rebuilt engine). These encode the GREEN contract (note §3.0/§3.2 + lead
// #64 concurrence) and FAIL now because the three compute handlers still route
// through internal/orchestrate (old plan schema + old response shapes) and the
// overlay sub-resource does not exist yet — i.e. they fail for the right reason.
//
// The existing serve_test.go (old-shape CRUD + compute assertions) stays GREEN
// during RED; its compute assertions get updated to the new shapes in GREEN
// (intentional behavior change), and internal/orchestrate stays until F7d.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/afewell-hh/aid/internal/oracle"
)

// --- seeding (DIET training bundle + optic overlay, written into the store) ---

func seedDIET(t *testing.T, dir, id, trainingPath string) {
	t.Helper()
	b, err := os.ReadFile(trainingPath)
	if err != nil {
		t.Fatalf("read training %s: %v", trainingPath, err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".yaml"), b, 0o644); err != nil {
		t.Fatalf("seed plan %s: %v", id, err)
	}
}

func seedOverlayFile(t *testing.T, dir, id, overlayPath string) {
	t.Helper()
	b, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatalf("read overlay %s: %v", overlayPath, err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".overlay.yaml"), b, 0o644); err != nil {
		t.Fatalf("seed overlay %s: %v", id, err)
	}
}

// calcResp is the new calc response contract (note §3.2): CalcOutput marshalled
// directly plus the derived is_valid boolean.
type calcResp struct {
	IsValid bool `json:"is_valid"`
	Errors  []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	SwitchQuantity []struct {
		ClassID  string `json:"class_id"`
		Quantity int    `json:"quantity"`
	} `json:"switch_quantity"`
	ServerQuantity []struct {
		ClassID  string `json:"class_id"`
		Quantity int    `json:"quantity"`
	} `json:"server_quantity"`
	Endpoints           []json.RawMessage `json:"endpoints"`
	TransceiverVerdicts []json.RawMessage `json:"transceiver_verdicts"`
}

func switchMap(r calcResp) map[string]int {
	m := map[string]int{}
	for _, q := range r.SwitchQuantity {
		m[q.ClassID] = q.Quantity
	}
	return m
}

// --- POST /api/plans/{id}/calc — reproduce computed quantities ----------------

func TestAPI_F7b_Calc_ReproducesQuantities(t *testing.T) {
	cases := []struct {
		comp       string
		wantSwitch map[string]int
	}{
		{"xoc-64-mesh-conv-ro", map[string]int{"soc_storage_scale_out_leaf": 2, "inb_mgmt_leaf": 1, "oob_leaf": 1}},
		{"xoc-256-2xopg128-clos-ro", map[string]int{"be-rail-leaf": 4, "be-spine": 2, "fe-leaf": 2, "fe-spine": 1}},
	}
	for _, c := range cases {
		t.Run(c.comp, func(t *testing.T) {
			training, _, _ := oracleArtifacts(t, c.comp)
			mux, dir := newTestAPI(t)
			seedDIET(t, dir, "p", training)

			rec := do(t, mux, http.MethodPost, "/api/plans/p/calc", nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("calc status: got %d want 200; body=%s", rec.Code, rec.Body.String())
			}
			var got calcResp
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode calc: %v; body=%s", err, rec.Body.String())
			}
			if !got.IsValid {
				t.Errorf("%s should be valid; errors=%+v", c.comp, got.Errors)
			}
			sm := switchMap(got)
			for id, want := range c.wantSwitch {
				if sm[id] != want {
					t.Errorf("%s switch_quantity[%s]=%d want %d (full=%+v)", c.comp, id, sm[id], want, sm)
				}
			}
			if len(got.Endpoints) == 0 {
				t.Errorf("%s calc produced no endpoints", c.comp)
			}
			if len(got.TransceiverVerdicts) == 0 {
				t.Errorf("%s calc produced no transceiver verdicts", c.comp)
			}
		})
	}
}

// --- GET /api/plans/{id}/bom — reproduce the committed bom.csv ----------------

type bomResp struct {
	Rows       []map[string]string `json:"rows"`
	Suppressed int                 `json:"suppressed_cable_assembly_count"`
}

func TestAPI_F7b_BOM_ReproducesOracle(t *testing.T) {
	for _, comp := range []string{"xoc-64-mesh-conv-ro", "xoc-256-2xopg128-clos-ro"} {
		t.Run(comp, func(t *testing.T) {
			training, overlay, _ := oracleArtifacts(t, comp)
			mux, dir := newTestAPI(t)
			seedDIET(t, dir, "p", training)
			seedOverlayFile(t, dir, "p", overlay)

			rec := do(t, mux, http.MethodGet, "/api/plans/p/bom", nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("bom status: got %d want 200; body=%s", rec.Code, rec.Body.String())
			}
			var got bomResp
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode bom: %v; body=%s", err, rec.Body.String())
			}

			// Build the expected rows from the committed bom.csv (header→object,
			// skipping the "# suppressed_..." footer) — the REST analogue of the
			// CLI byte-exact reproduction.
			var c oracle.Composition
			for _, cand := range oracle.Compositions() {
				if cand.Name == comp {
					c = cand
				}
			}
			csv, err := oracle.LoadCSV(filepath.Join(c.Dir(), "bom.csv"))
			if err != nil {
				t.Fatalf("load bom.csv: %v", err)
			}
			header := csv[0]
			var want []map[string]string
			wantSuppressed := 0
			for _, row := range csv[1:] {
				if len(row) > 0 && strings.HasPrefix(row[0], "#") {
					if len(row) > 1 {
						// "# suppressed_switch_cable_assembly_count, N"
						wantSuppressed, _ = strconv.Atoi(strings.TrimSpace(row[1]))
					}
					continue
				}
				obj := map[string]string{}
				for i, h := range header {
					if i < len(row) {
						obj[h] = row[i]
					}
				}
				want = append(want, obj)
			}
			if !reflect.DeepEqual(got.Rows, want) {
				t.Errorf("%s bom rows != committed bom.csv\n got=%+v\nwant=%+v", comp, got.Rows, want)
			}
			if got.Suppressed != wantSuppressed {
				t.Errorf("%s suppressed_cable_assembly_count=%d want %d", comp, got.Suppressed, wantSuppressed)
			}
		})
	}
}

// --- GET /api/plans/{id}/wiring/{fabric} --------------------------------------

// TestAPI_F7b_Wiring_ReproducesOracle fetches EVERY managed fabric via REST,
// assembles the fabric→YAML map, and asserts structural equivalence to the
// committed oracle wiring/ dir (the REST analogue of the engine wiring oracle) —
// not just that the CRD API-group string appears (devb RED finding 1).
func TestAPI_F7b_Wiring_ReproducesOracle(t *testing.T) {
	comp := "xoc-256-2xopg128-clos-ro"
	training, overlay, _ := oracleArtifacts(t, comp)
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)
	seedOverlayFile(t, dir, "p", overlay)

	var c oracle.Composition
	for _, cand := range oracle.Compositions() {
		if cand.Name == comp {
			c = cand
		}
	}
	computed := map[string][]byte{}
	for _, fabric := range c.Managed {
		rec := do(t, mux, http.MethodGet, "/api/plans/p/wiring/"+fabric, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("wiring/%s status: got %d want 200; body=%s", fabric, rec.Code, rec.Body.String())
		}
		computed[fabric] = append([]byte(nil), rec.Body.Bytes()...)
	}
	diff, err := oracle.CompareWiringHhfab(computed, filepath.Join(c.Dir(), "wiring"))
	if err != nil {
		t.Fatalf("CompareWiringHhfab: %v", err)
	}
	if !diff.Equal {
		t.Errorf("%s REST wiring != committed oracle: %v", comp, diff.Details)
	}
}

// --- GET/PUT /api/plans/{id}/overlay (new sub-resource) -----------------------

// TestAPI_F7b_Overlay_RoundTrip asserts EXACT byte fidelity: GET returns verbatim
// what PUT stored (devb RED finding 2), not merely that it contains "items".
func TestAPI_F7b_Overlay_RoundTrip(t *testing.T) {
	training, overlay, _ := oracleArtifacts(t, "xoc-64-mesh-conv-ro")
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "p", training)

	body, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatalf("read overlay: %v", err)
	}
	put := do(t, mux, http.MethodPut, "/api/plans/p/overlay", body)
	if put.Code != http.StatusOK && put.Code != http.StatusNoContent {
		t.Fatalf("PUT overlay: got %d want 200/204; body=%s", put.Code, put.Body.String())
	}
	get := do(t, mux, http.MethodGet, "/api/plans/p/overlay", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("GET overlay: got %d want 200; body=%s", get.Code, get.Body.String())
	}
	if !bytes.Equal(get.Body.Bytes(), body) {
		t.Errorf("overlay did not round-trip exactly: PUT %d bytes, GET %d bytes", len(body), get.Body.Len())
	}
}

// --- two-plane validation (note §3.0) -----------------------------------------

// over-alloc plan: calc returns 200 with is_valid:false + the violation as DATA.
func TestAPI_F7b_CalcErrorsSurfacedAsData(t *testing.T) {
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "bad", filepath.Join("..", "..", "tests", "fixtures", "f7a", "overalloc-training.yaml"))

	rec := do(t, mux, http.MethodPost, "/api/plans/bad/calc", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("calc-error plan must be 200 (validation as data), got %d; body=%s", rec.Code, rec.Body.String())
	}
	var got calcResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode calc: %v; body=%s", err, rec.Body.String())
	}
	if got.IsValid {
		t.Errorf("over-alloc plan must be invalid")
	}
	if len(got.Errors) == 0 {
		t.Errorf("over-alloc plan must carry calc errors as data; got %+v", got)
	}
}

// over-alloc plan: bom is GATED on no calc errors → exactly 422 (note §3.0), with
// a structured JSON error body — not a 200 BOM.
func TestAPI_F7b_BOMGatedOnCalcErrors(t *testing.T) {
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "bad", overAllocFixture())

	rec := do(t, mux, http.MethodGet, "/api/plans/bad/bom", nil)
	assertJSONError(t, rec, http.StatusUnprocessableEntity)
}

// over-alloc plan: wiring is GATED on no calc errors → exactly 422 (note §3.0)
// (devb GREEN finding 2 — the missing wiring-on-calc-error path).
func TestAPI_F7b_WiringGatedOnCalcErrors(t *testing.T) {
	mux, dir := newTestAPI(t)
	seedDIET(t, dir, "bad", overAllocFixture())

	rec := do(t, mux, http.MethodGet, "/api/plans/bad/wiring/soc-storage-scale-out", nil)
	assertJSONError(t, rec, http.StatusUnprocessableEntity)
}

// structural ingest failure (unparseable plan) → exactly 422 with a structured
// JSON error (NOT validation-as-data) (devb GREEN finding 1 — pin the status).
func TestAPI_F7b_StructuralFailure_422(t *testing.T) {
	mux, dir := newTestAPI(t)
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte("not: : valid: yaml\n  - ["), 0o644); err != nil {
		t.Fatalf("seed broken: %v", err)
	}
	rec := do(t, mux, http.MethodPost, "/api/plans/broken/calc", nil)
	assertJSONError(t, rec, http.StatusUnprocessableEntity)
}
