// Package ui carries the web frontend's static assets (Phase 6b Stage B),
// compiled into the aid binary (D4: single static binary; air-gapped — no CDN).
// The bundle is: vendored Bootstrap 5 (CSS + JS), the HTML shell (index.html),
// and the MoonBit->JS application bundle (app.js, produced by `make ui` from
// ui/src). Everything is served by `aid serve` from this embedded FS.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:static
var staticFS embed.FS

// minAppJS is the size of a real compiled app.js; a placeholder/empty bundle is
// far smaller. Used by Stale() / `make ui-check` (the #33-style freshness guard).
const minAppJS = 512

// Static returns the embedded static/ subtree as a filesystem.
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // embed path is a compile-time constant; this cannot fail
	}
	return sub
}

// Handler serves the embedded frontend: the HTML shell at "/" and the vendored
// Bootstrap 5 + compiled app.js under "/static/". All assets are served from the
// binary (air-gapped — no CDN).
func Handler() http.Handler {
	fsys := Static()
	files := http.FileServer(http.FS(fsys))
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", files))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	return mux
}

// AppJS returns the compiled MoonBit->JS bundle bytes (for the freshness guard).
func AppJS() ([]byte, error) {
	return staticFS.ReadFile("static/app.js")
}

// Stale reports whether the compiled app.js looks like a placeholder (i.e.
// `make ui` has not produced a real bundle).
func Stale() bool {
	b, err := AppJS()
	return err != nil || len(b) < minAppJS
}
