package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wmy2981/gourl/internal/store"
)

func TestTokenLifecycle(t *testing.T) {
	s, _ := newTestServer(t)

	// Generate.
	rec := do(t, s, http.MethodPost, "/api/v1/tokens", map[string]any{"note": "ci"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create token status = %d, body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Token) != 64 {
		t.Errorf("token length = %d, want 64 hex chars", len(created.Token))
	}

	// List shows only a preview.
	rec = do(t, s, http.MethodGet, "/api/v1/tokens", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list tokens status = %d", rec.Code)
	}
	var listed struct {
		Tokens []struct {
			ID    int64  `json:"id"`
			Token string `json:"token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Tokens) != 1 || strings.Contains(listed.Tokens[0].Token, created.Token) {
		t.Errorf("list must not expose the full token: %+v", listed.Tokens)
	}

	// Revoke.
	rec = do(t, s, http.MethodDelete, "/api/v1/tokens/"+itoa(created.ID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete token status = %d", rec.Code)
	}
	if rec := do(t, s, http.MethodGet, "/api/v1/tokens", nil); strings.Contains(rec.Body.String(), created.Token[:8]) {
		t.Error("revoked token still listed")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestUABlockManagement(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodPost, "/api/v1/ua-blocks", map[string]any{"pattern": "Googlebot"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create ua block status = %d", rec.Code)
	}
	// Duplicate → 409.
	if rec := do(t, s, http.MethodPost, "/api/v1/ua-blocks", map[string]any{"pattern": "Googlebot"}); rec.Code != http.StatusConflict {
		t.Errorf("duplicate ua block status = %d, want 409", rec.Code)
	}
	// Empty → 400.
	if rec := do(t, s, http.MethodPost, "/api/v1/ua-blocks", map[string]any{"pattern": "  "}); rec.Code != http.StatusBadRequest {
		t.Errorf("empty ua block status = %d, want 400", rec.Code)
	}

	rec = do(t, s, http.MethodGet, "/api/v1/ua-blocks", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Googlebot") {
		t.Fatalf("list ua blocks: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Blocks []struct {
			ID int64 `json:"id"`
		} `json:"ua_blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}

	if rec := do(t, s, http.MethodDelete, "/api/v1/ua-blocks/"+itoa(listed.Blocks[0].ID), nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete ua block status = %d", rec.Code)
	}
}

// TestUpdateConfigSetsUABlocks: ua_blocks is a regular config field managed
// from the settings page — a PUT replaces the whole list, and a PUT without
// the field clears it.
func TestUpdateConfigSetsUABlocks(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodPut, "/api/v1/config", map[string]any{
		"site": map[string]any{
			"name": "UA Test", "title": "T", "keywords": "", "description": "",
			"header": "", "footer": "",
		},
		"short_code_length": 4,
		"ua_blocks":         []string{"Googlebot", "Bingbot"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put config status = %d, body %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodGet, "/api/v1/ua-blocks", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Googlebot") {
		t.Fatalf("ua blocks after PUT: %d %s", rec.Code, rec.Body.String())
	}

	// A second PUT without ua_blocks clears the list (full-replace semantics).
	rec = do(t, s, http.MethodPut, "/api/v1/config", map[string]any{
		"site": map[string]any{
			"name": "UA Test", "title": "T", "keywords": "", "description": "",
			"header": "", "footer": "",
		},
		"short_code_length": 4,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("second put config status = %d", rec.Code)
	}
	rec = do(t, s, http.MethodGet, "/api/v1/ua-blocks", nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "Googlebot") {
		t.Fatalf("ua blocks after clearing PUT: %d %s", rec.Code, rec.Body.String())
	}
}

func TestConfigGetAndUpdate(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodGet, "/api/v1/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get config status = %d", rec.Code)
	}

	// Valid update: hot-swaps and persists to the YAML file.
	rec = do(t, s, http.MethodPut, "/api/v1/config", map[string]any{
		"site": map[string]any{
			"name": "Renamed", "title": "T", "keywords": "", "description": "",
			"header": "", "footer": "",
		},
		"short_code_length": 8,
		"base_url":          "https://s.example.com",
		"extra_base_urls":   []string{"https://s2.example.com"},
		"reserved_codes":    []string{"ops"},
		"icon":              "",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put config status = %d, body %s", rec.Code, rec.Body.String())
	}
	cfg := s.cfg.Get()
	if cfg.Site.Name != "Renamed" || cfg.ShortCodeLength != 8 || cfg.BaseURL != "https://s.example.com" {
		t.Errorf("config not hot-swapped: %+v", cfg)
	}

	// Invalid update rejected, old config intact.
	rec = do(t, s, http.MethodPut, "/api/v1/config", map[string]any{
		"site":             map[string]any{"name": "X"},
		"short_code_length": 99,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid config status = %d, want 400", rec.Code)
	}
	if s.cfg.Get().ShortCodeLength != 8 {
		t.Error("rejected update changed the config")
	}

	// New random codes honor the new length.
	rec = do(t, s, http.MethodPost, "/api/v1/links", map[string]any{"url": "https://e.com/x"})
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if len(l.Code) != 8 {
		t.Errorf("random code length = %d, want 8", len(l.Code))
	}
}

func uploadIconRequest(t *testing.T, s *Server, filename, content string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("icon", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/icon", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if testSession != nil {
		req.AddCookie(testSession)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestIconUploadAndDelete(t *testing.T) {
	s, _ := newTestServer(t)

	rec := uploadIconRequest(t, s, "logo.svg", `<svg xmlns="http://www.w3.org/2000/svg"><circle r="10"/></svg>`)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload svg status = %d, body %s", rec.Code, rec.Body.String())
	}
	if got := s.cfg.Get().Icon; got != "custom-icon.svg" {
		t.Errorf("config icon = %q, want custom-icon.svg", got)
	}
	stored := filepath.Join(s.assetsDir, "custom-icon.svg")
	if _, err := os.Stat(stored); err != nil {
		t.Errorf("stored icon missing: %v", err)
	}
	// The file is served under /assets/.
	if rec := get(t, s, "/assets/custom-icon.svg", nil); rec.Code != http.StatusOK {
		t.Errorf("serve icon status = %d", rec.Code)
	}

	// PNG replaces the SVG.
	rec = uploadIconRequest(t, s, "logo.png", "\x89PNG fake")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload png status = %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(s.assetsDir, "custom-icon.svg")); err == nil {
		t.Error("stale svg icon not removed")
	}

	// Disallowed types rejected.
	if rec := uploadIconRequest(t, s, "evil.exe", "MZ"); rec.Code != http.StatusBadRequest {
		t.Errorf("exe upload status = %d, want 400", rec.Code)
	}
	if rec := uploadIconRequest(t, s, "big.png", strings.Repeat("x", (1<<20)+1)); rec.Code != http.StatusBadRequest {
		t.Errorf("oversize upload status = %d, want 400", rec.Code)
	}

	// Delete restores default.
	if rec := do(t, s, http.MethodDelete, "/api/v1/icon", nil); rec.Code != http.StatusOK {
		t.Fatalf("delete icon status = %d", rec.Code)
	}
	if s.cfg.Get().Icon != "" {
		t.Errorf("config icon after delete = %q", s.cfg.Get().Icon)
	}
}

func TestDashboardAggregates(t *testing.T) {
	s, _ := newTestServer(t)
	ctx := context.Background()

	// Two links; seed daily clicks directly in the store. Dates are inside
	// the 14-day window around now (2023-11-14) in any timezone.
	createLink(t, s, "aaa", "https://e.com/a")
	createLink(t, s, "bbb", "https://e.com/b")
	now := s.now()
	if err := s.store.ApplyCounts(ctx,
		map[string]int64{"aaa": 10, "bbb": 5},
		[]store.DailyCount{{Code: "aaa", Date: "2023-11-13", Count: 10}, {Code: "bbb", Date: "2023-11-12", Count: 5}},
		now); err != nil {
		t.Fatal(err)
	}

	rec := do(t, s, http.MethodGet, "/api/v1/dashboard", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d", rec.Code)
	}
	var body struct {
		LinksTotal  int64 `json:"links_total"`
		ClicksTotal int64 `json:"clicks_total"`
		Daily       []struct {
			Date  string `json:"date"`
			Count int64  `json:"count"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.LinksTotal != 2 || body.ClicksTotal != 15 {
		t.Errorf("totals: links %d clicks %d", body.LinksTotal, body.ClicksTotal)
	}
	if len(body.Daily) < 1 {
		t.Fatalf("daily = %+v, want at least 1 entry", body.Daily)
	}
	for _, d := range body.Daily {
		if d.Date == "2023-11-13" && d.Count != 10 {
			t.Errorf("2023-11-13 count = %d, want 10", d.Count)
		}
	}
}
