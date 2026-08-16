package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/shortcode"
	"github.com/wmy2981/gourl/internal/store"
)

// batchLimit caps links per import request.
const batchLimit = 500

// batchCreateRequest is the POST /api/v1/links/batch body. A legacy bare
// array is also accepted (conflict = error).
type batchCreateRequest struct {
	// Conflict policy when an item's code already exists:
	//   error (default) — the item fails with code_taken
	//   skip             — the item is skipped (status "skipped")
	//   update           — the existing link is updated with the item's fields
	Conflict string             `json:"conflict"`
	Items    []createLinkRequest `json:"items"`
}

// batchCreate handles POST /api/v1/links/batch. Each item is validated and
// created independently; failures of one item never fail the others.
func (s *Server) batchCreate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBatchRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	items := req.Items
	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "batch must not be empty")
		return
	}
	if len(items) > batchLimit {
		writeError(w, http.StatusBadRequest, "invalid_request", "batch exceeds 500 items")
		return
	}
	if req.Conflict != "" && req.Conflict != "error" && req.Conflict != "skip" && req.Conflict != "update" {
		writeError(w, http.StatusBadRequest, "invalid_request", "conflict must be error, skip or update")
		return
	}

	cfg := s.cfg.Get()
	now := s.now()
	results := make([]map[string]any, 0, len(items))
	created, failed := 0, 0

	// Validate every item up front, then insert the valid ones in a single
	// transaction (one commit instead of one per item).
	type pending struct {
		index int // original position in the request, reported to the client
		item  createLinkRequest
		code  string
	}
	var valid []pending
	for i, item := range items {
		res := map[string]any{"index": i, "url": item.URL}
		code, verr := s.resolveCode(item, cfg)
		if verr != nil {
			failed++
			res["status"] = "error"
			res["error_code"] = verr.code
			res["error_message"] = verr.message
			results = append(results, res)
			continue
		}
		valid = append(valid, pending{index: i, item: item, code: code})
	}

	links := make([]store.Link, len(valid))
	for i, p := range valid {
		links[i] = store.Link{
			Code:        p.code,
			URL:         p.item.URL,
			Title:       p.item.Title,
			Description: p.item.Description,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if p.item.ExpiresAt != nil {
			links[i].ExpiresAt = int64(*p.item.ExpiresAt)
		}
		if p.item.ClickCount != nil {
			links[i].ClickCount = *p.item.ClickCount
		}
		if p.item.CreatedAt != nil {
			links[i].CreatedAt = *p.item.CreatedAt
		}
	}
	errs, err := s.store.CreateLinks(r.Context(), links)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create links")
		return
	}
	for i, err := range errs {
		res := map[string]any{"index": valid[i].index, "url": valid[i].item.URL}
		switch {
		case errors.Is(err, store.ErrTaken) && req.Conflict == "update":
			// Refresh the existing link with the imported fields.
			if uerr := s.applyConflictUpdate(r, valid[i].item, valid[i].code); uerr != nil {
				failed++
				res["status"] = "error"
				res["error_code"] = "internal_error"
				res["error_message"] = uerr.Error()
			} else {
				res["status"] = "updated"
				res["code"] = valid[i].code
			}
		case errors.Is(err, store.ErrTaken) && req.Conflict == "skip":
			res["status"] = "skipped"
			res["code"] = valid[i].code
		case errors.Is(err, store.ErrTaken):
			failed++
			res["status"] = "error"
			res["error_code"] = "code_taken"
			res["error_message"] = "code is already in use"
		default:
			created++
			res["status"] = "created"
			res["code"] = valid[i].code
			s.meta.enqueue(valid[i].code, valid[i].item.URL)
		}
		results = append(results, res)
	}
	// The response must echo the request order so the client can map results
	// back to its input lines by index.
	sort.SliceStable(results, func(a, b int) bool {
		return results[a]["index"].(int) < results[b]["index"].(int)
	})

	slog.Info("links batch created", "created", created, "failed", failed, "actor", actorFrom(r))
	writeJSON(w, http.StatusCreated, map[string]any{
		"results": results,
		"created": created,
		"failed":  failed,
	})
}

// decodeBatchRequest accepts both the current object form
// {"conflict": "...", "items": [...]} and the legacy bare-array form.
func decodeBatchRequest(r *http.Request) (*batchCreateRequest, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON body")
	}
	// Legacy form: a bare array of items.
	if len(raw) > 0 && raw[0] == '[' {
		var items []createLinkRequest
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("invalid JSON body")
		}
		return &batchCreateRequest{Items: items}, nil
	}
	var req batchCreateRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("invalid JSON body")
	}
	return &req, nil
}

// applyConflictUpdate merges an imported item into the existing link.
func (s *Server) applyConflictUpdate(r *http.Request, item createLinkRequest, code string) error {
	link, err := s.store.GetLink(r.Context(), code)
	if err != nil {
		return err
	}
	link.URL = item.URL
	if item.Title != "" {
		link.Title = item.Title
	}
	if item.Description != "" {
		link.Description = item.Description
	}
	if item.ExpiresAt != nil {
		link.ExpiresAt = int64(*item.ExpiresAt)
	}
	if item.ClickCount != nil {
		link.ClickCount = *item.ClickCount
	}
	if item.CreatedAt != nil {
		link.CreatedAt = *item.CreatedAt
	}
	link.UpdatedAt = s.now()
	return s.store.UpdateLink(r.Context(), link)
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
	if item.ExpiresAt != nil && *item.ExpiresAt < 0 {
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
