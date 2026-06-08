package main

import (
	"net/http"

	"github.com/spf13/cobra"
)

// Phase-6b REST endpoints. Documented here; stubbed (501) until Phase 6b.
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

// newServeMux returns the stub API mux: every documented route returns 501.
func newServeMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented (Phase 6b)", http.StatusNotImplemented)
	})
	return mux
}

func newServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the REST API server (Phase-6b stub: documented endpoints return 501)",
		RunE:  func(cmd *cobra.Command, args []string) error { return errNotImplemented },
	}
	cmd.Flags().IntVar(&port, "port", 8080, "listen port")
	return cmd
}
