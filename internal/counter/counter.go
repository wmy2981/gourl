// Package counter buffers click counts in Redis and flushes them to SQLite
// in batches. Counters are keyed by code (total) and by code+date (daily),
// so a crash loses at most the clicks of the current flush interval.
package counter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key prefixes.
const (
	TotalKey = "counter:" // + code
	DailyKey = "counter:" // + code + ":" + date
)

// Counter wraps the Redis connection used for click buffering.
type Counter struct {
	rdb *redis.Client
}

// New creates a Counter. The connection is lazy; failures are only observed
// on actual commands.
func New(addr string) *Counter {
	return &Counter{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

// NewFromClient wraps an existing client (used by tests with miniredis).
func NewFromClient(rdb *redis.Client) *Counter { return &Counter{rdb: rdb} }

// Close closes the underlying client.
func (c *Counter) Close() error { return c.rdb.Close() }

// Ping checks Redis connectivity.
func (c *Counter) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Incr records one click for a code on the given date (YYYY-MM-DD, in the
// process local timezone, i.e. the container TZ). Both the total and the
// daily counters are incremented in a single round trip.
func (c *Counter) Incr(ctx context.Context, code, date string) error {
	_, err := c.rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Incr(ctx, TotalKey+code)
		pipe.Incr(ctx, DailyKey+code+":"+date)
		return nil
	})
	return err
}

// GetAndReset atomically takes the current counter value, leaving the key at
// zero. Used by the flush task; a zero count means the key may not exist.
func (c *Counter) GetAndReset(ctx context.Context, key string) (int64, error) {
	v, err := c.rdb.GetDel(ctx, key).Int64()
	if err != nil {
		return 0, fmt.Errorf("getdel %s: %w", key, err)
	}
	return v, nil
}

// AddBack re-adds a value to a counter after a failed DB write, so the next
// flush retries it.
func (c *Counter) AddBack(ctx context.Context, key string, v int64) error {
	if err := c.rdb.IncrBy(ctx, key, v).Err(); err != nil {
		return fmt.Errorf("addback %s: %w", key, err)
	}
	return nil
}

// Keys returns the total and daily Redis keys for a code on a date.
func Keys(code, date string) (total, daily string) {
	return TotalKey + code, DailyKey + code + ":" + date
}

// Date returns today's date string in the process local timezone.
func Date(now time.Time) string { return now.Format("2006-01-02") }
