package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/logx"
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
	// The password hash is excluded from the JSON contract (json:"-"); carry
	// the existing one over so a plain PUT never wipes the stored hash.
	cfg.PasswordHash = s.cfg.Get().PasswordHash
	if err := s.cfg.Update(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}
	// The log level is part of the business config now: apply it immediately.
	logx.SetLevel(logx.ParseLevel(cfg.LogLevel))
	slog.Info("config updated", "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, s.cfg.Get())
}
