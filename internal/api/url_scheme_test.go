package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestNonHTTPSchemes covers non-http(s) targets like tcp:// and openapp://:
// they are valid destinations (302 redirects work in browsers/OS handlers)
// but never enqueue a title fetch.
func TestNonHTTPSchemes(t *testing.T) {
	s, _ := newTestServer(t)
	for _, url := range []string{
		"tcp://192.168.1.10:8080",
		"openapp://com.example.app/page",
		"mailto:user@example.com",
		"ftp://files.example.com/pub",
	} {
		rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
			"url": url, "code": "s" + url[len(url)-3:],
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s status = %d body %s", url, rec.Code, rec.Body.String())
		}
	}

	// Update an existing link to a non-http target works too.
	rec := do(t, s, http.MethodPatch, "/api/v1/links/spub", map[string]any{
		"url": "openapp://com.example.other/route",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body %s", rec.Code, rec.Body.String())
	}

	// Scheme-less and empty targets stay rejected.
	for _, url := range []string{"abc", ":x", ""} {
		rec = do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
			"url": url, "code": "bad",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create %q status = %d, want 400", url, rec.Code)
		}
	}

	// Batch import: mixed items — valid scheme creates, scheme-less fails alone.
	rec = do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{"url": "openapp://com.example.b/page", "code": "bx"},
			{"url": "not-a-url", "code": "by"},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("batch status = %d body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Results []struct {
			Status    string `json:"status"`
			ErrorCode string `json:"error_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("results = %d", len(resp.Results))
	}
	if r := resp.Results[0]; r.Status != "created" {
		t.Fatalf("item 0 = %+v, want created", r)
	}
	if r := resp.Results[1]; r.Status != "error" || r.ErrorCode != "invalid_request" {
		t.Fatalf("item 1 = %+v, want error invalid_request", r)
	}
}

// TestFetchableScheme pins down which targets may enter the meta queue.
func TestFetchableScheme(t *testing.T) {
	for raw, want := range map[string]bool{
		"http://example.com/x":  true,
		"https://example.com/x": true,
		"tcp://192.168.1.1:9":   false,
		"openapp://com.app/x":   false,
		"mailto:user@ex.com":    false,
		"not-a-url":             false,
	} {
		if got := fetchableScheme(raw); got != want {
			t.Errorf("fetchableScheme(%q) = %v, want %v", raw, got, want)
		}
	}
}
