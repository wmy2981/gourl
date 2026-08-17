package store

import (
	"sync"
	"time"
)

// linkCacheTTL bounds how long a GetLink result stays fresh in memory. Writes
// invalidate the affected entries immediately, so the TTL only guards against
// external edits to the SQLite file.
const linkCacheTTL = 60 * time.Second

type cacheEntry struct {
	link      *Link
	expiresAt time.Time
}

// linkCache is a small TTL cache of link rows keyed by code, safe for
// concurrent use. It lives inside the Store and is invisible to callers.
type linkCache struct {
	mu  sync.RWMutex
	m   map[string]cacheEntry
	now func() time.Time
}

func newLinkCache() *linkCache {
	return &linkCache{m: make(map[string]cacheEntry), now: time.Now}
}

// get returns a copy of the cached link, or nil on miss/expiry.
func (c *linkCache) get(code string) *Link {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[code]
	if !ok || c.now().After(e.expiresAt) {
		return nil
	}
	l := *e.link // copy: callers must never mutate the cached row
	return &l
}

func (c *linkCache) set(code string, l *Link) {
	cp := *l
	c.mu.Lock()
	c.m[code] = cacheEntry{link: &cp, expiresAt: c.now().Add(linkCacheTTL)}
	c.mu.Unlock()
}

func (c *linkCache) del(code string) {
	c.mu.Lock()
	delete(c.m, code)
	c.mu.Unlock()
}

// clear drops every entry (used after sweep operations that touch unknown
// codes, like clearing expired links).
func (c *linkCache) clear() {
	c.mu.Lock()
	c.m = make(map[string]cacheEntry)
	c.mu.Unlock()
}
