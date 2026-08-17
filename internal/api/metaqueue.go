package api

import (
	"context"
	"log/slog"
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

// enqueue schedules a meta fetch. A full queue drops the job — the link just
// stays without meta, and an edit can always retrigger a fetch.
func (q *metaQueue) enqueue(code, url string) {
	select {
	case q.ch <- metaJob{code: code, url: url}:
	default:
		slog.Debug("meta queue full, skipping fetch", "code", code)
	}
}

func (q *metaQueue) loop() {
	for job := range q.ch {
		title, desc, err := q.fetcher().Fetch(context.Background(), job.url)
		if err != nil {
			slog.Debug("fetch meta failed, leaving empty", "url", job.url, "error", err)
			continue
		}
		slog.Debug("fetch meta ok", "code", job.code, "url", job.url, "title_len", len(title))
		if err := q.st.UpdateMeta(context.Background(), job.code, title, desc, time.Now().Unix()); err != nil {
			// The link may have been deleted in the meantime; nothing to fix.
			slog.Debug("store meta failed", "code", job.code, "error", err)
		}
	}
}
