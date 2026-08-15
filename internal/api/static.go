package api

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wmy2981/gourl/internal/webui"
)

// favicon serves the configured custom icon if uploaded, else the built-in
// default brand icon.
func (s *Server) favicon(w http.ResponseWriter, r *http.Request) {
	if cfg := s.cfg.Get(); cfg.Icon != "" {
		if f, err := os.Open(filepath.Join(s.assetsDir, cfg.Icon)); err == nil {
			defer f.Close()
			if info, err := f.Stat(); err == nil {
				http.ServeContent(w, r, cfg.Icon, info.ModTime(), f)
				return
			}
		}
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(webui.DefaultIcon())
}

// spaIndex serves the SPA shell for /admin and every nested admin route.
func (s *Server) spaIndex(w http.ResponseWriter, r *http.Request) {
	index, err := webui.Dist().Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer index.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.Copy(w, index)
}

// assetsHandler serves uploaded custom icons first (single files in the
// assets data dir, e.g. custom-icon.svg), falling back to the embedded
// frontend build artifacts under /assets/. The two name spaces never
// collide: uploaded names are fixed, build artifacts are hashed.
func (s *Server) assetsHandler() http.Handler {
	// The embedded FS has the build output at its root (index.html,
	// assets/...); serve the assets/ subtree from it.
	assetsFS, err := fs.Sub(webui.Dist(), "assets")
	if err != nil {
		panic(err) // static embed layout; build-time invariant
	}
	embedded := http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/assets/")
		if name == "" {
			http.NotFound(w, r)
			return
		}
		// Uploaded icons are flat files: reject any path separators.
		if !strings.ContainsAny(name, `/\`) {
			path := filepath.Join(s.assetsDir, name)
			if f, err := os.Open(path); err == nil {
				f.Close()
				http.ServeFile(w, r, path)
				return
			}
		}
		embedded.ServeHTTP(w, r)
	})
}
