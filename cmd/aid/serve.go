package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/afewell-hh/aid/internal/orchestrate"
	"github.com/afewell-hh/aid/internal/plan"
	"github.com/afewell-hh/aid/internal/planstore"
	"github.com/afewell-hh/aid/ui"
	"github.com/spf13/cobra"
)

// Phase-6b REST endpoints (Stage A). The router maps these to handler methods.
var serveRoutes = []string{
	"GET /api/plans",
	"POST /api/plans",
	"GET /api/plans/{id}",
	"PUT /api/plans/{id}",
	"DELETE /api/plans/{id}",
	"POST /api/plans/{id}/calc",
	"GET /api/plans/{id}/bom",
	"GET /api/plans/{id}/wiring/{fabric}",
}

// api holds the handler dependencies (the plan store). Handlers reuse
// internal/orchestrate for calc/bom/wiring — no new topology behavior.
type api struct {
	store *planstore.Store
}

// writeJSON writes v as an indented JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// writeJSONError writes a structured JSON error: {"error": "..."}.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// fail maps a store error to a structured JSON error response.
func (a *api) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, planstore.ErrNotFound):
		writeJSONError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, planstore.ErrInvalidID), errors.Is(err, planstore.ErrInvalidPlan):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal error")
	}
}

// newServeMux builds the REST router over the plan store. Manual path dispatch
// (Go 1.21 has no ServeMux wildcard patterns; Go-version pressure tracked in
// #43).
func newServeMux(store *planstore.Store) http.Handler {
	a := &api{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/plans", a.routePlans)   // collection: GET list, POST create
	mux.HandleFunc("/api/plans/", a.routePlanID) // item + sub-resources
	mux.Handle("/", ui.Handler())                // embedded frontend (Bootstrap 5 + app.js)
	return mux
}

// routePlans dispatches the collection routes (/api/plans).
func (a *api) routePlans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listPlans(w, r)
	case http.MethodPost:
		a.createPlan(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// routePlanID dispatches item and sub-resource routes
// (/api/plans/{id}, /api/plans/{id}/calc, /api/plans/{id}/bom,
// /api/plans/{id}/wiring/{fabric}).
func (a *api) routePlanID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/plans/")
	segs := strings.Split(rest, "/")
	id := segs[0]
	// segs[0] = id; optional segs[1] = sub-resource; segs[2] = fabric.

	switch {
	case len(segs) == 1: // /api/plans/{id}
		switch r.Method {
		case http.MethodGet:
			a.getPlan(w, r, id)
		case http.MethodPut:
			a.updatePlan(w, r, id)
		case http.MethodDelete:
			a.deletePlan(w, r, id)
		default:
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case len(segs) == 2 && segs[1] == "calc": // POST /api/plans/{id}/calc
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		a.calcPlan(w, r, id)
	case len(segs) == 2 && segs[1] == "bom": // GET /api/plans/{id}/bom
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		a.bomPlan(w, r, id)
	case len(segs) == 3 && segs[1] == "wiring": // GET /api/plans/{id}/wiring/{fabric}
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		a.wiringPlan(w, r, id, segs[2])
	default:
		writeJSONError(w, http.StatusNotFound, "not found")
	}
}

// --- handlers ---------------------------------------------------------------

// listPlans: GET /api/plans → {"plans": [summaries]}.
func (a *api) listPlans(w http.ResponseWriter, _ *http.Request) {
	plans, err := a.store.List()
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
}

// createPlan: POST /api/plans (body = plan YAML) → 201 summary + Location.
func (a *api) createPlan(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	p, err := a.store.Create(body)
	if err != nil {
		a.fail(w, err)
		return
	}
	w.Header().Set("Location", "/api/plans/"+p.ID)
	writeJSON(w, http.StatusCreated, p)
}

// getPlan: GET /api/plans/{id} → 200 detail (incl. YAML).
func (a *api) getPlan(w http.ResponseWriter, _ *http.Request, id string) {
	p, err := a.store.Get(id)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// updatePlan: PUT /api/plans/{id} (body = plan YAML) → 200 summary.
func (a *api) updatePlan(w http.ResponseWriter, r *http.Request, id string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	p, err := a.store.Update(id, body)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// deletePlan: DELETE /api/plans/{id} → 204.
func (a *api) deletePlan(w http.ResponseWriter, _ *http.Request, id string) {
	if err := a.store.Delete(id); err != nil {
		a.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// calcPlan: POST /api/plans/{id}/calc → 200 {ir, validation}. Reuses
// orchestrate.Calculate; semantic validation is surfaced as DATA (is_valid may
// be false) with a 200 — never a 500.
func (a *api) calcPlan(w http.ResponseWriter, _ *http.Request, id string) {
	planJSON, ok := a.planJSON(w, id)
	if !ok {
		return
	}
	res, err := orchestrate.Calculate(planJSON)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if res.Ok == nil {
		// Kernel rejected the plan (undecodable/structurally invalid) — distinct
		// from a semantic validation failure (which returns Ok with is_valid:false).
		writeJSONError(w, http.StatusUnprocessableEntity, "kernel rejected plan: "+string(res.Err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ir":         res.Ok.IR,
		"validation": res.Ok.Validation,
	})
}

// bomPlan: GET /api/plans/{id}/bom → 200 BOM JSON (per-unit + fleet totals).
func (a *api) bomPlan(w http.ResponseWriter, _ *http.Request, id string) {
	planJSON, ok := a.planJSON(w, id)
	if !ok {
		return
	}
	out, err := orchestrate.ExportBOM(planJSON, "json")
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, "cannot compute BOM: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, out.Content)
}

// wiringPlan: GET /api/plans/{id}/wiring/{fabric} → 200 wiring YAML.
func (a *api) wiringPlan(w http.ResponseWriter, _ *http.Request, id, fabric string) {
	planJSON, ok := a.planJSON(w, id)
	if !ok {
		return
	}
	docs, err := orchestrate.ExportWiring(planJSON, fabric)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, "cannot export wiring: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml")
	w.WriteHeader(http.StatusOK)
	for i, d := range docs {
		if i > 0 {
			_, _ = io.WriteString(w, "---\n")
		}
		_, _ = io.WriteString(w, d.YAML)
		if !strings.HasSuffix(d.YAML, "\n") {
			_, _ = io.WriteString(w, "\n")
		}
	}
}

// planJSON loads the plan YAML for id and converts it to the kernel plan JSON.
// It writes the appropriate error response and returns ok=false on failure.
func (a *api) planJSON(w http.ResponseWriter, id string) ([]byte, bool) {
	yamlBytes, err := a.store.GetYAML(id)
	if err != nil {
		a.fail(w, err)
		return nil, false
	}
	planJSON, err := plan.YAMLToJSON(yamlBytes)
	if err != nil {
		writeJSONError(w, http.StatusUnprocessableEntity, "invalid plan YAML: "+err.Error())
		return nil, false
	}
	return planJSON, true
}

// defaultPlansDir is ~/.aid/plans (consistent with internal/state's ~/.aid).
func defaultPlansDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".aid", "plans")
	}
	return filepath.Join(home, ".aid", "plans")
}

func newServeCmd() *cobra.Command {
	var port int
	var plansDir string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the REST API server (plan CRUD + calc/bom/wiring over orchestrate)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := plansDir
			if dir == "" {
				dir = defaultPlansDir()
			}
			store, err := planstore.Open(dir)
			if err != nil {
				return err
			}
			addr := fmt.Sprintf(":%d", port)
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "aid serve listening on %s (plans: %s)\n", addr, dir)
			return http.ListenAndServe(addr, newServeMux(store))
		},
	}
	cmd.Flags().IntVar(&port, "port", 8080, "listen port")
	cmd.Flags().StringVar(&plansDir, "plans-dir", "", "plan store directory (default ~/.aid/plans)")
	return cmd
}
