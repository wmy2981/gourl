package api

import (
	"net/http"
	"testing"
)

// TestWebUIDisabledReturns404: `gourl webui off` hides the admin console
// (404 for /admin and its children) while /docs and the short-link routes
// keep working; re-enabling restores the SPA.
func TestWebUIDisabledReturns404(t *testing.T) {
	s, _ := newTestServer(t)

	upd := s.cfg.Get()
	upd.WebUIEnabled = false
	if err := s.cfg.Update(upd); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/admin", "/admin/login", "/admin/links"} {
		rec := do(t, s, http.MethodGet, path, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s with webui off = %d, want 404", path, rec.Code)
		}
	}
	// Swagger stays up.
	rec := do(t, s, http.MethodGet, "/docs/openapi.yaml", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /docs with webui off = %d, want 200", rec.Code)
	}

	upd = s.cfg.Get()
	upd.WebUIEnabled = true
	if err := s.cfg.Update(upd); err != nil {
		t.Fatal(err)
	}
	rec = do(t, s, http.MethodGet, "/admin", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /admin after re-enable = %d, want 200", rec.Code)
	}
}
