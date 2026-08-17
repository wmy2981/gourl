package api

import (
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"github.com/wmy2981/gourl/internal/webui"
)

// TestSPAAssetsAreServed verifies the embedded build artifacts are reachable
// through /assets/, which the admin SPA depends on to boot.
func TestSPAAssetsAreServed(t *testing.T) {
	s, _ := newTestServer(t)
	dist := webui.Dist()

	// The SPA shell is served at /admin.
	rec := get(t, s, "/admin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/admin status = %d, want 200", rec.Code)
	}

	// Every hashed asset in the embed must resolve over HTTP.
	var found int
	fs.WalkDir(dist, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rec := get(t, s, "/assets/"+strings.TrimPrefix(path, "assets/"), nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /assets/%s = %d, want 200", path, rec.Code)
		}
		found++
		return nil
	})
	if found == 0 {
		t.Fatal("embed contains no assets to serve")
	}
}

// TestFaviconServesDefaultAndCustom confirms /favicon.svg serves the
// built-in icon, switching to the uploaded one once configured.
func TestFaviconServesDefaultAndCustom(t *testing.T) {
	s, _ := newTestServer(t)
	rec := get(t, s, "/favicon.svg", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<svg") {
		t.Fatalf("default favicon: status %d body %q", rec.Code, rec.Body.String())
	}

	// After an upload the custom icon wins.
	rec = uploadIconRequest(t, s, "custom-icon.svg", "<svg xmlns='http://www.w3.org/2000/svg'><text>custom</text></svg>")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d", rec.Code)
	}
	rec = get(t, s, "/favicon.svg", nil)
	if !strings.Contains(rec.Body.String(), "custom") {
		t.Errorf("custom favicon not served: %q", rec.Body.String())
	}
}

// TestSwaggerUIServed verifies the API documentation page and its spec.
func TestSwaggerUIServed(t *testing.T) {
	s, _ := newTestServer(t)
	rec := get(t, s, "/docs/", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "swagger-ui") {
		t.Fatalf("/docs/ status = %d, want swagger ui html", rec.Code)
	}
	rec = get(t, s, "/docs/openapi.yaml", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "openapi: 3.0") {
		t.Fatalf("/docs/openapi.yaml status = %d, want spec", rec.Code)
	}
	rec = get(t, s, "/docs/swagger-ui-bundle.js", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("swagger bundle status = %d, want 200", rec.Code)
	}
}

// TestSPAIndexCarriesSiteMeta verifies the site description and keywords are
// injected into the SPA shell head (every admin page shares this shell),
// with values escaped for the HTML attribute context.
func TestSPAIndexCarriesSiteMeta(t *testing.T) {
	s, _ := newTestServer(t)
	cfg := s.cfg.Get()
	cfg.Site.Description = `Short links "service"`
	cfg.Site.Keywords = "gourl, links"
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatal(err)
	}
	rec := get(t, s, "/admin", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `<meta name="description" content="Short links &#34;service&#34;">`) {
		t.Errorf("description meta missing from SPA shell: %q", body)
	}
	if !strings.Contains(body, `<meta name="keywords" content="gourl, links">`) {
		t.Errorf("keywords meta missing from SPA shell: %q", body)
	}
}

// TestSPAIndexSkipsEmptyMeta: unset description/keywords produce no meta tags.
func TestSPAIndexSkipsEmptyMeta(t *testing.T) {
	s, _ := newTestServer(t)
	rec := get(t, s, "/admin", nil)
	body := rec.Body.String()
	if strings.Contains(body, `name="description"`) || strings.Contains(body, `name="keywords"`) {
		t.Errorf("empty site meta must not be injected: %q", body)
	}
}

// TestNotFoundPageCarriesSiteMeta: the public 404 page carries both meta tags
// too (it shares the site description and keywords with the SPA shell).
func TestNotFoundPageCarriesSiteMeta(t *testing.T) {
	s, _ := newTestServer(t)
	cfg := s.cfg.Get()
	cfg.Site.Description = "Short links service"
	cfg.Site.Keywords = "gourl, links"
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatal(err)
	}
	rec := get(t, s, "/missing-code-xyz", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing code status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<meta name="description" content="Short links service">`) {
		t.Errorf("404 page description meta missing: %q", body)
	}
	if !strings.Contains(body, `<meta name="keywords" content="gourl, links">`) {
		t.Errorf("404 page keywords meta missing: %q", body)
	}
}

// TestCustomIconTakesPrecedenceOverEmbedded confirms uploaded icons keep
// their /assets/custom-icon.* route.
func TestCustomIconTakesPrecedenceOverEmbedded(t *testing.T) {
	s, _ := newTestServer(t)
	rec := uploadIconRequest(t, s, "custom-icon.svg", "<svg xmlns='http://www.w3.org/2000/svg'/>")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload icon status = %d", rec.Code)
	}
	rec = get(t, s, "/assets/custom-icon.svg", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<svg") {
		t.Errorf("custom icon not served: status %d body %q", rec.Code, rec.Body.String())
	}
}
