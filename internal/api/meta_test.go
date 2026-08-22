package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wmy2981/gourl/internal/store"
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

// waitMeta polls the store until the link's title matches (the async meta
// worker runs on its own goroutine). Fails the test on timeout.
func waitMeta(t *testing.T, s *Server, code, wantTitle string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		l, err := s.store.GetLink(context.Background(), code)
		if err == nil && l.Title == wantTitle {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	l, err := s.store.GetLink(context.Background(), code)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}
	t.Fatalf("title = %q, want %q after async fetch", l.Title, wantTitle)
}

func TestCreateFetchesTitleAsync(t *testing.T) {
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
	// The response returns immediately with empty meta; the fetch lands later.
	if l.Title != "" || l.Description != "" {
		t.Fatalf("expected empty meta in the immediate response, got %+v", l)
	}
	waitMeta(t, s, "abc", "Fetched Title")
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
	// Give the worker a beat to fail, then confirm the meta stays empty.
	time.Sleep(100 * time.Millisecond)
	l, err := s.store.GetLink(context.Background(), "abc")
	if err != nil {
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
	waitMeta(t, s, "abc", "Old Title")

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
	if l.URL != "https://example.com/new" {
		t.Errorf("url = %q, want the updated url", l.URL)
	}
	// The background refetch lands on the stored link shortly after.
	waitMeta(t, s, "abc", "New Title")
}

func TestUpdateTitleDoesNotRefetch(t *testing.T) {
	s, _ := newTestServer(t)
	f := &mockFetcher{title: "Auto Title"}
	s.fetcher = f
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/page",
		"code": "abc",
	})
	waitMeta(t, s, "abc", "Auto Title")
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
	time.Sleep(100 * time.Millisecond)
	if f.calls.Load() != callsAfterCreate {
		t.Errorf("title-only patch must not trigger a fetch: calls %d -> %d", callsAfterCreate, f.calls.Load())
	}
}

// Any mutation re-fetches the title in the background, not just URL changes —
// an expiry edit keeps meta fresh too.
func TestUpdateAnyFieldRefetchesTitle(t *testing.T) {
	s, _ := newTestServer(t)
	f := &mockFetcher{title: "Old Title"}
	s.fetcher = f
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/page",
		"code": "abc",
	})
	waitMeta(t, s, "abc", "Old Title")

	f.title = "Refreshed Title"
	rec := do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"expires_at": 0,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", rec.Code, rec.Body.String())
	}
	waitMeta(t, s, "abc", "Refreshed Title")
}

// Lenient fetching parses odd responses too; a refetch that finds nothing
// usable must not wipe meta the link already has.
func TestEmptyFetchResultKeepsExistingMeta(t *testing.T) {
	s, _ := newTestServer(t)
	s.fetcher = &mockFetcher{title: "First Title"}
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/page",
		"code": "abc",
	})
	waitMeta(t, s, "abc", "First Title")

	s.fetcher = &mockFetcher{} // no title, no description
	do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"url": "https://example.com/new",
	})
	time.Sleep(100 * time.Millisecond)
	l, err := s.store.GetLink(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if l.Title != "First Title" {
		t.Errorf("title = %q, want the previous title kept", l.Title)
	}
}

// A stored description is sticky: once non-empty, no fetch result ever
// overwrites it (user-entered descriptions must survive refetches), while a
// fetched title still lands.
func TestFetchNeverOverwritesExistingDescription(t *testing.T) {
	s, _ := newTestServer(t)
	f := &mockFetcher{title: "Site Title", desc: "Site Description"}
	s.fetcher = f
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/page",
		"code": "abc",
	})
	waitMeta(t, s, "abc", "Site Title")
	if l, _ := s.store.GetLink(context.Background(), "abc"); l.Description != "Site Description" {
		t.Fatalf("description = %q, want the fetched value on an empty link", l.Description)
	}

	// The user edits the description; a later fetch offers different meta.
	rec := do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"description": "My Own Description",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d", rec.Code)
	}
	f.title = "Newer Title"
	f.desc = "Newer Description"
	do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"expires_at": 0,
	})
	waitMeta(t, s, "abc", "Newer Title")
	l, err := s.store.GetLink(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if l.Description != "My Own Description" {
		t.Errorf("description = %q, want the user's text kept", l.Description)
	}
}

// The async worker must not leave a deleted link dangling (store miss is a
// debug note, never a crash or a stuck worker).
func TestMetaAfterDeleteIsHarmless(t *testing.T) {
	s, _ := newTestServer(t)
	s.fetcher = &mockFetcher{title: "Slow Title"}
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/page",
		"code": "abc",
	})
	if rec := do(t, s, http.MethodDelete, "/api/v1/links/abc", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := s.store.GetLink(context.Background(), "abc"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected the link to stay deleted, got %v", err)
	}
}
