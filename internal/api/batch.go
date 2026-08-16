package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/shortcode"
	"github.com/wmy2981/gourl/internal/store"
)

// batchLimit caps links per import request.
const batchLimit = 500

// batchCreate handles POST /api/v1/links/batch. Each item is validated and
// created independently; failures of one item never fail the others.
func (s *Server) batchCreate(w http.ResponseWriter, r *http.Request) {
	var items []createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "batch must not be empty")
		return
	}
	if len(items) > batchLimit {
		writeError(w, http.StatusBadRequest, "invalid_request", "batch exceeds 500 items")
		return
	}

	cfg := s.cfg.Get()
	now := s.now()
	results := make([]map[string]any, 0, len(items))
	created, failed := 0, 0

	for i, item := range items {
		res := map[string]any{"index": i, "url": item.URL}
		code, err := s.resolveCode(item, cfg)
		if err != nil {
			failed++
			res["status"] = "error"
			res["error_code"] = err.code
			res["error_message"] = err.message
			results = append(results, res)
			continue
		}
		link := &store.Link{
			Code:      code,
			URL:       item.URL,
			ExpiresAt: item.ExpiresAt,
			CreatedAt: now,
			UpdatedAt: now,
		}
		s.attachMeta(r, link)
		if err := s.store.CreateLink(r.Context(), link); err != nil {
			if errors.Is(err, store.ErrTaken) {
				failed++
				res["status"] = "error"
				res["error_code"] = "code_taken"
				res["error_message"] = "code is already in use"
				results = append(results, res)
				continue
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create links")
			return
		}
		created++
		res["status"] = "created"
		res["code"] = code
		results = append(results, res)
	}

	slog.Info("links batch created", "created", created, "failed", failed, "actor", actorFrom(r))
	writeJSON(w, http.StatusCreated, map[string]any{
		"results": results,
		"created": created,
		"failed":  failed,
	})
}

// codeError carries a per-item validation error.
type codeError struct {
	code    string
	message string
}

// resolveCode validates the item and returns the code to use (custom or
// generated). It does not check DB uniqueness, which is handled per insert.
func (s *Server) resolveCode(item createLinkRequest, cfg *config.Config) (string, *codeError) {
	if !isAbsoluteHTTPURL(item.URL) {
		return "", &codeError{"invalid_request", "url must be an absolute http(s) URL"}
	}
	if item.ExpiresAt < 0 {
		return "", &codeError{"invalid_request", "expires_at must be >= 0"}
	}
	if item.Code == "" {
		generated, err := shortcode.Random(cfg.ShortCodeLength)
		if err != nil {
			return "", &codeError{"internal_error", "failed to generate code"}
		}
		return generated, nil
	}
	if err := shortcode.Validate(item.Code); err != nil {
		return "", &codeError{"invalid_code", err.Error()}
	}
	if shortcode.IsReserved(item.Code, cfg.ReservedCodes) {
		return "", &codeError{"reserved_code", "code is a reserved system path"}
	}
	return item.Code, nil
}
