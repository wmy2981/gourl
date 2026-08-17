package store

import (
	"context"
	"fmt"
)

// DailySummary is one day's aggregated click count.
type DailySummary struct {
	Date  string
	Count int64
}

// StatsOverview aggregates dashboard numbers: total links, total clicks and
// per-day click sums from fromDate onwards (inclusive). The click totals come
// from the daily table — independent of link rows — so deleting a link keeps
// its historical clicks in the totals and the trend chart.
func (s *Store) StatsOverview(ctx context.Context, fromDate string) (linksTotal, clicksTotal int64, daily []DailySummary, err error) {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM links WHERE deleted = 0`).Scan(&linksTotal); err != nil {
		return 0, 0, nil, fmt.Errorf("count links: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(count), 0) FROM daily_clicks`).Scan(&clicksTotal); err != nil {
		return 0, 0, nil, fmt.Errorf("sum clicks: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT date, SUM(count) FROM daily_clicks WHERE date >= ? GROUP BY date ORDER BY date`, fromDate)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("daily clicks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d DailySummary
		if err := rows.Scan(&d.Date, &d.Count); err != nil {
			return 0, 0, nil, fmt.Errorf("scan daily: %w", err)
		}
		daily = append(daily, d)
	}
	return linksTotal, clicksTotal, daily, rows.Err()
}
