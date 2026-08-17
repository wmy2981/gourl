package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/wmy2981/gourl/internal/store"
)

// tokenPreview masks a token except for its first 8 characters.
func tokenPreview(t string) string {
	if len(t) <= 8 {
		return t
	}
	return t[:8] + "…"
}

// listTokens handles GET /api/v1/tokens. Full tokens are never returned;
// only a preview plus metadata.
func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.store.ListTokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list tokens")
		return
	}
	out := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, map[string]any{
			"id":          t.ID,
			"token":       tokenPreview(t.Token),
			"note":        t.Note,
			"created_at":  t.CreatedAt,
		})
	}
	slog.Debug("tokens listed", "count", len(tokens), "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, map[string]any{"tokens": out})
}

// createToken handles POST /api/v1/tokens. The full token is returned only
// in this response; afterwards only the preview is visible.
func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to generate token")
		return
	}
	token := hex.EncodeToString(raw)
	id, err := s.store.CreateToken(r.Context(), token, body.Note, s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to store token")
		return
	}
	slog.Info("api token created", "note", body.Note, "actor", actorFrom(r))
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         id,
		"token":      token, // full value, shown exactly once
		"note":       body.Note,
		"created_at": s.now(),
	})
}

// deleteToken handles DELETE /api/v1/tokens/{id}.
func (s *Server) deleteToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid token id")
		return
	}
	if err := s.store.DeleteToken(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "token not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete token")
		return
	}
	slog.Info("api token revoked", "id", id, "actor", actorFrom(r))
	w.WriteHeader(http.StatusNoContent)
}
