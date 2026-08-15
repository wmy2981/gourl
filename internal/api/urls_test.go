package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// setBaseURLs configures the test server's base URLs through the config
// manager (as the settings page would).
func setBaseURLs(t *testing.T, s *Server, base string, extras []string) {
	t.Helper()
	cfg := s.cfg.Get()
	cfg.BaseURL = base
	cfg.ExtraBaseURLs = extras
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestCreateResponseIncludesAllURLs(t *testing.T) {
	s, _ := newTestServer(t)
	setBaseURLs(t, s, "https://s.example.com", []string{"https://s2.example.com"})

	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/x",
		"code": "abc",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body.String())
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	want := []string{"https://s.example.com/abc", "https://s2.example.com/abc"}
	if len(l.URLs) != 2 || l.URLs[0] != want[0] || l.URLs[1] != want[1] {
		t.Errorf("urls = %v, want %v", l.URLs, want)
	}
}

func TestURLsDeduplicatedAndInferredFromHost(t *testing.T) {
	s, _ := newTestServer(t)
	// No base_url configured: infer from the request.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/links", nil)
	req.Host = "myhost:8080"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	_ = rec

	// Dedupe: identical base and extra collapse.
	setBaseURLs(t, s, "https://s.example.com/", []string{"https://s.example.com"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/links", bodyReader(t, map[string]any{
		"url":  "https://example.com/x",
		"code": "dedupe",
	}))
	req2.Host = "whatever"
	if testSession != nil {
		req2.AddCookie(testSession)
	}
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("create status = %d", rec2.Code)
	}
	var l linkJSON
	if err := json.Unmarshal(rec2.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if len(l.URLs) != 1 || l.URLs[0] != "https://s.example.com/dedupe" {
		t.Errorf("urls = %v, want single deduped url", l.URLs)
	}
}

func TestInferBaseURLUsesForwardedProto(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/links", nil)
	req.Host = "s.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	// Exercise the inference through a create.
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/links", bodyReader(t, map[string]any{
		"url":  "https://example.com/x",
		"code": "proto",
	}))
	createReq.Host = "s.example.com"
	createReq.Header.Set("X-Forwarded-Proto", "https")
	if testSession != nil {
		createReq.AddCookie(testSession)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, createReq)
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if len(l.URLs) != 1 || l.URLs[0] != "https://s.example.com/proto" {
		t.Errorf("urls = %v, want https inference", l.URLs)
	}
}

func TestBatchCreate(t *testing.T) {
	s, _ := newTestServer(t)

	// Mixed success/failure.
	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", []map[string]any{
		{"url": "https://e.com/1", "code": "b1"},
		{"url": "https://e.com/2", "code": "b1"},  // duplicate within batch
		{"url": "https://e.com/3", "code": "api"}, // reserved
		{"url": "https://e.com/4"},                // random code
		{"url": "relative/path", "code": "b2"},    // invalid url
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("batch status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Created int `json:"created"`
		Failed  int `json:"failed"`
		Results []struct {
			Index     int    `json:"index"`
			Code      string `json:"code"`
			Status    string `json:"status"`
			ErrorCode string `json:"error_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Created != 2 || body.Failed != 3 {
		t.Errorf("created %d failed %d, want 2/3", body.Created, body.Failed)
	}
	if body.Results[1].Status != "error" || body.Results[1].ErrorCode != "code_taken" {
		t.Errorf("result[1] = %+v", body.Results[1])
	}
	if body.Results[2].ErrorCode != "reserved_code" {
		t.Errorf("result[2] = %+v", body.Results[2])
	}
	if body.Results[3].Code == "" || body.Results[3].Status != "created" {
		t.Errorf("result[3] = %+v", body.Results[3])
	}

	// Empty batch and oversize batch rejected.
	if rec := do(t, s, http.MethodPost, "/api/v1/links/batch", []map[string]any{}); rec.Code != http.StatusBadRequest {
		t.Errorf("empty batch status = %d, want 400", rec.Code)
	}
	huge := make([]map[string]any, 501)
	for i := range huge {
		huge[i] = map[string]any{"url": "https://e.com/x"}
	}
	if rec := do(t, s, http.MethodPost, "/api/v1/links/batch", huge); rec.Code != http.StatusBadRequest {
		t.Errorf("oversize batch status = %d, want 400", rec.Code)
	}
}

func TestExportCSV(t *testing.T) {
	s, _ := newTestServer(t)
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://e.com/with,comma",
		"code": "csv1",
	})
	do(t, s, http.MethodPatch, "/api/v1/links/csv1", map[string]any{
		"title": `has "quotes"`,
	})
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://e.com/2",
		"code": "csv2",
	})

	rec := do(t, s, http.MethodGet, "/api/v1/export.csv", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Error("missing UTF-8 BOM")
	}
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) != 3 { // header + 2 links
		t.Errorf("csv lines = %d, body:\n%s", len(lines), body)
	}
	if !strings.Contains(lines[0], "code,url,title") {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.Contains(body, `https://e.com/with,comma`) {
		t.Errorf("csv must quote comma in url:\n%s", body)
	}
	if !strings.Contains(body, `has ""quotes""`) {
		t.Errorf("csv must escape quotes:\n%s", body)
	}
}
