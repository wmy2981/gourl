package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wmy2981/gourl/internal/shortcode"
	"github.com/wmy2981/gourl/internal/store"
)

func timeNow() int64 { return time.Now().Unix() }

// isAbsoluteHTTPURL rejects anything but http/https absolute URLs.
func isAbsoluteHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// listLinks handles GET /api/v1/links.
func (s *Server) listLinks(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize == 0 {
		pageSize = 20
	}
	opts := store.ListOptions{
		Query:    r.URL.Query().Get("q"),
		Page:     page,
		PageSize: pageSize,
		Sort:     r.URL.Query().Get("sort"),
		Order:    r.URL.Query().Get("order"),
	}
	links, total, err := s.store.ListLinks(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list links")
		return
	}
	cfg := s.cfg.Get()
	out := make([]linkJSON, 0, len(links))
	for i := range links {
		out = append(out, toLinkJSON(&links[i], fullURLs(cfg, r, links[i].Code)))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"links":     out,
		"total":     total,
		"page":      opts.Page,
		"page_size": opts.PageSize,
	})
}

// createLinkRequest is the POST /api/v1/links body.
type createLinkRequest struct {
	URL       string `json:"url"`
	Code      string `json:"code"`
	ExpiresAt int64  `json:"expires_at"`
}

// createLink handles POST /api/v1/links.
func (s *Server) createLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if !isAbsoluteHTTPURL(req.URL) {
		writeError(w, http.StatusBadRequest, "invalid_request", "url must be an absolute http(s) URL")
		return
	}
	if req.ExpiresAt < 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "expires_at must be >= 0")
		return
	}

	cfg := s.cfg.Get()
	code := req.Code
	if code == "" {
		generated, err := shortcode.Random(cfg.ShortCodeLength)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to generate code")
			return
		}
		code = generated
	} else {
		if err := shortcode.Validate(code); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_code", err.Error())
			return
		}
		if shortcode.IsReserved(code, cfg.ReservedCodes) {
			writeError(w, http.StatusBadRequest, "reserved_code", "code is a reserved system path")
			return
		}
	}

	now := s.now()
	link := &store.Link{
		Code:      code,
		URL:       req.URL,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.CreateLink(r.Context(), link); err != nil {
		if errors.Is(err, store.ErrTaken) {
			writeError(w, http.StatusConflict, "code_taken", "code is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create link")
		return
	}
	// Title/description are fetched in the background so a slow target site
	// never delays the response; the meta lands on a later list refetch.
	s.meta.enqueue(code, req.URL)
	slog.Info("link created", "code", code, "url", req.URL, "actor", actorFrom(r))
	writeJSON(w, http.StatusCreated, toLinkJSON(link, fullURLs(cfg, r, code)))
}

// getLink handles GET /api/v1/links/{code}.
func (s *Server) getLink(w http.ResponseWriter, r *http.Request) {
	link, err := s.store.GetLink(r.Context(), pathCode(r.PathValue("code")))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get link")
		return
	}
	writeJSON(w, http.StatusOK, toLinkJSON(link, fullURLs(s.cfg.Get(), r, link.Code)))
}

// updateLinkRequest is the PATCH /api/v1/links/{code} body. Pointer fields
// distinguish "not provided" from explicit null/empty.
type updateLinkRequest struct {
	Code        *string `json:"code"`
	URL         *string `json:"url"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	ExpiresAt   *int64  `json:"expires_at"`
}

// updateLink handles PATCH /api/v1/links/{code}.
func (s *Server) updateLink(w http.ResponseWriter, r *http.Request) {
	var req updateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	oldCode := pathCode(r.PathValue("code"))
	link, err := s.store.GetLink(r.Context(), oldCode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get link")
		return
	}
	cfg := s.cfg.Get()

	// Changing the URL re-fetches the title/description (in the background),
	// unless the caller overrides them explicitly in the same request.
	refetchMeta := false
	if req.URL != nil {
		if !isAbsoluteHTTPURL(*req.URL) {
			writeError(w, http.StatusBadRequest, "invalid_request", "url must be an absolute http(s) URL")
			return
		}
		link.URL = *req.URL
		refetchMeta = req.Title == nil && req.Description == nil
	}
	if req.Title != nil {
		link.Title = *req.Title
	}
	if req.Description != nil {
		link.Description = *req.Description
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt < 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "expires_at must be >= 0")
			return
		}
		link.ExpiresAt = *req.ExpiresAt
	}
	link.UpdatedAt = s.now()

	if req.Code != nil && *req.Code != oldCode {
		newCode := *req.Code
		if err := shortcode.Validate(newCode); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_code", err.Error())
			return
		}
		if shortcode.IsReserved(newCode, cfg.ReservedCodes) {
			writeError(w, http.StatusBadRequest, "reserved_code", "code is a reserved system path")
			return
		}
		if err := s.store.RenameLink(r.Context(), oldCode, newCode, link.UpdatedAt); err != nil {
			if errors.Is(err, store.ErrTaken) {
				writeError(w, http.StatusConflict, "code_taken", "code is already in use")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to update link")
			return
		}
		link.Code = newCode
	}

	if err := s.store.UpdateLink(r.Context(), link); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to update link")
		return
	}
	if refetchMeta {
		s.meta.enqueue(link.Code, link.URL)
	}
	slog.Info("link updated", "code", link.Code, "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, toLinkJSON(link, fullURLs(cfg, r, link.Code)))
}

// deleteLink handles DELETE /api/v1/links/{code}.
func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) {
	code := pathCode(r.PathValue("code"))
	if err := s.store.DeleteLink(r.Context(), code); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete link")
		return
	}
	slog.Info("link deleted", "code", code, "actor", actorFrom(r))
	w.WriteHeader(http.StatusNoContent)
}

// pathCode normalizes a multi-level code captured by {code...}: the wildcard
// preserves leading slashes, strip them for DB lookups.
func pathCode(code string) string { return strings.TrimPrefix(code, "/") }
