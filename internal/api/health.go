package api

import (
	"net/http"
	"time"

	"github.com/wmy2981/gourl/internal/version"
)

// health handles GET /api/v1/health (public). It reports service identity
// and probes both dependencies; a failing dependency yields 503 so Docker
// HEALTHCHECK and orchestrators can act on it.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK

	redisStatus := "ok"
	if err := s.counter.Ping(r.Context()); err != nil {
		redisStatus = "error"
		status = http.StatusServiceUnavailable
	}
	sqliteStatus := "ok"
	if err := s.store.Ping(r.Context()); err != nil {
		sqliteStatus = "error"
		status = http.StatusServiceUnavailable
	}

	writeJSON(w, status, map[string]any{
		"name":       s.cfg.Get().Site.Name,
		"version":    version.Version,
		"start_time": s.startTime.Unix(),
		"uptime":     int64(time.Since(s.startTime).Seconds()),
		"redis":      redisStatus,
		"sqlite":     sqliteStatus,
	})
}
