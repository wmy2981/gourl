package counter

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/wmy2981/gourl/internal/store"
)

// Flusher periodically moves buffered click counts from Redis into SQLite.
// Each flush is all-or-nothing: if the DB write fails, every value taken is
// added back to Redis so the next flush retries it. A crash loses at most
// one interval of clicks.
type Flusher struct {
	store    *store.Store
	counter  *Counter
	interval time.Duration
	now      func() time.Time // injectable clock for tests
}

// NewFlusher creates a Flusher with the given flush interval.
func NewFlusher(st *store.Store, ctr *Counter, interval time.Duration) *Flusher {
	return &Flusher{store: st, counter: ctr, interval: interval, now: time.Now}
}

// Run flushes every interval until ctx is cancelled.
func (f *Flusher) Run(ctx context.Context) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := f.FlushOnce(ctx); err != nil {
				slog.Error("flush counters failed", "error", err)
			}
		}
	}
}

// FlushOnce scans all counter keys, resets them atomically and applies the
// values to SQLite. Returns an error (with values added back) if the DB
// write fails.
func (f *Flusher) FlushOnce(ctx context.Context) error {
	keys, err := f.counter.scanKeys(ctx)
	if err != nil {
		return fmt.Errorf("scan counters: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}

	vals, err := f.counter.getAndResetMany(ctx, keys)
	if err != nil {
		return fmt.Errorf("reset counters: %w", err)
	}

	totals := make(map[string]int64)
	var dailies []store.DailyCount
	for key, v := range vals {
		if v <= 0 {
			continue
		}
		if code, date, ok := parseKey(key); ok {
			dailies = append(dailies, store.DailyCount{Code: code, Date: date, Count: v})
		} else {
			totals[code] += v
		}
	}

	if err := f.store.ApplyCounts(ctx, totals, dailies, f.now().Unix()); err != nil {
		f.addBack(ctx, vals)
		return fmt.Errorf("apply counts: %w", err)
	}
	return nil
}

// addBack returns every taken value to Redis after a failed DB write.
func (f *Flusher) addBack(ctx context.Context, vals map[string]int64) {
	for key, v := range vals {
		if v <= 0 {
			continue
		}
		if err := f.counter.AddBack(ctx, key, v); err != nil {
			// Both paths failed; the clicks are lost. Log loudly.
			slog.Error("clicks lost: failed to add back counter", "key", key, "value", v, "error", err)
		}
	}
}

// scanKeys lists all counter keys via SCAN (non-blocking on the server).
func (c *Counter) scanKeys(ctx context.Context) ([]string, error) {
	var keys []string
	iter := c.rdb.Scan(ctx, 0, TotalKey+"*", 1000).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

// getAndResetMany atomically takes all given keys in one pipeline, returning
// key → value for keys that had a value.
func (c *Counter) getAndResetMany(ctx context.Context, keys []string) (map[string]int64, error) {
	cmds, err := c.rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, k := range keys {
			pipe.GetDel(ctx, k)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(keys))
	for i, cmd := range cmds {
		sc, ok := cmd.(*redis.StringCmd) // GetDel returns a StringCmd
		if !ok {
			return nil, fmt.Errorf("unexpected command type %T", cmd)
		}
		v, err := sc.Int64()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[keys[i]] = v
	}
	return out, nil
}

// parseKey splits a counter key into a code and, for daily keys, a date
// (YYYY-MM-DD). Codes never contain ':' (see shortcode.Validate), so the
// last ':' separator is unambiguous.
func parseKey(key string) (code, date string, isDaily bool) {
	rest := strings.TrimPrefix(key, TotalKey)
	if i := strings.LastIndex(rest, ":"); i > 0 {
		if d := rest[i+1:]; isDate(d) {
			return rest[:i], d, true
		}
	}
	return rest, "", false
}

func isDate(s string) bool {
	if len(s) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
