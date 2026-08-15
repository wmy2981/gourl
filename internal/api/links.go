package api

import (
	"encoding/json"
	"errors"
	"log"
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
	out := make([]linkJSON, 0, len(links))
	for i := range links {
		out = append(out, toLinkJSON(&links[i]))
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
	// Auto-fetch the title/description; a fetch failure never blocks creation.
	s.attachMeta(r, link)
	if err := s.store.CreateLink(r.Context(), link); err != nil {
		if errors.Is(err, store.ErrTaken) {
			writeError(w, http.StatusConflict, "code_taken", "code is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create link")
		return
	}
	writeJSON(w, http.StatusCreated, toLinkJSON(link))
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
	writeJSON(w, http.StatusOK, toLinkJSON(link))
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

	if req.URL != nil {
		if !isAbsoluteHTTPURL(*req.URL) {
			writeError(w, http.StatusBadRequest, "invalid_request", "url must be an absolute http(s) URL")
			return
		}
		link.URL = *req.URL
		// Changing the URL re-fetches the title/description, unless the
		// caller overrides them explicitly in the same request.
		if req.Title == nil && req.Description == nil {
			s.attachMeta(r, link)
		}
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
	writeJSON(w, http.StatusOK, toLinkJSON(link))
}

// deleteLink handles DELETE /api/v1/links/{code}.
func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteLink(r.Context(), pathCode(r.PathValue("code"))); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete link")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pathCode normalizes a multi-level code captured by {code...}: the wildcard
// preserves leading slashes, strip them for DB lookups.
func pathCode(code string) string { return strings.TrimPrefix(code, "/") }

// attachMeta fetches title/description for the link's URL and stores them.
// Fetch failures are logged and leave the meta empty; they never fail the
// surrounding operation.
func (s *Server) attachMeta(r *http.Request, link *store.Link) {
	if s.fetcher == nil {
		return
	}
	title, desc, err := s.fetcher.Fetch(r.Context(), link.URL)
	if err != nil {
		log.Printf("fetch meta for %s: %v", link.URL, err)
		return
	}
	link.Title = title
	link.Description = desc
}
