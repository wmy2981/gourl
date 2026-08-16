package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// deleteLinks handles DELETE /api/v1/links: batch deletion by codes.
func (s *Server) deleteLinks(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Codes []string `json:"codes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if len(body.Codes) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "codes must not be empty")
		return
	}
	if len(body.Codes) > batchLimit {
		writeError(w, http.StatusBadRequest, "invalid_request", "batch exceeds 500 items")
		return
	}
	deleted, err := s.store.DeleteLinks(r.Context(), body.Codes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete links")
		return
	}
	slog.Info("links batch deleted", "deleted", deleted, "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}

// expiredCount handles GET /api/v1/links/expired: how many links are past
// their expiry (drives the clear-expired confirmation dialog).
func (s *Server) expiredCount(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.CountExpired(r.Context(), s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to count expired links")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

// deleteExpired handles DELETE /api/v1/links/expired: removes every link
// whose expiry has passed (daily click history is kept, as always).
func (s *Server) deleteExpired(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.store.DeleteExpired(r.Context(), s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete expired links")
		return
	}
	slog.Info("expired links deleted", "deleted", deleted, "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}
