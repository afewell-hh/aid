package main

import (
	"fmt"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := fmt.Sprintf(":%d", port)
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "aid serve (Phase-6b stub) listening on %s\n", addr)
			fmt.Fprintln(out, "documented endpoints (all return 501 until Phase 6b):")
			for _, r := range serveRoutes {
				fmt.Fprintf(out, "  %s\n", r)
			}
			return http.ListenAndServe(addr, newServeMux())
		},
	}
	cmd.Flags().IntVar(&port, "port", 8080, "listen port")
	return cmd
}
