package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/wmy2981/gourl/internal/config"
)

// getConfig handles GET /api/v1/config.
func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Get())
}

// updateConfig handles PUT /api/v1/config. The body is the full config
// (ua_blocks included — the settings page manages it like any other list
// field); it is validated, written back atomically to config.yaml and
// hot-swapped in memory.
func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if err := s.cfg.Update(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}
	slog.Info("config updated", "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, s.cfg.Get())
}
