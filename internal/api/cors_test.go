package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Capacitor app talks to a remote instance cross-origin with a Bearer
// token; the API must answer preflights and carry the allow-origin header.
func TestCORSPreflight(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/status", nil)
	req.Header.Set("Origin", "capacitor://localhost")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allow-origin = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("allow-headers missing (Authorization needed for the bearer token)")
	}
}

func TestCORSHeadersOnAPIRequests(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/auth/status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allow-origin = %q, want * on api responses", got)
	}
}

// Non-API routes (short links, the SPA) carry no CORS headers.
func TestCORSNotOnOtherRoutes(t *testing.T) {
	s, _ := newTestServer(t)
	for _, path := range []string{"/admin", "/some-link"} {
		rec := do(t, s, http.MethodGet, path, nil)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("GET %s: allow-origin = %q, want none", path, got)
		}
	}
}
