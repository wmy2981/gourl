package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestBatchImportLenientParsing: item parsing is forgiving — case-insensitive
// field names, unknown fields ignored, number/string coercion, multiple date
// formats, nulls defaulted, and click_count dropped.
func TestBatchImportLenientParsing(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{
				"URL":         "https://example.com/u1",
				"CODE":        "upper1",
				"TITLE":       "Upper Case",
				"expires_at":  "2030-01-02T00:00:00+08:00",
				"created_at":  "1000",
				"click_count": 999, // must be dropped
				"bogus_field": "ignored",
			},
			{
				"url":        "https://example.com/u2",
				"code":       "nulls1",
				"expires_at": nil,
				"created_at": nil,
			},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	l, err := s.store.GetLink(context.Background(), "upper1")
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "Upper Case" || l.ClickCount != 0 || l.CreatedAt != 1000 {
		t.Fatalf("upper1: %+v", l)
	}
	// RFC3339 with a +08:00 offset, converted to unix seconds.
	want := time.Date(2030, 1, 2, 0, 0, 0, 0, time.FixedZone("", 8*3600)).Unix()
	if l.ExpiresAt != want {
		t.Errorf("upper1 expires_at = %d, want %d", l.ExpiresAt, want)
	}

	l2, err := s.store.GetLink(context.Background(), "nulls1")
	if err != nil {
		t.Fatal(err)
	}
	if l2.ExpiresAt != 0 || l2.CreatedAt != 0 {
		t.Errorf("nulls must default to zero: %+v", l2)
	}
}

// TestBatchImportSkipsDeletedItems: items flagged deleted (re-imported export
// dumps) are skipped without touching the database, whatever their other
// fields; the flag parses leniently (bool or "true"/"false" strings).
func TestBatchImportSkipsDeletedItems(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{"url": "https://example.com/x", "code": "skipx", "deleted": true},
			{"url": "not-a-valid-url-at-all", "code": "skipbad", "deleted": "true"},
			{"url": "https://example.com/y", "code": "keepy"},
			{"url": "https://example.com/z", "code": "keepz", "deleted": false},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Created int      `json:"created"`
		Skipped int      `json:"skipped"`
		Failed  int      `json:"failed"`
		Results []struct {
			Status string `json:"status"`
			Code   string `json:"code"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Created != 2 || resp.Skipped != 2 || resp.Failed != 0 {
		t.Fatalf("counts = created %d skipped %d failed %d, want 2/2/0",
			resp.Created, resp.Skipped, resp.Failed)
	}
	// A deleted flag must be honored even when the rest of the item is broken.
	statuses := map[string]string{}
	for _, r := range resp.Results {
		statuses[r.Code] = r.Status
	}
	if statuses["skipx"] != "skipped" || statuses["skipbad"] != "skipped" ||
		statuses["keepy"] != "created" || statuses["keepz"] != "created" {
		t.Fatalf("per-item statuses = %v", statuses)
	}
	for _, code := range []string{"keepy", "keepz"} {
		if _, err := s.store.GetLink(context.Background(), code); err != nil {
			t.Errorf("GetLink(%s): %v", code, err)
		}
	}
}

// TestBatchImportLenientDateStrings: unix timestamps and yyyy-mm-dd still
// work, including their string forms.
func TestBatchImportLenientDateStrings(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{
			{"url": "https://example.com/d1", "code": "d1", "expires_at": "1750000000"},
			{"url": "https://example.com/d2", "code": "d2", "expires_at": 1750000000},
			{"url": "https://example.com/d3", "code": "d3", "expires_at": "2030-01-02"},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	for code, want := range map[string]int64{
		"d1": 1750000000,
		"d2": 1750000000,
		"d3": time.Date(2030, 1, 2, 0, 0, 0, 0, time.Local).Unix(),
	} {
		l, err := s.store.GetLink(context.Background(), code)
		if err != nil {
			t.Fatal(err)
		}
		if l.ExpiresAt != want {
			t.Errorf("%s expires_at = %d, want %d", code, l.ExpiresAt, want)
		}
	}
}

// TestBatchImportRejectsMissingURL: an item without a url fails the whole
// request (the field cannot be defaulted).
func TestBatchImportRejectsMissingURL(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{{"code": "nourl"}},
	})
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "invalid_request" {
		t.Fatalf("missing url: status %d, code %q", rec.Code, decodeError(t, rec))
	}
}

// TestBatchImportBadURLFailsTheItem: a non-url still fails that item (batch
// semantics: the request is 201, the item reports error).
func TestBatchImportBadURLFailsTheItem(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{{"url": "not-a-url"}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (batch semantics)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error"`) {
		t.Errorf("item must report an error, body: %s", rec.Body.String())
	}
}

// TestBatchImportResponseCounts: the response carries per-status counts and
// the code lists for failed/skipped/updated items.
func TestBatchImportResponseCounts(t *testing.T) {
	s, _ := newTestServer(t)
	createLink(t, s, "exist1", "https://example.com/exist1")

	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"conflict": "update",
		"items": []map[string]any{
			{"url": "https://example.com/new1", "code": "new1"},
			{"url": "https://example.com/new2", "code": "new2"},
			{"url": "https://example.com/upd1", "code": "exist1"},
			{"url": "not-a-url", "code": "badv1"},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Succeeded    int      `json:"succeeded"`
		Skipped      int      `json:"skipped"`
		Updated      int      `json:"updated"`
		Failed       int      `json:"failed"`
		FailedCodes  []string `json:"failed_codes"`
		SkippedCodes []string `json:"skipped_codes"`
		UpdatedCodes []string `json:"updated_codes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Succeeded != 2 || body.Failed != 1 || body.Updated != 1 {
		t.Errorf("counts = %+v", body)
	}
	if len(body.UpdatedCodes) != 1 || body.UpdatedCodes[0] != "exist1" {
		t.Errorf("updated codes = %v", body.UpdatedCodes)
	}
	if len(body.FailedCodes) != 1 || body.FailedCodes[0] != "badv1" {
		t.Errorf("failed codes = %v", body.FailedCodes)
	}
	// The bare error path also lists the offending code.
	rec = do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"items": []map[string]any{{"url": "not-a-url", "code": "badv2"}},
	})
	body = struct {
		Succeeded    int      `json:"succeeded"`
		Skipped      int      `json:"skipped"`
		Updated      int      `json:"updated"`
		Failed       int      `json:"failed"`
		FailedCodes  []string `json:"failed_codes"`
		SkippedCodes []string `json:"skipped_codes"`
		UpdatedCodes []string `json:"updated_codes"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.FailedCodes) != 1 || body.FailedCodes[0] != "badv2" {
		t.Errorf("error-path failed codes = %v", body.FailedCodes)
	}
}

// TestBatchImportConflictCaseInsensitive: the conflict value and the top-level
// keys are matched case-insensitively.
func TestBatchImportConflictCaseInsensitive(t *testing.T) {
	s, _ := newTestServer(t)
	createLink(t, s, "dup1", "https://example.com/original")
	rec := do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"CoNfLiCt": "SKIP",
		"ITEMS": []map[string]any{
			{"url": "https://example.com/new", "code": "dup1"},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	// The existing link is untouched (skip policy).
	l, err := s.store.GetLink(context.Background(), "dup1")
	if err != nil {
		t.Fatal(err)
	}
	if l.URL != "https://example.com/original" {
		t.Errorf("skip must leave the existing link alone: %+v", l)
	}
}
