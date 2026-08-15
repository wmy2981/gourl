package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
)

// mockFetcher is a deterministic TitleFetcher for tests.
type mockFetcher struct {
	title, desc string
	err         error
	calls       atomic.Int32
}

func (m *mockFetcher) Fetch(ctx context.Context, rawURL string) (string, string, error) {
	m.calls.Add(1)
	return m.title, m.desc, m.err
}

func TestCreateAutoFetchesTitle(t *testing.T) {
	s, _ := newTestServer(t)
	s.fetcher = &mockFetcher{title: "Fetched Title", desc: "Fetched Description"}

	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/page",
		"code": "abc",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if l.Title != "Fetched Title" || l.Description != "Fetched Description" {
		t.Errorf("meta not attached: %+v", l)
	}
}

func TestCreateFetchFailureIsSilent(t *testing.T) {
	s, _ := newTestServer(t)
	s.fetcher = &mockFetcher{err: errors.New("boom")}

	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/page",
		"code": "abc",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creation must succeed despite fetch failure: %d, body %s", rec.Code, rec.Body.String())
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if l.Title != "" || l.Description != "" {
		t.Errorf("meta should be empty on failure: %+v", l)
	}
}

func TestUpdateURLRefetchesTitle(t *testing.T) {
	s, _ := newTestServer(t)
	f := &mockFetcher{title: "Old Title"}
	s.fetcher = f
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/old",
		"code": "abc",
	})

	f.title = "New Title"
	rec := do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"url": "https://example.com/new",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", rec.Code, rec.Body.String())
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if l.URL != "https://example.com/new" || l.Title != "New Title" {
		t.Errorf("url change should refetch title: %+v", l)
	}
}

func TestUpdateTitleDoesNotRefetch(t *testing.T) {
	s, _ := newTestServer(t)
	f := &mockFetcher{title: "Auto Title"}
	s.fetcher = f
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/page",
		"code": "abc",
	})
	callsAfterCreate := f.calls.Load()

	rec := do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"title": "Manual Title",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d", rec.Code)
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if l.Title != "Manual Title" {
		t.Errorf("title = %q, want Manual Title", l.Title)
	}
	if f.calls.Load() != callsAfterCreate {
		t.Error("title-only patch must not trigger a fetch")
	}
}
