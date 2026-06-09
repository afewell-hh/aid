package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/afewell-hh/aid/internal/planstore"
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

// notImplemented is the RED-phase handler body (Stage A GREEN fills these in).
func (a *api) notImplemented(w http.ResponseWriter, _ *http.Request) {
	writeJSONError(w, http.StatusNotImplemented, "not implemented (Stage A GREEN)")
}

// newServeMux builds the REST router over the plan store. Manual path dispatch
// (Go 1.21 has no ServeMux wildcard patterns; Go-version pressure tracked in
// #43). All handler bodies are RED stubs until Stage A GREEN.
func newServeMux(store *planstore.Store) http.Handler {
	a := &api{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/plans", a.routePlans)  // collection: GET list, POST create
	mux.HandleFunc("/api/plans/", a.routePlanID) // item + sub-resources
	return mux
}

// routePlans dispatches the collection routes (/api/plans).
func (a *api) routePlans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.notImplemented(w, r) // listPlans
	case http.MethodPost:
		a.notImplemented(w, r) // createPlan
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
	// segs[0] = id; optional segs[1] = sub-resource; segs[2] = fabric.

	switch {
	case len(segs) == 1: // /api/plans/{id}
		switch r.Method {
		case http.MethodGet:
			a.notImplemented(w, r) // getPlan
		case http.MethodPut:
			a.notImplemented(w, r) // updatePlan
		case http.MethodDelete:
			a.notImplemented(w, r) // deletePlan
		default:
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case len(segs) == 2 && segs[1] == "calc": // POST /api/plans/{id}/calc
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		a.notImplemented(w, r) // calcPlan
	case len(segs) == 2 && segs[1] == "bom": // GET /api/plans/{id}/bom
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		a.notImplemented(w, r) // bomPlan
	case len(segs) == 3 && segs[1] == "wiring": // GET /api/plans/{id}/wiring/{fabric}
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		a.notImplemented(w, r) // wiringPlan
	default:
		writeJSONError(w, http.StatusNotFound, "not found")
	}
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
