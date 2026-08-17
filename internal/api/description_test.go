package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestDescriptionLengthLimit covers the 500-character cap (counted in runes,
// so CJK text is allowed its full 500 characters) on create, update and batch
// import.
func TestDescriptionLengthLimit(t *testing.T) {
	s, _ := newTestServer(t)
	long := strings.Repeat("a", 501)
	exact := strings.Repeat("a", 500)
	cjk := strings.Repeat("中", 500) // 1500 bytes, 500 runes — must pass

	// Create: too long rejected with the stable error code.
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":         "https://example.com/a",
		"code":        "a",
		"description": long,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d body %s", rec.Code, rec.Body.String())
	}
	if got := decodeError(t, rec); got != "description_too_long" {
		t.Fatalf("create error code = %q, want description_too_long", got)
	}

	// Create: exactly 500 and 500 CJK runes both pass.
	for name, desc := range map[string]string{"exact": exact, "cjk": cjk} {
		rec = do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
			"url": "https://example.com/" + name, "code": name, "description": desc,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s status = %d body %s", name, rec.Code, rec.Body.String())
		}
	}

	// Update: too long rejected, valid value still accepted.
	rec = do(t, s, http.MethodPatch, "/api/v1/links/exact", map[string]any{
		"description": long,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("update status = %d body %s", rec.Code, rec.Body.String())
	}
	if got := decodeError(t, rec); got != "description_too_long" {
		t.Fatalf("update error code = %q, want description_too_long", got)
	}
	rec = do(t, s, http.MethodPatch, "/api/v1/links/exact", map[string]any{
		"description": "short",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body %s", rec.Code, rec.Body.String())
	}

	// Batch import: the offending item fails on its own, the valid one still
	// imports (the response keeps the request order).
	rec = do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{"url": "https://example.com/x", "code": "x", "description": long},
			{"url": "https://example.com/y", "code": "y", "description": exact},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("batch status = %d body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Results []struct {
			Index     int    `json:"index"`
			Status    string `json:"status"`
			ErrorCode string `json:"error_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode batch response: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(resp.Results))
	}
	if r := resp.Results[0]; r.Status != "error" || r.ErrorCode != "description_too_long" {
		t.Fatalf("item 0 = %+v, want error description_too_long", r)
	}
	if r := resp.Results[1]; r.Status != "created" {
		t.Fatalf("item 1 = %+v, want created", r)
	}
}
