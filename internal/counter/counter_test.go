package counter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestCounter(t *testing.T) (*Counter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return NewFromClient(rdb), mr
}

func TestIncrWritesTotalAndDaily(t *testing.T) {
	c, mr := newTestCounter(t)
	ctx := context.Background()
	if err := c.Incr(ctx, "abc", "2026-08-15"); err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if err := c.Incr(ctx, "abc", "2026-08-15"); err != nil {
		t.Fatalf("Incr: %v", err)
	}
	if err := c.Incr(ctx, "abc", "2026-08-16"); err != nil {
		t.Fatalf("Incr: %v", err)
	}
	total, err := mr.Get(TotalKey + "abc")
	if err != nil {
		t.Fatal(err)
	}
	if total != "3" {
		t.Errorf("total = %s, want 3", total)
	}
	d15, _ := mr.Get(DailyKey + "abc:2026-08-15")
	if d15 != "2" {
		t.Errorf("daily 08-15 = %s, want 2", d15)
	}
	d16, _ := mr.Get(DailyKey + "abc:2026-08-16")
	if d16 != "1" {
		t.Errorf("daily 08-16 = %s, want 1", d16)
	}
}

func TestGetAndReset(t *testing.T) {
	c, mr := newTestCounter(t)
	ctx := context.Background()
	key := TotalKey + "abc"
	mr.Set(key, "7")

	v, err := c.GetAndReset(ctx, key)
	if err != nil {
		t.Fatalf("GetAndReset: %v", err)
	}
	if v != 7 {
		t.Errorf("value = %d, want 7", v)
	}
	if got, err := mr.Get(key); err == nil && got != "0" {
		t.Errorf("after reset key = %q, want 0", got)
	}
}

func TestAddBack(t *testing.T) {
	c, mr := newTestCounter(t)
	ctx := context.Background()
	key := TotalKey + "abc"
	if err := c.AddBack(ctx, key, 5); err != nil {
		t.Fatalf("AddBack: %v", err)
	}
	if err := c.AddBack(ctx, key, 3); err != nil {
		t.Fatalf("AddBack: %v", err)
	}
	got, _ := mr.Get(key)
	if got != "8" {
		t.Errorf("after addback = %s, want 8", got)
	}
}

func TestDateUsesLocalTimezone(t *testing.T) {
	// 2023-11-14T22:30:00Z is 2023-11-15 in UTC+8 and 2023-11-14 in UTC.
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	got := time.Date(2023, 11, 14, 22, 30, 0, 0, time.UTC).In(loc).Format("2006-01-02")
	if got != "2023-11-15" {
		t.Errorf("date in Asia/Shanghai = %s, want 2023-11-15", got)
	}
	if d := Date(time.Date(2023, 11, 14, 22, 30, 0, 0, time.UTC)); d != "2023-11-14" {
		t.Errorf("date in local tz = %s, want 2023-11-14", d)
	}
}

func TestPing(t *testing.T) {
	c, _ := newTestCounter(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}
