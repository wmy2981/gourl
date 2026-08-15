package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/wmy2981/gourl/internal/store"
)

// listUABlocks handles GET /api/v1/ua-blocks.
func (s *Server) listUABlocks(w http.ResponseWriter, r *http.Request) {
	blocks, err := s.store.ListUABlocks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list ua blocks")
		return
	}
	out := make([]map[string]any, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, map[string]any{
			"id":         b.ID,
			"pattern":    b.Pattern,
			"created_at": b.CreatedAt,
		})
	}
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
	id, err := s.store.CreateUABlock(r.Context(), pattern, s.now())
	if err != nil {
		if errors.Is(err, store.ErrTaken) {
			writeError(w, http.StatusConflict, "pattern_taken", "pattern already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create ua block")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "pattern": pattern})
}

// deleteUABlock handles DELETE /api/v1/ua-blocks/{id}.
func (s *Server) deleteUABlock(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid ua block id")
		return
	}
	if err := s.store.DeleteUABlock(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "ua block not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete ua block")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
