package store

import (
	"context"
	"testing"
)

func TestApplyCounts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("abc")); err != nil {
		t.Fatal(err)
	}

	err := s.ApplyCounts(ctx,
		map[string]int64{"abc": 5, "gone": 3},
		[]DailyCount{{Code: "abc", Date: "2026-08-15", Count: 5}},
		100)
	if err != nil {
		t.Fatalf("ApplyCounts: %v", err)
	}

	link, _ := s.GetLink(ctx, "abc")
	if link.ClickCount != 5 || link.UpdatedAt != 100 {
		t.Errorf("link after apply: clicks %d, updated %d", link.ClickCount, link.UpdatedAt)
	}
	// Unknown code is silently skipped, not an error.
	if err := s.ApplyCounts(ctx, map[string]int64{"gone": 9}, nil, 101); err != nil {
		t.Errorf("apply for missing link: %v", err)
	}

	// Second apply accumulates.
	if err := s.ApplyCounts(ctx,
		map[string]int64{"abc": 2},
		[]DailyCount{{Code: "abc", Date: "2026-08-15", Count: 2}},
		102); err != nil {
		t.Fatal(err)
	}
	link, _ = s.GetLink(ctx, "abc")
	if link.ClickCount != 7 {
		t.Errorf("accumulated clicks = %d, want 7", link.ClickCount)
	}
	var daily int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count FROM daily_clicks WHERE code = 'abc' AND date = '2026-08-15'`).Scan(&daily); err != nil {
		t.Fatal(err)
	}
	if daily != 7 {
		t.Errorf("daily count = %d, want 7", daily)
	}
}
