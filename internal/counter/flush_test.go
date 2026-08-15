package counter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/wmy2981/gourl/internal/store"
)

func newFlushTest(t *testing.T) (*Flusher, *miniredis.Miniredis, *store.Store) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	f := NewFlusher(st, NewFromClient(rdb), 30*time.Second)
	f.now = func() time.Time { return time.Unix(1700000000, 0) }
	return f, mr, st
}

func TestFlushOnceAppliesCounts(t *testing.T) {
	f, mr, st := newFlushTest(t)
	ctx := context.Background()
	if err := st.CreateLink(ctx, &store.Link{Code: "abc", URL: "https://e.com/x", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	// Simulate clicks buffered in Redis.
	mr.Set(TotalKey+"abc", "5")
	mr.Set(DailyKey+"abc:2023-11-15", "3")
	mr.Set(DailyKey+"abc:2023-11-16", "2")

	if err := f.FlushOnce(ctx); err != nil {
		t.Fatalf("FlushOnce: %v", err)
	}

	link, err := st.GetLink(ctx, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if link.ClickCount != 5 {
		t.Errorf("click_count = %d, want 5", link.ClickCount)
	}
	if link.UpdatedAt != 1700000000 {
		t.Errorf("updated_at = %d, want 1700000000", link.UpdatedAt)
	}

	// Redis keys must be drained.
	if _, err := mr.Get(TotalKey + "abc"); err == nil {
		t.Error("total key not drained")
	}
	if _, err := mr.Get(DailyKey + "abc:2023-11-15"); err == nil {
		t.Error("daily key not drained")
	}
}

func TestFlushOnceNothingToDo(t *testing.T) {
	f, _, st := newFlushTest(t)
	ctx := context.Background()
	if err := st.CreateLink(ctx, &store.Link{Code: "abc", URL: "https://e.com/x", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := f.FlushOnce(ctx); err != nil {
		t.Fatalf("FlushOnce with no keys: %v", err)
	}
	link, _ := st.GetLink(ctx, "abc")
	if link.ClickCount != 0 {
		t.Errorf("click_count = %d, want 0", link.ClickCount)
	}
}

func TestFlushOnceSkipsDeletedLinks(t *testing.T) {
	f, _, st := newFlushTest(t)
	ctx := context.Background()
	if err := st.CreateLink(ctx, &store.Link{Code: "abc", URL: "https://e.com/x", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteLink(ctx, "abc"); err != nil {
		t.Fatal(err)
	}

	f.counter.rdb.Set(ctx, TotalKey+"abc", "9", 0) // clicks for a deleted link
	if err := f.FlushOnce(ctx); err != nil {
		t.Errorf("FlushOnce with deleted link: %v", err)
	}
}

func TestFlushOnceAddsBackOnDBFailure(t *testing.T) {
	f, mr, st := newFlushTest(t)
	ctx := context.Background()
	if err := st.CreateLink(ctx, &store.Link{Code: "abc", URL: "https://e.com/x", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	mr.Set(TotalKey+"abc", "4")
	mr.Set(DailyKey+"abc:2023-11-15", "4")

	st.Close() // make the DB write fail

	if err := f.FlushOnce(ctx); err == nil {
		t.Fatal("FlushOnce should fail when the store is closed")
	}

	// Values must be back in Redis, ready for the next flush.
	if got, err := mr.Get(TotalKey + "abc"); err != nil || got != "4" {
		t.Errorf("total added back = %q (err %v), want 4", got, err)
	}
	if got, err := mr.Get(DailyKey + "abc:2023-11-15"); err != nil || got != "4" {
		t.Errorf("daily added back = %q (err %v), want 4", got, err)
	}
}

func TestParseKey(t *testing.T) {
	cases := []struct {
		key     string
		code    string
		date    string
		isDaily bool
	}{
		{TotalKey + "abc", "abc", "", false},
		{TotalKey + "link1/link2", "link1/link2", "", false},
		{DailyKey + "abc:2023-11-15", "abc", "2023-11-15", true},
		{DailyKey + "link1/link2:2023-11-15", "link1/link2", "2023-11-15", true},
		{TotalKey + "abc:2023-11-155", "abc:2023-11-155", "", false}, // not a date
		{TotalKey + "abc:notadate", "abc:notadate", "", false},
	}
	for _, tc := range cases {
		code, date, ok := parseKey(tc.key)
		if code != tc.code || date != tc.date || ok != tc.isDaily {
			t.Errorf("parseKey(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.key, code, date, ok, tc.code, tc.date, tc.isDaily)
		}
	}
}
