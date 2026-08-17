package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestCacheTTLExpiry: the link cache serves fresh entries and drops them after
// the TTL even when the database row is untouched (e.g. external edits).
func TestCacheTTLExpiry(t *testing.T) {
	c := newLinkCache()
	now := time.Unix(1_000_000, 0)
	c.now = func() time.Time { return now }
	c.set("abc", &Link{Code: "abc", URL: "https://example.com/abc"})

	if l := c.get("abc"); l == nil || l.URL != "https://example.com/abc" {
		t.Fatalf("fresh entry not served: %+v", l)
	}
	now = now.Add(linkCacheTTL + time.Second)
	if l := c.get("abc"); l != nil {
		t.Fatalf("expired entry still served: %+v", l)
	}
}

// TestGetLinkServedFromCache: once fetched, GetLink returns the cached copy —
// and callers cannot mutate it (the store hands out a fresh struct each time).
func TestGetLinkServedFromCache(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetLink(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	a, _ := s.GetLink(ctx, "abc")
	b, _ := s.GetLink(ctx, "abc")
	a.URL = "https://mutated.example.com/"
	if b.URL == "https://mutated.example.com/" {
		t.Error("GetLink handed out a shared mutable cache entry")
	}
}

// TestWritesInvalidateCache: every write path must make the next GetLink
// observe the new database state, never a stale cache hit.
func TestWritesInvalidateCache(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetLink(ctx, "abc"); err != nil {
		t.Fatal(err)
	}

	// UpdateLink.
	upd := sampleLink("abc")
	upd.URL = "https://updated.example.com/"
	upd.UpdatedAt = 2
	if err := s.UpdateLink(ctx, upd); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetLink(ctx, "abc"); got.URL != "https://updated.example.com/" {
		t.Errorf("UpdateLink not visible: %+v", got)
	}

	// UpdateMeta.
	if err := s.UpdateMeta(ctx, "abc", "New Title", "desc", 3); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetLink(ctx, "abc"); got.Title != "New Title" {
		t.Errorf("UpdateMeta not visible: %+v", got)
	}

	// RenameLink: old code must disappear, new code resolves.
	if err := s.RenameLink(ctx, "abc", "def", 4); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetLink(ctx, "abc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("old code after rename: %v", err)
	}
	if got, err := s.GetLink(ctx, "def"); err != nil || got.URL != "https://updated.example.com/" {
		t.Errorf("new code after rename: %v %+v", err, got)
	}

	// DeleteLink.
	if err := s.DeleteLink(ctx, "def"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetLink(ctx, "def"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted code still resolves: %v", err)
	}
}

// TestDeleteExpiredClearsCache: the sweep touches unknown codes, so the whole
// cache must be dropped.
func TestDeleteExpiredClearsCache(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	l := sampleLink("abc")
	l.ExpiresAt = 1_000_000
	if err := s.CreateLink(ctx, l); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetLink(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteExpired(ctx, 1_000_001); err != nil {
		t.Fatal(err)
	}
	// The row is gone from the DB; a stale cache hit would resurrect it.
	if _, err := s.GetLink(ctx, "abc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired link still resolves after sweep: %v", err)
	}
}
