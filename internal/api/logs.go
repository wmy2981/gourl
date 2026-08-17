package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/wmy2981/gourl/internal/logx"
)

// logHistory handles GET /api/v1/logs: the most recent log records read back
// from the mirrored file (LOG_DIR), newest first. "available" is false when
// no log file exists (the frontend then shows the live stream only).
func (s *Server) logHistory(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 200
	}
	if limit > logx.HistoryLimit {
		limit = logx.HistoryLimit
	}
	if offset < 0 {
		offset = 0
	}
	records, err := logx.ReadHistory(limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to read logs")
		return
	}
	slog.Debug("log history read", "limit", limit, "offset", offset, "count", len(records))
	writeJSON(w, http.StatusOK, map[string]any{
		"available": logx.HistoryEnabled(),
		"records":   records,
	})
}

// logStream handles GET /api/v1/logs/stream: a Server-Sent Events stream of
// live log records. Subscribers that fall behind are dropped by logx; a
// keep-alive comment every 30s holds the connection through idle periods.
func (s *Server) logStream(w http.ResponseWriter, r *http.Request) {
	slog.Debug("log stream opened", "remote", r.RemoteAddr)
	defer slog.Debug("log stream closed", "remote", r.RemoteAddr)
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	// Push the headers out immediately; the client's request blocks on them
	// until the first body write or flush otherwise.
	rc.Flush()

	ch, cancel := logx.Subscribe(256)
	defer cancel()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case rec := <-ch:
			writeSSEEvent(w, rc, rec)
		case <-ticker.C:
			// Comments are ignored by EventSource and keep the connection
			// alive through proxies with idle timeouts.
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				slog.Debug("log stream write failed", "error", err)
				return
			}
			rc.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// writeSSEEvent emits one `event: log` frame carrying the record as JSON.
func writeSSEEvent(w http.ResponseWriter, rc *http.ResponseController, rec logx.Record) {
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	if _, err := w.Write([]byte("event: log\ndata: ")); err != nil {
		return
	}
	if _, err := w.Write(data); err != nil {
		return
	}
	if _, err := w.Write([]byte("\n\n")); err != nil {
		slog.Debug("log stream write failed", "error", err)
		return
	}
	rc.Flush()
}
