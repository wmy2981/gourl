package store

import (
	"context"
	"errors"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleLink(code string) *Link {
	return &Link{Code: code, URL: "https://example.com/" + code, ExpiresAt: 0, CreatedAt: 1, UpdatedAt: 1}
}

func TestCreateGetLink(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("abc")); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	got, err := s.GetLink(ctx, "abc")
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if got.URL != "https://example.com/abc" || got.ExpiresAt != 0 {
		t.Errorf("unexpected link: %+v", got)
	}
}

func TestCreateLinkDuplicateReturnsErrTaken(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("abc")); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLink(ctx, sampleLink("abc")); !errors.Is(err, ErrTaken) {
		t.Errorf("second create: %v, want ErrTaken", err)
	}
}

func TestGetLinkNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetLink(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestUpdateLink(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("abc")); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetLink(ctx, "abc")
	got.URL = "https://new.example.com"
	got.Title = "New Title"
	got.ExpiresAt = 123
	got.UpdatedAt = 2
	if err := s.UpdateLink(ctx, got); err != nil {
		t.Fatalf("UpdateLink: %v", err)
	}
	after, _ := s.GetLink(ctx, "abc")
	if after.URL != "https://new.example.com" || after.Title != "New Title" || after.ExpiresAt != 123 {
		t.Errorf("update not persisted: %+v", after)
	}
	if after.ClickCount != 0 {
		t.Errorf("click count changed: %d", after.ClickCount)
	}
}

func TestUpdateLinkNotFound(t *testing.T) {
	s := newTestStore(t)
	l := sampleLink("missing")
	if err := s.UpdateLink(context.Background(), l); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestRenameLinkMovesDailyClicks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("old")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO daily_clicks (code, date, count) VALUES ('old', '2026-08-15', 3)`); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameLink(ctx, "old", "new", 2); err != nil {
		t.Fatalf("RenameLink: %v", err)
	}
	if _, err := s.GetLink(ctx, "old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("old code still exists: %v", err)
	}
	if _, err := s.GetLink(ctx, "new"); err != nil {
		t.Errorf("new code missing: %v", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count FROM daily_clicks WHERE code = 'new'`).Scan(&count); err != nil {
		t.Fatalf("daily click not moved: %v", err)
	}
	if count != 3 {
		t.Errorf("daily count = %d, want 3", count)
	}
}

func TestRenameLinkConflictReturnsErrTaken(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("a")); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLink(ctx, sampleLink("b")); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameLink(ctx, "a", "b", 2); !errors.Is(err, ErrTaken) {
		t.Errorf("got %v, want ErrTaken", err)
	}
}

// TestDeleteLinkKeepsDailyClicks: daily click records survive link deletion
// so the dashboard history stays complete.
func TestDeleteLinkKeepsDailyClicks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO daily_clicks (code, date, count) VALUES ('abc', '2026-08-15', 5)`); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLink(ctx, "abc"); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if _, err := s.GetLink(ctx, "abc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("link still exists: %v", err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM daily_clicks WHERE code = 'abc'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("daily clicks lost on delete: %d rows, want 1", n)
	}
}

func TestDeleteLinkNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteLink(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestListLinksPaginationAndSearch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i, code := range []string{"aaa", "bbb", "ccc", "ddd"} {
		l := sampleLink(code)
		l.CreatedAt = int64(i)
		l.UpdatedAt = int64(i)
		l.Description = "docs for " + code
		if err := s.CreateLink(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	// Search by url.
	links, total, err := s.ListLinks(ctx, ListOptions{Query: "example.com/bb"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(links) != 1 || links[0].Code != "bbb" {
		t.Errorf("url search: got %d total, %d rows, %v", total, len(links), links)
	}
	// Search by description.
	links, total, err = s.ListLinks(ctx, ListOptions{Query: "docs for dd"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(links) != 1 || links[0].Code != "ddd" {
		t.Errorf("description search: got %d total, %d rows, %v", total, len(links), links)
	}
	// Pagination, default sort = created_at desc.
	page1, total, err := s.ListLinks(ctx, ListOptions{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(page1) != 2 || page1[0].Code != "ddd" || page1[1].Code != "ccc" {
		t.Errorf("page1: %d total, %v", total, page1)
	}
	page2, _, err := s.ListLinks(ctx, ListOptions{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 2 || page2[0].Code != "bbb" {
		t.Errorf("page2: %v", page2)
	}
	// Sort by code ascending.
	byCode, _, err := s.ListLinks(ctx, ListOptions{Page: 1, PageSize: 100, Sort: "code", Order: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	if byCode[0].Code != "aaa" || byCode[3].Code != "ddd" {
		t.Errorf("sort by code: %v", byCode)
	}
	// Page size clamp.
	clamped, _, err := s.ListLinks(ctx, ListOptions{Page: 1, PageSize: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(clamped) != 4 {
		t.Errorf("page size not clamped: %d rows", len(clamped))
	}
}

func TestMigrationIsIdempotentOnReopen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateLink(context.Background(), sampleLink("abc")); err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Reopening an existing database must not re-run migrations or lose data.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	if _, err := s2.GetLink(context.Background(), "abc"); err != nil {
		t.Errorf("data lost on reopen: %v", err)
	}
}
