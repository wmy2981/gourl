package api

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/wmy2981/gourl/internal/store"
)

// metaQueue fetches titles/descriptions asynchronously for freshly created or
// edited links, so a slow or unreachable target site never blocks the API
// request (the create/edit latency driver). Jobs that time out or fail leave
// the meta empty — the same end state the old synchronous fetch produced.
type metaQueue struct {
	ch chan metaJob
	st *store.Store
	// fetcher resolves the current title fetcher at job time (tests swap
	// s.fetcher after construction).
	fetcher func() TitleFetcher
}

type metaJob struct {
	code string
	url  string
}

// newMetaQueue builds a queue with the given number of worker goroutines.
func newMetaQueue(st *store.Store, fetcher func() TitleFetcher, workers int) *metaQueue {
	q := &metaQueue{
		ch:      make(chan metaJob, 1024),
		st:      st,
		fetcher: fetcher,
	}
	for range workers {
		go q.loop()
	}
	return q
}

// enqueue schedules a meta fetch. Targets that can never yield a title
// (tcp://, openapp:// …) are dropped here; a full queue also drops the job —
// the link just stays without meta, and an edit can always retrigger a fetch.
func (q *metaQueue) enqueue(code, url string) {
	if !fetchableScheme(url) {
		return
	}
	select {
	case q.ch <- metaJob{code: code, url: url}:
	default:
		slog.Debug("meta queue full, skipping fetch", "code", code)
	}
}

// fetchableScheme reports whether a target could yield a title: only
// http/https are fetchable.
func fetchableScheme(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func (q *metaQueue) loop() {
	for job := range q.ch {
		title, desc, err := q.fetcher().Fetch(context.Background(), job.url)
		if err != nil {
			// Failures were silent before, which made "why is there no title"
			// impossible to diagnose; surface the reason at warning level.
			slog.Warn("meta fetch failed, leaving empty", "code", job.code, "url", job.url, "error", err)
			continue
		}
		if title == "" && desc == "" {
			// Lenient fetching parsed the body but found nothing usable;
			// keep whatever meta the link already has.
			slog.Debug("meta fetch found no title or description", "code", job.code, "url", job.url)
			continue
		}
		slog.Debug("fetch meta ok", "code", job.code, "url", job.url, "title_len", len(title))
		if err := q.st.UpdateMeta(context.Background(), job.code, title, desc, time.Now().Unix()); err != nil {
			// The link may have been deleted in the meantime; nothing to fix.
			slog.Debug("store meta failed", "code", job.code, "error", err)
		}
	}
}
