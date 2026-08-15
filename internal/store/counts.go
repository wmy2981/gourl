package store

import (
	"context"
	"fmt"
)

// DailyCount is one daily click record to apply.
type DailyCount struct {
	Code  string
	Date  string
	Count int64
}

// ApplyCounts applies flushed click counts in one transaction: totals are
// added to links.click_count (links that no longer exist are skipped), and
// daily counts are upserted. now stamps updated_at on touched links.
func (s *Store) ApplyCounts(ctx context.Context, totals map[string]int64, dailies []DailyCount, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for code, n := range totals {
		if n <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE links SET click_count = click_count + ?, updated_at = ? WHERE code = ?`,
			n, now, code); err != nil {
			return fmt.Errorf("apply total for %s: %w", code, err)
		}
	}
	for _, d := range dailies {
		if d.Count <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO daily_clicks (code, date, count) VALUES (?, ?, ?)
			 ON CONFLICT(code, date) DO UPDATE SET count = count + excluded.count`,
			d.Code, d.Date, d.Count); err != nil {
			return fmt.Errorf("apply daily for %s/%s: %w", d.Code, d.Date, err)
		}
	}
	return tx.Commit()
}
