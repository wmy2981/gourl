package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wmy2981/gourl/internal/counter"
)

func createLink(t *testing.T, s *Server, code, url string) {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{"url": url, "code": code})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %s: status %d, body %s", code, rec.Code, rec.Body.String())
	}
}

func get(t *testing.T, s *Server, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestRedirect302(t *testing.T) {
	s, _ := newTestServer(t)
	createLink(t, s, "abc", "https://example.com/target")
	rec := get(t, s, "/abc", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://example.com/target" {
		t.Errorf("Location = %q", loc)
	}
}

func TestRedirectMultiLevel(t *testing.T) {
	s, _ := newTestServer(t)
	createLink(t, s, "link1/link2", "https://example.com/deep")
	rec := get(t, s, "/link1/link2", nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://example.com/deep" {
		t.Errorf("multi-level redirect: status %d, location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRedirectCountsClick(t *testing.T) {
	s, mr := newTestServer(t)
	createLink(t, s, "abc", "https://example.com/target")
	for i := 0; i < 3; i++ {
		get(t, s, "/abc", nil)
	}
	total, daily := counter.Keys("abc", counter.Date(time.Unix(s.now(), 0)))
	if got, err := mr.Get(total); err != nil || got != "3" {
		t.Errorf("total counter = %q (err %v), want 3", got, err)
	}
	if got, err := mr.Get(daily); err != nil || got != "3" {
		t.Errorf("daily counter = %q (err %v), want 3", got, err)
	}
}

func TestRedirectUABlockedNotCounted(t *testing.T) {
	s, mr := newTestServer(t)
	cfg := s.cfg.Get()
	cfg.UABlocks = []string{"Googlebot"}
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatal(err)
	}
	createLink(t, s, "abc", "https://example.com/target")

	rec := get(t, s, "/abc", map[string]string{"User-Agent": "Mozilla/5.0 (compatible; Googlebot/2.1)"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("blocked UA status = %d, want 403", rec.Code)
	}
	total, _ := counter.Keys("abc", "")
	if _, err := mr.Get(total); err == nil {
		t.Error("blocked UA must not be counted")
	}

	// A normal UA still works.
	rec = get(t, s, "/abc", map[string]string{"User-Agent": "Mozilla/5.0 Chrome/126"})
	if rec.Code != http.StatusFound {
		t.Errorf("normal UA status = %d, want 302", rec.Code)
	}
	if got, _ := mr.Get(total); got != "1" {
		t.Errorf("normal UA count = %q, want 1", got)
	}
}

func TestRedirectExpiredShowsBilingualPage(t *testing.T) {
	s, _ := newTestServer(t)
	createLink(t, s, "abc", "https://example.com/target")
	rec := do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{"expires_at": s.now() - 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("set expiry: %d", rec.Code)
	}

	rec = get(t, s, "/abc", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expired status = %d, want 200 page", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "This link has expired") {
		t.Errorf("en page missing heading: %s", rec.Body.String())
	}

	rec = get(t, s, "/abc?lang=zh", nil)
	if !strings.Contains(rec.Body.String(), "链接已过期") {
		t.Errorf("zh page missing heading: %s", rec.Body.String())
	}
}

func TestRedirectNotFound(t *testing.T) {
	s, _ := newTestServer(t)
	rec := get(t, s, "/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Page not found") {
		t.Errorf("404 page missing heading: %s", rec.Body.String())
	}
}

func TestRedirectReservedPrefixWins(t *testing.T) {
	s, _ := newTestServer(t)
	for _, path := range []string{"/api/anything", "/expired", "/health"} {
		rec := get(t, s, path, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404 (reserved prefix)", path, rec.Code)
		}
	}
	// /admin serves the SPA shell instead of being shadowed by short codes.
	rec := get(t, s, "/admin/x", nil)
	if rec.Code != http.StatusOK || !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Errorf("/admin/x status = %d, want SPA index", rec.Code)
	}
}

func TestRootRedirectsToAdmin(t *testing.T) {
	s, _ := newTestServer(t)
	rec := get(t, s, "/", nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/admin" {
		t.Errorf("root: status %d, location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRedirectKeepsWorkingWhenRedisDown(t *testing.T) {
	s, mr := newTestServer(t)
	createLink(t, s, "abc", "https://example.com/target")

	mr.Close() // simulate a Redis outage
	rec := get(t, s, "/abc", nil)
	if rec.Code != http.StatusFound {
		t.Errorf("redirect with redis down: status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://example.com/target" {
		t.Errorf("Location = %q", loc)
	}
}
