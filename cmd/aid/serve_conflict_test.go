package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/afewell-hh/aid/internal/planstore"
)

// TestAPI_CreatePlan_DuplicateID_Conflict is the #88 defense-in-depth contract at
// the API layer: POST /api/plans for an id that already exists must return a clear
// non-2xx conflict (the lead's preferred behavior is strict 409 Conflict) and must
// NOT overwrite the stored plan. PUT /api/plans/{id} stays the explicit update path.
//
// RED: createPlan → store.Create currently overwrites and returns 201 again, so
// the 409 assertion and the "first content survives" assertion both fail. GREEN
// maps the store's conflict error to 409 (via a.fail).
func TestAPI_CreatePlan_DuplicateID_Conflict(t *testing.T) {
	mux, _ := newTestAPI(t)

	first := []byte("id: dup\nname: First Plan\nstatus: draft\n")
	if rec := do(t, mux, http.MethodPost, "/api/plans", first); rec.Code != http.StatusCreated {
		t.Fatalf("first create: got %d want 201; body=%s", rec.Code, rec.Body.String())
	}

	// Second POST of the same id must be refused as a conflict (structured 409).
	second := []byte("id: dup\nname: Second Plan\nstatus: active\n")
	rec := do(t, mux, http.MethodPost, "/api/plans", second)
	assertJSONError(t, rec, http.StatusConflict)

	// The stored plan must be unchanged (the first content survives).
	g := do(t, mux, http.MethodGet, "/api/plans/dup", nil)
	if g.Code != http.StatusOK {
		t.Fatalf("get after duplicate create: got %d; body=%s", g.Code, g.Body.String())
	}
	var p planstore.Plan
	if err := json.Unmarshal(g.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if p.Name != "First Plan" || p.Status != "draft" {
		t.Errorf("duplicate POST overwrote the plan: got name=%q status=%q, want First Plan/draft", p.Name, p.Status)
	}
}

// TestAPI_UpdatePlan_RemainsSeparate is a green-stays-green guard: PUT /api/plans/
// {id} is the explicit overwrite path and must keep returning 200 + persisting the
// change after the create-duplicate guard lands. Passes today; must stay passing
// through GREEN (AC "existing update semantics via PUT remain unchanged").
func TestAPI_UpdatePlan_RemainsSeparate(t *testing.T) {
	mux, _ := newTestAPI(t)

	if rec := do(t, mux, http.MethodPost, "/api/plans", []byte("id: dup\nname: First Plan\nstatus: draft\n")); rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d want 201; body=%s", rec.Code, rec.Body.String())
	}

	rec := do(t, mux, http.MethodPut, "/api/plans/dup", []byte("id: dup\nname: Renamed\nstatus: active\n"))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT update: got %d want 200; body=%s", rec.Code, rec.Body.String())
	}

	g := do(t, mux, http.MethodGet, "/api/plans/dup", nil)
	var p planstore.Plan
	if err := json.Unmarshal(g.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if p.Name != "Renamed" || p.Status != "active" {
		t.Errorf("PUT did not update: got name=%q status=%q, want Renamed/active", p.Name, p.Status)
	}
}
