package store

import (
	"context"
	"testing"
)

// TestStatsOverviewKeepsClicksAfterDelete: click totals come from the daily
// table, so deleting a link must not shrink the historical totals or the
// per-day trend.
func TestStatsOverviewKeepsClicksAfterDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.CreateLink(ctx, sampleLink("abc")); err != nil {
		t.Fatal(err)
	}

	if err := s.ApplyCounts(ctx,
		map[string]int64{"abc": 7, "gone": 3},
		[]DailyCount{
			{Code: "abc", Date: "2026-08-15", Count: 5},
			{Code: "abc", Date: "2026-08-16", Count: 2},
			{Code: "gone", Date: "2026-08-16", Count: 3},
		}, 100); err != nil {
		t.Fatal(err)
	}

	// Before deletion: one link, seven clicks across two days (the "gone"
	// clicks target no live link and are dropped under id-keyed counting).
	links, total, daily, err := s.StatsOverview(ctx, "2026-08-03")
	if err != nil {
		t.Fatal(err)
	}
	if links != 1 || total != 7 {
		t.Fatalf("before delete: links=%d total=%d, want 1/7", links, total)
	}
	if len(daily) != 2 || daily[0].Count+daily[1].Count != 7 {
		t.Fatalf("daily = %+v, want 2 days summing to 7", daily)
	}

	// Delete the link; the historical totals must survive.
	if err := s.DeleteLink(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	links, total, daily, err = s.StatsOverview(ctx, "2026-08-03")
	if err != nil {
		t.Fatal(err)
	}
	if links != 0 || total != 7 {
		t.Fatalf("after delete: links=%d total=%d, want 0/7", links, total)
	}
	if len(daily) != 2 || daily[0].Count+daily[1].Count != 7 {
		t.Fatalf("daily after delete = %+v, want 2 days summing to 7", daily)
	}
}
