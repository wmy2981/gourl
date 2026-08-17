package store

import (
	"context"
	"errors"
	"testing"
)

// TestSoftDeleteHidesAndFreesCode: a deleted link vanishes from every read
// path, keeps its row (id preserved), and its code becomes reusable.
func TestSoftDeleteHidesAndFreesCode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	l := sampleLink("abc")
	if err := s.CreateLink(ctx, l); err != nil {
		t.Fatal(err)
	}
	oldID := l.ID
	if oldID == 0 {
		t.Fatal("expected a positive id")
	}

	if err := s.DeleteLink(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetLink(ctx, "abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetLink after delete = %v, want ErrNotFound", err)
	}
	if links, _, err := s.ListLinks(ctx, ListOptions{}); err != nil || len(links) != 0 {
		t.Fatalf("ListLinks after delete = %d (%v), want 0", len(links), err)
	}
	if all, err := s.ListAllLinks(ctx); err != nil || len(all) != 0 {
		t.Fatalf("ListAllLinks after delete = %d (%v), want 0", len(all), err)
	}
	if n, err := s.CountExpired(ctx, 1<<62); err != nil || n != 0 {
		t.Fatalf("CountExpired after delete = %d (%v), want 0", n, err)
	}
	// Re-deleting the same code is a not-found, not a double delete.
	if err := s.DeleteLink(ctx, "abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}

	// The code is free again: the partial unique index ignores deleted rows.
	again := sampleLink("abc")
	if err := s.CreateLink(ctx, again); err != nil {
		t.Fatalf("reuse after delete: %v", err)
	}
	if again.ID == oldID {
		t.Errorf("reused link id = %d, want a fresh id", again.ID)
	}
}

// TestSoftDeleteCountsFromZero: clicking a reused code after deletion counts
// for the new link only; the old rows keep the old id and still feed the
// global totals (permanent history).
func TestSoftDeleteCountsFromZero(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	old := sampleLink("abc")
	if err := s.CreateLink(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyCounts(ctx, map[string]int64{"abc": 100},
		[]DailyCount{{Code: "abc", Date: "2026-08-15", Count: 100}}, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLink(ctx, "abc"); err != nil {
		t.Fatal(err)
	}

	fresh := sampleLink("abc")
	if err := s.CreateLink(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyCounts(ctx, map[string]int64{"abc": 5},
		[]DailyCount{{Code: "abc", Date: "2026-08-16", Count: 5}}, 2); err != nil {
		t.Fatal(err)
	}

	// The new link's own total is 5, not 105.
	got, err := s.GetLink(ctx, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != fresh.ID {
		t.Fatalf("got id = %d, want %d", got.ID, fresh.ID)
	}
	if got.ClickCount != 5 {
		t.Errorf("reused link click_count = %d, want 5 (count from zero)", got.ClickCount)
	}

	// Global totals still sum both histories.
	_, total, _, err := s.StatsOverview(ctx, "2026-08-01")
	if err != nil {
		t.Fatal(err)
	}
	if total != 105 {
		t.Errorf("global total = %d, want 105 (old history kept)", total)
	}
}

// TestSoftDeleteToken: revoked tokens disappear from reads and auth, the row
// stays, and the key stays permanently taken.
func TestSoftDeleteToken(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, err := s.CreateToken(ctx, "tok-1", "note", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteToken(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetToken(ctx, "tok-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetToken after delete = %v, want ErrNotFound", err)
	}
	if tokens, err := s.ListTokens(ctx); err != nil || len(tokens) != 0 {
		t.Fatalf("ListTokens after delete = %d (%v), want 0", len(tokens), err)
	}
	if _, err := s.CreateToken(ctx, "tok-1", "reuse", 2); err == nil {
		t.Fatal("reusing a deleted token key must fail (permanently taken)")
	}
}
