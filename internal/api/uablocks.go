package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// UA block patterns live in config.yaml (business config) so they persist
// with the settings file and survive database wipes. IDs are 1-based indexes
// into the configured list.

// listUABlocks handles GET /api/v1/ua-blocks.
func (s *Server) listUABlocks(w http.ResponseWriter, r *http.Request) {
	blocks := s.cfg.Get().UABlocks
	out := make([]map[string]any, 0, len(blocks))
	for i, p := range blocks {
		out = append(out, map[string]any{"id": i + 1, "pattern": p})
	}
	slog.Debug("ua blocks listed", "count", len(blocks), "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{"ua_blocks": out})
}

// createUABlock handles POST /api/v1/ua-blocks.
func (s *Server) createUABlock(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pattern string `json:"pattern"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	pattern := strings.TrimSpace(body.Pattern)
	if pattern == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "pattern must not be empty")
		return
	}
	cfg := s.cfg.Get()
	for _, p := range cfg.UABlocks {
		if strings.EqualFold(p, pattern) {
			writeError(w, http.StatusConflict, "pattern_taken", "pattern already exists")
			return
		}
	}
	cfg.UABlocks = append(cfg.UABlocks, pattern)
	if err := s.cfg.Update(cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}
	slog.Info("ua block added", "pattern", pattern, "actor", actorFrom(r))
	writeJSON(w, http.StatusCreated, map[string]any{"id": len(cfg.UABlocks), "pattern": pattern})
}

// deleteUABlock handles DELETE /api/v1/ua-blocks/{id}.
func (s *Server) deleteUABlock(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid ua block id")
		return
	}
	cfg := s.cfg.Get()
	if int(id) > len(cfg.UABlocks) {
		writeError(w, http.StatusNotFound, "not_found", "ua block not found")
		return
	}
	cfg.UABlocks = append(cfg.UABlocks[:id-1], cfg.UABlocks[id:]...)
	if err := s.cfg.Update(cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}
	slog.Info("ua block removed", "id", id, "actor", actorFrom(r))
	w.WriteHeader(http.StatusNoContent)
}
