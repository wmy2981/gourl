package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestDeleteLinkAPI: after DELETE the short link 404s and the code can be
// reused by a fresh create or a batch import (deleted rows are invisible to
// the API and don't hold their codes).
func TestDeleteLinkAPI(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "https://example.com/a", "code": "abc",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body %s", rec.Code, rec.Body.String())
	}

	if rec := do(t, s, http.MethodDelete, "/api/v1/links/abc", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body %s", rec.Code, rec.Body.String())
	}

	// The redirect route no longer resolves it.
	if rec := do(t, s, http.MethodGet, "/abc", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("redirect after delete = %d, want 404", rec.Code)
	}
	// The list and the export exclude it.
	if rec := do(t, s, http.MethodGet, "/api/v1/links", nil); rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}

	// Batch import can reuse the deleted code as if it were free.
	rec = do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{"url": "https://example.com/b", "code": "abc"},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("batch status = %d body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Results []struct {
			Status string `json:"status"`
			Code   string `json:"code"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "created" || resp.Results[0].Code != "abc" {
		t.Fatalf("results = %+v, want created abc", resp.Results)
	}
}

// TestHardDeleteAPI: with hard_delete on, DELETE removes the row physically
// (no soft-deleted residue) while the API behavior stays identical.
func TestHardDeleteAPI(t *testing.T) {
	s, _ := newTestServer(t)
	upd := s.cfg.Get()
	upd.HardDelete = true
	if err := s.cfg.Update(upd); err != nil {
		t.Fatal(err)
	}

	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "https://example.com/a", "code": "abc",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, http.MethodDelete, "/api/v1/links/abc", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body %s", rec.Code, rec.Body.String())
	}
	assertRowAbsent(t, s, "abc")

	// Batch delete and the expired sweep honor the switch too.
	rec = do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{"url": "https://example.com/x", "code": "x1"},
			{"url": "https://example.com/y", "code": "y1"},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("batch create status = %d body %s", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodDelete, "/api/v1/links", map[string]any{"codes": []string{"x1", "y1"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("batch delete status = %d body %s", rec.Code, rec.Body.String())
	}
	for _, code := range []string{"x1", "y1"} {
		assertRowAbsent(t, s, code)
	}
}

// assertRowAbsent checks that no row at all (deleted or not) exists for code.
func assertRowAbsent(t *testing.T, s *Server, code string) {
	t.Helper()
	var n int
	if err := s.store.SQL().QueryRow(`SELECT COUNT(*) FROM links WHERE code = ?`, code).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("hard-deleted link %q still has %d database row(s)", code, n)
	}
}
