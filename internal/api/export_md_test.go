package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The markdown export shares the CSV/JSON link set; these tests pin the
// document shape: header metadata, the 7-column table and the cell escapes.
func TestExportMarkdown(t *testing.T) {
	s, _ := newTestServer(t)

	// One row exercising every escape path: a pipe in the title, a newline in
	// the description, never-expires. CreatedAt comes from the pinned test
	// clock (1700000000 → 2023/11/14 in server-local time).
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":         "https://example.com/page",
		"code":        "demo",
		"title":       "ti|tle",
		"description": "first line\nsecond line",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodGet, "/api/v1/export.md", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d, body %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/markdown") {
		t.Errorf("content-type = %q, want text/markdown", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	// Timestamp is server-local; the pinned clock renders differently across
	// TZ offsets (CI is UTC), so only pin the prefix shape.
	if !strings.HasPrefix(cd, `attachment; filename="gourl-links-2023-`) || !strings.HasSuffix(cd, `.md"`) {
		t.Errorf("content-disposition = %q, want gourl-links timestamp", cd)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"---\nsite: gourl\nversion: dev\ncount: 1\nexported_at: \"",
		"| code | url | title | description | click_count | expires_at | created_at |",
		"`demo`",
		"[https://example.com/page](https://example.com/page)",
		"ti\\|tle",
		"first line<br>second line",
		"| — |", // never-expires cell
	} {
		if !strings.Contains(body, want) {
			t.Errorf("export body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "ti|tle") || strings.Contains(body, "first line\nsecond") {
		t.Errorf("unescaped content leaked into the table:\n%s", body)
	}
}

func TestExportMarkdownEmpty(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/export.md", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d", rec.Code)
	}
	body := rec.Body.String()
	// Header still renders; only the table header rows follow (no data rows).
	if !strings.Contains(body, "count: 0") || !strings.Contains(body, "| code |") {
		t.Errorf("empty export body unexpected:\n%s", body)
	}
	if strings.Count(body, "\n|") > 2 { // header + separator only
		t.Errorf("empty export carries data rows:\n%s", body)
	}
}

func TestExportMarkdownRequiresAuth(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/export.md", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauth status = %d, want 401", rec.Code)
	}
}

// The JSON export wraps the 7-field rows in a meta object mirroring the
// markdown front matter; the CSV carries the same metadata as #-comment
// lines ahead of the header row.
func TestExportMetaInfo(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "https://example.com/meta", "code": "meta1",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d", rec.Code)
	}

	// JSON: {"meta": {site, version, count, exported_at}, "items": [...]}
	rec = do(t, s, http.MethodGet, "/api/v1/export.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("json export status = %d", rec.Code)
	}
	var dump struct {
		Meta  map[string]any `json:"meta"`
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dump); err != nil {
		t.Fatalf("decode json export: %v", err)
	}
	if dump.Meta["site"] != "gourl" || dump.Meta["version"] == "" || dump.Meta["exported_at"] == "" {
		t.Errorf("json meta incomplete: %+v", dump.Meta)
	}
	if len(dump.Items) != 1 || dump.Items[0].Code != "meta1" {
		t.Errorf("json items wrong: %+v", dump.Items)
	}

	// CSV: #-comment metadata lines, then the header row.
	rec = do(t, s, http.MethodGet, "/api/v1/export.csv", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("csv export status = %d", rec.Code)
	}
	lines := strings.Split(strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF"), "\n")
	if !strings.HasPrefix(lines[0], "# site: gourl") ||
		!strings.HasPrefix(lines[1], "# version:") ||
		!strings.HasPrefix(lines[2], "# count: 1") ||
		!strings.HasPrefix(lines[3], "# exported_at:") {
		t.Errorf("csv metadata lines wrong:\n%s", strings.Join(lines[:4], "\n"))
	}
	if lines[4] != "code,url,title,description,expires_at,click_count,created_at" {
		t.Errorf("csv header row wrong: %q", lines[4])
	}
}
