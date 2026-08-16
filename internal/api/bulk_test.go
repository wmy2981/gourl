package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func createRaw(t *testing.T, s *Server, url, code string, expiresAt any) {
	t.Helper()
	body := map[string]any{"url": url, "code": code}
	if expiresAt != nil {
		body["expires_at"] = expiresAt
	}
	if rec := do(t, s, http.MethodPost, "/api/v1/links", body); rec.Code != http.StatusCreated {
		t.Fatalf("create %s: status %d, body %s", code, rec.Code, rec.Body.String())
	}
}

func TestBatchDeleteLinks(t *testing.T) {
	s, _ := newTestServer(t)
	for _, c := range []string{"a", "b", "c"} {
		createRaw(t, s, "https://example.com/"+c, c, nil)
	}

	rec := do(t, s, http.MethodDelete, "/api/v1/links", map[string]any{
		"codes": []string{"a", "c", "missing"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Deleted != 2 {
		t.Fatalf("deleted = %d, want 2 (missing codes are skipped)", body.Deleted)
	}
	// "b" survives; daily clicks of deleted links survive too (checked elsewhere).
	for _, code := range []string{"a", "c"} {
		if _, err := s.store.GetLink(context.Background(), code); err == nil {
			t.Fatalf("link %s should be deleted", code)
		}
	}
	if _, err := s.store.GetLink(context.Background(), "b"); err != nil {
		t.Fatalf("link b should survive: %v", err)
	}
}

func TestBatchDeleteValidation(t *testing.T) {
	s, _ := newTestServer(t)
	if rec := do(t, s, http.MethodDelete, "/api/v1/links", map[string]any{"codes": []string{}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty codes, got %d", rec.Code)
	}
}

func TestExpiredCountAndDelete(t *testing.T) {
	s, _ := newTestServer(t)
	// The test server pins its clock at 1700000000; express expiry relative
	// to that so the expired/not-expired split is deterministic.
	past := int64(1690000000)
	future := int64(1710000000)
	createRaw(t, s, "https://example.com/old", "old", past)
	createRaw(t, s, "https://example.com/fresh", "fresh", future)
	createRaw(t, s, "https://example.com/never", "never", nil)

	rec := do(t, s, http.MethodGet, "/api/v1/links/expired", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("count status = %d", rec.Code)
	}
	var cnt struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cnt); err != nil {
		t.Fatal(err)
	}
	if cnt.Count != 1 {
		t.Fatalf("count = %d, want 1", cnt.Count)
	}

	rec = do(t, s, http.MethodDelete, "/api/v1/links/expired", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body %s", rec.Code, rec.Body.String())
	}
	var del struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &del); err != nil {
		t.Fatal(err)
	}
	if del.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", del.Deleted)
	}
	for _, code := range []string{"fresh", "never"} {
		if _, err := s.store.GetLink(context.Background(), code); err != nil {
			t.Fatalf("link %s should survive clear-expired: %v", code, err)
		}
	}
	if _, err := s.store.GetLink(context.Background(), "old"); err == nil {
		t.Fatal("expired link should be gone")
	}
}

func TestBatchCreateConflictDefaultError(t *testing.T) {
	s, _ := newTestServer(t)
	createRaw(t, s, "https://example.com/a", "dup", nil)

	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{"url": "https://example.com/b", "code": "fresh"},
			{"url": "https://example.com/c", "code": "dup"},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Created int `json:"created"`
		Failed  int `json:"failed"`
		Results []struct {
			Index     int    `json:"index"`
			Status    string `json:"status"`
			ErrorCode string `json:"error_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Created != 1 || body.Failed != 1 {
		t.Fatalf("created/failed = %d/%d, want 1/1", body.Created, body.Failed)
	}
	if body.Results[0].Index != 0 || body.Results[0].Status != "created" {
		t.Fatalf("unexpected result[0]: %+v", body.Results[0])
	}
	if body.Results[1].Status != "error" || body.Results[1].ErrorCode != "code_taken" {
		t.Fatalf("unexpected result[1]: %+v", body.Results[1])
	}
}

func TestBatchCreateConflictSkip(t *testing.T) {
	s, _ := newTestServer(t)
	createRaw(t, s, "https://example.com/keep", "dup", nil)

	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"conflict": "skip",
		"items": []map[string]any{
			{"url": "https://example.com/other", "code": "dup"},
		},
	})
	var body struct {
		Created int `json:"created"`
		Failed  int `json:"failed"`
		Results []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Created != 0 || body.Failed != 0 || body.Results[0].Status != "skipped" {
		t.Fatalf("expected skip, got %+v", body)
	}
	// The existing link must be untouched.
	l, err := s.store.GetLink(context.Background(), "dup")
	if err != nil {
		t.Fatal(err)
	}
	if l.URL != "https://example.com/keep" {
		t.Fatalf("skipped link must stay unchanged, url = %q", l.URL)
	}
}

func TestBatchCreateConflictUpdate(t *testing.T) {
	s, _ := newTestServer(t)
	createRaw(t, s, "https://example.com/old", "dup", nil)

	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"conflict": "update",
		"items": []map[string]any{
			{"url": "https://example.com/new", "code": "dup", "title": "Imported"},
		},
	})
	var body struct {
		Created int `json:"created"`
		Failed  int `json:"failed"`
		Results []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Results[0].Status != "updated" {
		t.Fatalf("expected updated, got %+v", body)
	}
	l, err := s.store.GetLink(context.Background(), "dup")
	if err != nil {
		t.Fatal(err)
	}
	if l.URL != "https://example.com/new" || l.Title != "Imported" {
		t.Fatalf("update conflict must refresh fields: %+v", l)
	}
}

func TestBatchCreateMetaAndOverrides(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{
				"url":         "https://example.com/page",
				"code":        "meta1",
				"title":       "The Title",
				"description": "The Description",
				"expires_at":  "2030-01-02",
				"click_count": 42,
				"created_at":  1000,
			},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	l, err := s.store.GetLink(context.Background(), "meta1")
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "The Title" || l.Description != "The Description" {
		t.Fatalf("meta not stored: %+v", l)
	}
	if l.ClickCount != 42 || l.CreatedAt != 1000 {
		t.Fatalf("overrides not stored: %+v", l)
	}
	want := time.Date(2030, 1, 2, 0, 0, 0, 0, time.Local).Unix()
	if l.ExpiresAt != want {
		t.Fatalf("date-string expiry = %d, want %d", l.ExpiresAt, want)
	}
}

func TestSingleCreateIgnoresStatsOverrides(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":         "https://example.com/page",
		"code":        "single",
		"click_count": 99,
		"created_at":  1,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	l, err := s.store.GetLink(context.Background(), "single")
	if err != nil {
		t.Fatal(err)
	}
	if l.ClickCount != 0 || l.CreatedAt == 1 {
		t.Fatalf("single create must ignore stat overrides: %+v", l)
	}
}

func TestListExpiresFilter(t *testing.T) {
	s, _ := newTestServer(t)
	// Test clock is 1700000000.
	createRaw(t, s, "https://example.com/old", "old", 1690000000)
	createRaw(t, s, "https://example.com/fresh", "fresh", 1710000000)
	createRaw(t, s, "https://example.com/never", "never", nil)

	var list struct {
		Links []struct {
			Code string `json:"code"`
		} `json:"links"`
	}
	rec := do(t, s, http.MethodGet, "/api/v1/links?expires=expired", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Links) != 1 || list.Links[0].Code != "old" {
		t.Fatalf("expired filter = %+v, want only 'old'", list.Links)
	}

	rec = do(t, s, http.MethodGet, "/api/v1/links?expires=active", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Links) != 2 {
		t.Fatalf("active filter = %+v, want fresh + never", list.Links)
	}

	// Combined with the keyword search the AND join must keep both conditions.
	rec = do(t, s, http.MethodGet, "/api/v1/links?q=fresh&expires=active", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Links) != 1 || list.Links[0].Code != "fresh" {
		t.Fatalf("combined filter = %+v, want only 'fresh'", list.Links)
	}
}

func TestChineseShortCodeRoundTrip(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/目标页面",
		"code": "中文码",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body.String())
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if l.Code != "中文码" {
		t.Fatalf("code = %q", l.Code)
	}

	// The redirect route resolves the percent-encoded path back to the code.
	req := httptest.NewRequest(http.MethodGet, "/中文码", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("redirect status = %d", rr.Code)
	}
	// http.Redirect percent-encodes the non-ASCII location; browsers decode
	// it transparently, so assert on the decoded form.
	loc := rr.Header().Get("Location")
	if decoded, err := url.QueryUnescape(loc); err != nil || decoded != "https://example.com/目标页面" {
		t.Fatalf("location = %q (decoded %q)", loc, decoded)
	}

	// Search finds it by code.
	rec = do(t, s, http.MethodGet, "/api/v1/links?q="+url.QueryEscape("中文"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "中文码") {
		t.Fatalf("search should find the Chinese code, body %s", rec.Body.String())
	}
}

func TestBatchCreateInvalidConflict(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"conflict": "explode",
		"items":    []map[string]any{{"url": "https://example.com/x"}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid conflict, got %d", rec.Code)
	}
}
