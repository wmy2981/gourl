package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSelfLinkTarget covers the "no link may point at this instance's own
// short links" rule: same host:port as a configured base URL plus a first
// path segment outside the reserved codes is rejected on create, update and
// batch import, while reserved paths (admin pages) and foreign hosts pass.
func TestSelfLinkTarget(t *testing.T) {
	s, _ := newTestServer(t)
	cfg := s.cfg.Get()
	cfg.BaseURL = "http://base.example"
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatalf("config: %v", err)
	}

	n := 0
	create := func(url string) *httptest.ResponseRecorder {
		n++
		return do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
			"url": url, "code": fmt.Sprintf("ok%d", n),
		})
	}

	// Same host, non-reserved path → rejected.
	rec := create("http://base.example/abc")
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "self_link_target" {
		t.Fatalf("self target status = %d code %q body %s", rec.Code, decodeError(t, rec), rec.Body.String())
	}

	// Same host with the default port written out → still rejected.
	rec = create("http://base.example:80/abc")
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "self_link_target" {
		t.Fatalf("default-port self target status = %d code %q", rec.Code, decodeError(t, rec))
	}

	// Reserved paths (the admin pages) and the bare root are fine.
	for _, url := range []string{"http://base.example/admin", "http://base.example/docs", "http://base.example/"} {
		rec = create(url)
		if rec.Code != http.StatusCreated {
			t.Fatalf("reserved/root target %s status = %d body %s", url, rec.Code, rec.Body.String())
		}
	}

	// Foreign hosts pass, including a different port on the same hostname.
	for _, url := range []string{"http://other.example/abc", "http://base.example:9999/abc", "https://base.example/abc"} {
		rec = create(url)
		if rec.Code != http.StatusCreated {
			t.Fatalf("foreign target %s status = %d body %s", url, rec.Code, rec.Body.String())
		}
	}

	// Update to a self target is rejected too (ok3 = first successful create).
	rec = do(t, s, http.MethodPatch, "/api/v1/links/ok3", map[string]any{
		"url": "http://base.example/self",
	})
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "self_link_target" {
		t.Fatalf("update status = %d code %q", rec.Code, decodeError(t, rec))
	}

	// Batch import: the offending item fails alone, the valid one imports.
	rec = do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{"url": "http://base.example/self", "code": "bx"},
			{"url": "http://other.example/ok", "code": "by"},
		},
	})
	var resp struct {
		Results []struct {
			Status    string `json:"status"`
			ErrorCode string `json:"error_code"`
		} `json:"results"`
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("batch status = %d body %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("results = %d", len(resp.Results))
	}
	if r := resp.Results[0]; r.Status != "error" || r.ErrorCode != "self_link_target" {
		t.Fatalf("item 0 = %+v, want error self_link_target", r)
	}
	if r := resp.Results[1]; r.Status != "created" {
		t.Fatalf("item 1 = %+v, want created", r)
	}
}

// TestSelfLinkTargetInferredBase covers the base_url-empty case: the request
// host stands in for the base URL, so local targets are rejected.
func TestSelfLinkTargetInferredBase(t *testing.T) {
	s, _ := newTestServer(t) // BaseURL stays empty
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "http://example.com/local", "code": "loc",
	})
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "self_link_target" {
		t.Fatalf("inferred self target status = %d code %q body %s", rec.Code, decodeError(t, rec), rec.Body.String())
	}
	rec = do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "http://example.com/admin", "code": "adm",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("inferred reserved target status = %d body %s", rec.Code, rec.Body.String())
	}
}

// TestSelfLinkTargetExtraBase extends the matching to extra base URLs.
func TestSelfLinkTargetExtraBase(t *testing.T) {
	s, _ := newTestServer(t)
	cfg := s.cfg.Get()
	cfg.BaseURL = "http://base.example"
	cfg.ExtraBaseURLs = []string{"http://extra.example:8081"}
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatalf("config: %v", err)
	}
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "http://extra.example:8081/x", "code": "ex",
	})
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "self_link_target" {
		t.Fatalf("extra base target status = %d code %q", rec.Code, decodeError(t, rec))
	}
	// The same host on a different port is not this instance.
	rec = do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "http://extra.example:9999/x", "code": "ex2",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("foreign port status = %d body %s", rec.Code, rec.Body.String())
	}
}
